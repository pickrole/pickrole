package app

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path"
	"testing"
	"time"

	"github.com/pickrole/pickrole/internal/i18n"
	"github.com/pickrole/pickrole/internal/update"
)

// fakeReleases serves a GitHub-like release list with one release, tag,
// whose assets are files, and points the updater at it.
func fakeReleases(t *testing.T, tag string, files map[string][]byte) {
	t.Helper()
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	src := update.Source{ReleasesURL: srv.URL + "/releases", DownloadPrefix: srv.URL + "/download/", HTTP: srv.Client()}
	var assets []map[string]string
	for name := range files {
		assets = append(assets, map[string]string{"name": name, "browser_download_url": src.DownloadPrefix + name})
	}
	mux.HandleFunc("/releases", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]any{{
			"tag_name": tag, "prerelease": true, "html_url": "https://github.com/pickrole/pickrole/releases/tag/" + tag, "assets": assets,
		}})
	})
	mux.HandleFunc("/download/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(files[path.Base(r.URL.Path)])
	})
	prev := newUpdateSource
	newUpdateSource = func() update.Source { return src }
	t.Cleanup(func() { newUpdateSource = prev })
}

func withInstallation(t *testing.T, inst update.Installation) {
	t.Helper()
	prev := detectInstallation
	detectInstallation = func() update.Installation { return inst }
	t.Cleanup(func() { detectInstallation = prev })
}

// zipped returns a .zip holding pickrole.exe with content, and SHA256SUMS
// for it under name.
func zipped(t *testing.T, name, content string) (zipData, sums []byte) {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("pickrole.exe")
	_, _ = w.Write([]byte(content))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(buf.Bytes())
	return buf.Bytes(), []byte(hex.EncodeToString(sum[:]) + "  " + name + "\n")
}

func TestCheckUpdate(t *testing.T) {
	isolate(t)
	name := "pickrole_0.2.0-beta.5_windows_amd64.zip"
	data, sums := zipped(t, name, "new")
	fakeReleases(t, "v0.2.0-beta.5", map[string][]byte{name: data, "SHA256SUMS": sums})
	withInstallation(t, update.Installation{Format: update.FormatZip})

	svc, start := New(Build{Version: "0.2.0-beta.4"})
	start(context.Background(), &fakePlatform{})
	info, err := svc.CheckUpdate(false)
	if err != nil {
		t.Fatal(err)
	}
	if !info.Available || info.Version != "0.2.0-beta.5" || !info.CanInstall || info.URL == "" {
		t.Errorf("unexpected: %+v", info)
	}

	// No package for this installation: the notice still shows, with the page.
	withInstallation(t, update.Installation{Format: update.FormatDeb})
	if info, _ := svc.CheckUpdate(false); !info.Available || info.CanInstall {
		t.Errorf("want a notice without install: %+v", info)
	}

	// Turned off, or a local build: no check at all.
	cfg := svc.DefaultConfig()
	cfg.SSO.StartURL = "https://example.awsapps.com/start"
	cfg.Preferences.CheckUpdates = false
	if _, err := svc.SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	if info, _ := svc.CheckUpdate(false); info.Available {
		t.Error("checked with the preference off")
	}
	// "Check for updates" in About still checks.
	if info, _ := svc.CheckUpdate(true); !info.Available {
		t.Error("a manual check should ignore the preference")
	}
	dev, startDev := New(Build{Version: "dev"})
	startDev(context.Background(), &fakePlatform{})
	if info, _ := dev.CheckUpdate(false); info.Available {
		t.Error("a local build should not check")
	}
}

// A failed check says what it means, not a URL and a status code.
func TestExplainUpdateError(t *testing.T) {
	i18n.SetLanguage(i18n.English)
	for err, key := range map[error]string{
		&update.HTTPError{Host: "api.github.com", Status: 403, RateLimited: true, Reset: time.Unix(1790000000, 0)}: "update.rate_limited_until",
		&update.HTTPError{Host: "api.github.com", Status: 429, RateLimited: true}:                                  "update.rate_limited",
		&update.HTTPError{Host: "api.github.com", Status: 403}:                                                     "update.refused",
		&update.HTTPError{Host: "api.github.com", Status: 502}:                                                     "update.http_status",
		fmt.Errorf("get: %w", context.DeadlineExceeded):                                                            "update.timeout",
	} {
		var got *i18n.Error
		if !errors.As(explainUpdateError(err), &got) || got.Key() != key {
			t.Errorf("%v: want %s, got %v", err, key, explainUpdateError(err))
		}
	}
	if err := errors.New("other"); explainUpdateError(err) != err {
		t.Error("other errors should pass through")
	}
}
