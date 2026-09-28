package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/pickrole/pickrole/internal/update"
)

// The whole update on Windows: download, check against SHA256SUMS, replace
// the .exe, start the new one and quit.
func TestApplyUpdateWindows(t *testing.T) {
	isolate(t)
	name := "pickrole_0.2.0-beta.5_windows_amd64.zip"
	data, sums := zipped(t, name, "new version")
	fakeReleases(t, "v0.2.0-beta.5", map[string][]byte{name: data, "SHA256SUMS": sums})
	exe := filepath.Join(t.TempDir(), "pickrole.exe")
	if err := os.WriteFile(exe, []byte("old version"), 0o755); err != nil {
		t.Fatal(err)
	}
	withInstallation(t, update.Installation{Format: update.FormatZip, Exe: exe})
	var relaunched string
	prev := relaunch
	relaunch = func(path string) error { relaunched = path; return nil }
	t.Cleanup(func() { relaunch = prev })

	p := &fakePlatform{}
	svc, start := New(Build{Version: "0.2.0-beta.4"})
	start(context.Background(), p)
	if _, err := svc.ApplyUpdate(); err == nil {
		t.Error("ApplyUpdate before CheckUpdate should fail")
	}
	if _, err := svc.CheckUpdate(false); err != nil {
		t.Fatal(err)
	}
	res, err := svc.ApplyUpdate()
	if err != nil || res.ManualCommand != "" {
		t.Fatalf("ApplyUpdate = %+v, %v", res, err)
	}
	if got, _ := os.ReadFile(exe); string(got) != "new version" {
		t.Errorf("exe = %q", got)
	}
	if relaunched != exe || !p.quit {
		t.Errorf("relaunched %q, quit %v", relaunched, p.quit)
	}
}
