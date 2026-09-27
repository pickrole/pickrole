package app

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/pickrole/pickrole/internal/config"
)

type fakePlatform struct{ clipboard, opened, openPath string }

func (f *fakePlatform) OpenURL(url string)                      { f.opened = url }
func (f *fakePlatform) SetClipboard(text string) error          { f.clipboard = text; return nil }
func (f *fakePlatform) SetSecretClipboard(text string) error    { f.clipboard = text; return nil }
func (f *fakePlatform) OpenFile(string) (string, error)         { return f.openPath, nil }
func (f *fakePlatform) SaveFile(string, string) (string, error) { return "", nil }

func isolate(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(dir, ".cache"))
	// Windows: os.UserHomeDir, UserConfigDir and UserCacheDir read these instead.
	t.Setenv("USERPROFILE", dir)
	t.Setenv("APPDATA", filepath.Join(dir, "AppData", "Roaming"))
	t.Setenv("LOCALAPPDATA", filepath.Join(dir, "AppData", "Local"))
	t.Setenv("AWS_CONFIG_FILE", filepath.Join(dir, ".aws", "config"))
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", filepath.Join(dir, ".aws", "credentials"))
	// Never reach a real or local AWS unless a test sets these itself.
	for _, v := range []string{"", "_SSO", "_SSO_OIDC", "_CODEARTIFACT"} {
		t.Setenv("AWS_ENDPOINT_URL"+v, "")
	}
	return dir
}

func TestFirstRunAndSaveConfig(t *testing.T) {
	isolate(t)
	svc, start := New(Build{Version: "test"})
	start(context.Background(), &fakePlatform{})

	o := svc.Overview()
	if o.Configured {
		t.Fatal("fresh install should not be configured")
	}
	if len(o.Accounts) != 0 || o.Accounts == nil {
		t.Errorf("accounts should be an empty list, got %#v", o.Accounts)
	}

	cfg := svc.DefaultConfig()
	if _, err := svc.SaveConfig(cfg); err == nil {
		t.Error("config without start URL should be rejected")
	}

	cfg.SSO.StartURL = "https://example.awsapps.com/start"
	o, err := svc.SaveConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !o.Configured || o.Session.LoggedIn {
		t.Errorf("after save: configured=%v loggedIn=%v", o.Configured, o.Session.LoggedIn)
	}

	// A new process sees the saved config.
	svc2, start2 := New(Build{Version: "test"})
	start2(context.Background(), &fakePlatform{})
	if got := svc2.Overview().Config.SSO.StartURL; got != cfg.SSO.StartURL {
		t.Errorf("reloaded start URL = %q", got)
	}
}

