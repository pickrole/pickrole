package app

import (
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"

	"github.com/pickrole/pickrole/internal/awsfiles"
	"github.com/pickrole/pickrole/internal/config"
	"github.com/pickrole/pickrole/internal/fsutil"
)

// Repository is where the source code lives.
const Repository = "https://github.com/pickrole/pickrole"

// Build identifies the binary. main.go fills it from -ldflags; fields left
// empty fall back to what the Go toolchain recorded, if anything.
type Build struct {
	Version string
	Commit  string
	Date    string // RFC 3339, UTC
}

// withVCS fills an empty commit from the VCS stamp that `go build` records
// when it runs inside a git checkout (as `wails dev` and `wails build` do).
func (b Build) withVCS() Build {
	if b.Commit != "" {
		return b
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return b
	}
	var modified bool
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			b.Commit = s.Value
		case "vcs.modified":
			modified = s.Value == "true"
		}
	}
	if len(b.Commit) > 7 {
		b.Commit = b.Commit[:7]
	}
	if b.Commit != "" && modified {
		b.Commit += "-modificado"
	}
	return b
}

// About is what the "Sobre" dialog shows: build details and where PickRole
// reads and writes, which is what a bug report usually needs.
type About struct {
	Version    string `json:"version"`
	Commit     string `json:"commit"`
	BuildDate  string `json:"buildDate"`
	Go         string `json:"go"`
	Platform   string `json:"platform"`
	Repository string `json:"repository"`
	Files      []File `json:"files"`
}

// File is a path PickRole uses, shown relative to the home directory. Kind
// is one of credentials, ssoCache, config or maven; the UI translates it.
type File struct {
	Kind string `json:"kind"`
	Path string `json:"path"`
}

// About returns the build details and the files PickRole uses.
func (s *Service) About() About {
	s.mu.Lock()
	cfg := s.cfg
	s.mu.Unlock()

	files := []File{{Kind: "credentials", Path: homeRelative(awsfiles.CredentialsPath())}}
	if home, err := os.UserHomeDir(); err == nil {
		files = append(files, File{Kind: "ssoCache", Path: homeRelative(filepath.Join(home, ".aws", "sso", "cache"))})
	}
	if path, err := config.Path(); err == nil {
		files = append(files, File{Kind: "config", Path: homeRelative(path)})
	}
	if m := cfg.CodeArtifact.Tools.Maven; cfg.CodeArtifact.Enabled && m.Enabled {
		files = append(files, File{Kind: "maven", Path: homeRelative(fsutil.ExpandHome(m.SettingsPath))})
	}

	return About{
		Version:    s.build.Version,
		Commit:     s.build.Commit,
		BuildDate:  s.build.Date,
		Go:         runtime.Version(),
		Platform:   runtime.GOOS + "/" + runtime.GOARCH,
		Repository: Repository,
		Files:      files,
	}
}
