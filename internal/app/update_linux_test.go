package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

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

// syncPlatform records Emit and Quit from the background goroutines.
type syncPlatform struct {
	fakePlatform
	mu sync.Mutex
}

func (p *syncPlatform) Emit(event string, _ ...any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = append(p.events, event)
}

func (p *syncPlatform) Quit() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.quit = true
}

func (p *syncPlatform) state() ([]string, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Clone(p.events), p.quit
}

// replaceFile does what a package manager does to the executable.
func replaceFile(t *testing.T, path string) {
	t.Helper()
	tmp := path + ".new"
	if err := os.WriteFile(tmp, []byte("new version"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tmp, path); err != nil {
		t.Fatal(err)
	}
}

func waitFor(t *testing.T, what string, ok func() bool) {
	t.Helper()
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if ok() {
			return
		}
	}
	t.Fatal("timed out waiting for " + what)
}

func fastInstallCheck(t *testing.T) {
	prev := installCheckEvery
	installCheckEvery = 20 * time.Millisecond
	t.Cleanup(func() { installCheckEvery = prev })
}

// On a pbrun machine with a terminal, PickRole opens it running the install
// command, and restarts by itself once the new version is in place.
func TestApplyUpdateLinuxPbrunOpensATerminal(t *testing.T) {
	isolate(t)
	fastInstallCheck(t)
	bin := t.TempDir()
	argsFile := filepath.Join(bin, "terminal-args")
	if err := os.WriteFile(filepath.Join(bin, "gnome-terminal"), []byte("#!/bin/sh\nprintf '%s\n' \"$@\" > "+argsFile+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	exe := filepath.Join(t.TempDir(), "pickrole")
	if err := os.WriteFile(exe, []byte("old version"), 0o755); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	relaunched := ""
	prev := relaunch
	relaunch = func(path string) error { mu.Lock(); relaunched = path; mu.Unlock(); return nil }
	t.Cleanup(func() { relaunch = prev })

	name := "pickrole_0.2.0-beta.5_el8_x86_64.rpm"
	pkg := []byte("rpm contents")
	sum := sha256.Sum256(pkg)
	fakeReleases(t, "v0.2.0-beta.5", map[string][]byte{name: pkg, "SHA256SUMS": []byte(hex.EncodeToString(sum[:]) + "  " + name + "\n")})
	withInstallation(t, update.Installation{Format: update.FormatRPM, Variant: "el8", Exe: exe, Elevator: "pbrun"})

	p := &syncPlatform{}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	svc, start := New(Build{Version: "0.2.0-beta.4"})
	start(ctx, p)
	if _, err := svc.CheckUpdate(false); err != nil {
		t.Fatal(err)
	}
	res, err := svc.ApplyUpdate()
	if err != nil || !res.Terminal || !strings.HasPrefix(res.ManualCommand, "pbrun dnf install '") {
		t.Fatalf("result = %+v, %v", res, err)
	}
	var args []byte
	waitFor(t, "the terminal", func() bool { args, _ = os.ReadFile(argsFile); return len(args) > 0 })
	if want := "--\nsh\n-c\n" + res.ManualCommand + " || { echo; echo "; !strings.HasPrefix(string(args), want) {
		t.Errorf("terminal args = %q, want prefix %q", args, want)
	}

	replaceFile(t, exe)
	waitFor(t, "the restart", func() bool { _, quit := p.state(); return quit })
	mu.Lock()
	defer mu.Unlock()
	if relaunched != exe {
		t.Errorf("relaunched %q, want %q", relaunched, exe)
	}
}

// Without a terminal to open, the command is shown, as before.
func TestApplyUpdateLinuxPbrunWithoutTerminal(t *testing.T) {
	isolate(t)
	t.Setenv("PATH", t.TempDir())
	name := "pickrole_0.2.0-beta.5_el8_x86_64.rpm"
	pkg := []byte("rpm contents")
	sum := sha256.Sum256(pkg)
	fakeReleases(t, "v0.2.0-beta.5", map[string][]byte{name: pkg, "SHA256SUMS": []byte(hex.EncodeToString(sum[:]) + "  " + name + "\n")})
	withInstallation(t, update.Installation{Format: update.FormatRPM, Variant: "el8", Exe: "/usr/bin/pickrole", Elevator: "pbrun"})

	svc, start := New(Build{Version: "0.2.0-beta.4"})
	start(context.Background(), &fakePlatform{})
	if _, err := svc.CheckUpdate(false); err != nil {
		t.Fatal(err)
	}
	res, err := svc.ApplyUpdate()
	if err != nil || res.Terminal || !strings.HasPrefix(res.ManualCommand, "pbrun dnf install '") || res.Reason == "" {
		t.Errorf("result = %+v, %v", res, err)
	}
}

// A version installed by hand while PickRole is open: the UI is told, and
// PickRole doesn't restart on its own.
func TestInstalledWhileOpen(t *testing.T) {
	isolate(t)
	fastInstallCheck(t)
	exe := filepath.Join(t.TempDir(), "pickrole")
	if err := os.WriteFile(exe, []byte("old version"), 0o755); err != nil {
		t.Fatal(err)
	}
	withInstallation(t, update.Installation{Format: update.FormatRPM, Variant: "el8", Exe: exe})
	p := &syncPlatform{}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	_, start := New(Build{Version: "0.2.0-beta.4"})
	start(ctx, p)
	time.Sleep(100 * time.Millisecond)
	if events, _ := p.state(); slices.Contains(events, "update-installed") {
		t.Fatal("nothing was installed yet")
	}
	replaceFile(t, exe)
	waitFor(t, "the notice", func() bool { events, _ := p.state(); return slices.Contains(events, "update-installed") })
	if _, quit := p.state(); quit {
		t.Error("PickRole restarted without being asked")
	}
}
