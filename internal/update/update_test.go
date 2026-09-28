package update

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

func mustVersion(t *testing.T, s string) Version {
	t.Helper()
	v, ok := ParseVersion(s)
	if !ok {
		t.Fatalf("ParseVersion(%q) failed", s)
	}
	return v
}

func TestParseVersion(t *testing.T) {
	for s, ok := range map[string]bool{
		"0.2.0": true, "v0.2.0-beta.4": true, "1.10.3-rc.1+build.5": true,
		"dev": false, "a1b2c3d": false, "0.2": false, "01.2.3": false, "0.2.0-": false, "0.2.0-beta..1": false,
	} {
		if _, got := ParseVersion(s); got != ok {
			t.Errorf("ParseVersion(%q) ok = %v, want %v", s, got, ok)
		}
	}
	if v := mustVersion(t, "v0.2.0-beta.4"); v.String() != "0.2.0-beta.4" || !v.Prerelease() {
		t.Errorf("got %v", v)
	}
}

func TestCompare(t *testing.T) {
	// Each is older than the next, as in the semver spec.
	order := []string{"0.1.0", "0.2.0-alpha", "0.2.0-alpha.1", "0.2.0-beta.4", "0.2.0-beta.10", "0.2.0-rc.1", "0.2.0", "0.2.1", "0.10.0", "1.0.0"}
	for i := 0; i+1 < len(order); i++ {
		a, b := mustVersion(t, order[i]), mustVersion(t, order[i+1])
		if a.Compare(b) != -1 || b.Compare(a) != 1 || a.Compare(a) != 0 {
			t.Errorf("%s vs %s: %d, %d", a, b, a.Compare(b), b.Compare(a))
		}
	}
}

// fakeGitHub serves a release list and the assets of each release.
func fakeGitHub(t *testing.T, releases []map[string]any, files map[string][]byte) Source {
	t.Helper()
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	src := Source{
		ReleasesURL:    srv.URL + "/api/releases",
		DownloadPrefix: srv.URL + "/download/",
		HTTP:           srv.Client(),
	}
	for _, r := range releases {
		var assets []map[string]string
		for name := range files {
			assets = append(assets, map[string]string{"name": name, "browser_download_url": src.DownloadPrefix + r["tag_name"].(string) + "/" + name})
		}
		r["assets"] = assets
		r["html_url"] = "https://github.com/pickrole/pickrole/releases/tag/" + r["tag_name"].(string)
	}
	mux.HandleFunc("/api/releases", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("GitHub needs a User-Agent")
		}
		_ = json.NewEncoder(w).Encode(releases)
	})
	mux.HandleFunc("/download/", func(w http.ResponseWriter, r *http.Request) {
		data, ok := files[filepath.Base(r.URL.Path)]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(data)
	})
	return src
}

func TestNewer(t *testing.T) {
	releases := []map[string]any{
		{"tag_name": "v0.2.0-beta.4", "prerelease": true},
		{"tag_name": "v0.2.0-beta.5", "prerelease": true},
		{"tag_name": "v0.3.0-beta.1", "prerelease": true, "draft": true},
		{"tag_name": "v0.1.0"},
		{"tag_name": "not-a-version"},
	}
	src := fakeGitHub(t, releases, nil)
	ctx := context.Background()

	// On a beta: the newest release, betas included; drafts never.
	rel, err := src.Newer(ctx, mustVersion(t, "0.2.0-beta.4"))
	if err != nil {
		t.Fatal(err)
	}
	if rel == nil || rel.Version.String() != "0.2.0-beta.5" || rel.URL == "" {
		t.Errorf("want 0.2.0-beta.5, got %+v", rel)
	}

	// On the newest: nothing.
	if rel, _ := src.Newer(ctx, mustVersion(t, "0.2.0-beta.5")); rel != nil {
		t.Errorf("want nothing, got %s", rel.Version)
	}

	// On a final release: only final releases, so the betas don't count.
	if rel, _ := src.Newer(ctx, mustVersion(t, "0.0.9")); rel == nil || rel.Version.String() != "0.1.0" {
		t.Errorf("a final release should only be offered final releases, got %+v", rel)
	}
}

