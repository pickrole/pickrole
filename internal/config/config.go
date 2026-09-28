// Package config holds PickRole's settings: where the SSO lives, how
// CodeArtifact is reached and which build tools get configured.
package config

import (
	"encoding/json"
	"errors"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/pickrole/pickrole/internal/awsenv"
	"github.com/pickrole/pickrole/internal/awsfiles"
	"github.com/pickrole/pickrole/internal/fsutil"
	"github.com/pickrole/pickrole/internal/i18n"
)

// CurrentVersion is the config file format version.
const CurrentVersion = 1

// Profile modes.
const (
	ProfileDefault = "default" // write to the [default] profile
	ProfileNamed   = "named"   // one named profile per account/role
)

// DefaultProdPattern matches account names such as "payments-prod",
// "prod-data" or "platform_production", but not "products".
const DefaultProdPattern = `(?i)(^|[^a-z])prod(uction)?([^a-z]|$)`

type SSO struct {
	StartURL    string `json:"startUrl"`
	Region      string `json:"region"`
	SessionName string `json:"sessionName"`
}

type Maven struct {
	Enabled bool `json:"enabled"`
	// ServerIDs are the <server> entries of settings.xml that receive the
	// token: one per repository or mirror that points to CodeArtifact.
	ServerIDs []string `json:"serverIds"`
	// ServerID is the single id of configs written before ServerIDs; it
	// is moved into ServerIDs when the config is read.
	ServerID     string `json:"serverId,omitempty"`
	SettingsPath string `json:"settingsPath"`
}

// Tools lists the build tools PickRole configures with the CodeArtifact
// token. Only Maven is implemented so far; the others are placeholders so
// the file format does not change when they land.
type Tools struct {
	Maven  Maven `json:"maven"`
	Gradle bool  `json:"gradle"`
	Npm    bool  `json:"npm"`
	Pip    bool  `json:"pip"`
}

type CodeArtifact struct {
	Enabled     bool   `json:"enabled"`
	Domain      string `json:"domain"`
	DomainOwner string `json:"domainOwner"`
	Region      string `json:"region"`
	Tools       Tools  `json:"tools"`
}

type Preferences struct {
	ProfileMode string `json:"profileMode"`
	// ProfileFormat names the profiles in the named mode, with {account},
	// {accountId} and {role} (see awsfiles.ProfileName).
	ProfileFormat string `json:"profileFormat"`
	// AlsoDefault also writes the credentials to [default] in the named
	// mode, for tools and scripts that don't set AWS_PROFILE.
	AlsoDefault bool `json:"alsoDefault"`
	AutoRenew   bool `json:"autoRenew"`
	// CheckUpdates looks for a newer release on GitHub (docs/adr/0024).
	CheckUpdates bool   `json:"checkUpdates"`
	Theme        string `json:"theme"`    // dark (default) | light | system
	Language     string `json:"language"` // system | en | pt-BR
	ProdPattern  string `json:"prodPattern"`
}

// Proxy is how PickRole reaches AWS. Mode is awsenv.ProxySystem (the
// environment variables, then the system settings), ProxyManual (URL and
// NoProxy) or ProxyNone.
type Proxy struct {
	Mode string `json:"mode"`
	// URL is the proxy address, such as http://proxy.example.com:3128.
	URL string `json:"url"`
	// NoProxy lists hosts that skip the proxy: names, ".suffix" or
	// "*.suffix" domains, IP addresses and CIDR ranges.
	NoProxy string `json:"noProxy"`
}

// Settings returns the proxy as awsenv takes it.
func (p Proxy) Settings() awsenv.ProxySettings {
	return awsenv.ProxySettings{Mode: p.Mode, URL: p.URL, NoProxy: p.NoProxy}
}

type Config struct {
	Version      int          `json:"version"`
	SSO          SSO          `json:"sso"`
	CodeArtifact CodeArtifact `json:"codeArtifact"`
	Proxy        Proxy        `json:"proxy"`
	Preferences  Preferences  `json:"preferences"`
}

