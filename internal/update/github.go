package update

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Doer sends HTTP requests; the AWS client of awsenv fits, so updates go
// through the same proxy as everything else.
type Doer interface {
	Do(*http.Request) (*http.Response, error)
}

// Source is where releases come from. The zero value is not usable: see
// GitHub.
type Source struct {
	// ReleasesURL lists the releases (GitHub REST API).
	ReleasesURL string
	// FeedURL is the Atom feed of the release page, used when the API
	// refuses (see Newer).
	FeedURL string
	// DownloadPrefix is the only place assets are downloaded from.
	DownloadPrefix string
	// RedirectHosts are where a download may be redirected to: GitHub
	// serves release files from its own storage hosts.
	RedirectHosts []string
	HTTP          Doer
}

// GitHub is PickRole's release page.
func GitHub(client Doer) Source {
	return Source{
		ReleasesURL:    "https://api.github.com/repos/pickrole/pickrole/releases?per_page=30",
		FeedURL:        "https://github.com/pickrole/pickrole/releases.atom",
		DownloadPrefix: "https://github.com/pickrole/pickrole/releases/download/",
		RedirectHosts: []string{
			"objects.githubusercontent.com",
			"release-assets.githubusercontent.com",
			"github-releases.githubusercontent.com",
		},
		HTTP: client,
	}
}

// Release is a published release newer than the running version.
type Release struct {
	Version Version
	Tag     string
	// URL is the release page, with the notes.
	URL string
	// Assets maps file names to download URLs.
	Assets map[string]string
	// AssetBase is set instead of Assets when the release came from the
	// feed, which doesn't list files: see AssetURL.
	AssetBase string
	// Security is true when this release, or any other one between the
	// running version and it, has a "Security" section in its notes (the
	// CHANGELOG convention), so skipping versions doesn't hide a fix.
	Security bool
}

const (
	maxListing = 4 << 20   // the releases JSON
	maxSums    = 64 << 10  // SHA256SUMS
	maxAsset   = 300 << 20 // a package
)

// Newer returns the newest release above current, or nil when there is
// none. Pre-releases count only when current is one: people on a beta
// get the next beta, people on a final release only final releases.
//
// The list comes from the GitHub API. When the API answers with an error,
// most often 403 because GitHub limits unauthenticated calls per IP address
// and a company network shares one, or because a proxy only lets github.com
// through, the Atom feed of the release page is used instead: it is served
// by github.com, like the downloads, without that limit.
func (s Source) Newer(ctx context.Context, current Version) (*Release, error) {
	list, err := s.listAPI(ctx)
	var status *HTTPError
	if errors.As(err, &status) && s.FeedURL != "" {
		if feed, ferr := s.listFeed(ctx); ferr == nil {
			list, err = feed, nil
		}
	}
	if err != nil {
		return nil, err
	}
	var best *Release
	security := false
	for _, r := range list {
		v, ok := ParseVersion(r.tag)
		if !ok || r.draft || (v.Prerelease() || r.prerelease) && !current.Prerelease() {
			continue
		}
		if v.Compare(current) <= 0 {
			continue
		}
		security = security || r.security
		if best != nil && v.Compare(best.Version) <= 0 {
			continue
		}
		rel := &Release{Version: v, Tag: r.tag, URL: r.url, Assets: r.assets}
		if rel.Assets == nil {
			// The feed doesn't list the files: they are where every
			// release keeps them, and Download checks they exist.
			rel.AssetBase = s.DownloadPrefix + r.tag + "/"
		}
		best = rel
	}
	if best != nil {
		best.Security = security
	}
	return best, nil
}

// listed is a release as the API or the feed describes it.
type listed struct {
	tag, url          string
	draft, prerelease bool
	security          bool
	assets            map[string]string
}

func (s Source) listAPI(ctx context.Context) ([]listed, error) {
	body, err := s.get(ctx, s.ReleasesURL, maxListing)
	if err != nil {
		return nil, err
	}
	var list []struct {
		Tag        string `json:"tag_name"`
		URL        string `json:"html_url"`
		Draft      bool   `json:"draft"`
		Prerelease bool   `json:"prerelease"`
		Body       string `json:"body"`
		Assets     []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(body, &list); err != nil {
		return nil, fmt.Errorf("release list: %w", err)
	}
	out := make([]listed, 0, len(list))
	for _, r := range list {
		l := listed{tag: r.Tag, url: r.URL, draft: r.Draft, prerelease: r.Prerelease,
			security: hasSecuritySection(r.Body), assets: map[string]string{}}
		for _, a := range r.Assets {
			l.assets[a.Name] = a.URL
		}
		out = append(out, l)
	}
	return out, nil
}

// listFeed reads the Atom feed of the release page. It has no drafts, and
// pre-releases are told apart by their version (v0.2.0-beta.7).
func (s Source) listFeed(ctx context.Context) ([]listed, error) {
	body, err := s.get(ctx, s.FeedURL, maxListing)
	if err != nil {
		return nil, err
	}
	var feed struct {
		Entries []struct {
			Link struct {
				Href string `xml:"href,attr"`
			} `xml:"link"`
			Content string `xml:"content"`
		} `xml:"entry"`
	}
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, fmt.Errorf("release feed: %w", err)
	}
	var out []listed
	for _, e := range feed.Entries {
		_, tag, ok := strings.Cut(e.Link.Href, "/releases/tag/")
		if !ok {
			continue
		}
		if tag, err = url.PathUnescape(tag); err != nil || strings.ContainsAny(tag, "/?#") {
			continue
		}
		out = append(out, listed{tag: tag, url: e.Link.Href, security: htmlSecurityHeading.MatchString(e.Content)})
	}
	return out, nil
}

