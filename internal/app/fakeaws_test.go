package app

import (
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pickrole/pickrole/internal/config"
	"github.com/pickrole/pickrole/internal/fakeaws"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func requireLoginRequired(t *testing.T, step string, err error) {
	t.Helper()
	if err == nil || !strings.HasPrefix(err.Error(), LoginRequired) {
		t.Errorf("%s: want a %s error, got %v", step, LoginRequired, err)
	}
}

// TestFullFlowAgainstFakeAWS drives the backend through the real AWS SDK
// clients against internal/fakeaws: login, account list, loading profiles
// with and without CodeArtifact access, and session expiry.
func TestFullFlowAgainstFakeAWS(t *testing.T) {
	dir := isolate(t)
	fake := fakeaws.New(fakeaws.Options{})
	srv := httptest.NewServer(fake)
	defer srv.Close()
	t.Setenv("AWS_ENDPOINT_URL", srv.URL)

	// Existing content that PickRole must keep.
	credPath := filepath.Join(dir, ".aws", "credentials")
	writeFile(t, credPath, "[pessoal]\naws_access_key_id = AKIAPESSOAL\n")
	settings := filepath.Join(dir, ".m2", "settings.xml")
	writeFile(t, settings, "<settings>\n  <mirrors/>\n</settings>\n")

	p := &fakePlatform{}
	svc, start := New(Build{Version: "test"})
	start(context.Background(), p)

	cfg := svc.DefaultConfig()
	cfg.SSO.StartURL = "https://pickrole-fake.awsapps.com/start"
	cfg.CodeArtifact.Enabled = true
	cfg.CodeArtifact.Domain = "pickrole"
	cfg.CodeArtifact.DomainOwner = "999999999999"
	cfg.CodeArtifact.Tools.Maven.SettingsPath = settings
	if _, err := svc.SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}

	// Login: the browser opens on the page with the code, the user approves.
	auth, err := svc.StartLogin()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(p.opened, auth.UserCode) {
		t.Errorf("opened %q, want the verification URL with code %s", p.opened, auth.UserCode)
	}
	fake.Approve(auth.UserCode)
	o, err := svc.WaitLogin()
	if err != nil {
		t.Fatal(err)
	}
	if !o.Session.LoggedIn {
		t.Error("should be logged in after approval")
	}
	if caches, _ := filepath.Glob(filepath.Join(dir, ".aws", "sso", "cache", "*.json")); len(caches) != 1 {
		t.Errorf("want one SSO cache file in AWS CLI format, got %v", caches)
	}

	// Every account and role comes back through the paginators.
	if got, want := len(o.Accounts), len(fakeaws.DefaultAccounts()); got != want {
		t.Fatalf("got %d accounts, want %d", got, want)
	}
	prod := map[string]bool{}
	for _, a := range o.Accounts {
		prod[a.Name] = a.Production
		if len(a.Roles) == 0 {
			t.Errorf("account %s has no roles", a.Name)
		}
	}
	for name, want := range map[string]bool{
		"platform-prod": true, "logistics_production": true,
		"platform-dev": false, "products-analytics": false,
	} {
		if prod[name] != want {
			t.Errorf("production(%s) = %v, want %v", name, prod[name], want)
		}
	}

	// A role with CodeArtifact: credentials plus the Maven token.
	res, err := svc.LoadProfile("111111111111", "Developer")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Warnings) != 0 || !res.Active.CodeArtifact {
		t.Errorf("Developer: warnings=%v codeArtifact=%v", res.Warnings, res.Active.CodeArtifact)
	}
	creds := readFile(t, credPath)
	for _, want := range []string{"[pessoal]\naws_access_key_id = AKIAPESSOAL", "[default]", "ASIAFAKE"} {
		if !strings.Contains(creds, want) {
			t.Errorf("credentials missing %q:\n%s", want, creds)
		}
	}
	maven := readFile(t, settings)
	for _, want := range []string{"<mirrors/>", "<id>codeartifact</id>", "fakecodeartifact"} {
		if !strings.Contains(maven, want) {
			t.Errorf("settings.xml missing %q:\n%s", want, maven)
		}
	}

	// A role without CodeArtifact: AccessDenied is expected, not a warning.
	res, err = svc.LoadProfile("111111111111", "ReadOnly")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Warnings) != 0 || res.Active.CodeArtifact {
		t.Errorf("ReadOnly: warnings=%v codeArtifact=%v", res.Warnings, res.Active.CodeArtifact)
	}
	access := svc.Overview().CodeArtifactAccess
	if !access["111111111111/Developer"] || access["111111111111/ReadOnly"] {
		t.Errorf("codeArtifactAccess = %v", access)
	}

	// AWS rejects the access token: the UI is told to log in, and the
	// refresh token renews the session without the browser.
	fake.ExpireAccessTokens()
	_, err = svc.RefreshAccounts()
	requireLoginRequired(t, "refresh with expired token", err)
	if _, err := svc.RenewSession(); err != nil {
		t.Fatalf("renew: %v", err)
	}
	if _, err := svc.RefreshAccounts(); err != nil {
		t.Fatalf("refresh after renew: %v", err)
	}

	// With the refresh token revoked too, only a new login helps.
	fake.RevokeAll()
	_, err = svc.RenewSession()
	requireLoginRequired(t, "renew with revoked refresh token", err)
}