func TestDownloadChecksAndRefuses(t *testing.T) {
	pkg := []byte("package contents")
	sum := sha256.Sum256(pkg)
	name := "pickrole_0.2.0-beta.5_el8_x86_64.rpm"
	files := map[string][]byte{
		name:         pkg,
		"SHA256SUMS": []byte(hex.EncodeToString(sum[:]) + "  " + name + "\n" + "00  other\n"),
	}
	src := fakeGitHub(t, []map[string]any{{"tag_name": "v0.2.0-beta.5", "prerelease": true}}, files)
	ctx := context.Background()
	rel, err := src.Newer(ctx, mustVersion(t, "0.2.0-beta.4"))
	if err != nil || rel == nil {
		t.Fatalf("Newer: %v, %v", rel, err)
	}

	dir := t.TempDir()
	path, err := src.Download(ctx, rel, name, dir)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); string(got) != string(pkg) {
		t.Errorf("saved %q", got)
	}

	// A file that doesn't match SHA256SUMS is not kept.
	files[name] = []byte("tampered")
	os.Remove(path)
	if _, err := src.Download(ctx, rel, name, dir); !errors.Is(err, ErrChecksum) {
		t.Errorf("want ErrChecksum, got %v", err)
	}
	if _, err := os.Stat(path); err == nil {
		t.Error("a file that failed the check was kept")
	}

	// Nothing is downloaded from outside the project's release downloads.
	rel.Assets[name] = "https://example.com/evil.rpm"
	if _, err := src.Download(ctx, rel, name, dir); err == nil {
		t.Error("downloaded from another host")
	}
}

func TestAssetAndManualCommand(t *testing.T) {
	v := mustVersion(t, "0.2.0-beta.5")
	for _, c := range []struct {
		inst Installation
		want string
	}{
		{Installation{Format: FormatZip}, "pickrole_0.2.0-beta.5_windows_amd64.zip"},
		{Installation{Format: FormatRPM, Variant: "el8"}, "pickrole_0.2.0-beta.5_el8_x86_64.rpm"},
		{Installation{Format: FormatRPM, Variant: "webkit41"}, "pickrole_0.2.0-beta.5_fedora_x86_64.rpm"},
		{Installation{Format: FormatDeb, Variant: "webkit41"}, "pickrole_0.2.0-beta.5_amd64.deb"},
		{Installation{}, ""},
	} {
		if got := c.inst.Asset(v); got != c.want {
			t.Errorf("%+v: asset %q, want %q", c.inst, got, c.want)
		}
	}
	rpm := Installation{Format: FormatRPM, Variant: "el8"}
	if got := rpm.ManualCommand("/home/me/.cache/it's.rpm"); got != `sudo dnf install '/home/me/.cache/it'\''s.rpm'` {
		t.Errorf("manual command: %s", got)
	}
	pbrun := Installation{Format: FormatRPM, Variant: "el8", Elevator: "pbrun"}
	if got := pbrun.ManualCommand("/x.rpm"); got != "pbrun dnf install '/x.rpm'" || !pbrun.TerminalInstall() || rpm.TerminalInstall() {
		t.Errorf("pbrun manual command: %s", got)
	}
	if got := rpm.installCommand("/x.rpm"); len(got) != 5 || got[0] != "pkexec" || got[1] != "dnf" {
		t.Errorf("install command: %v", got)
	}
}

// GitHub answers downloads with a redirect to its storage, and the client
// that carries the proxy doesn't follow redirects: get follows them, to the
// allowed hosts only.
func TestDownloadFollowsAllowedRedirects(t *testing.T) {
	pkg := []byte("zip contents")
	sum := sha256.Sum256(pkg)
	name := "pickrole_0.2.0-beta.5_windows_amd64.zip"
	sums := []byte(hex.EncodeToString(sum[:]) + "  " + name + "\n")

	mux := http.NewServeMux()
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()
	mux.HandleFunc("/download/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/storage/"+filepath.Base(r.URL.Path), http.StatusFound)
	})
	mux.HandleFunc("/storage/", func(w http.ResponseWriter, r *http.Request) {
		switch filepath.Base(r.URL.Path) {
		case name:
			_, _ = w.Write(pkg)
		case "SHA256SUMS":
			_, _ = w.Write(sums)
		}
	})
	client := srv.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	u, _ := url.Parse(srv.URL)
	src := Source{ReleasesURL: srv.URL + "/api", DownloadPrefix: srv.URL + "/download/", RedirectHosts: []string{u.Hostname()}, HTTP: client}
	rel := &Release{Tag: "v0.2.0-beta.5", Assets: map[string]string{
		name:         src.DownloadPrefix + name,
		"SHA256SUMS": src.DownloadPrefix + "SHA256SUMS",
	}}
	if _, err := src.Download(context.Background(), rel, name, t.TempDir()); err != nil {
		t.Fatalf("redirect to an allowed host: %v", err)
	}

	src.RedirectHosts = []string{"objects.githubusercontent.com"}
	if _, err := src.Download(context.Background(), rel, name, t.TempDir()); err == nil {
		t.Error("followed a redirect to a host that isn't allowed")
	}
}

