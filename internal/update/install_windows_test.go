package update

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

// The running .exe can't be overwritten, so Apply renames it to .old and
// puts the new one in its place.
func TestApplyReplacesExe(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "pickrole.exe")
	if err := os.WriteFile(exe, []byte("old version"), 0o755); err != nil {
		t.Fatal(err)
	}
	zipPath := filepath.Join(dir, "update.zip")
	f, _ := os.Create(zipPath)
	zw := zip.NewWriter(f)
	w, _ := zw.Create("pickrole.exe")
	_, _ = w.Write([]byte("new version"))
	_ = zw.Close()
	_ = f.Close()

	inst := Installation{Format: FormatZip, Exe: exe}
	if err := inst.Apply(zipPath); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(exe); string(got) != "new version" {
		t.Errorf("exe = %q", got)
	}
	if got, _ := os.ReadFile(exe + ".old"); string(got) != "old version" {
		t.Errorf("old = %q", got)
	}

	// A zip without pickrole.exe leaves everything as it was.
	empty := filepath.Join(dir, "empty.zip")
	f, _ = os.Create(empty)
	_ = zip.NewWriter(f).Close()
	_ = f.Close()
	if err := inst.Apply(empty); err == nil {
		t.Error("want an error for a zip without pickrole.exe")
	}
	if got, _ := os.ReadFile(exe); string(got) != "new version" {
		t.Errorf("exe changed on a failed update: %q", got)
	}
}

func TestProcessAlive(t *testing.T) {
	if !processAlive(os.Getpid()) {
		t.Error("this process should be alive")
	}
}