// Default returns a config with sensible defaults and no SSO filled in.
func Default() Config {
	return Config{
		Version: CurrentVersion,
		SSO:     SSO{Region: "us-east-1", SessionName: "pickrole"},
		CodeArtifact: CodeArtifact{
			Region: "us-east-1",
			Tools: Tools{Maven: Maven{
				Enabled:      true,
				ServerIDs:    []string{"codeartifact"},
				SettingsPath: "~/.m2/settings.xml",
			}},
		},
		Proxy: Proxy{Mode: awsenv.ProxySystem},
		Preferences: Preferences{
			ProfileMode:   ProfileDefault,
			ProfileFormat: awsfiles.DefaultProfileFormat,
			AutoRenew:     true,
			CheckUpdates:  true,
			Theme:         "dark",
			Language:      "system",
			ProdPattern:   DefaultProdPattern,
		},
	}
}

// Dir is PickRole's config directory (~/.config/pickrole on Linux).
func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "pickrole"), nil
}

// Path is the config file location.
func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// Load reads the config file. found is false when there is none yet, in
// which case the defaults are returned.
func Load() (cfg Config, found bool, err error) {
	path, err := Path()
	if err != nil {
		return Default(), false, err
	}
	cfg = Default()
	found, err = fsutil.ReadJSON(path, &cfg)
	if err != nil {
		return Default(), false, i18n.Wrap("config.reading", err, path)
	}
	cfg.fillDefaults()
	return cfg, found, nil
}

// Save validates and writes the config file with 0600 permissions.
func Save(cfg Config) error {
	cfg.fillDefaults()
	if err := cfg.Validate(); err != nil {
		return err
	}
	path, err := Path()
	if err != nil {
		return err
	}
	return fsutil.WriteJSON(path, cfg, 0o600)
}

// Decode parses a config exported by another PickRole install. Fields that
// are missing keep their defaults.
func Decode(data []byte) (Config, error) {
	cfg := Default()
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, i18n.Wrap("config.invalid_file", err)
	}
	cfg.fillDefaults()
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Encode returns the config as indented JSON, ready to share with a team.
func Encode(cfg Config) ([]byte, error) {
	return json.MarshalIndent(cfg, "", "  ")
}

func (c *Config) fillDefaults() {
	d := Default()
	if c.Version == 0 {
		c.Version = CurrentVersion
	}
	if c.SSO.SessionName == "" {
		c.SSO.SessionName = d.SSO.SessionName
	}
	if c.Preferences.ProfileMode == "" {
		c.Preferences.ProfileMode = d.Preferences.ProfileMode
	}
	c.Preferences.ProfileFormat = strings.TrimSpace(c.Preferences.ProfileFormat)
	if c.Preferences.ProfileFormat == "" {
		c.Preferences.ProfileFormat = d.Preferences.ProfileFormat
	}
	if c.Preferences.Language == "" {
		c.Preferences.Language = d.Preferences.Language
	}
	if c.Preferences.Theme == "" {
		c.Preferences.Theme = d.Preferences.Theme
	}
	if c.Preferences.ProdPattern == "" {
		c.Preferences.ProdPattern = d.Preferences.ProdPattern
	}
	if c.Proxy.Mode == "" {
		c.Proxy.Mode = d.Proxy.Mode
	}
	c.Proxy.URL = strings.TrimSpace(c.Proxy.URL)
	c.Proxy.NoProxy = strings.TrimSpace(c.Proxy.NoProxy)
	m := &c.CodeArtifact.Tools.Maven
	// A config from before ServerIDs has only serverId, so ServerIDs still
	// holds the default: the old id replaces it.
	if m.ServerID != "" {
		m.ServerIDs = []string{m.ServerID}
		m.ServerID = ""
	}
	m.ServerIDs = CleanServerIDs(m.ServerIDs)
	if len(m.ServerIDs) == 0 {
		m.ServerIDs = d.CodeArtifact.Tools.Maven.ServerIDs
	}
	if c.CodeArtifact.Tools.Maven.SettingsPath == "" {
		c.CodeArtifact.Tools.Maven.SettingsPath = d.CodeArtifact.Tools.Maven.SettingsPath
	}
	c.SSO.StartURL = strings.TrimSpace(c.SSO.StartURL)
	c.CodeArtifact.Domain = strings.TrimSpace(c.CodeArtifact.Domain)
	c.CodeArtifact.DomainOwner = strings.TrimSpace(c.CodeArtifact.DomainOwner)
	c.CodeArtifact.Tools.Maven.SettingsPath = strings.TrimSpace(c.CodeArtifact.Tools.Maven.SettingsPath)
}

