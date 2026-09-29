// Package app is PickRole's backend: the methods the UI calls. It does not
// import Wails, so it can be built and tested anywhere; main.go adapts it
// to the desktop runtime through the Platform interface.
package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/pickrole/pickrole/internal/awsenv"
	"github.com/pickrole/pickrole/internal/awsfiles"
	"github.com/pickrole/pickrole/internal/codeartifact"
	"github.com/pickrole/pickrole/internal/config"
	"github.com/pickrole/pickrole/internal/fsutil"
	"github.com/pickrole/pickrole/internal/i18n"
	"github.com/pickrole/pickrole/internal/maven"
	"github.com/pickrole/pickrole/internal/sso"
	"github.com/pickrole/pickrole/internal/store"
	"github.com/pickrole/pickrole/internal/update"
)

// LoginRequired prefixes errors that mean "open the login screen again".
// The UI checks for it, since errors reach JavaScript as plain strings.
const LoginRequired = "LOGIN_REQUIRED"

// Platform is what the backend needs from the desktop shell.
type Platform interface {
	OpenURL(url string)
	SetClipboard(text string) error
	// SetSecretClipboard copies secrets, kept out of clipboard history and
	// cloud sync where the OS allows it.
	SetSecretClipboard(text string) error
	OpenFile(title string) (string, error)
	SaveFile(title, defaultName string) (string, error)
	// Emit tells the UI that something changed in the background.
	Emit(event string, data ...any)
	// Quit closes the app, after an update started the new version.
	Quit()
}

