package config

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// AWSConfigPath returns the AWS CLI config file location, honouring
// AWS_CONFIG_FILE.
func AWSConfigPath() string {
	if p := os.Getenv("AWS_CONFIG_FILE"); p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".aws", "config")
}

// DetectSSO looks for an existing SSO setup in the AWS CLI config, so a new
// user who already ran `aws configure sso` does not have to type anything.
// It prefers an [sso-session] block and falls back to a legacy profile with
// sso_start_url.
func DetectSSO(path string) (SSO, bool) {
	f, err := os.Open(path) // #nosec G304 -- ~/.aws/config or AWS_CONFIG_FILE, read only
	if err != nil {
		return SSO{}, false
	}
	defer f.Close()
	return detectSSO(f)
}

func detectSSO(r io.Reader) (SSO, bool) {
	type block struct {
		name string
		keys map[string]string
	}
	var blocks []*block
	var cur *block

	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			cur = &block{name: strings.TrimSpace(line[1 : len(line)-1]), keys: map[string]string{}}
			blocks = append(blocks, cur)
			continue
		}
		if cur == nil {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		cur.keys[strings.ToLower(strings.TrimSpace(k))] = strings.TrimSpace(v)
	}

	for _, b := range blocks {
		if name, ok := strings.CutPrefix(b.name, "sso-session "); ok {
			if b.keys["sso_start_url"] != "" && b.keys["sso_region"] != "" {
				return SSO{
					StartURL:    b.keys["sso_start_url"],
					Region:      b.keys["sso_region"],
					SessionName: strings.TrimSpace(name),
				}, true
			}
		}
	}
	for _, b := range blocks {
		if b.keys["sso_start_url"] != "" && b.keys["sso_region"] != "" {
			return SSO{
				StartURL:    b.keys["sso_start_url"],
				Region:      b.keys["sso_region"],
				SessionName: "pickrole",
			}, true
		}
	}
	return SSO{}, false
}
