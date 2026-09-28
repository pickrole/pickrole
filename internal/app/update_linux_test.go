package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pickrole/pickrole/internal/update"
)

// Without a desktop to ask for the password (as in CI), pkexec can't install:
// the package is still downloaded and checked, and the user gets the command.
func TestApplyUpdateLinuxFallsBackToCommand(t *testing.T) {
	isolate(t)
	t.Setenv("PATH", t.TempDir()) // no pkexec, no dnf
	name := "pickrole_0.2.0-beta.5_el8_x86_64.rpm"
	pkg := []byte("rpm contents")
	sum := sha256.Sum256(pkg)
	fakeReleases(t, "v0.2.0-beta.5", map[string][]byte{name: pkg, "SHA256SUMS": []byte(hex.EncodeToString(sum[:]) + "  " + name + "\n")})
	withInstallation(t, update.Installation{Format: update.FormatRPM, Variant: "el8", Exe: "/usr/bin/pickrole"})

	p := &fakePlatform{}
	svc, start := New(Build{Version: "0.2.0-beta.4"})
	start(context.Background(), p)
	if _, err := svc.CheckUpdate(false); err != nil {
		t.Fatal(err)
	}
	res, err := svc.ApplyUpdate()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(res.ManualCommand, "sudo dnf install '") || !strings.Contains(res.ManualCommand, name) {
		t.Errorf("manual command = %q", res.ManualCommand)
	}
	if p.quit {
		t.Error("PickRole must keep running when the install didn't happen")
	}
}

// Where administrator commands go through pbrun, pkexec is never tried: the
// package is downloaded and checked, and the user gets the pbrun command.
func TestApplyUpdateLinuxPbrun(t *testing.T) {
	isolate(t)
	bin := t.TempDir()
	marker := filepath.Join(bin, "pkexec-ran")
	if err := os.WriteFile(filepath.Join(bin, "pkexec"), []byte("#!/bin/sh\ntouch "+marker+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	name := "pickrole_0.2.0-beta.5_el8_x86_64.rpm"
	pkg := []byte("rpm contents")
	sum := sha256.Sum256(pkg)
	fakeReleases(t, "v0.2.0-beta.5", map[string][]byte{name: pkg, "SHA256SUMS": []byte(hex.EncodeToString(sum[:]) + "  " + name + "\n")})
	withInstallation(t, update.Installation{Format: update.FormatRPM, Variant: "el8", Exe: "/usr/bin/pickrole", Elevator: "pbrun"})

	svc, start := New(Build{Version: "0.2.0-beta.4"})
	start(context.Background(), &fakePlatform{})
	info, err := svc.CheckUpdate(false)
	if err != nil || !info.TerminalInstall {
		t.Fatalf("CheckUpdate = %+v, %v", info, err)
	}
	res, err := svc.ApplyUpdate()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(res.ManualCommand, "pbrun dnf install '") || res.Reason == "" {
		t.Errorf("result = %+v", res)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Error("pkexec was run on a pbrun machine")
	}
}
