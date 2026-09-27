// Package awsfiles writes temporary credentials to the shared AWS
// credentials file, so every terminal and IDE picks them up without any
// environment variables.
package awsfiles

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/pickrole/pickrole/internal/fsutil"
	"github.com/pickrole/pickrole/internal/i18n"
)

// Credentials are temporary role credentials.
type Credentials struct {
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
	Expiration      time.Time
}

// CredentialsPath returns the shared credentials file, honouring
// AWS_SHARED_CREDENTIALS_FILE.
func CredentialsPath() string {
	if p := os.Getenv("AWS_SHARED_CREDENTIALS_FILE"); p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".aws", "credentials")
}

// credentialRe covers access key IDs, secret keys and session tokens (base64).
var credentialRe = regexp.MustCompile(`^[A-Za-z0-9+/=_.:-]+$`)

// Validate rejects values that could break out of their line: a value with a
// newline would inject entries into the credentials file, and one with shell
// characters would inject commands into the `export` lines when pasted.
func (c Credentials) Validate() error {
	for name, v := range map[string]string{
		"access key": c.AccessKeyID, "secret key": c.SecretAccessKey, "session token": c.SessionToken,
	} {
		if !credentialRe.MatchString(v) {
			return i18n.New("awsfiles.invalid_credential", name)
		}
	}
	return nil
}

var profileNameRe = regexp.MustCompile(`[^A-Za-z0-9_.-]+`)

// ProfileName builds a named profile such as "platform-dev.Developer".
func ProfileName(accountName, role string) string {
	clean := func(s string) string {
		return strings.Trim(profileNameRe.ReplaceAllString(s, "-"), "-")
	}
	return clean(accountName) + "." + clean(role)
}

// UpsertProfile writes creds into the [profile] section of the credentials
// file at path. Other sections, comments and ordering are preserved; only
// the target section is replaced.
func UpsertProfile(path, profile string, creds Credentials) error {
	if profile == "" || strings.ContainsAny(profile, "[]\r\n") {
		return i18n.New("awsfiles.invalid_profile", profile)
	}
	if err := creds.Validate(); err != nil {
		return err
	}
	existing, err := os.ReadFile(path) // #nosec G304 -- ~/.aws/credentials or AWS_SHARED_CREDENTIALS_FILE
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	out := upsert(string(existing), profile, creds)
	return fsutil.WriteFileAtomic(path, []byte(out), 0o600)
}

func section(profile string, c Credentials) []string {
	lines := []string{
		"[" + profile + "]",
		"# Gerenciado pelo PickRole",
	}
	if !c.Expiration.IsZero() {
		lines[1] += " · expira em " + c.Expiration.UTC().Format(time.RFC3339)
	}
	return append(lines,
		"aws_access_key_id = "+c.AccessKeyID,
		"aws_secret_access_key = "+c.SecretAccessKey,
		"aws_session_token = "+c.SessionToken,
	)
}

func isHeader(line string) bool {
	t := strings.TrimSpace(line)
	return strings.HasPrefix(t, "[") && strings.HasSuffix(t, "]")
}

func upsert(content, profile string, creds Credentials) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	var lines []string
	if content != "" {
		lines = strings.Split(strings.TrimRight(content, "\n"), "\n")
	}
	header := "[" + profile + "]"
	block := section(profile, creds)

	start := -1
	for i, l := range lines {
		if strings.TrimSpace(l) == header {
			start = i
			break
		}
	}

	if start < 0 {
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, block...)
		return strings.Join(lines, "\n") + "\n"
	}

	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		if isHeader(lines[i]) {
			end = i
			break
		}
	}
	// Keep the blank lines that separated this section from the next one.
	tail := lines[end:]
	sep := []string{}
	if len(tail) > 0 {
		sep = []string{""}
	}
	result := append([]string{}, lines[:start]...)
	result = append(result, block...)
	result = append(result, sep...)
	result = append(result, tail...)
	return strings.Join(result, "\n") + "\n"
}

// ExportLines returns shell export statements for creds, for people who
// still want environment variables in one specific terminal.
func ExportLines(creds Credentials, region string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "export AWS_ACCESS_KEY_ID=%s\n", creds.AccessKeyID)
	fmt.Fprintf(&b, "export AWS_SECRET_ACCESS_KEY=%s\n", creds.SecretAccessKey)
	fmt.Fprintf(&b, "export AWS_SESSION_TOKEN=%s\n", creds.SessionToken)
	if region != "" {
		fmt.Fprintf(&b, "export AWS_REGION=%s\n", region)
	}
	return b.String()
}
