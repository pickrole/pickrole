package update

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWatchNoticesAReplacedFile(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "pickrole")
	if err := os.WriteFile(exe, []byte("old"), 0o700); err != nil {
		t.Fatal(err)
	}
	w := NewWatch(exe)
	if w.Replaced() {
		t.Fatal("nothing changed yet")
	}
	// Like a package manager: a new file renamed over the old one.
	tmp := filepath.Join(dir, "pickrole.new")
	if err := os.WriteFile(tmp, []byte("new version"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(tmp, time.Now(), time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tmp, exe); err != nil {
		t.Fatal(err)
	}
	if !w.Replaced() {
		t.Error("the replacement went unnoticed")
	}

	var none *Watch
	if none.Replaced() || NewWatch("") != nil || NewWatch(filepath.Join(dir, "missing")) != nil {
		t.Error("nothing to watch should never report a replacement")
	}
}
