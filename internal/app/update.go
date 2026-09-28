package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/pickrole/pickrole/internal/awsenv"
	"github.com/pickrole/pickrole/internal/i18n"
	"github.com/pickrole/pickrole/internal/update"
)

// UpdateInfo is the result of CheckUpdate.
type UpdateInfo struct {
	// Available is true when a newer release exists.
	Available bool   `json:"available"`
	Version   string `json:"version"`
	// URL is the release page, with the notes.
	URL string `json:"url"`
	// CanInstall is true when PickRole can install it itself (the Windows
	// .exe, or the RPM/.deb in /usr/bin); otherwise the UI links to the page.
	CanInstall bool `json:"canInstall"`
}

// UpdateResult is the result of ApplyUpdate that isn't a restart.
type UpdateResult struct {
	// ManualCommand installs the downloaded and verified package in a
	// terminal, when the automatic install wasn't possible.
	ManualCommand string `json:"manualCommand"`
	// Reason is why the automatic install didn't work.
	Reason string `json:"reason"`
}

// Replaced in tests, which must not touch the running executable.
var (
	newUpdateSource    = func() update.Source { return update.GitHub(awsenv.HTTPClient()) }
	detectInstallation = update.Detect
	relaunch           = update.Relaunch
)

// CheckUpdate looks for a release newer than this build on GitHub, through
// the configured proxy (docs/adr/0024). Local builds (version "dev") and a
// disabled preference never check.
func (s *Service) CheckUpdate() (UpdateInfo, error) {
	s.mu.Lock()
	on := s.cfg.Preferences.CheckUpdates
	ctx := s.ctx
	s.mu.Unlock()
	current, ok := update.ParseVersion(s.build.Version)
	if !on || !ok {
		return UpdateInfo{}, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	rel, err := newUpdateSource().Newer(ctx, current)
	if err != nil {
		return UpdateInfo{}, i18n.Wrap("update.check", err)
	}
	s.mu.Lock()
	s.newer = rel
	s.mu.Unlock()
	if rel == nil {
		return UpdateInfo{}, nil
	}
	inst := detectInstallation()
	return UpdateInfo{
		Available:  true,
		Version:    rel.Version.String(),
		URL:        rel.URL,
		CanInstall: inst.Supported() && rel.Assets[inst.Asset(rel.Version)] != "",
	}, nil
}

// ApplyUpdate downloads the release found by CheckUpdate for this
// installation, checks it against the release's SHA256SUMS, installs it and
// restarts PickRole. When the install can't be done automatically (Linux
// without pkexec, or not allowed), it returns the command to run instead.
func (s *Service) ApplyUpdate() (UpdateResult, error) {
	s.mu.Lock()
	rel, ctx, p := s.newer, s.ctx, s.platform
	s.mu.Unlock()
	if rel == nil {
		return UpdateResult{}, i18n.New("update.none")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	inst := detectInstallation()
	if !inst.Supported() {
		return UpdateResult{}, i18n.New("update.none")
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return UpdateResult{}, i18n.Wrap("update.download", err)
	}
	dctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	pkg, err := newUpdateSource().Download(dctx, rel, inst.Asset(rel.Version), filepath.Join(cache, "pickrole", "updates"))
	if err != nil {
		return UpdateResult{}, i18n.Wrap("update.download", err)
	}

	if err := inst.Apply(pkg); err != nil {
		var manual *update.ManualError
		if errors.As(err, &manual) {
			return UpdateResult{ManualCommand: manual.Command, Reason: manual.Error()}, nil
		}
		return UpdateResult{}, i18n.Wrap("update.install", err)
	}
	if err := relaunch(inst.Exe); err != nil {
		return UpdateResult{}, i18n.Wrap("update.install", err)
	}
	if p != nil {
		p.Quit()
	}
	return UpdateResult{}, nil
}
