package awsenv

import (
	"net"
	"regexp"
	"strings"

	"golang.org/x/net/http/httpproxy"
)

// gsettingsGetter reads one GSettings key, as printed by `gsettings get`.
type gsettingsGetter func(schema, key string) (string, error)

// gvariantString matches a quoted GVariant string, such as 'proxy.corp'.
var gvariantString = regexp.MustCompile(`'((?:[^'\\]|\\.)*)'`)

// gnomeProxy reads the GNOME proxy settings (org.gnome.system.proxy), the
// ones set in Settings → Network → Proxy. It lives outside the Linux-only
// file so it is tested on every system.
//
// pac reports that GNOME is set to "automatic" (a PAC script), which
// PickRole doesn't run. ok is false when there's no manual proxy to use.
func gnomeProxy(get gsettingsGetter) (cfg httpproxy.Config, pac, ok bool) {
	mode, err := get("org.gnome.system.proxy", "mode")
	if err != nil {
		return cfg, false, false
	}
	switch unquote(mode) {
	case "manual":
	case "auto":
		return cfg, true, false
	default:
		return cfg, false, false
	}

	address := func(protocol, scheme string) string {
		host, err := get("org.gnome.system.proxy."+protocol, "host")
		if err != nil || unquote(host) == "" {
			return ""
		}
		port, err := get("org.gnome.system.proxy."+protocol, "port")
		if err != nil {
			return ""
		}
		port = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(port), "int32"))
		if port == "" || port == "0" {
			return ""
		}
		h := unquote(host)
		if strings.Contains(h, "://") {
			return h
		}
		return scheme + "://" + net.JoinHostPort(h, port)
	}
	// GNOME's HTTPS proxy is an HTTP proxy that tunnels HTTPS (CONNECT).
	https := address("https", "http")
	http := address("http", "http")
	if https == "" {
		https = http
	}
	if https == "" {
		https = address("socks", "socks5")
		http = https
	}
	if https == "" {
		return cfg, false, false
	}
	cfg.HTTPSProxy, cfg.HTTPProxy = https, http

	if ignore, err := get("org.gnome.system.proxy", "ignore-hosts"); err == nil {
		var hosts []string
		for _, m := range gvariantString.FindAllStringSubmatch(ignore, -1) {
			hosts = append(hosts, m[1])
		}
		cfg.NoProxy = NormalizeNoProxy(strings.Join(hosts, ","))
	}
	return cfg, false, true
}

// unquote turns a GVariant string ('text') into text.
func unquote(v string) string {
	if m := gvariantString.FindStringSubmatch(strings.TrimSpace(v)); m != nil {
		return strings.ReplaceAll(m[1], `\'`, `'`)
	}
	return strings.TrimSpace(v)
}
