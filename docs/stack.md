# Stack

Reference versions as of `v0.1.0`. The exact ones are in `go.mod` and `frontend/package.json`.

## Application

| Layer | Technology | Why |
|---|---|---|
| Backend language | **Go** (code needs 1.26+; builds use the toolchain pinned in `go.mod`) | A single binary with no runtime to install; the official AWS SDK; builds for Linux and Windows. [ADR 0021](adr/0021-pinned-go-toolchain.md) |
| Desktop | **Wails v2.16** | Native window using the system webview (WebKitGTK on Linux, WebView2 on Windows): a ~15 MB binary, no bundled browser. [ADR 0003](adr/0003-wails-with-system-webview.md) |
| AWS | **AWS SDK for Go v2**: `sso`, `ssooidc`, `codeartifact` | Official clients; endpoints can be swapped with `AWS_ENDPOINT_URL*` for tests. |
| Proxy | `github.com/mattn/go-ieproxy`, `golang.org/x/net/http/httpproxy` | The system proxy on Windows (PAC included); `NO_PROXY`-style matching for the manual and GNOME proxies. On Linux, the GNOME settings come from `gsettings`. [ADR 0012](adr/0012-proxy.md), [ADR 0020](adr/0020-proxy-settings-and-gnome.md) |
| Win32 | `golang.org/x/sys/windows` | Protected clipboard copies and real paths (junctions). |
| UI | **Svelte 5** (runes) + **TypeScript 6** | Small reactive components with a light runtime. [ADR 0004](adr/0004-svelte-frontend.md) |
| Styling | **Tailwind CSS v4** with tokens in `frontend/src/app.css` | Colors only through tokens, in light and dark themes. [Design](design.md) |
| Frontend build | **Vite 8** | Fast builds and a dev server with the mock backend. |
| Fonts | IBM Plex Sans, IBM Plex Mono, Space Grotesk via `@fontsource` | Embedded in the binary: the app works offline. |
| Languages | Own small i18n module (frontend) and `internal/i18n` (Go) | English and Brazilian Portuguese, no extra dependency. [ADR 0018](adr/0018-english-first-localized-ui.md) |

## Development and quality

| Tool | Use |
|---|---|
| Wails CLI v2.16 | `wails dev` (live reload) and `wails build`. |
| `go test` | Backend tests, including the whole flow against the fake AWS. CI also runs them with `-race`. |
| `svelte-check` | `npm run check`, which fails on any warning. |
| `internal/fakeaws` | Fake AWS speaking the real REST-JSON format, used by the tests and for manual testing. [ADR 0009](adr/0009-fake-aws-for-tests.md) |
| `gosec` | Static security analysis for Go. Suppressions carry `#nosec <rule> -- reason`. |
| `govulncheck`, `npm audit` | Known vulnerabilities in dependencies; CI and releases run `govulncheck` on the built binaries. |

## Build, CI and distribution

| Tool | Use |
|---|---|
| GitHub Actions | CI on every pull request: Go tests, frontend, Windows build and both Linux builds. A release on every `v*` tag, with Linux install checks. |
| `almalinux:8` container | Linux build compatible with RHEL 8 and 9 (glibc 2.28, webkit2gtk-4.0). [ADR 0010](adr/0010-linux-build-on-el8.md) |
| `ubuntu:22.04` container | Linux build for current distributions (glibc 2.35, webkit2gtk-4.1). [ADR 0019](adr/0019-second-linux-build-for-webkit2gtk-4-1.md) |
| `nfpm` | Builds the RPMs and the `.deb` (`build/linux/nfpm-*.yaml`, `scripts/package-linux.sh`). |
| Dependabot | Keeps the actions, which are pinned to commits, up to date. [ADR 0015](adr/0015-actions-pinned-by-commit.md) |
