package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func validConfig() Config {
	c := Default()
	c.SSO.StartURL = "https://example.awsapps.com/start"
	return c
}

func TestValidate(t *testing.T) {
	if err := validConfig().Validate(); err != nil {
		t.Fatalf("expected valid config, got %v", err)
	}

	c := validConfig()
	c.SSO.StartURL = "http://example.awsapps.com/start"
	if c.Validate() == nil {
		t.Error("plain http start URL should be rejected")
	}

	c = validConfig()
	c.CodeArtifact.Enabled = true
	c.CodeArtifact.Domain = "acme"
	c.CodeArtifact.DomainOwner = "1234"
	if c.Validate() == nil {
		t.Error("short domain owner should be rejected")
	}
	c.CodeArtifact.DomainOwner = "123456789012"
	if err := c.Validate(); err != nil {
		t.Errorf("expected valid CodeArtifact config, got %v", err)
	}
}

// The Maven settings file receives the CodeArtifact token and its path can
// come from a shared config file, so it must stay in the user's home.
func TestValidateMavenSettingsPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	base := validConfig()
	base.CodeArtifact.Enabled = true
	base.CodeArtifact.Domain = "acme"
	base.CodeArtifact.DomainOwner = "123456789012"
	base.CodeArtifact.Tools.Maven.Enabled = true
	base.CodeArtifact.Tools.Maven.ServerIDs = []string{"codeartifact"}

	for path, ok := range map[string]bool{
		"~/.m2/settings.xml":                            true,
		filepath.Join(home, ".m2", "settings-work.xml"): true,
		`\\attacker\share\settings.xml`:                 false,
		"//attacker/share/settings.xml":                 false,
		`\Windows\settings.xml`:                         false,
		"~/../other-user/.m2/settings.xml":              false,
		filepath.Join(home, "..", "settings.xml"):       false,
		filepath.Join(filepath.Dir(home), "x", "a.xml"): false,
		"~/.bashrc":        false,
		".m2/settings.xml": false,
	} {
		c := base
		c.CodeArtifact.Tools.Maven.SettingsPath = path
		if err := c.Validate(); (err == nil) != ok {
			t.Errorf("settingsPath %q: accepted=%v, want %v (err: %v)", path, err == nil, ok, err)
		}
	}

	// Spaces around the path: it is used exactly as given, so it must be exact.
	c := base
	c.CodeArtifact.Tools.Maven.SettingsPath = " ~/.m2/settings.xml"
	if ValidateSettingsPath(c.CodeArtifact.Tools.Maven.SettingsPath) == nil {
		t.Error("a path with a leading space should be rejected")
	}
	if runtime.GOOS == "windows" {
		for path, ok := range map[string]bool{
			`~\.m2\settings.xml`: true,
			filepath.Join(home, ".m2", "settings.xml") + ":stream.xml": false,
			`\\?\` + filepath.Join(home, ".m2", "settings.xml"):        false,
		} {
			if err := ValidateSettingsPath(path); (err == nil) != ok {
				t.Errorf("settingsPath %q: accepted=%v, want %v", path, err == nil, ok)
			}
		}
	}

	for id, ok := range map[string]bool{"codeartifact": true, "my_repo.v2-x": true, "a</id><x>": false, "a b": false, "": false} {
		c := base
		c.CodeArtifact.Tools.Maven.SettingsPath = "~/.m2/settings.xml"
		c.CodeArtifact.Tools.Maven.ServerIDs = []string{id}
		if err := c.Validate(); (err == nil) != ok {
			t.Errorf("serverId %q: accepted=%v, want %v", id, err == nil, ok)
		}
	}
}

func TestDecodeKeepsDefaults(t *testing.T) {
	cfg, err := Decode([]byte(`{"sso":{"startUrl":"https://x.awsapps.com/start","region":"sa-east-1"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SSO.Region != "sa-east-1" {
		t.Errorf("region = %q", cfg.SSO.Region)
	}
	if ids := cfg.CodeArtifact.Tools.Maven.ServerIDs; !slices.Equal(ids, []string{"codeartifact"}) {
		t.Errorf("default Maven server ids lost: %v", ids)
	}
	if cfg.Preferences.ProfileMode != ProfileDefault {
		t.Errorf("profile mode = %q", cfg.Preferences.ProfileMode)
	}
}

func TestIsProduction(t *testing.T) {
	c := Default()
	cases := map[string]bool{
		"platform-prod":       true,
		"prod-data":           true,
		"payments_production": true,
		"PROD":                true,
		"products-dev":        false,
		"platform-dev":        false,
		"reproduce":           false,
	}
	for name, want := range cases {
		if got := c.IsProduction(name); got != want {
			t.Errorf("IsProduction(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestDetectSSO(t *testing.T) {
	const awsConfig = `
[profile legacy]
sso_start_url = https://legacy.awsapps.com/start
sso_region = us-west-2

[sso-session acme]
sso_start_url = https://acme.awsapps.com/start
sso_region = sa-east-1
sso_registration_scopes = sso:account:access
`
	got, ok := detectSSO(strings.NewReader(awsConfig))
	if !ok {
		t.Fatal("expected to detect an SSO session")
	}
	want := SSO{StartURL: "https://acme.awsapps.com/start", Region: "sa-east-1", SessionName: "acme"}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}

	got, ok = detectSSO(strings.NewReader("[profile legacy]\nsso_start_url = https://l.awsapps.com/start\nsso_region = us-west-2\n"))
	if !ok || got.StartURL != "https://l.awsapps.com/start" || got.Region != "us-west-2" {
		t.Errorf("legacy profile not detected: %+v %v", got, ok)
	}

	if _, ok := detectSSO(strings.NewReader("[default]\nregion = us-east-1\n")); ok {
		t.Error("config without SSO should not be detected")
	}
}

// A link inside the home that leads outside it must not pass: the check
// follows it. On Windows a junction is used, which needs no privileges.
func TestValidateSettingsPathFollowsLinks(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	outside := filepath.Join(root, "outside")
	for _, d := range []string{home, outside, filepath.Join(home, "real")} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	link := filepath.Join(home, "link")
	if runtime.GOOS == "windows" {
		if out, err := exec.Command("cmd", "/c", "mklink", "/J", link, outside).CombinedOutput(); err != nil {
			t.Skipf("mklink /J: %v %s", err, out)
		}
	} else if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink: %v", err)
	}

	if err := ValidateSettingsPath(filepath.Join(link, "settings.xml")); err == nil {
		t.Error("a link pointing outside the home should be rejected")
	}
	if err := ValidateSettingsPath(filepath.Join(link, "new-dir", "settings.xml")); err == nil {
		t.Error("a missing directory under such a link should be rejected too")
	}
	if err := ValidateSettingsPath(filepath.Join(home, "real", "new-dir", "settings.xml")); err != nil {
		t.Errorf("a missing directory inside the home should be accepted: %v", err)
	}
}

func TestValidateProxy(t *testing.T) {
	for _, c := range []struct {
		proxy Proxy
		ok    bool
	}{
		{Proxy{Mode: "system"}, true},
		{Proxy{Mode: "none"}, true},
		{Proxy{Mode: "manual", URL: "http://proxy.example.com:3128"}, true},
		{Proxy{Mode: "manual", URL: "http://10.0.0.5:8080/", NoProxy: "*.corp.example; localhost, 10.0.0.0/8, [::1]"}, true},
		{Proxy{Mode: "manual", URL: "socks5://socks.example.com:1080"}, true},
		{Proxy{Mode: "auto"}, false},
		{Proxy{Mode: "manual"}, false},
		{Proxy{Mode: "manual", URL: "proxy.example.com:3128"}, false},
		{Proxy{Mode: "manual", URL: "ftp://proxy.example.com:21"}, false},
		{Proxy{Mode: "manual", URL: "http://proxy.example.com:3128/path"}, false},
		{Proxy{Mode: "manual", URL: "http://user:secret@proxy.example.com:3128"}, false},
		{Proxy{Mode: "manual", URL: "http://proxy.example.com:3128", NoProxy: "ok.example, bad\"host"}, false},
	} {
		if err := c.proxy.Validate(); (err == nil) != c.ok {
			t.Errorf("%+v: got %v, want ok=%v", c.proxy, err, c.ok)
		}
	}

	// Configs saved before the proxy setting existed get the system proxy.
	cfg, err := Decode([]byte(`{"sso":{"startUrl":"https://example.awsapps.com/start","region":"us-east-1"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Proxy.Mode != "system" {
		t.Errorf("proxy mode = %q, want system", cfg.Proxy.Mode)
	}
}

// Loading a role in production asks for confirmation unless its name says it
// only reads; a substring such as "read" in "DataReadWrite" must not count.
func TestIsReadOnlyRole(t *testing.T) {
	for role, want := range map[string]bool{
		"ReadOnly":            true,
		"AWSReadOnlyAccess":   true,
		"readonly":            true,
		"read-only":           true,
		"ViewOnlyAccess":      true,
		"SecurityAudit":       true,
		"Billing":             true,
		"AWSBilling":          true,
		"DataReadWrite":       false,
		"ReadWriteAdmin":      false,
		"ReadOnlyAdmin":       false,
		"OverviewAdmin":       false,
		"AuditWriter":         false,
		"Breadmaker":          false,
		"ReadyDeploy":         false,
		"AdministratorAccess": false,
		"Developer":           false,
		"PowerUserAccess":     false,
		"BillingFullAccess":   false,
		"":                    false,
	} {
		if got := IsReadOnlyRole(role); got != want {
			t.Errorf("IsReadOnlyRole(%q) = %v, want %v", role, got, want)
		}
	}
}

// Configs written before ServerIDs have a single serverId; it becomes the
// list, and the old field isn't written back.
func TestMavenServerIDs(t *testing.T) {
	cfg, err := Decode([]byte(`{"sso":{"startUrl":"https://example.awsapps.com/start","region":"us-east-1"},
		"codeArtifact":{"tools":{"maven":{"enabled":true,"serverId":"legacy","settingsPath":"~/.m2/settings.xml"}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	m := cfg.CodeArtifact.Tools.Maven
	if !slices.Equal(m.ServerIDs, []string{"legacy"}) || m.ServerID != "" {
		t.Errorf("serverId not moved into serverIds: %+v", m)
	}
	data, _ := Encode(cfg)
	if strings.Contains(string(data), `"serverId"`) {
		t.Errorf("the old field should not be written back:\n%s", data)
	}

	cfg = validConfig()
	cfg.CodeArtifact.Tools.Maven.ServerIDs = CleanServerIDs([]string{" a ", "b", "", "a"})
	if !slices.Equal(cfg.CodeArtifact.Tools.Maven.ServerIDs, []string{"a", "b"}) {
		t.Errorf("CleanServerIDs = %v", cfg.CodeArtifact.Tools.Maven.ServerIDs)
	}

	cfg.CodeArtifact.Enabled = true
	cfg.CodeArtifact.Domain = "acme"
	cfg.CodeArtifact.DomainOwner = "123456789012"
	cfg.CodeArtifact.Tools.Maven.ServerIDs = []string{"ok", "bad id"}
	if cfg.Validate() == nil {
		t.Error("an id with a space should be rejected")
	}
	cfg.CodeArtifact.Tools.Maven.ServerIDs = nil
	if cfg.Validate() == nil {
		t.Error("an empty list should be rejected")
	}
	cfg.CodeArtifact.Tools.Maven.ServerIDs = []string{"ca-releases", "ca-snapshots"}
	if err := cfg.Validate(); err != nil {
		t.Errorf("valid list rejected: %v", err)
	}
}

// New configs are dark; configs written before still load, including the
// removed repository and startMinimized fields, and keep their theme.
func TestThemeDefaultAndRemovedFields(t *testing.T) {
	if Default().Preferences.Theme != "dark" {
		t.Errorf("default theme = %q, want dark", Default().Preferences.Theme)
	}
	cfg, err := Decode([]byte(`{"sso":{"startUrl":"https://example.awsapps.com/start","region":"us-east-1"},
		"codeArtifact":{"repository":"releases"},"preferences":{"startMinimized":true,"theme":"system"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Preferences.Theme != "system" {
		t.Errorf("a saved theme must be kept, got %q", cfg.Preferences.Theme)
	}
	data, _ := Encode(cfg)
	if strings.Contains(string(data), "repository") || strings.Contains(string(data), "startMinimized") {
		t.Errorf("removed fields written back:\n%s", data)
	}
}

func TestValidateProfileFormat(t *testing.T) {
	for format, ok := range map[string]bool{
		"{account}.{role}":                    true,
		"{accountId}_{role}":                  true,
		"prefix-{account}-{accountId}.{role}": true,
		"{role}":                              false, // not unique per account
		"{account}":                           false, // not unique per role
		"{accountId} {role}":                  false,
		"{accountId}]_{role}":                 false,
		"{acount}_{role}":                     false,
	} {
		c := validConfig()
		c.Preferences.ProfileFormat = format
		if err := c.Validate(); (err == nil) != ok {
			t.Errorf("%q: accepted=%v, want %v", format, err == nil, ok)
		}
	}
	// Configs from before the setting keep today's names.
	cfg, err := Decode([]byte(`{"sso":{"startUrl":"https://example.awsapps.com/start","region":"us-east-1"},"preferences":{"profileMode":"named"}}`))
	if err != nil || cfg.Preferences.ProfileFormat != "{account}.{role}" || cfg.Preferences.AlsoDefault {
		t.Errorf("defaults: %+v, %v", cfg.Preferences, err)
	}
}