// A security fix in any release between the running version and the newest
// marks the update, so skipping versions doesn't hide it; one in a release
// already installed doesn't.
func TestNewerSecurity(t *testing.T) {
	releases := []map[string]any{
		{"tag_name": "v0.2.0-beta.3", "prerelease": true, "body": "### Security\n\n- An old fix."},
		{"tag_name": "v0.2.0-beta.5", "prerelease": true, "body": "### Fixed\n\n- Something."},
		{"tag_name": "v0.2.0-beta.6", "prerelease": true, "body": "Intro.\r\n\r\n### Security\r\n\r\n- Built with a patched Go."},
		{"tag_name": "v0.2.0-beta.7", "prerelease": true, "body": "### Added\n\n- A feature. Mentions security in passing."},
	}
	src := fakeGitHub(t, releases, nil)
	ctx := context.Background()

	rel, err := src.Newer(ctx, mustVersion(t, "0.2.0-beta.5"))
	if err != nil || rel == nil || rel.Version.String() != "0.2.0-beta.7" || !rel.Security {
		t.Errorf("from beta.5: want beta.7 with a security fix on the way, got %+v, %v", rel, err)
	}
	if rel, _ := src.Newer(ctx, mustVersion(t, "0.2.0-beta.6")); rel == nil || rel.Security {
		t.Errorf("from beta.6: the fix is already installed, got %+v", rel)
	}
}

// When the API refuses (GitHub's rate limit for a shared company address,
// or a proxy that only lets github.com through), the release feed is used,
// and the files are downloaded from where every release keeps them.
func TestNewerFallsBackToTheFeed(t *testing.T) {
	pkg := []byte("package contents")
	sum := sha256.Sum256(pkg)
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	src := Source{
		ReleasesURL:    srv.URL + "/api/releases",
		FeedURL:        srv.URL + "/releases.atom",
		DownloadPrefix: srv.URL + "/download/",
		HTTP:           srv.Client(),
	}
	mux.HandleFunc("/api/releases", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.Header().Set("X-RateLimit-Reset", "1790000000")
		w.WriteHeader(http.StatusForbidden)
	})
	mux.HandleFunc("/releases.atom", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <entry><link rel="alternate" type="text/html" href="https://github.com/pickrole/pickrole/releases/tag/v0.2.0-beta.8"/>
    <content type="html">&lt;h3&gt;Fixed&lt;/h3&gt;</content></entry>
  <entry><link rel="alternate" type="text/html" href="https://github.com/pickrole/pickrole/releases/tag/v0.2.0-beta.7"/>
    <content type="html">&lt;h3&gt;Security&lt;/h3&gt;&lt;ul&gt;&lt;li&gt;A fix.&lt;/li&gt;&lt;/ul&gt;</content></entry>
  <entry><link rel="alternate" type="text/html" href="https://github.com/pickrole/pickrole/releases/tag/v0.3.0"/>
    <content type="html">&lt;p&gt;A final release.&lt;/p&gt;</content></entry>
  <entry><link rel="alternate" type="text/html" href="https://example.com/elsewhere"/></entry>
</feed>`)
	})
	mux.HandleFunc("/download/v0.3.0/", func(w http.ResponseWriter, r *http.Request) {
		switch filepath.Base(r.URL.Path) {
		case "SHA256SUMS":
			_, _ = io.WriteString(w, hex.EncodeToString(sum[:])+"  pickrole.zip\n")
		case "pickrole.zip":
			_, _ = w.Write(pkg)
		default:
			http.NotFound(w, r)
		}
	})
	ctx := context.Background()

	rel, err := src.Newer(ctx, mustVersion(t, "0.2.0-beta.6"))
	if err != nil || rel == nil || rel.Version.String() != "0.3.0" || !rel.Security {
		t.Fatalf("want 0.3.0 with a security fix on the way, got %+v, %v", rel, err)
	}
	if rel, _ := src.Newer(ctx, mustVersion(t, "0.2.0")); rel == nil || rel.Version.String() != "0.3.0" || rel.Security {
		t.Errorf("a final release should only be offered final releases, got %+v", rel)
	}
	if got := rel.AssetURL("pickrole.zip"); got != src.DownloadPrefix+"v0.3.0/pickrole.zip" {
		t.Errorf("asset URL: %s", got)
	}
	path, err := src.Download(ctx, rel, "pickrole.zip", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(path); !bytes.Equal(data, pkg) {
		t.Error("wrong file")
	}

	// Without the feed, the API's answer is reported as it is.
	src.FeedURL = ""
	_, err = src.Newer(ctx, mustVersion(t, "0.2.0-beta.6"))
	var status *HTTPError
	if !errors.As(err, &status) || status.Status != http.StatusForbidden || !status.RateLimited || status.Reset.Unix() != 1790000000 {
		t.Errorf("want the rate limit, got %v", err)
	}
}
