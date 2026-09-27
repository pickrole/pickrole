package awsenv

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/net/http/httpproxy"
)

// Proxy modes, as stored in the config.
const (
	// ProxySystem uses HTTPS_PROXY/HTTP_PROXY/NO_PROXY when set and
	// otherwise the operating system's proxy settings.
	ProxySystem = "system"
	// ProxyManual uses the proxy the user typed in PickRole.
	ProxyManual = "manual"
	// ProxyNone connects directly.
	ProxyNone = "none"
)

// Where a route's proxy came from.
const (
	SourceManual      = "manual"
	SourceEnvironment = "environment"
	SourceWindows     = "windows"
	SourceGNOME       = "gnome"
	SourceNone        = "none"
)

// ProxySettings is the proxy chosen in PickRole's settings.
type ProxySettings struct {
	Mode    string
	URL     string
	NoProxy string
}

// Route is how AWS calls leave the machine.
type Route struct {
	// Proxy picks the proxy for a request; nil means a direct connection.
	Proxy func(*http.Request) (*url.URL, error)
	// Source says where the setting came from (Source* constants).
	Source string
	// Note is an i18n key for a system setting PickRole couldn't use, such
	// as a PAC script on Linux; empty when there's nothing to report.
	Note string
}

var current atomic.Pointer[Route]

// SetProxy makes every AWS client use the given proxy settings from now on,
// including clients already created.
func SetProxy(p ProxySettings) {
	r := Resolve(p)
	current.Store(&r)
}

func currentRoute() Route {
	if r := current.Load(); r != nil {
		return *r
	}
	r := Resolve(ProxySettings{Mode: ProxySystem})
	current.CompareAndSwap(nil, &r)
	return *current.Load()
}

// Resolve turns proxy settings into a route, reading the environment and
// the system settings as they are now.
func Resolve(p ProxySettings) Route {
	switch p.Mode {
	case ProxyNone:
		return Route{Source: SourceNone}
	case ProxyManual:
		return Route{Source: SourceManual, Proxy: proxyFunc(httpproxy.Config{
			HTTPProxy:  p.URL,
			HTTPSProxy: p.URL,
			NoProxy:    NormalizeNoProxy(p.NoProxy),
		})}
	}
	if env := httpproxy.FromEnvironment(); env.HTTPProxy != "" || env.HTTPSProxy != "" {
		return Route{Source: SourceEnvironment, Proxy: proxyFunc(*env)}
	}
	return systemRoute()
}

func proxyFunc(c httpproxy.Config) func(*http.Request) (*url.URL, error) {
	f := c.ProxyFunc()
	return func(r *http.Request) (*url.URL, error) { return f(r.URL) }
}

// NormalizeNoProxy accepts the forms people usually write in proxy
// exception lists — separated by commas, semicolons or spaces, with
// "*.example.com" wildcards — and returns the comma-separated list, with
// ".example.com" suffixes, that NO_PROXY uses.
func NormalizeNoProxy(list string) string {
	fields := strings.FieldsFunc(list, func(r rune) bool {
		return r == ',' || r == ';' || r == ' ' || r == '\t' || r == '\n' || r == '\r'
	})
	out := fields[:0]
	for _, f := range fields {
		if strings.HasPrefix(f, "*.") {
			f = f[1:]
		}
		out = append(out, f)
	}
	return strings.Join(out, ",")
}

// ConnectionTest is the result of TestConnection.
type ConnectionTest struct {
	// Proxy is the proxy used for the target ("host:port"), or empty for a
	// direct connection.
	Proxy  string
	Source string
	Note   string
	// Err is why the target couldn't be reached; nil when it answered.
	Err error
}

// TestConnection sends a request to target through the route of the given
// settings. Any HTTP answer counts as reachable: the point is the network
// path, not the endpoint's reply.
func TestConnection(ctx context.Context, p ProxySettings, target string) ConnectionTest {
	route := Resolve(p)
	result := ConnectionTest{Source: route.Source, Note: route.Note}

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		result.Err = err
		return result
	}
	proxy := withLoopbackBypass(route.Proxy)
	if u, err := proxy(req); err != nil {
		result.Err = err
		return result
	} else if u != nil {
		result.Proxy = u.Host
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = proxy
	client := &http.Client{Transport: transport}
	resp, err := client.Do(req) // #nosec G107 -- the AWS endpoint, or its local override
	if err != nil {
		result.Err = err
		return result
	}
	_ = resp.Body.Close()
	return result
}

// withLoopbackBypass keeps loopback hosts (the local fake of AWS) off any
// proxy.
func withLoopbackBypass(proxy func(*http.Request) (*url.URL, error)) func(*http.Request) (*url.URL, error) {
	return func(r *http.Request) (*url.URL, error) {
		if proxy == nil || isLoopback(r.URL.Hostname()) {
			return nil, nil
		}
		return proxy(r)
	}
}
