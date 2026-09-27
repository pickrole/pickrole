// Package fakeaws is a local stand-in for the AWS APIs PickRole calls: SSO
// OIDC (device-code login), the SSO portal (accounts, roles and role
// credentials) and CodeArtifact (authorization token).
//
// It speaks the same REST-JSON wire format as AWS, so the real SDK clients
// talk to it unchanged once AWS_ENDPOINT_URL points at it (see
// internal/awsenv). The tests use it through httptest, and cmd/fakeaws runs
// it for manual testing of the desktop app without an AWS account.
package fakeaws

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Account is an AWS account the fake user can access, with its roles.
type Account struct {
	ID    string
	Name  string
	Email string
	Roles []string
}

// Options configures the fake. Zero values get sensible defaults.
type Options struct {
	Accounts []Account
	// CodeArtifactDenied lists roles that get AccessDeniedException from
	// CodeArtifact, like a ReadOnly role would.
	CodeArtifactDenied []string
	// AutoApprove approves device codes without the browser page.
	AutoApprove bool
	// AccessTokenTTL is how long SSO access tokens last.
	AccessTokenTTL time.Duration
	// PollInterval is the device-code polling interval sent to the client.
	PollInterval time.Duration
	// PageSize is the default page size for accounts and roles, so the SDK
	// paginators are exercised.
	PageSize int
	// Log, when set, receives one line per request.
	Log io.Writer
}

// DefaultAccounts is a small organisation with dev, staging and production
// accounts. "products-analytics" checks that the production pattern does
// not match "products".
func DefaultAccounts() []Account {
	acc := func(id, name string, roles ...string) Account {
		return Account{ID: id, Name: name, Email: "aws+" + name + "@example.com", Roles: roles}
	}
	return []Account{
		acc("111111111111", "platform-dev", "Developer", "ReadOnly"),
		acc("111111111112", "platform-staging", "Developer", "ReadOnly"),
		acc("111111111113", "platform-prod", "Admin", "Developer", "ReadOnly"),
		acc("222222222221", "payments-dev", "Developer", "ReadOnly"),
		acc("222222222223", "payments-prod", "Admin", "ReadOnly"),
		acc("333333333331", "logistics-dev", "Developer"),
		acc("333333333333", "logistics_production", "ReadOnly"),
		acc("444444444444", "products-analytics", "Developer", "ReadOnly"),
		acc("555555555555", "security-audit", "SecurityAudit", "ReadOnly"),
		acc("666666666666", "sandbox", "Admin"),
		acc("999999999999", "shared-tools", "Developer", "ReadOnly"),
	}
}

const (
	bearerHeader = "X-Amz-Sso_bearer_token" // #nosec G101 -- a header name, not a credential
	credsTTL     = time.Hour
	caTokenTTL   = 12 * time.Hour
	deviceTTL    = 10 * time.Minute
)

type device struct {
	code     string
	userCode string
	clientID string
	expires  time.Time
	approved bool
	denied   bool
	used     bool
}

type roleRef struct{ account, role string }

// Server is the fake. It implements http.Handler.
type Server struct {
	opts Options
	mux  *http.ServeMux

	mu         sync.Mutex
	clients    map[string]string // client ID -> secret
	devices    map[string]*device
	byUserCode map[string]*device
	access     map[string]time.Time // access token -> expiry
	refresh    map[string]bool
	keys       map[string]roleRef // access key ID -> account/role
	logins     int
}

