package awsenv

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestEndpoint(t *testing.T) {
	t.Setenv("AWS_ENDPOINT_URL", "")
	t.Setenv("AWS_ENDPOINT_URL_SSO", "")
	if got := Endpoint(SSO); got != nil {
		t.Errorf("want nil without overrides, got %q", *got)
	}
	t.Setenv("AWS_ENDPOINT_URL", "http://127.0.0.1:4599")
	if got := Endpoint(SSO); got == nil || *got != "http://127.0.0.1:4599" {
		t.Errorf("global override not used: %v", got)
	}
	t.Setenv("AWS_ENDPOINT_URL_SSO", "http://127.0.0.1:1")
	if got := Endpoint(SSO); got == nil || *got != "http://127.0.0.1:1" {
		t.Errorf("service override should win: %v", got)
	}
}

// proxyHost returns the proxy host proxy picks for rawURL, or "" for a
// direct connection.
func proxyHost(t *testing.T, proxy func(*http.Request) (*url.URL, error), rawURL string) string {
	t.Helper()
	req, _ := http.NewRequest("POST", rawURL, nil)
	got, err := proxy(req)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		return ""
	}
	return got.Host
}

func clearProxyEnv(t *testing.T) {
	for _, name := range []string{"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy", "NO_PROXY", "no_proxy"} {
		t.Setenv(name, "")
	}
}

func TestHTTPClientFollowsSetProxy(t *testing.T) {
	clearProxyEnv(t)
	t.Cleanup(func() { current.Store(nil) })
	proxy := HTTPClient().GetTransport().Proxy

	SetProxy(ProxySettings{Mode: ProxyManual, URL: "http://proxy.example.com:3128", NoProxy: "*.corp.example, 10.0.0.0/8"})
	for rawURL, want := range map[string]string{
		"https://oidc.us-east-1.amazonaws.com/token": "proxy.example.com:3128",
		"https://git.corp.example/":                  "",
		"https://10.1.2.3/":                          "",
		"http://127.0.0.1:4599/token":                "",
		"http://localhost:4599/token":                "",
	} {
		if got := proxyHost(t, proxy, rawURL); got != want {
			t.Errorf("manual: proxy for %s = %q, want %q", rawURL, got, want)
		}
	}

	// A client created earlier picks up a later change.
	SetProxy(ProxySettings{Mode: ProxyNone})
	if got := proxyHost(t, proxy, "https://oidc.us-east-1.amazonaws.com/token"); got != "" {
		t.Errorf("none: want a direct connection, got %q", got)
	}
}

func TestResolveEnvironment(t *testing.T) {
	clearProxyEnv(t)
	t.Setenv("HTTPS_PROXY", "http://env-proxy.example.com:8080")
	t.Setenv("NO_PROXY", "internal.example")

	r := Resolve(ProxySettings{Mode: ProxySystem})
	if r.Source != SourceEnvironment {
		t.Fatalf("source = %q, want %q", r.Source, SourceEnvironment)
	}
	if got := proxyHost(t, r.Proxy, "https://sso.us-east-1.amazonaws.com/"); got != "env-proxy.example.com:8080" {
		t.Errorf("proxy = %q", got)
	}
	if got := proxyHost(t, r.Proxy, "https://internal.example/"); got != "" {
		t.Errorf("NO_PROXY ignored: %q", got)
	}

	// The manual setting wins over the environment.
	r = Resolve(ProxySettings{Mode: ProxyManual, URL: "http://manual.example.com:3128"})
	if got := proxyHost(t, r.Proxy, "https://sso.us-east-1.amazonaws.com/"); r.Source != SourceManual || got != "manual.example.com:3128" {
		t.Errorf("manual: source %q, proxy %q", r.Source, got)
	}
}

func TestNormalizeNoProxy(t *testing.T) {
	got := NormalizeNoProxy(" *.corp.example; localhost,10.0.0.0/8\n.internal  ")
	if want := ".corp.example,localhost,10.0.0.0/8,.internal"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got := NormalizeNoProxy(""); got != "" {
		t.Errorf("empty list: got %q", got)
	}
}