// A config saved by an older version, or edited by hand, is not validated on
// load: the Maven path is checked again before the token is written.
func TestLoadProfileRefusesMavenPathOutsideHome(t *testing.T) {
	dir := isolate(t)
	fake := fakeaws.New(fakeaws.Options{AutoApprove: true})
	srv := httptest.NewServer(fake)
	defer srv.Close()
	t.Setenv("AWS_ENDPOINT_URL", srv.URL)

	outside := filepath.Join(filepath.Dir(dir), filepath.Base(dir)+"-outside", "settings.xml")
	cfg := config.Default()
	cfg.SSO.StartURL = "https://pickrole-fake.awsapps.com/start"
	cfg.CodeArtifact.Enabled = true
	cfg.CodeArtifact.Domain = "pickrole"
	cfg.CodeArtifact.DomainOwner = "999999999999"
	cfg.CodeArtifact.Tools.Maven.SettingsPath = outside
	path, err := config.Path()
	if err != nil {
		t.Fatal(err)
	}
	data, _ := config.Encode(cfg)
	writeFile(t, path, string(data))

	svc, start := New(Build{Version: "test"})
	start(context.Background(), &fakePlatform{})
	if _, err := svc.StartLogin(); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.WaitLogin(); err != nil {
		t.Fatal(err)
	}
	res, err := svc.LoadProfile("111111111111", "Developer")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0], "home folder") {
		t.Errorf("want one warning about the Maven path, got %v", res.Warnings)
	}
	if _, err := os.Stat(outside); !os.IsNotExist(err) {
		t.Errorf("token written outside the home directory: %v", err)
	}
}

// The role comes from the UI: one AWS did not list for the account is refused.
func TestLoadProfileRejectsUnknownRole(t *testing.T) {
	isolate(t)
	fake := fakeaws.New(fakeaws.Options{AutoApprove: true})
	srv := httptest.NewServer(fake)
	defer srv.Close()
	t.Setenv("AWS_ENDPOINT_URL", srv.URL)

	svc, start := New(Build{Version: "test"})
	start(context.Background(), &fakePlatform{})
	cfg := svc.DefaultConfig()
	cfg.SSO.StartURL = "https://pickrole-fake.awsapps.com/start"
	if _, err := svc.SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.StartLogin(); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.WaitLogin(); err != nil {
		t.Fatal(err)
	}
	// platform-dev has Developer and ReadOnly, not Admin.
	if _, err := svc.LoadProfile("111111111111", "Admin"); err == nil {
		t.Error("a role not listed for the account should be refused")
	}
	if len(svc.Overview().Recents) != 0 {
		t.Error("a refused role must not be added to the recents")
	}
}

func TestConnectionAgainstFakeAWS(t *testing.T) {
	isolate(t)
	srv := httptest.NewServer(fakeaws.New(fakeaws.Options{}))
	defer srv.Close()
	t.Setenv("AWS_ENDPOINT_URL", srv.URL)
	svc, start := New(Build{Version: "test"})
	start(context.Background(), &fakePlatform{})

	cfg := svc.DefaultConfig()
	res, err := svc.TestConnection(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.Error != "" || res.Proxy != "" || res.Target != strings.TrimPrefix(srv.URL, "http://") {
		t.Errorf("unexpected result: %+v", res)
	}

	cfg.Proxy = config.Proxy{Mode: "manual", URL: "http://user:secret@proxy.example.com:3128"}
	if _, err := svc.TestConnection(cfg); err == nil {
		t.Error("a proxy URL with credentials should be rejected")
	}
}

// A cached OIDC client registration that AWS no longer knows (here, a
// restarted fake) is replaced instead of failing every sign-in.
func TestLoginReRegistersUnknownClient(t *testing.T) {
	isolate(t)
	first := httptest.NewServer(fakeaws.New(fakeaws.Options{AutoApprove: true, PollInterval: time.Second}))
	t.Setenv("AWS_ENDPOINT_URL", first.URL)

	svc, start := New(Build{Version: "test"})
	start(context.Background(), &fakePlatform{})
	cfg := svc.DefaultConfig()
	cfg.SSO.StartURL = "https://pickrole-fake.awsapps.com/start"
	if _, err := svc.SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.StartLogin(); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.WaitLogin(); err != nil {
		t.Fatal(err)
	}
	first.Close()

	second := httptest.NewServer(fakeaws.New(fakeaws.Options{AutoApprove: true, PollInterval: time.Second}))
	defer second.Close()
	t.Setenv("AWS_ENDPOINT_URL", second.URL)
	svc, start = New(Build{Version: "test"})
	start(context.Background(), &fakePlatform{})
	if _, err := svc.StartLogin(); err != nil {
		t.Fatalf("sign-in with a stale client registration: %v", err)
	}
	if _, err := svc.WaitLogin(); err != nil {
		t.Fatal(err)
	}
}
