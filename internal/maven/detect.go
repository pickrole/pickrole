package maven

import (
	"regexp"
	"slices"
	"strings"
)

// Domain is a CodeArtifact domain as it appears in a repository URL:
// https://<domain>-<owner>.d.codeartifact.<region>.amazonaws.com/maven/<repository>/
type Domain struct {
	Domain string `json:"domain"`
	Owner  string `json:"owner"`
	Region string `json:"region"`
}

// Detection is what Detect found in a settings.xml.
type Detection struct {
	// ServerIDs are the ids that should receive the CodeArtifact token, in
	// the order they appear in the file.
	ServerIDs []string `json:"serverIds"`
	// Domains are the CodeArtifact domains the repositories and mirrors
	// point to, in the order they appear.
	Domains []Domain `json:"domains"`
}

var (
	repoBlockRes = []*regexp.Regexp{
		regexp.MustCompile(`(?s)<repository>.*?</repository>`),
		regexp.MustCompile(`(?s)<pluginRepository>.*?</pluginRepository>`),
		regexp.MustCompile(`(?s)<mirror>.*?</mirror>`),
	}
	idRe  = regexp.MustCompile(`(?s)<id>\s*(.*?)\s*</id>`)
	urlRe = regexp.MustCompile(`(?s)<url>\s*(.*?)\s*</url>`)
	// The domain can contain hyphens; the owner is always the 12 digits
	// after the last one.
	codeArtifactHostRe = regexp.MustCompile(`^https?://([a-z0-9-]+)-(\d{12})\.d\.codeartifact\.([a-z0-9-]+)\.amazonaws\.com(?:[:/]|$)`)
	// A password that reads the token from the environment, as in the
	// AWS instructions: ${env.CODEARTIFACT_AUTH_TOKEN}.
	envTokenRe = regexp.MustCompile(`(?is)<password>\s*\$\{env\.[A-Z0-9_]*CODEARTIFACT[A-Z0-9_]*\}\s*</password>`)
)

// Detect finds the <server> ids that need the CodeArtifact token in the
// content of a settings.xml:
//
//   - the id of every <repository>, <pluginRepository> and <mirror> whose URL
//     is a CodeArtifact repository (only of domain d, when d is set);
//   - the id of every <server> whose password reads the token from an
//     environment variable, such as ${env.CODEARTIFACT_AUTH_TOKEN}.
//
// Entries inside comments are ignored.
func Detect(content string, d Domain) Detection {
	det := Detection{ServerIDs: []string{}, Domains: []Domain{}}
	comments := commentRe.FindAllStringIndex(content, -1)
	add := func(id string) {
		if id != "" && !slices.Contains(det.ServerIDs, id) {
			det.ServerIDs = append(det.ServerIDs, id)
		}
	}

	type found struct {
		at int
		id string
	}
	var ids []found
	for _, re := range repoBlockRes {
		for _, loc := range re.FindAllStringIndex(content, -1) {
			if inComment(comments, loc[0]) {
				continue
			}
			block := content[loc[0]:loc[1]]
			u := urlRe.FindStringSubmatch(block)
			if u == nil {
				continue
			}
			m := codeArtifactHostRe.FindStringSubmatch(strings.ToLower(u[1]))
			if m == nil {
				continue
			}
			dom := Domain{Domain: m[1], Owner: m[2], Region: m[3]}
			if !slices.Contains(det.Domains, dom) {
				det.Domains = append(det.Domains, dom)
			}
			if d.Domain != "" && !strings.EqualFold(dom.Domain, d.Domain) ||
				d.Owner != "" && dom.Owner != d.Owner ||
				d.Region != "" && dom.Region != d.Region {
				continue
			}
			if id := idRe.FindStringSubmatch(block); id != nil {
				ids = append(ids, found{loc[0], id[1]})
			}
		}
	}
	for _, loc := range serverBlockRe.FindAllStringIndex(content, -1) {
		block := content[loc[0]:loc[1]]
		if inComment(comments, loc[0]) || !envTokenRe.MatchString(block) {
			continue
		}
		if id := idRe.FindStringSubmatch(block); id != nil {
			ids = append(ids, found{loc[0], id[1]})
		}
	}
	slices.SortStableFunc(ids, func(a, b found) int { return a.at - b.at })
	for _, f := range ids {
		add(f.id)
	}
	return det
}