// New creates a fake with opts.
func New(opts Options) *Server {
	if opts.Accounts == nil {
		opts.Accounts = DefaultAccounts()
	}
	if opts.CodeArtifactDenied == nil {
		opts.CodeArtifactDenied = []string{"ReadOnly"}
	}
	if opts.AccessTokenTTL <= 0 {
		opts.AccessTokenTTL = 8 * time.Hour
	}
	if opts.PollInterval <= 0 {
		opts.PollInterval = time.Second
	}
	if opts.PageSize <= 0 {
		opts.PageSize = 4
	}
	s := &Server{
		opts:       opts,
		mux:        http.NewServeMux(),
		clients:    map[string]string{},
		devices:    map[string]*device{},
		byUserCode: map[string]*device{},
		access:     map[string]time.Time{},
		refresh:    map[string]bool{},
		keys:       map[string]roleRef{},
	}
	// SSO OIDC
	s.mux.HandleFunc("POST /client/register", s.registerClient)
	s.mux.HandleFunc("POST /device_authorization", s.startDeviceAuthorization)
	s.mux.HandleFunc("POST /token", s.createToken)
	// SSO portal
	s.mux.HandleFunc("GET /assignment/accounts", s.listAccounts)
	s.mux.HandleFunc("GET /assignment/roles", s.listAccountRoles)
	s.mux.HandleFunc("GET /federation/credentials", s.getRoleCredentials)
	// CodeArtifact
	s.mux.HandleFunc("POST /v1/authorization-token", s.getAuthorizationToken)
	// Browser pages
	s.mux.HandleFunc("GET /device", s.devicePage)
	s.mux.HandleFunc("POST /device", s.deviceDecision)
	s.mux.HandleFunc("GET /{$}", s.indexPage)
	s.mux.HandleFunc("POST /admin/expire", s.adminExpire)
	s.mux.HandleFunc("POST /admin/revoke", s.adminRevoke)
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.opts.Log != nil {
		fmt.Fprintf(s.opts.Log, "%s %s %s\n", time.Now().Format("15:04:05"), r.Method, r.URL.RequestURI())
	}
	s.mux.ServeHTTP(w, r)
}

// ExpireAccessTokens invalidates every SSO access token but keeps the
// refresh tokens, so the client can renew the session silently.
func (s *Server) ExpireAccessTokens() {
	s.mu.Lock()
	defer s.mu.Unlock()
	clear(s.access)
}

// RevokeAll invalidates access and refresh tokens, forcing a new login.
func (s *Server) RevokeAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	clear(s.access)
	clear(s.refresh)
}

// Approve approves a pending device code, as the user would in the browser.
func (s *Server) Approve(userCode string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.byUserCode[userCode]
	if ok {
		d.approved = true
	}
	return ok
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// awsError writes an error the way REST-JSON services do: the SDK reads the
// exception type from X-Amzn-ErrorType.
func awsError(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("X-Amzn-ErrorType", code)
	writeJSON(w, status, map[string]string{"message": msg, "error": code, "error_description": msg})
}

func readBody(r *http.Request, v any) error {
	return json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(v)
}

// --- SSO OIDC ---------------------------------------------------------------

func (s *Server) registerClient(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ClientName string `json:"clientName"`
		ClientType string `json:"clientType"`
	}
	if err := readBody(r, &in); err != nil || in.ClientName == "" || in.ClientType != "public" {
		awsError(w, 400, "InvalidRequestException", "clientName and clientType=public are required")
		return
	}
	id, secret := "fakeclient-"+randomHex(8), randomHex(24)
	s.mu.Lock()
	s.clients[id] = secret
	s.mu.Unlock()
	now := time.Now()
	writeJSON(w, 200, map[string]any{
		"clientId":              id,
		"clientSecret":          secret,
		"clientIdIssuedAt":      now.Unix(),
		"clientSecretExpiresAt": now.Add(90 * 24 * time.Hour).Unix(),
	})
}

func (s *Server) validClient(id, secret string) bool {
	want, ok := s.clients[id]
	return ok && want == secret
}

func (s *Server) startDeviceAuthorization(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ClientID     string `json:"clientId"`
		ClientSecret string `json:"clientSecret"`
		StartURL     string `json:"startUrl"`
	}
	if err := readBody(r, &in); err != nil || in.StartURL == "" {
		awsError(w, 400, "InvalidRequestException", "clientId, clientSecret and startUrl are required")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.validClient(in.ClientID, in.ClientSecret) {
		awsError(w, 400, "InvalidClientException", "unknown client")
		return
	}
	u := strings.ToUpper(randomHex(4))
	d := &device{
		code:     randomHex(32),
		userCode: u[:4] + "-" + u[4:],
		clientID: in.ClientID,
		expires:  time.Now().Add(deviceTTL),
		approved: s.opts.AutoApprove,
	}
	s.devices[d.code] = d
	s.byUserCode[d.userCode] = d
	base := "http://" + r.Host + "/device"
	writeJSON(w, 200, map[string]any{
		"deviceCode":              d.code,
		"userCode":                d.userCode,
		"verificationUri":         base,
		"verificationUriComplete": base + "?user_code=" + d.userCode,
		"expiresIn":               int(deviceTTL.Seconds()),
		"interval":                int(s.opts.PollInterval.Seconds()),
	})
}

