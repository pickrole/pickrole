package maven

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type settingsDoc struct {
	Servers []struct {
		ID       string `xml:"id"`
		Username string `xml:"username"`
		Password string `xml:"password"`
	} `xml:"servers>server"`
}

func parse(t *testing.T, s string) settingsDoc {
	t.Helper()
	var d settingsDoc
	if err := xml.Unmarshal([]byte(s), &d); err != nil {
		t.Fatalf("result is not valid XML: %v\n%s", err, s)
	}
	return d
}

func TestUpsertNewFileContent(t *testing.T) {
	got, err := upsert(newSettings, "codeartifact", "tok&en<1>")
	if err != nil {
		t.Fatal(err)
	}
	d := parse(t, got)
	if len(d.Servers) != 1 || d.Servers[0].ID != "codeartifact" || d.Servers[0].Password != "tok&en<1>" || d.Servers[0].Username != "aws" {
		t.Errorf("unexpected servers: %+v\n%s", d.Servers, got)
	}
}

func TestUpsertReplacesExistingServerOnly(t *testing.T) {
	const before = `<?xml version="1.0"?>
<settings>
  <!-- keep this comment -->
  <servers>
    <server>
      <id>nexus</id>
      <username>me</username>
      <password>nexus-secret</password>
    </server>
    <server>
      <id>codeartifact</id>
      <username>aws</username>
      <password>${env.CODEARTIFACT_AUTH_TOKEN}</password>
    </server>
  </servers>
</settings>
`
	got, err := upsert(before, "codeartifact", "NEWTOKEN")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "<!-- keep this comment -->") || !strings.Contains(got, "nexus-secret") {
		t.Errorf("unrelated content changed:\n%s", got)
	}
	d := parse(t, got)
	if len(d.Servers) != 2 || d.Servers[1].Password != "NEWTOKEN" {
		t.Errorf("unexpected servers: %+v", d.Servers)
	}
}

func TestUpsertAddsServerToExistingServers(t *testing.T) {
	const before = "<settings>\n  <servers>\n    <server>\n      <id>nexus</id>\n    </server>\n  </servers>\n</settings>\n"
	got, err := upsert(before, "codeartifact", "T")
	if err != nil {
		t.Fatal(err)
	}
	d := parse(t, got)
	if len(d.Servers) != 2 || d.Servers[1].ID != "codeartifact" {
		t.Errorf("unexpected servers: %+v\n%s", d.Servers, got)
	}
}

func TestUpsertAddsServersElement(t *testing.T) {
	for _, before := range []string{
		"<settings>\n  <localRepository>/tmp/m2</localRepository>\n</settings>\n",
		"<settings>\n  <servers/>\n</settings>\n",
	} {
		got, err := upsert(before, "codeartifact", "T")
		if err != nil {
			t.Fatal(err)
		}
		d := parse(t, got)
		if len(d.Servers) != 1 || d.Servers[0].Password != "T" {
			t.Errorf("unexpected servers for %q: %+v\n%s", before, d.Servers, got)
		}
	}
}

func TestUpsertAddsMissingPassword(t *testing.T) {
	const before = "<settings>\n  <servers>\n    <server>\n      <id>codeartifact</id>\n    </server>\n  </servers>\n</settings>\n"
	got, err := upsert(before, "codeartifact", "T")
	if err != nil {
		t.Fatal(err)
	}
	d := parse(t, got)
	if len(d.Servers) != 1 || d.Servers[0].Password != "T" || d.Servers[0].Username != "aws" {
		t.Errorf("unexpected servers: %+v\n%s", d.Servers, got)
	}
}

func TestUpsertRejectsBrokenFile(t *testing.T) {
	if _, err := upsert("<settings>", "codeartifact", "T"); err != ErrInvalidSettings {
		t.Errorf("err = %v, want ErrInvalidSettings", err)
	}
}

// homeDir makes a temporary home, since UpsertServer only writes inside it.
func homeDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	return dir
}

func TestUpsertServerKeepsBackup(t *testing.T) {
	dir := homeDir(t)
	path := filepath.Join(dir, "settings.xml")
	original := "<settings>\n</settings>\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := UpsertServers(path, []string{"codeartifact"}, "T1"); err != nil {
		t.Fatal(err)
	}
	if err := UpsertServers(path, []string{"codeartifact"}, "T2"); err != nil {
		t.Fatal(err)
	}
	backup, err := os.ReadFile(path + ".pickrole.bak")
	if err != nil {
		t.Fatal(err)
	}
	if string(backup) != original {
		t.Errorf("backup should hold the original file, got:\n%s", backup)
	}
	info, _ := os.Stat(path)
	// The file now holds a token: never readable by others, even if it was.
	// Windows has no Unix permission bits.
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Errorf("permissions = %o, want 600", info.Mode().Perm())
	}
	got, _ := os.ReadFile(path)
	if d := parse(t, string(got)); len(d.Servers) != 1 || d.Servers[0].Password != "T2" {
		t.Errorf("unexpected servers: %+v", d.Servers)
	}
}

func TestUpsertServerRefusesPathOutsideHome(t *testing.T) {
	homeDir(t)
	outside := filepath.Join(t.TempDir(), "settings.xml")
	if err := UpsertServers(outside, []string{"codeartifact"}, "T"); err == nil {
		t.Fatal("writing outside the home should be refused")
	}
	if _, err := os.Stat(outside); !os.IsNotExist(err) {
		t.Error("nothing should be written outside the home")
	}
}

// A link planted where the backup goes must not be followed.
func TestUpsertServerDoesNotFollowBackupLink(t *testing.T) {
	dir := homeDir(t)
	path := filepath.Join(dir, "settings.xml")
	if err := os.WriteFile(path, []byte("<settings>\n<!-- old token: T0 -->\n</settings>\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "stolen")
	if err := os.Symlink(target, path+".pickrole.bak"); err != nil {
		t.Skipf("symlinks unavailable here: %v", err)
	}
	if err := UpsertServers(path, []string{"codeartifact"}, "T1"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Error("the backup followed the planted link")
	}
}

// A commented-out example with the same id must not get the token.
func TestUpsertIgnoresCommentedServer(t *testing.T) {
	in := `<settings>
  <!--
  <servers>
    <server>
      <id>codeartifact</id>
      <password>EXAMPLE</password>
    </server>
  </servers>
  -->
  <servers>
    <server>
      <id>codeartifact</id>
      <password>OLD</password>
    </server>
  </servers>
</settings>
`
	got, err := upsert(in, "codeartifact", "NEW")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "<password>EXAMPLE</password>") {
		t.Error("the commented example was changed")
	}
	if strings.Contains(got, "OLD") || !strings.Contains(got, "<password>NEW</password>") {
		t.Errorf("the real entry was not updated:\n%s", got)
	}

	// Only a commented </servers>: a real <servers> is added, not one inside the comment.
	got, err = upsert("<settings>\n  <!-- <servers></servers> -->\n</settings>\n", "codeartifact", "NEW")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "-->\n  <servers>") {
		t.Errorf("entry added inside the comment:\n%s", got)
	}
}
