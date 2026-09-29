package app

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
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
	// Security is true when the update includes a security fix: the notice
	// then uses the warning color (docs/adr/0025).
	Security bool `json:"security"`
	// TerminalInstall is true where privileges go through pbrun: PickRole
	// downloads and checks the package, and shows the command to install it.
	TerminalInstall bool `json:"terminalInstall"`
}

// UpdateResult is the result of ApplyUpdate that isn't a restart.
type UpdateResult struct {
	// ManualCommand installs the downloaded and verified package in a
	// terminal, when the automatic install wasn't possible.
	ManualCommand string `json:"manualCommand"`
	// Reason is why the automatic install didn't work.
	Reason string `json:"reason"`
	// Terminal is true when the install runs in a terminal PickRole opened
	// (pbrun); PickRole restarts when it's done. ManualCommand is still set,
	// in case the terminal didn't show up.
	Terminal bool `json:"terminal"`
}

// Replaced in tests, which must not touch the running executable.
var (
	newUpdateSource    = func() update.Source { return update.GitHub(awsenv.HTTPClient()) }
	detectInstallation = update.Detect
	relaunch           = update.Relaunch
)

// CheckUpdate looks for a release newer than this build on GitHub, through
// the configured proxy (docs/adr/0024). The periodic check (manual false)
// respects the preference; "Check for updates" in About (manual true) always
// checks. Local builds (version "dev") never do.
func (s *Service) CheckUpdate(manual bool) (UpdateInfo, error) {
	s.mu.Lock()
	on := s.cfg.Preferences.CheckUpdates
	ctx := s.ctx
	s.mu.Unlock()
	current, ok := update.ParseVersion(s.build.Version)
	if !on && !manual || !ok {
		return UpdateInfo{}, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	rel, err := newUpdateSource().Newer(ctx, current)
	if err != nil {
		return UpdateInfo{}, i18n.Wrap("update.check", explainUpdateError(err))
	}
	s.mu.Lock()
	s.newer = rel
	s.mu.Unlock()
	if rel == nil {
		return UpdateInfo{}, nil
	}
	inst := detectInstallation()
	return UpdateInfo{
		Available:       true,
		Version:         rel.Version.String(),
		URL:             rel.URL,
		CanInstall:      inst.Supported() && rel.AssetURL(inst.Asset(rel.Version)) != "",
		Security:        rel.Security,
		TerminalInstall: inst.TerminalInstall(),
	}, nil
}

// explainUpdateError says what a failed check means in the user's
// language, instead of a URL and a status code.
func explainUpdateError(err error) error {
	var status *update.HTTPError
	switch {
	case errors.As(err, &status) && status.RateLimited && !status.Reset.IsZero():
		return i18n.New("update.rate_limited_until", status.Reset.Local().Format("15:04"))
	case errors.As(err, &status) && status.RateLimited:
		return i18n.New("update.rate_limited")
	case errors.As(err, &status) && (status.Status == http.StatusForbidden || status.Status == http.StatusProxyAuthRequired):
		return i18n.New("update.refused", status.Host, status.Status)
	case errors.As(err, &status):
		return i18n.New("update.http_status", status.Host, status.Status)
	case errors.Is(err, context.DeadlineExceeded):
		return i18n.New("update.timeout")
	}
	return err
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
	inst.ClosePrompt = i18n.T("update.close_terminal")
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
			if errors.Is(err, update.ErrTerminalOpened) {
				s.mu.Lock()
				s.restartWhenInstalled = true
				s.mu.Unlock()
				return UpdateResult{ManualCommand: manual.Command, Terminal: true}, nil
			}
			reason := manual.Error()
			if errors.Is(err, update.ErrTerminalInstall) {
				reason = i18n.T("update.pbrun")
			}
			return UpdateResult{ManualCommand: manual.Command, Reason: reason}, nil
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

// installCheckEvery is how often watchInstall looks at the executable.
var installCheckEvery = 3 * time.Second

// watchInstall notices when a new version is installed while PickRole is
// open (from the terminal PickRole opened, or by hand): the running process
// is still the old one. After an update started here it restarts right
// away; otherwise the UI offers to restart (event "update-installed").
// Local builds (version "dev") aren't watched: rebuilding would count.
// Only Linux needs it: on Windows the update itself restarts PickRole, and
// the running .exe can't be overwritten.
func (s *Service) watchInstall(ctx context.Context) {
	if _, ok := update.ParseVersion(s.build.Version); !ok || runtime.GOOS != "linux" {
		return
	}
	exe := detectInstallation().Exe
	w := update.NewWatch(exe)
	if w == nil {
		return
	}
	t := time.NewTicker(installCheckEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		if !w.Replaced() {
			continue
		}
		// Let the package manager finish before starting the new version.
		time.Sleep(time.Second)
		s.mu.Lock()
		auto, p := s.restartWhenInstalled, s.platform
		s.mu.Unlock()
		if auto && s.RestartApp() == nil {
			return
		}
		if p != nil {
			p.Emit("update-installed")
		}
		return
	}
}

// RestartApp starts the installed version and quits this one: after an
// update installed while PickRole was open.
func (s *Service) RestartApp() error {
	s.mu.Lock()
	p := s.platform
	s.mu.Unlock()
	if err := relaunch(detectInstallation().Exe); err != nil {
		return i18n.Wrap("update.restart", err)
	}
	if p != nil {
		p.Quit()
	}
	return nil
}