func TestGnomeProxy(t *testing.T) {
	settings := func(values map[string]string) gsettingsGetter {
		return func(schema, key string) (string, error) {
			v, ok := values[schema+" "+key]
			if !ok {
				return "", errors.New("no such key")
			}
			return v + "\n", nil
		}
	}
	const base = "org.gnome.system.proxy"

	cases := []struct {
		name        string
		values      map[string]string
		pac, ok     bool
		https, http string
		noProxy     string
	}{
		{name: "gsettings missing", values: map[string]string{}},
		{name: "no proxy", values: map[string]string{base + " mode": "'none'"}},
		{name: "pac", values: map[string]string{base + " mode": "'auto'"}, pac: true},
		{
			name: "manual",
			values: map[string]string{
				base + " mode":         "'manual'",
				base + ".https host":   "'proxy.corp.example'",
				base + ".https port":   "3128",
				base + ".http host":    "'proxy.corp.example'",
				base + ".http port":    "8080",
				base + " ignore-hosts": `['localhost', '127.0.0.0/8', '::1', '*.corp.example']`,
			},
			ok: true, https: "http://proxy.corp.example:3128", http: "http://proxy.corp.example:8080",
			noProxy: "localhost,127.0.0.0/8,::1,.corp.example",
		},
		{
			name: "only the http proxy",
			values: map[string]string{
				base + " mode":         "'manual'",
				base + ".https host":   "''",
				base + ".https port":   "0",
				base + ".http host":    "'10.0.0.5'",
				base + ".http port":    "3128",
				base + " ignore-hosts": "@as []",
			},
			ok: true, https: "http://10.0.0.5:3128", http: "http://10.0.0.5:3128",
		},
		{
			name: "socks",
			values: map[string]string{
				base + " mode":       "'manual'",
				base + ".socks host": "'socks.corp.example'",
				base + ".socks port": "1080",
			},
			ok: true, https: "socks5://socks.corp.example:1080", http: "socks5://socks.corp.example:1080",
		},
		{name: "manual without a host", values: map[string]string{base + " mode": "'manual'"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg, pac, ok := gnomeProxy(settings(c.values))
			if pac != c.pac || ok != c.ok {
				t.Fatalf("pac, ok = %v, %v; want %v, %v", pac, ok, c.pac, c.ok)
			}
			if cfg.HTTPSProxy != c.https || cfg.HTTPProxy != c.http || cfg.NoProxy != c.noProxy {
				t.Errorf("got https %q, http %q, no_proxy %q", cfg.HTTPSProxy, cfg.HTTPProxy, cfg.NoProxy)
			}
		})
	}
}

func TestTestConnection(t *testing.T) {
	clearProxyEnv(t)

	// A proxy that answers every request itself and records what it got.
	var asked string
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = r.Host
		w.WriteHeader(http.StatusForbidden) // any answer means "reachable"
	}))
	defer proxy.Close()

	res := TestConnection(context.Background(), ProxySettings{Mode: ProxyManual, URL: proxy.URL}, "http://oidc.us-east-1.amazonaws.com/")
	if res.Err != nil {
		t.Fatalf("unexpected error: %v", res.Err)
	}
	if res.Proxy != proxy.Listener.Addr().String() || res.Source != SourceManual {
		t.Errorf("proxy %q, source %q", res.Proxy, res.Source)
	}
	if asked != "oidc.us-east-1.amazonaws.com" {
		t.Errorf("proxy was asked for %q", asked)
	}

	// Nothing listens on a closed port.
	l, _ := net.Listen("tcp", "127.0.0.1:0")
	closed := "http://" + l.Addr().String()
	_ = l.Close()
	res = TestConnection(context.Background(), ProxySettings{Mode: ProxyManual, URL: closed}, "http://oidc.us-east-1.amazonaws.com/")
	if res.Err == nil {
		t.Error("want an error through a proxy that isn't there")
	}

	// Loopback targets (the local fake of AWS) go direct in every mode.
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer target.Close()
	res = TestConnection(context.Background(), ProxySettings{Mode: ProxyManual, URL: closed}, target.URL)
	if res.Err != nil || res.Proxy != "" {
		t.Errorf("loopback: proxy %q, err %v", res.Proxy, res.Err)
	}
}
