package app

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path"
	"testing"

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
	info, err := svc.CheckUpdate()
	if err != nil {
		t.Fatal(err)
	}
	if !info.Available || info.Version != "0.2.0-beta.5" || !info.CanInstall || info.URL == "" {
		t.Errorf("unexpected: %+v", info)
	}

	// No package for this installation: the notice still shows, with the page.
	withInstallation(t, update.Installation{Format: update.FormatDeb})
	if info, _ := svc.CheckUpdate(); !info.Available || info.CanInstall {
		t.Errorf("want a notice without install: %+v", info)
	}

	// Turned off, or a local build: no check at all.
	cfg := svc.DefaultConfig()
	cfg.SSO.StartURL = "https://example.awsapps.com/start"
	cfg.Preferences.CheckUpdates = false
	if _, err := svc.SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	if info, _ := svc.CheckUpdate(); info.Available {
		t.Error("checked with the preference off")
	}
	dev, startDev := New(Build{Version: "dev"})
	startDev(context.Background(), &fakePlatform{})
	if info, _ := dev.CheckUpdate(); info.Available {
		t.Error("a local build should not check")
	}
}