// htmlSecurityHeading is hasSecuritySection for the notes as HTML (the feed).
var htmlSecurityHeading = regexp.MustCompile(`(?i)<h[1-6][^>]*>\s*Security\s*</h[1-6]>`)

// hasSecuritySection reports whether release notes have a "Security"
// heading, as CHANGELOG.md does for security fixes (Keep a Changelog).
func hasSecuritySection(notes string) bool {
	for _, line := range strings.Split(notes, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") && strings.EqualFold(strings.TrimSpace(strings.TrimLeft(line, "#")), "Security") {
			return true
		}
	}
	return false
}

// AssetURL is where the release's file name is downloaded from, or "" when
// the release doesn't have it.
func (r *Release) AssetURL(name string) string {
	if u, ok := r.Assets[name]; ok {
		return u
	}
	if r.AssetBase != "" {
		return r.AssetBase + name
	}
	return ""
}

// HTTPError is an answer other than 200 OK, kept apart so the app can say
// what it means (GitHub's rate limit, a proxy refusing) instead of a URL.
type HTTPError struct {
	Host   string
	Status int
	// RateLimited is true when GitHub says the limit of calls for this
	// address is used up; Reset is when it starts over.
	RateLimited bool
	Reset       time.Time
}

func (e *HTTPError) Error() string {
	if e.RateLimited {
		return fmt.Sprintf("%s: HTTP %d, rate limit exceeded", e.Host, e.Status)
	}
	return fmt.Sprintf("%s: HTTP %d", e.Host, e.Status)
}

func httpError(resp *http.Response) *HTTPError {
	e := &HTTPError{Host: resp.Request.URL.Hostname(), Status: resp.StatusCode}
	if (e.Status == http.StatusForbidden || e.Status == http.StatusTooManyRequests) &&
		resp.Header.Get("X-RateLimit-Remaining") == "0" {
		e.RateLimited = true
		if sec, err := strconv.ParseInt(resp.Header.Get("X-RateLimit-Reset"), 10, 64); err == nil {
			e.Reset = time.Unix(sec, 0)
		}
	}
	return e
}

// Download saves asset of rel in dir and checks it against the release's
// SHA256SUMS; the file is only kept when it matches.
func (s Source) Download(ctx context.Context, rel *Release, asset, dir string) (string, error) {
	sumsURL, assetURL := rel.AssetURL("SHA256SUMS"), rel.AssetURL(asset)
	if sumsURL == "" || assetURL == "" {
		return "", fmt.Errorf("%s has no %s", rel.Tag, asset)
	}
	sums, err := s.get(ctx, sumsURL, maxSums)
	if err != nil {
		return "", err
	}
	want, err := checksum(sums, asset)
	if err != nil {
		return "", err
	}
	data, err := s.get(ctx, assetURL, maxAsset)
	if err != nil {
		return "", err
	}
	got := sha256.Sum256(data)
	if hex.EncodeToString(got[:]) != want {
		return "", ErrChecksum
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	path := filepath.Join(dir, asset)
	return path, os.WriteFile(path, data, 0o600)
}

// ErrChecksum means the download doesn't match SHA256SUMS.
var ErrChecksum = errors.New("the download doesn't match the release's SHA256SUMS")

// checksum finds name in a SHA256SUMS file ("<hex>  <name>" per line).
func checksum(sums []byte, name string) (string, error) {
	sc := bufio.NewScanner(strings.NewReader(string(sums)))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == name && len(fields[0]) == 64 {
			return strings.ToLower(fields[0]), nil
		}
	}
	return "", fmt.Errorf("SHA256SUMS has no line for %s", name)
}

func (s Source) get(ctx context.Context, target string, limit int64) ([]byte, error) {
	// The listing comes from the API or the feed; everything else only
	// from the project's own release downloads.
	if target != s.ReleasesURL && (target != s.FeedURL || s.FeedURL == "") && !strings.HasPrefix(target, s.DownloadPrefix) {
		return nil, fmt.Errorf("refusing to download from %s", target)
	}
	// The AWS client that carries the proxy doesn't follow redirects, and
	// GitHub answers downloads with one to its storage: follow a few, to
	// the known hosts only, over HTTPS.
	for hops := 0; ; hops++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "pickrole-updater")
		if target == s.ReleasesURL {
			req.Header.Set("Accept", "application/vnd.github+json")
		}
		resp, err := s.HTTP.Do(req)
		if err != nil {
			return nil, err
		}
		if loc := resp.Header.Get("Location"); isRedirect(resp.StatusCode) && loc != "" {
			_ = resp.Body.Close()
			next, err := resp.Request.URL.Parse(loc)
			if err != nil || hops >= 5 || !s.redirectAllowed(next) {
				return nil, fmt.Errorf("refusing the redirect from %s to %s", target, loc)
			}
			target = next.String()
			continue
		}
		defer resp.Body.Close() //nolint:errcheck // read only
		if resp.StatusCode != http.StatusOK {
			return nil, httpError(resp)
		}
		data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
		if err != nil {
			return nil, err
		}
		if int64(len(data)) > limit {
			return nil, fmt.Errorf("%s is larger than expected", target)
		}
		return data, nil
	}
}

func isRedirect(status int) bool {
	switch status {
	case http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther,
		http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
		return true
	}
	return false
}

func (s Source) redirectAllowed(u *url.URL) bool {
	return u.Scheme == "https" && slices.Contains(s.RedirectHosts, u.Hostname())
}