func TestDefaultConfigDetectsAWSConfig(t *testing.T) {
	dir := isolate(t)
	awsDir := filepath.Join(dir, ".aws")
	if err := os.MkdirAll(awsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	content := "[sso-session acme]\nsso_start_url = https://acme.awsapps.com/start\nsso_region = sa-east-1\n"
	if err := os.WriteFile(filepath.Join(awsDir, "config"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	svc, start := New(Build{Version: "test"})
	start(context.Background(), &fakePlatform{})
	got := svc.DefaultConfig().SSO
	want := config.SSO{StartURL: "https://acme.awsapps.com/start", Region: "sa-east-1", SessionName: "acme"}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestCopyExportNeedsLoadedProfile(t *testing.T) {
	isolate(t)
	svc, start := New(Build{Version: "test"})
	start(context.Background(), &fakePlatform{})
	if err := svc.CopyExport(); err == nil {
		t.Error("expected an error before any profile is loaded")
	}
}

func TestLoginRequiresConfig(t *testing.T) {
	isolate(t)
	svc, start := New(Build{Version: "test"})
	start(context.Background(), &fakePlatform{})
	if _, err := svc.StartLogin(); err == nil {
		t.Error("login without config should fail")
	}
	if _, err := svc.LoadProfile("123456789012", "Dev"); err == nil {
		t.Error("loading a profile without config should fail")
	}
}

func TestAbout(t *testing.T) {
	isolate(t)
	svc, start := New(Build{Version: "1.2.3", Commit: "abc1234", Date: "2026-09-26T19:00:00Z"})
	start(context.Background(), &fakePlatform{})

	a := svc.About()
	if a.Version != "1.2.3" || a.Commit != "abc1234" || a.BuildDate != "2026-09-26T19:00:00Z" {
		t.Errorf("build info not passed through: %+v", a)
	}
	if a.Platform != runtime.GOOS+"/"+runtime.GOARCH || a.Go == "" || a.Repository == "" {
		t.Errorf("platform = %q, go = %q, repository = %q", a.Platform, a.Go, a.Repository)
	}
	paths := map[string]string{}
	for _, f := range a.Files {
		paths[f.Kind] = f.Path
	}
	for label, want := range map[string]string{
		"credentials": "~/.aws/credentials",
		"ssoCache":    "~/.aws/sso/cache",
	} {
		if paths[label] != want {
			t.Errorf("%s = %q, want %q", label, paths[label], want)
		}
	}
	if !strings.HasPrefix(paths["config"], "~/") || !strings.HasSuffix(paths["config"], "pickrole/config.json") {
		t.Errorf("config = %q", paths["config"])
	}
	if _, ok := paths["maven"]; ok {
		t.Error("Maven should only be listed when CodeArtifact and Maven are enabled")
	}
}

func TestBrowserURL(t *testing.T) {
	for raw, ok := range map[string]bool{
		"https://device.sso.us-east-1.amazonaws.com/?user_code=ABCD-EFGH": true,
		"http://127.0.0.1:4599/device?user_code=ABCD-EFGH":                true,
		"http://localhost:4599/device":                                    true,
		"http://device.sso.us-east-1.amazonaws.com/":                      false,
		"file:///C:/Windows/System32/calc.exe":                            false,
		"javascript:alert(1)":                                             false,
		"ms-settings:":                                                    false,
		"https://user:pass@evil.example/":                                 false,
		"":                                                                false,
	} {
		if got := browserURL(raw); got != ok {
			t.Errorf("browserURL(%q) = %v, want %v", raw, got, ok)
		}
	}
}

func TestOpenURLOnlyOpensSafeURLs(t *testing.T) {
	isolate(t)
	p := &fakePlatform{}
	svc, start := New(Build{Version: "test"})
	start(context.Background(), p)
	svc.OpenURL("file:///C:/Windows/System32/calc.exe")
	svc.OpenURL("https://user:pass@evil.example/")
	if p.opened != "" {
		t.Errorf("opened %q", p.opened)
	}
	svc.OpenURL("http://127.0.0.1:4599/device")
	if p.opened != "http://127.0.0.1:4599/device" {
		t.Errorf("loopback URL of the fake AWS not opened: %q", p.opened)
	}
}

func TestImportConfigRejectsHugeFile(t *testing.T) {
	dir := isolate(t)
	path := filepath.Join(dir, "pickrole.json")
	if err := os.WriteFile(path, []byte(`{"x":"`+strings.Repeat("a", 2<<20)+`"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	svc, start := New(Build{Version: "test"})
	start(context.Background(), &fakePlatform{openPath: path})
	if _, err := svc.ImportConfig(); err == nil || !strings.Contains(err.Error(), "too large") {
		t.Errorf("want a size error, got %v", err)
	}
}

// The UI sets the language; backend messages follow it.
func TestSetLanguage(t *testing.T) {
	isolate(t)
	svc, start := New(Build{Version: "test"})
	start(context.Background(), &fakePlatform{})
	defer svc.SetLanguage("en")

	svc.SetLanguage("pt-BR")
	if _, err := svc.StartLogin(); err == nil || !strings.Contains(err.Error(), "Configure a conexão SSO") {
		t.Errorf("pt-BR message expected, got %v", err)
	}
	svc.SetLanguage("en")
	if _, err := svc.StartLogin(); err == nil || !strings.Contains(err.Error(), "Set up the SSO connection") {
		t.Errorf("English message expected, got %v", err)
	}
}

func TestDetectMaven(t *testing.T) {
	dir := isolate(t)
	svc, start := New(Build{Version: "test"})
	start(context.Background(), &fakePlatform{})
	cfg := svc.DefaultConfig()
	cfg.CodeArtifact.Tools.Maven.SettingsPath = "~/.m2/settings.xml"

	if _, err := svc.DetectMaven(cfg); err == nil {
		t.Error("a missing settings.xml should be reported")
	}

	writeFile(t, filepath.Join(dir, ".m2", "settings.xml"), `<settings><servers>
  <server><id>a</id><password>${env.CODEARTIFACT_AUTH_TOKEN}</password></server>
  <server><id>b</id><password>${env.CODEARTIFACT_AUTH_TOKEN}</password></server>
</servers></settings>`)
	det, err := svc.DetectMaven(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(det.ServerIDs, ",") != "a,b" {
		t.Errorf("server ids = %v", det.ServerIDs)
	}

	cfg.CodeArtifact.Tools.Maven.SettingsPath = filepath.Join(t.TempDir(), "settings.xml")
	if _, err := svc.DetectMaven(cfg); err == nil {
		t.Error("a settings.xml outside the home folder should be refused")
	}
}