func (s *Server) createToken(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ClientID     string `json:"clientId"`
		ClientSecret string `json:"clientSecret"`
		GrantType    string `json:"grantType"`
		DeviceCode   string `json:"deviceCode"`
		RefreshToken string `json:"refreshToken"`
	}
	if err := readBody(r, &in); err != nil {
		awsError(w, 400, "InvalidRequestException", "invalid body")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.validClient(in.ClientID, in.ClientSecret) {
		awsError(w, 401, "InvalidClientException", "unknown client")
		return
	}
	switch in.GrantType {
	case "urn:ietf:params:oauth:grant-type:device_code":
		d, ok := s.devices[in.DeviceCode]
		switch {
		case !ok || d.clientID != in.ClientID || d.used:
			awsError(w, 400, "InvalidGrantException", "invalid device code")
		case time.Now().After(d.expires):
			awsError(w, 400, "ExpiredTokenException", "device code expired")
		case d.denied:
			awsError(w, 400, "AccessDeniedException", "the user denied the request")
		case !d.approved:
			awsError(w, 400, "AuthorizationPendingException", "authorization pending")
		default:
			d.used = true
			s.logins++
			s.issueTokens(w, randomHex(32))
		}
	case "refresh_token":
		if !s.refresh[in.RefreshToken] {
			awsError(w, 400, "InvalidGrantException", "invalid refresh token")
			return
		}
		s.issueTokens(w, in.RefreshToken)
	default:
		awsError(w, 400, "UnsupportedGrantTypeException", "unsupported grant type")
	}
}

// issueTokens must be called with s.mu held.
func (s *Server) issueTokens(w http.ResponseWriter, refresh string) {
	access := "fakeaccess-" + randomHex(32)
	s.access[access] = time.Now().Add(s.opts.AccessTokenTTL)
	s.refresh[refresh] = true
	writeJSON(w, 200, map[string]any{
		"accessToken":  access,
		"tokenType":    "Bearer",
		"expiresIn":    int(s.opts.AccessTokenTTL.Seconds()),
		"refreshToken": refresh,
	})
}

// --- SSO portal -------------------------------------------------------------

// authorized checks the bearer token and writes UnauthorizedException when
// it is missing or expired.
func (s *Server) authorized(w http.ResponseWriter, r *http.Request) bool {
	s.mu.Lock()
	exp, ok := s.access[r.Header.Get(bearerHeader)]
	s.mu.Unlock()
	if !ok || time.Now().After(exp) {
		awsError(w, 401, "UnauthorizedException", "Session token not found or invalid")
		return false
	}
	return true
}

func (s *Server) account(id string) (Account, bool) {
	for _, a := range s.opts.Accounts {
		if a.ID == id {
			return a, true
		}
	}
	return Account{}, false
}

// page returns the slice of n items selected by next_token and max_result,
// and the token of the following page ("" when it is the last).
func (s *Server) page(r *http.Request, n int) (start, end int, next string) {
	start, _ = strconv.Atoi(r.URL.Query().Get("next_token"))
	size, err := strconv.Atoi(r.URL.Query().Get("max_result"))
	if err != nil || size <= 0 {
		size = s.opts.PageSize
	}
	start = min(max(start, 0), n)
	end = min(start+size, n)
	if end < n {
		next = strconv.Itoa(end)
	}
	return start, end, next
}

func withNext(m map[string]any, next string) map[string]any {
	if next != "" {
		m["nextToken"] = next
	}
	return m
}

func (s *Server) listAccounts(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(w, r) {
		return
	}
	start, end, next := s.page(r, len(s.opts.Accounts))
	list := []map[string]string{}
	for _, a := range s.opts.Accounts[start:end] {
		list = append(list, map[string]string{"accountId": a.ID, "accountName": a.Name, "emailAddress": a.Email})
	}
	writeJSON(w, 200, withNext(map[string]any{"accountList": list}, next))
}