// Session describes the SSO session.
type Session struct {
	LoggedIn  bool      `json:"loggedIn"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// Account is an account as shown in the list.
type Account struct {
	store.Account
	Production bool `json:"production"`
	Favorite   bool `json:"favorite"`
	// ReadOnlyRoles lists the roles whose names say they only read; in
	// production, the others ask for confirmation before loading.
	ReadOnlyRoles []string `json:"readOnlyRoles"`
}

// Recent is a recently used profile as shown on the start screen.
type Recent struct {
	store.Recent
	Production bool `json:"production"`
}

// Overview is everything the UI needs to render.
type Overview struct {
	Version        string        `json:"version"`
	Configured     bool          `json:"configured"`
	Config         config.Config `json:"config"`
	Session        Session       `json:"session"`
	Accounts       []Account     `json:"accounts"`
	CacheUpdatedAt time.Time     `json:"cacheUpdatedAt"`
	Active         *store.Active `json:"active"`
	Recents        []Recent      `json:"recents"`
	// CodeArtifactAccess maps "accountID/role" to whether that role had
	// CodeArtifact access the last time it was loaded. Unknown roles are
	// absent.
	CodeArtifactAccess map[string]bool `json:"codeArtifactAccess"`
}

// LoadResult reports what loading a profile changed.
type LoadResult struct {
	Active   store.Active `json:"active"`
	Written  []string     `json:"written"`
	Warnings []string     `json:"warnings"`
}

// Service holds the backend state. All exported methods are called from
// the UI.
type Service struct {
	build Build

	mu         sync.Mutex
	ctx        context.Context
	platform   Platform
	cfg        config.Config
	configured bool
	client     *sso.Client
	state      store.State
	snapshot   store.Snapshot
	lastCreds  *awsfiles.Credentials
	pending    *sso.DeviceAuth

	// renewing keeps two renewals from running at once; renewErr is the
	// last background error shown, so it isn't repeated every minute.
	renewing sync.Mutex
	renewErr string

	// newer is the newer release found by CheckUpdate.
	newer *update.Release
	// restartWhenInstalled is set when ApplyUpdate opened a terminal to
	// install: PickRole restarts as soon as the new version is in place.
	restartWhenInstalled bool
}

// New returns the service and the function that starts it once the
// desktop runtime is ready. Start is returned separately, not as a method,
// so it is not exposed to the UI.
func New(build Build) (*Service, func(ctx context.Context, p Platform)) {
	s := &Service{build: build.withVCS()}
	return s, s.start
}

func (s *Service) start(ctx context.Context, p Platform) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ctx, s.platform = ctx, p

	cfg, found, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "pickrole:", err)
	}
	s.cfg, s.configured = cfg, found
	awsenv.SetProxy(cfg.Proxy.Settings())
	s.state, _ = store.LoadState()
	s.snapshot, _, _ = store.LoadSnapshot()
	s.resetClient()
	go s.renewLoop(ctx)
	if w := s.installWatch(); w != nil {
		go s.watchInstall(ctx, w, installCheckEvery)
	}
}

func (s *Service) resetClient() {
	s.client = nil
	if s.configured {
		s.client = sso.New(s.cfg.SSO.StartURL, s.cfg.SSO.Region, s.cfg.SSO.SessionName)
	}
	if s.snapshot.StartURL != s.cfg.SSO.StartURL {
		s.snapshot = store.Snapshot{}
	}
}

func wrap(err error) error {
	if errors.Is(err, sso.ErrLoginRequired) {
		return fmt.Errorf("%s: %w", LoginRequired, err)
	}
	return err
}

func (s *Service) requireClient() (*sso.Client, error) {
	if s.client == nil {
		return nil, i18n.New("app.configure_sso_first")
	}
	return s.client, nil
}

func readOnlyRoles(roles []string) []string {
	out := []string{}
	for _, r := range roles {
		if config.IsReadOnlyRole(r) {
			out = append(out, r)
		}
	}
	return out
}

// Overview returns the current state for the UI.
func (s *Service) Overview() Overview {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.overviewLocked()
}

func (s *Service) overviewLocked() Overview {
	o := Overview{
		Version:            s.build.Version,
		Configured:         s.configured,
		Config:             s.cfg,
		CacheUpdatedAt:     s.snapshot.UpdatedAt,
		Active:             s.state.Active,
		Accounts:           []Account{},
		Recents:            []Recent{},
		CodeArtifactAccess: map[string]bool{},
	}
	for k, v := range s.state.CodeArtifactAccess {
		o.CodeArtifactAccess[k] = v
	}
	if s.client != nil {
		tok, ok := s.client.Session()
		o.Session = Session{LoggedIn: ok, ExpiresAt: tok.Expiry()}
	}
	fav := map[string]bool{}
	for _, id := range s.state.Favorites {
		fav[id] = true
	}
	for _, a := range s.snapshot.Accounts {
		o.Accounts = append(o.Accounts, Account{
			Account:       a,
			Production:    s.cfg.IsProduction(a.Name),
			Favorite:      fav[a.ID],
			ReadOnlyRoles: readOnlyRoles(a.Roles),
		})
	}
	for _, r := range s.state.Recents {
		o.Recents = append(o.Recents, Recent{Recent: r, Production: s.cfg.IsProduction(r.AccountName)})
	}
	return o
}

// DetectSSO returns the SSO found in ~/.aws/config, or nil.
func (s *Service) DetectSSO() *config.SSO {
	found, ok := config.DetectSSO(config.AWSConfigPath())
	if !ok {
		return nil
	}
	return &found
}

// DefaultConfig returns a blank config for the first-run form.
func (s *Service) DefaultConfig() config.Config {
	cfg := config.Default()
	if found := s.DetectSSO(); found != nil {
		cfg.SSO = *found
	}
	return cfg
}

// SaveConfig validates and stores the config.
func (s *Service) SaveConfig(cfg config.Config) (Overview, error) {
	if err := config.Save(cfg); err != nil {
		return Overview{}, err
	}
	saved, _, err := config.Load()
	if err != nil {
		return Overview{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ssoChanged := !s.configured || saved.SSO != s.cfg.SSO
	s.cfg, s.configured = saved, true
	awsenv.SetProxy(saved.Proxy.Settings())
	if ssoChanged {
		s.resetClient()
	}
	return s.overviewLocked(), nil
}

// ImportConfig asks for a file shared by the team and returns its config
// without saving it, so the UI can show it before the user confirms.
// It returns nil when the dialog is cancelled.
func (s *Service) ImportConfig() (*config.Config, error) {
	path, err := s.platform.OpenFile(i18n.T("app.import_title"))
	if err != nil || path == "" {
		return nil, err
	}
	f, err := os.Open(path) // #nosec G304 -- a file the user picked in the open dialog
	if err != nil {
		return nil, err
	}
	defer f.Close() //nolint:errcheck // read only
	// A real config is a few hundred bytes; do not load a huge file into memory.
	const maxConfig = 1 << 20
	data, err := io.ReadAll(io.LimitReader(f, maxConfig+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxConfig {
		return nil, i18n.New("app.config_too_large")
	}
	cfg, err := config.Decode(data)
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

// ExportConfig writes the current config to a file the team can import.
// It holds no secrets: only URLs, regions and names.
func (s *Service) ExportConfig() (string, error) {
	s.mu.Lock()
	cfg := s.cfg
	s.mu.Unlock()
	path, err := s.platform.SaveFile(i18n.T("app.export_title"), "pickrole.json")
	if err != nil || path == "" {
		return "", err
	}
	data, err := config.Encode(cfg)
	if err != nil {
		return "", err
	}
	// #nosec G306 -- meant to be shared with the team; it holds no secrets
	return path, os.WriteFile(path, append(data, '\n'), 0o644)
}

// StartLogin opens the browser on the AWS device authorisation page and
// returns the code the user must confirm there.
func (s *Service) StartLogin() (*sso.DeviceAuth, error) {
	s.mu.Lock()
	client, err := s.requireClient()
	ctx := s.ctx
	s.mu.Unlock()
	if err != nil {
		return nil, err
	}
	auth, err := client.StartLogin(ctx)
	if err != nil {
		return nil, err
	}
	if !browserURL(auth.VerificationURL) {
		return nil, i18n.New("app.unexpected_verification", auth.VerificationURL)
	}
	s.mu.Lock()
	s.pending = auth
	s.mu.Unlock()
	s.platform.OpenURL(auth.VerificationURL)
	return auth, nil
}

// browserURL reports whether url is safe to hand to the browser: https, or
// http on loopback for the local fake AWS used in tests.
func browserURL(raw string) bool {
	u, err := url.Parse(raw)
	// user:password@ in a URL is a classic way to disguise the real host.
	if err != nil || u.Host == "" || u.User != nil {
		return false
	}
	switch u.Scheme {
	case "https":
		return true
	case "http":
		ip := net.ParseIP(u.Hostname())
		return u.Hostname() == "localhost" || (ip != nil && ip.IsLoopback())
	}
	return false
}

// DetectMaven reads the settings.xml of cfg, which may not be saved yet, and
// returns the <server> ids that need the CodeArtifact token and the
// CodeArtifact domains its repositories point to. Only ids of cfg's domain
// are returned when one is set.
func (s *Service) DetectMaven(cfg config.Config) (maven.Detection, error) {
	m := cfg.CodeArtifact.Tools.Maven
	if err := config.ValidateSettingsPath(m.SettingsPath); err != nil {
		return maven.Detection{}, err
	}
	data, err := os.ReadFile(fsutil.ExpandHome(m.SettingsPath)) // #nosec G304 -- validated above: inside the home directory
	if errors.Is(err, fs.ErrNotExist) {
		return maven.Detection{}, i18n.New("maven.settings_not_found", m.SettingsPath)
	}
	if err != nil {
		return maven.Detection{}, err
	}
	ca := cfg.CodeArtifact
	return maven.Detect(string(data), maven.Domain{Domain: ca.Domain, Owner: ca.DomainOwner, Region: ca.Region}), nil
}

// ConnectionTest is the result of TestConnection, ready to show.
type ConnectionTest struct {
	// Target is the host that was tried: the SSO sign-in endpoint.
	Target string `json:"target"`
	// Proxy is the proxy used ("host:port"), or empty for a direct
	// connection.
	Proxy string `json:"proxy"`
	// Source is where the proxy setting came from: manual, environment,
	// windows, gnome or none.
	Source string `json:"source"`
	// Note is a system proxy setting PickRole couldn't use.
	Note string `json:"note"`
	// Error is why the target couldn't be reached; empty when it answered.
	Error string `json:"error"`
}

// TestConnection tries the SSO sign-in endpoint through the proxy settings
// of cfg, which may not be saved yet, so the settings screen can check them
// before saving.
func (s *Service) TestConnection(cfg config.Config) (ConnectionTest, error) {
	if err := cfg.Proxy.Validate(); err != nil {
		return ConnectionTest{}, err
	}
	region := cfg.SSO.Region
	if !config.ValidRegion(region) {
		region = config.Default().SSO.Region
	}
	target := "https://oidc." + region + ".amazonaws.com/"
	if e := awsenv.Endpoint(awsenv.SSOOIDC); e != nil {
		target = *e
	}

	s.mu.Lock()
	ctx := s.ctx
	s.mu.Unlock()
	if ctx == nil {
		ctx = context.Background()
	}
	res := awsenv.TestConnection(ctx, cfg.Proxy.Settings(), target)
	out := ConnectionTest{Target: target, Proxy: res.Proxy, Source: res.Source}
	if u, err := url.Parse(target); err == nil {
		out.Target = u.Host
	}
	if res.Note != "" {
		out.Note = i18n.T(res.Note)
	}
	if res.Err != nil {
		out.Error = i18n.T("proxy.unreachable", out.Target) + ": " + res.Err.Error()
	}
	return out, nil
}

// SetLanguage sets the language of the messages the backend returns. The UI
// calls it with the resolved language (never "system"); unknown values mean
// English.
func (s *Service) SetLanguage(lang string) {
	i18n.SetLanguage(i18n.Lang(lang))
}

// OpenURL opens url in the browser if it is safe (see browserURL).
func (s *Service) OpenURL(url string) {
	if browserURL(url) {
		s.platform.OpenURL(url)
	}
}

// WaitLogin blocks until the user approves the code shown by StartLogin,
// then loads the account list if there is no cache for this SSO yet.
func (s *Service) WaitLogin() (Overview, error) {
	s.mu.Lock()
	client, err := s.requireClient()
	auth, ctx := s.pending, s.ctx
	s.mu.Unlock()
	if err != nil {
		return Overview{}, err
	}
	if auth == nil {
		return Overview{}, i18n.New("app.no_login_in_progress")
	}
	if err := client.WaitLogin(ctx, auth); err != nil {
		return Overview{}, err
	}
	s.mu.Lock()
	s.pending = nil
	needAccounts := len(s.snapshot.Accounts) == 0
	s.mu.Unlock()
	if needAccounts {
		return s.RefreshAccounts()
	}
	return s.Overview(), nil
}

// RenewSession renews the SSO session without the browser when possible.
// On failure the error starts with LoginRequired.
func (s *Service) RenewSession() (Overview, error) {
	s.mu.Lock()
	client, err := s.requireClient()
	ctx := s.ctx
	s.mu.Unlock()
	if err != nil {
		return Overview{}, err
	}
	if err := client.Refresh(ctx); err != nil {
		return Overview{}, wrap(err)
	}
	return s.Overview(), nil
}

// RefreshAccounts reloads accounts and roles from AWS and updates the cache.
func (s *Service) RefreshAccounts() (Overview, error) {
	s.mu.Lock()
	client, err := s.requireClient()
	ctx, startURL := s.ctx, s.cfg.SSO.StartURL
	s.mu.Unlock()
	if err != nil {
		return Overview{}, err
	}
	accounts, err := client.ListAccounts(ctx)
	if err != nil {
		return Overview{}, wrap(err)
	}
	snap := store.Snapshot{StartURL: startURL, UpdatedAt: time.Now(), Accounts: accounts}
	if err := store.SaveSnapshot(snap); err != nil {
		return Overview{}, i18n.Wrap("app.saving_cache", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshot = snap
	return s.overviewLocked(), nil
}

func (s *Service) findAccount(id string) (store.Account, bool) {
	for _, a := range s.snapshot.Accounts {
		if a.ID == id {
			return a, true
		}
	}
	return store.Account{}, false
}

func homeRelative(path string) string {
	if home, err := os.UserHomeDir(); err == nil {
		if rel, err := filepath.Rel(home, path); err == nil && !strings.HasPrefix(rel, "..") {
			return "~/" + filepath.ToSlash(rel)
		}
	}
	return path
}

// LoadProfile writes the credentials of accountID/role to the shared AWS
// credentials file and, when configured, refreshes the CodeArtifact token
// in the build tools. Every terminal and IDE picks the change up on its
// next command.
func (s *Service) LoadProfile(accountID, role string) (LoadResult, error) {
	return s.loadProfile(accountID, role, true)
}

// loadProfile loads a profile; used says whether the user picked it, which
// makes it the most recent one. Background renewals don't.
func (s *Service) loadProfile(accountID, role string, used bool) (LoadResult, error) {
	s.mu.Lock()
	client, err := s.requireClient()
	ctx, cfg := s.ctx, s.cfg
	account, found := s.findAccount(accountID)
	s.mu.Unlock()
	if err != nil {
		return LoadResult{}, err
	}
	if !found {
		return LoadResult{}, i18n.New("app.account_not_found")
	}
	// The role comes from the UI: only the ones AWS listed for this account.
	if !slices.Contains(account.Roles, role) {
		return LoadResult{}, i18n.New("app.role_not_found")
	}

	creds, err := client.RoleCredentials(ctx, accountID, role)
	if err != nil {
		return LoadResult{}, wrap(err)
	}

	// The first profile is the one reported as active.
	profiles := []string{"default"}
	if p := cfg.Preferences; p.ProfileMode == config.ProfileNamed {
		profiles = []string{awsfiles.ProfileName(p.ProfileFormat, account.Name, account.ID, role)}
		if p.AlsoDefault {
			profiles = append(profiles, "default")
		}
	}
	profile := profiles[0]
	credPath := awsfiles.CredentialsPath()
	for _, pr := range profiles {
		if err := awsfiles.UpsertProfile(credPath, pr, creds); err != nil {
			return LoadResult{}, i18n.Wrap("app.writing_credentials", err)
		}
	}

	res := LoadResult{
		Active: store.Active{
			AccountID:   accountID,
			AccountName: account.Name,
			Role:        role,
			Profile:     profile,
			ExpiresAt:   creds.Expiration,
		},
		Written:  []string{homeRelative(credPath) + " [" + strings.Join(profiles, "], [") + "]"},
		Warnings: []string{},
	}

	hasAccess, noAccess := true, false
	var access *bool // nil when CodeArtifact is off or the check failed

	if ca := cfg.CodeArtifact; ca.Enabled {
		tok, err := codeartifact.GetToken(ctx, creds, ca.Region, ca.Domain, ca.DomainOwner)
		switch {
		case errors.Is(err, codeartifact.ErrNoAccess):
			// Expected for roles without CodeArtifact; nothing to warn about.
			access = &noAccess
		case err != nil:
			res.Warnings = append(res.Warnings, err.Error())
		default:
			access = &hasAccess
			res.Active.CodeArtifact = true
			res.Active.CodeArtifactExpiresAt = tok.ExpiresAt
			if m := ca.Tools.Maven; m.Enabled {
				// UpsertServers validates the path again: a config written by an
				// older version, or edited by hand, was never checked on save.
				if err := maven.UpsertServers(m.SettingsPath, m.ServerIDs, tok.Value); err != nil {
					res.Warnings = append(res.Warnings, "Maven: "+err.Error())
				} else {
					res.Written = append(res.Written, m.SettingsPath)
				}
			}
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastCreds = &creds
	active := res.Active
	s.state.Active = &active
	if access != nil {
		s.state.SetCodeArtifactAccess(accountID, role, *access)
	}
	if used {
		s.state.AddRecent(store.Recent{
			AccountID:   accountID,
			AccountName: account.Name,
			Role:        role,
			UsedAt:      time.Now(),
		})
	}
	if err := store.SaveState(s.state); err != nil {
		res.Warnings = append(res.Warnings, i18n.T("app.history_not_saved")+": "+err.Error())
	}
	return res, nil
}

// ToggleFavorite adds or removes an account from the favourites.
func (s *Service) ToggleFavorite(accountID string) (Overview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.ToggleFavorite(accountID)
	if err := store.SaveState(s.state); err != nil {
		return Overview{}, err
	}
	return s.overviewLocked(), nil
}

// CopyText puts text on the clipboard (account IDs, device codes).
func (s *Service) CopyText(text string) error {
	return s.platform.SetClipboard(text)
}

// CopyExport puts `export AWS_…` lines for the loaded profile on the
// clipboard, for the rare terminal that needs environment variables.
func (s *Service) CopyExport() error {
	s.mu.Lock()
	creds := s.lastCreds
	s.mu.Unlock()
	if creds == nil {
		return i18n.New("app.load_profile_first")
	}
	lines, err := awsfiles.ExportLines(*creds, "")
	if err != nil {
		return err
	}
	return s.platform.SetSecretClipboard(lines)
}
