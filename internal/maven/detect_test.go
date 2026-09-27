package maven

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// A settings.xml like the ones the AWS instructions produce: several
// CodeArtifact servers reading ${env.CODEARTIFACT_AUTH_TOKEN}, one per
// repository in several profiles, next to servers of other repositories.
const manyServers = `<?xml version="1.0" encoding="UTF-8"?>
<settings>
  <servers>
    <server>
      <id>ca-releases</id>
      <username>aws</username>
      <password>${env.CODEARTIFACT_AUTH_TOKEN}</password>
    </server>
    <server>
      <id>nexus</id>
      <username>me</username>
      <password>nexus-secret</password>
    </server>
    <server>
      <id>ca-snapshots</id>
      <username>aws</username>
      <password>${env.CODEARTIFACT_AUTH_TOKEN}</password>
    </server>
    <!--
    <server>
      <id>ca-old</id>
      <password>${env.CODEARTIFACT_AUTH_TOKEN}</password>
    </server>
    -->
  </servers>
  <mirrors>
    <mirror>
      <id>ca-mirror</id>
      <mirrorOf>*</mirrorOf>
      <url>https://my-org-111122223333.d.codeartifact.us-east-1.amazonaws.com/maven/central/</url>
    </mirror>
  </mirrors>
  <profiles>
    <profile>
      <id>team-a</id>
      <repositories>
        <repository>
          <id>ca-releases</id>
          <url>https://my-org-111122223333.d.codeartifact.us-east-1.amazonaws.com/maven/releases/</url>
        </repository>
        <repository>
          <id>nexus</id>
          <url>https://nexus.example.com/repository/maven-public/</url>
        </repository>
      </repositories>
      <pluginRepositories>
        <pluginRepository>
          <id>ca-plugins</id>
          <url>https://my-org-111122223333.d.codeartifact.us-east-1.amazonaws.com/maven/plugins/</url>
        </pluginRepository>
      </pluginRepositories>
    </profile>
    <profile>
      <id>other-domain</id>
      <repositories>
        <repository>
          <id>other-ca</id>
          <url>https://other-444455556666.d.codeartifact.sa-east-1.amazonaws.com/maven/libs/</url>
        </repository>
      </repositories>
    </profile>
  </profiles>
</settings>
`

func TestDetect(t *testing.T) {
	det := Detect(manyServers, Domain{})

	// In file order; ca-releases appears as a server and as a repository,
	// once; ca-old is commented out; nexus is not CodeArtifact.
	want := []string{"ca-releases", "ca-snapshots", "ca-mirror", "ca-plugins", "other-ca"}
	if !slices.Equal(det.ServerIDs, want) {
		t.Errorf("server ids = %v, want %v", det.ServerIDs, want)
	}
	wantDomains := []Domain{
		{Domain: "my-org", Owner: "111122223333", Region: "us-east-1"},
		{Domain: "other", Owner: "444455556666", Region: "sa-east-1"},
	}
	if !slices.Equal(det.Domains, wantDomains) {
		t.Errorf("domains = %+v, want %+v", det.Domains, wantDomains)
	}

	// With the configured domain, repositories of other domains are left
	// out: their token would be a different one.
	det = Detect(manyServers, Domain{Domain: "my-org", Owner: "111122223333", Region: "us-east-1"})
	want = []string{"ca-releases", "ca-snapshots", "ca-mirror", "ca-plugins"}
	if !slices.Equal(det.ServerIDs, want) {
		t.Errorf("server ids for my-org = %v, want %v", det.ServerIDs, want)
	}
}

func TestDetectNothing(t *testing.T) {
	det := Detect("<settings>\n</settings>\n", Domain{})
	if len(det.ServerIDs) != 0 || len(det.Domains) != 0 {
		t.Errorf("want nothing, got %+v", det)
	}
}

// Every detected server gets the token in one write, one backup, and the
// other servers keep their passwords.
func TestUpsertServersUpdatesAll(t *testing.T) {
	dir := homeDir(t)
	path := filepath.Join(dir, "settings.xml")
	if err := os.WriteFile(path, []byte(manyServers), 0o600); err != nil {
		t.Fatal(err)
	}
	ids := Detect(manyServers, Domain{Domain: "my-org"}).ServerIDs
	if err := UpsertServers(path, ids, "TOKEN"); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	d := parse(t, string(got))
	passwords := map[string]string{}
	for _, s := range d.Servers {
		passwords[s.ID] = s.Password
	}
	for _, id := range ids {
		if passwords[id] != "TOKEN" {
			t.Errorf("server %s: password %q, want the token", id, passwords[id])
		}
	}
	if passwords["nexus"] != "nexus-secret" {
		t.Errorf("nexus password changed to %q", passwords["nexus"])
	}
	if strings.Count(string(got), "${env.CODEARTIFACT_AUTH_TOKEN}") != 1 {
		t.Errorf("only the commented-out server should keep the variable:\n%s", got)
	}
	backup, err := os.ReadFile(path + ".pickrole.bak")
	if err != nil || string(backup) != manyServers {
		t.Errorf("backup should hold the original file (err %v)", err)
	}
}
