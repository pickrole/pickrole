# 0020. Proxy settings in the app, and the GNOME proxy on Linux

- Status: accepted
- Date: 2026-09-26
- Extends: [0012](0012-proxy.md)

## Context

[ADR 0012](0012-proxy.md) reads the proxy from `HTTPS_PROXY`/`HTTP_PROXY`/`NO_PROXY` and, on Windows, from the system.
On Linux that leaves a gap: an app started from the application menu doesn't load `~/.bashrc`, so the variables often
don't reach it, and the proxy set in GNOME's network settings isn't read. On a network that requires a proxy, PickRole
only worked when started from a terminal. On any system, there was no way to fix a wrong system proxy from the app, nor
to check whether AWS was reachable.

## Decision

- **Settings → Network** has a proxy mode, stored in the config (`proxy.mode`):
  - **Automatic** (default): the variables when set; otherwise the system settings — Windows (static proxy or PAC, as
    before) or GNOME's manual proxy on Linux (`gsettings get org.gnome.system.proxy …`, including its ignore list).
  - **Manual**: a proxy address (`http://`, `https://` or `socks5://`) and a list of exceptions, which win over the
    variables and the system.
  - **No proxy**: always connect directly.
- **Test connection** sends a request to the SSO sign-in endpoint of the configured region with the settings on screen,
  before they're saved, and shows whether it answered, through which proxy and where that proxy came from.
- A change applies to AWS clients already created: their transport looks up the current setting on every request.
- The proxy address can't hold a user or password. The setting goes into the config file that teams share, so
  credentials would leak with it. A proxy that needs them is set through `HTTPS_PROXY`, as before.
- Loopback addresses never go through a proxy, as before.

## Alternatives considered

- **Only read the GNOME settings**: fixes the common case, but leaves no way out when detection is wrong, and nothing
  for KDE or other desktops.
- **Run PAC scripts on Linux**: needs a JavaScript engine in the app. When GNOME is set to automatic (PAC), the test
  says so and suggests the manual mode instead.
- **Read `/etc/environment` or systemd user environment**: many variations, and the manual mode covers the same need.

## Consequences

- On Linux with GNOME, PickRole follows the desktop's manual proxy with no setup, even when started from the menu.
- When detection doesn't fit, users set the proxy in the app and check it with **Test connection** before signing in.
- A PAC script on Linux, other desktops' proxy settings, and proxies with NTLM or Kerberos authentication still need
  the manual mode, the variables or a local authenticating proxy.