// CleanServerIDs trims the ids and drops empty ones and repeats, keeping the
// order.
func CleanServerIDs(ids []string) []string {
	out := []string{}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id != "" && !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	return out
}

var (
	accountIDRe = regexp.MustCompile(`^\d{12}$`)
	regionRe    = regexp.MustCompile(`^[a-z]{2}(-[a-z]+)+-\d$`)
	serverIDRe  = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
	// Literal text allowed in a profile name, plus the placeholders.
	profileFormatRe = regexp.MustCompile(`^([A-Za-z0-9._-]|\{account\}|\{accountId\}|\{role\})+$`)
)

// ValidRegion reports whether region looks like an AWS region (us-east-1).
func ValidRegion(region string) bool {
	return regionRe.MatchString(region)
}

// Validate reports the first problem that would stop PickRole from working.
func (c Config) Validate() error {
	u, err := url.Parse(c.SSO.StartURL)
	if c.SSO.StartURL == "" || err != nil || u.Scheme != "https" || u.Host == "" {
		return i18n.New("config.start_url")
	}
	if !regionRe.MatchString(c.SSO.Region) {
		return i18n.New("config.sso_region", c.SSO.Region)
	}
	switch c.Preferences.ProfileMode {
	case ProfileDefault, ProfileNamed:
	default:
		return i18n.New("config.profile_mode", c.Preferences.ProfileMode)
	}
	if f := c.Preferences.ProfileFormat; !profileFormatRe.MatchString(f) || !strings.Contains(f, "{role}") ||
		!strings.Contains(f, "{account}") && !strings.Contains(f, "{accountId}") {
		return i18n.New("config.profile_format", f)
	}
	switch c.Preferences.Language {
	case "system", string(i18n.English), string(i18n.Portuguese):
	default:
		return i18n.New("config.language", c.Preferences.Language)
	}
	if _, err := regexp.Compile(c.Preferences.ProdPattern); err != nil {
		return i18n.Wrap("config.prod_pattern", err)
	}
	if err := c.Proxy.Validate(); err != nil {
		return err
	}
	ca := c.CodeArtifact
	if !ca.Enabled {
		return nil
	}
	if ca.Domain == "" {
		return i18n.New("config.ca_domain")
	}
	if !accountIDRe.MatchString(ca.DomainOwner) {
		return i18n.New("config.ca_owner")
	}
	if !regionRe.MatchString(ca.Region) {
		return i18n.New("config.ca_region", ca.Region)
	}
	if m := ca.Tools.Maven; m.Enabled {
		if len(m.ServerIDs) == 0 {
			return i18n.New("config.maven_no_server_ids")
		}
		for _, id := range m.ServerIDs {
			if !serverIDRe.MatchString(id) {
				return i18n.New("config.maven_server_id", id)
			}
		}
		if err := ValidateSettingsPath(m.SettingsPath); err != nil {
			return err
		}
	}
	return nil
}

// noProxyEntryRe allows host names, wildcards, IPv4/IPv6 addresses, CIDR
// ranges and ports: nothing that could smuggle another setting in.
var noProxyEntryRe = regexp.MustCompile(`^[A-Za-z0-9.*:/\[\]_-]+$`)

// Validate checks the proxy. The URL can come from a config shared by
// someone else, so it holds no credentials: they would end up in a file
// meant to be shared. Proxies that need a password are set through
// HTTPS_PROXY instead.
func (p Proxy) Validate() error {
	switch p.Mode {
	case awsenv.ProxySystem, awsenv.ProxyNone:
	case awsenv.ProxyManual:
		u, err := url.Parse(p.URL)
		if p.URL == "" || err != nil || u.Host == "" || u.Hostname() == "" || u.Opaque != "" ||
			(u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
			return i18n.New("config.proxy_url")
		}
		switch u.Scheme {
		case "http", "https", "socks5":
		default:
			return i18n.New("config.proxy_url")
		}
		if u.User != nil {
			return i18n.New("config.proxy_credentials")
		}
	default:
		return i18n.New("config.proxy_mode", p.Mode)
	}
	for _, entry := range strings.Split(awsenv.NormalizeNoProxy(p.NoProxy), ",") {
		if entry != "" && !noProxyEntryRe.MatchString(entry) {
			return i18n.New("config.no_proxy", entry)
		}
	}
	return nil
}