func (s *Server) listAccountRoles(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(w, r) {
		return
	}
	a, ok := s.account(r.URL.Query().Get("account_id"))
	if !ok {
		awsError(w, 404, "ResourceNotFoundException", "account not found")
		return
	}
	start, end, next := s.page(r, len(a.Roles))
	list := []map[string]string{}
	for _, role := range a.Roles[start:end] {
		list = append(list, map[string]string{"roleName": role, "accountId": a.ID})
	}
	writeJSON(w, 200, withNext(map[string]any{"roleList": list}, next))
}

func (s *Server) getRoleCredentials(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(w, r) {
		return
	}
	q := r.URL.Query()
	a, ok := s.account(q.Get("account_id"))
	role := q.Get("role_name")
	if !ok || !slices.Contains(a.Roles, role) {
		awsError(w, 404, "ResourceNotFoundException", "no access to this account/role")
		return
	}
	key := "ASIAFAKE" + strings.ToUpper(randomHex(6))
	s.mu.Lock()
	s.keys[key] = roleRef{account: a.ID, role: role}
	s.mu.Unlock()
	writeJSON(w, 200, map[string]any{"roleCredentials": map[string]any{
		"accessKeyId":     key,
		"secretAccessKey": "fakesecret" + randomHex(15),
		"sessionToken":    "fakesession" + randomHex(40),
		"expiration":      time.Now().Add(credsTTL).UnixMilli(),
	}})
}

// --- CodeArtifact -----------------------------------------------------------

// accessKey extracts the key ID from a SigV4 Authorization header. The
// signature itself is not checked.
func accessKey(r *http.Request) string {
	_, after, ok := strings.Cut(r.Header.Get("Authorization"), "Credential=")
	if !ok {
		return ""
	}
	key, _, _ := strings.Cut(after, "/")
	return key
}

func (s *Server) getAuthorizationToken(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	ref, ok := s.keys[accessKey(r)]
	s.mu.Unlock()
	if !ok {
		awsError(w, 403, "UnrecognizedClientException", "The security token included in the request is invalid.")
		return
	}
	q := r.URL.Query()
	if q.Get("domain") == "" || q.Get("domain-owner") == "" {
		awsError(w, 400, "ValidationException", "domain and domain-owner are required")
		return
	}
	if slices.Contains(s.opts.CodeArtifactDenied, ref.role) {
		awsError(w, 403, "AccessDeniedException", fmt.Sprintf(
			"User: arn:aws:sts::%s:assumed-role/%s/fake is not authorized to perform: codeartifact:GetAuthorizationToken",
			ref.account, ref.role))
		return
	}
	writeJSON(w, 200, map[string]any{
		"authorizationToken": "fakecodeartifact" + randomHex(32),
		"expiration":         float64(time.Now().Add(caTokenTTL).Unix()),
	})
}

// --- Browser pages ----------------------------------------------------------

