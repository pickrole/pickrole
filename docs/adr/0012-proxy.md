# 0012. Proxy from environment variables and, on Windows, from the system

- Status: accepted, extended by [0020](0020-proxy-settings-and-gnome.md)
- Date: 2026-09-26

## Context

Corporate networks often require a proxy. Go's default HTTP client only reads `HTTPS_PROXY`, `HTTP_PROXY` and
`NO_PROXY`, while on Windows the proxy is usually set in the system, sometimes through a PAC script.

## Decision

The AWS clients use the proxy from those variables when set and, on Windows, otherwise the system proxy, PAC included
(`github.com/mattn/go-ieproxy`). Loopback addresses never go through the proxy.

## Consequences

- On Windows it works with no setup on most networks.
- On Linux only the variables count: the GNOME proxy settings aren't read, and an app started from the application menu
  doesn't load `~/.bashrc`. Reading the GNOME settings is on the roadmap.
- Proxies that require NTLM or Kerberos authentication aren't supported.