// ValidateSettingsPath keeps the Maven settings file, which receives the
// CodeArtifact token, inside the user's home directory. The path can come
// from a config file shared by someone else: without this check it could
// point at a network share (\\host\share on Windows, which would also send
// the user's NTLM hash) or at any other XML file on the machine. Paths that
// start with a backslash (UNC, or rooted without a drive) are rejected outright.
//
// The check is on the path as given and again on where it really leads, so a
// link or junction inside the home that points outside it is refused too.
func ValidateSettingsPath(path string) error {
	bad := i18n.New("config.maven_settings_path")
	// The path is used exactly as given, so what is checked must be the same.
	if path != strings.TrimSpace(path) {
		return bad
	}
	expanded := fsutil.ExpandHome(path)
	if !strings.EqualFold(filepath.Ext(expanded), ".xml") || !filepath.IsAbs(expanded) ||
		strings.HasPrefix(expanded, `\`) || strings.HasPrefix(expanded, "//") ||
		// A colon after the drive letter names an NTFS alternate data stream.
		strings.Contains(expanded[len(filepath.VolumeName(expanded)):], ":") {
		return bad
	}
	home, err := os.UserHomeDir()
	if err != nil || !within(home, filepath.Clean(expanded)) {
		return bad
	}
	realHome, err := fsutil.RealPath(home)
	if err != nil {
		return bad
	}
	real, err := resolveExisting(filepath.Clean(expanded))
	if err != nil || !within(realHome, real) {
		return bad
	}
	return nil
}

// within reports whether path is strictly inside dir.
func within(dir, path string) bool {
	rel, err := filepath.Rel(dir, path)
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// resolveExisting follows links and junctions in the longest part of path
// that exists and appends the rest, which is created later.
func resolveExisting(path string) (string, error) {
	rest := ""
	for p := path; ; {
		real, err := fsutil.RealPath(p)
		if err == nil {
			return filepath.Join(real, rest), nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(p)
		if parent == p {
			return "", err
		}
		rest = filepath.Join(filepath.Base(p), rest)
		p = parent
	}
}

// IsProduction reports whether an account name looks like production.
var (
	camelLowerUpper = regexp.MustCompile(`([a-z0-9])([A-Z])`)
	camelAcronym    = regexp.MustCompile(`([A-Z]+)([A-Z][a-z])`)
	nonWord         = regexp.MustCompile(`[^a-z0-9]+`)
)

// roleWords splits a role name into lowercase words: "AWSReadOnlyAccess" is
// aws, read, only, access; "data-read_write" is data, read, write.
func roleWords(role string) []string {
	s := camelLowerUpper.ReplaceAllString(role, "$1 $2")
	s = camelAcronym.ReplaceAllString(s, "$1 $2")
	return strings.Fields(nonWord.ReplaceAllString(strings.ToLower(s), " "))
}

// IsReadOnlyRole reports whether a role name says it only reads, so loading
// it in production needs no confirmation. It compares whole words, never
// substrings: ReadOnly, ViewOnly, SecurityAudit and Billing count, but
// DataReadWrite or OverviewAdmin don't. A word that suggests writing
// (write, admin, full, power…) always means the role needs confirmation.
func IsReadOnlyRole(role string) bool {
	words := roleWords(role)
	for _, w := range words {
		switch w {
		case "write", "writer", "readwrite", "rw", "admin", "administrator", "full", "power", "poweruser",
			"owner", "developer", "dev", "deploy", "deployer", "operator", "manage", "manager", "editor":
			return false
		}
	}
	has := func(seq ...string) bool {
		for i := 0; i+len(seq) <= len(words); i++ {
			if slices.Equal(words[i:i+len(seq)], seq) {
				return true
			}
		}
		return false
	}
	return has("read", "only") || has("readonly") || has("view", "only") || has("viewonly") ||
		has("security", "audit") || has("securityaudit") || has("billing")
}

func (c Config) IsProduction(accountName string) bool {
	re, err := regexp.Compile(c.Preferences.ProdPattern)
	if err != nil {
		re = regexp.MustCompile(DefaultProdPattern)
	}
	return re.MatchString(accountName)
}