var pages = template.Must(template.New("").Parse(`
{{define "head"}}<!doctype html><html lang="en"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>PickRole fake AWS</title>
<style>
 :root { color-scheme: light dark; --bg:#f6f7f9; --fg:#1b1d22; --muted:#5d6470; --card:#fff; --line:#d9dde3; --accent:#1f8f94; }
 @media (prefers-color-scheme: dark) { :root { --bg:#131418; --fg:#e8eaee; --muted:#9aa1ad; --card:#1c1e24; --line:#2d3039; --accent:#4FC3C8; } }
 body { margin:0; background:var(--bg); color:var(--fg); font:15px/1.5 system-ui, sans-serif; }
 main { max-width:720px; margin:0 auto; padding:32px 16px; }
 .card { background:var(--card); border:1px solid var(--line); border-radius:10px; padding:20px; margin:16px 0; }
 .muted { color:var(--muted); }
 .code { font:600 32px/1 ui-monospace, monospace; letter-spacing:4px; margin:12px 0; }
 button { font:inherit; padding:8px 16px; border-radius:6px; border:1px solid var(--line); background:var(--card); color:var(--fg); cursor:pointer; }
 button.primary { background:var(--accent); border-color:var(--accent); color:#0b1214; font-weight:600; }
 table { border-collapse:collapse; width:100%; } td, th { text-align:left; padding:4px 8px; border-bottom:1px solid var(--line); }
 code { font-family:ui-monospace, monospace; }
 form { display:inline; }
</style></head><body><main>
<p class="muted">PickRole fake AWS · local testing only</p>{{end}}

{{define "device"}}{{template "head"}}
<div class="card">
{{if not .Found}}<h1>Code not found</h1><p>Start the sign-in again in PickRole.</p>
{{else if .Done}}<h1>{{if .Denied}}Access denied{{else}}Access authorized{{end}}</h1><p>You can go back to PickRole.</p>
{{else}}<h1>Authorize PickRole?</h1>
<p>Check that the code below matches the one PickRole shows.</p>
<div class="code">{{.UserCode}}</div>
<form method="post"><input type="hidden" name="user_code" value="{{.UserCode}}">
<button class="primary" name="decision" value="approve">Authorize</button>
<button name="decision" value="deny">Deny</button></form>
{{end}}</div></main></body></html>{{end}}

{{define "index"}}{{template "head"}}
<div class="card"><h1>PickRole fake AWS</h1>
<p>Endpoint: <code>{{.Base}}</code> · completed sign-ins: {{.Logins}} · valid access tokens: {{.Active}}</p>
<p>In PickRole: start URL <code>https://pickrole-fake.awsapps.com/start</code> (any https URL works), any region.
CodeArtifact: domain <code>pickrole</code>, owner account <code>999999999999</code>.
Roles <code>{{.Denied}}</code> get AccessDenied from CodeArtifact.</p></div>
<div class="card"><h2>Simulate the session</h2>
<form method="post" action="/admin/expire"><button>Expire access tokens</button></form>
<span class="muted">the refresh token still works: renewing the session needs no browser</span><br><br>
<form method="post" action="/admin/revoke"><button>Revoke everything</button></form>
<span class="muted">the refresh token stops working too: only a new sign-in helps</span></div>
<div class="card"><h2>Accounts</h2><table><tr><th>ID</th><th>Name</th><th>Roles</th></tr>
{{range .Accounts}}<tr><td><code>{{.ID}}</code></td><td>{{.Name}}</td><td>{{range $i, $r := .Roles}}{{if $i}}, {{end}}{{$r}}{{end}}</td></tr>{{end}}
</table></div></main></body></html>{{end}}
`))

func (s *Server) devicePage(w http.ResponseWriter, r *http.Request) {
	s.renderDevice(w, r.URL.Query().Get("user_code"))
}

func (s *Server) deviceDecision(w http.ResponseWriter, r *http.Request) {
	code := r.FormValue("user_code")
	s.mu.Lock()
	if d, ok := s.byUserCode[code]; ok && !d.used {
		d.approved = r.FormValue("decision") == "approve"
		d.denied = !d.approved
	}
	s.mu.Unlock()
	s.renderDevice(w, code)
}

func (s *Server) renderDevice(w http.ResponseWriter, code string) {
	s.mu.Lock()
	d, ok := s.byUserCode[code]
	data := map[string]any{"Found": ok, "UserCode": code}
	if ok {
		data["Done"] = d.approved || d.denied || d.used
		data["Denied"] = d.denied
	}
	s.mu.Unlock()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = pages.ExecuteTemplate(w, "device", data)
}

func (s *Server) indexPage(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	active := 0
	for _, exp := range s.access {
		if time.Now().Before(exp) {
			active++
		}
	}
	data := map[string]any{
		"Base":     "http://" + r.Host,
		"Logins":   s.logins,
		"Active":   active,
		"Accounts": s.opts.Accounts,
		"Denied":   strings.Join(s.opts.CodeArtifactDenied, ", "),
	}
	s.mu.Unlock()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = pages.ExecuteTemplate(w, "index", data)
}

func (s *Server) adminExpire(w http.ResponseWriter, r *http.Request) {
	s.ExpireAccessTokens()
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) adminRevoke(w http.ResponseWriter, r *http.Request) {
	s.RevokeAll()
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
