# Contributing to PickRole

Thanks for your interest! Before a big change, open an issue to talk it through. Worth reading first:

- [Architecture](docs/architecture.md): components, flows and the files PickRole writes
- [Stack](docs/stack.md): technologies and why
- [ADRs](docs/adr/README.md): decisions to keep
- [Design](docs/design.md): colors, brand and languages

## Setup

Requirements: Go 1.26+ (the `toolchain` line in `go.mod` makes Go download the version releases use), Node 22+ and the [Wails CLI](https://wails.io) v2.16.

- **Linux**: also the GTK and WebKit development libraries: `gtk3-devel webkit2gtk3-devel` on RHEL,
  `gtk3-devel webkit2gtk4.1-devel` on Fedora, `libgtk-3-dev libwebkit2gtk-4.1-dev` on Debian/Ubuntu. With
  webkit2gtk-4.1, build with `wails dev -tags webkit2_41` (or `WAILS_TAGS=webkit2_41 scripts/build-linux.sh`).
- **Windows**: nothing else, since WebView2 ships with the system.

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@v2.16.0
wails doctor        # checks the environment
wails dev           # app with live reload
```

To work on the UI only, without Go, Wails or AWS, run the frontend in a browser. It uses sample data from
`frontend/src/lib/mock.ts`:

```bash
cd frontend && npm ci && npm run dev
```

## Testing without an AWS account

`cmd/fakeaws` is a local fake AWS ([ADR 0009](docs/adr/0009-fake-aws-for-tests.md)). On Windows, this script builds the
app and opens it against the fake, with an isolated user folder in `%TEMP%\pickrole-mock`:

```powershell
scripts\mock-aws.ps1 -Build -Reset
```

Elsewhere, run `go run ./cmd/fakeaws` and start the app with `AWS_ENDPOINT_URL=http://127.0.0.1:4599` and a separate
`HOME`; otherwise the app writes to your real `~/.aws` and `~/.m2`. The dashboard at `http://127.0.0.1:4599/` lists the
accounts and simulates an expired or revoked session.

## Before opening a pull request

```bash
go vet ./...
go test ./internal/...
cd frontend && npm run check      # fails on any warning
```

CI runs the same, plus the tests with `-race`, the Windows build and the RHEL build in an AlmaLinux 8 container.

If the change touches something sensitive (files written, network, clipboard, workflows), also run:

```bash
go install github.com/securego/gosec/v2/cmd/gosec@latest && gosec ./...
go install golang.org/x/vuln/cmd/govulncheck@latest && govulncheck ./...
```

## Conventions

- **Language**: code, comments, docs, commits and pull requests in English. User-facing strings go through the i18n
  catalogs, in every language ([ADR 0018](docs/adr/0018-english-first-localized-ui.md)):
  `frontend/src/lib/i18n/` for the UI and `internal/i18n/messages.go` for backend messages.
- **Logic outside `main.go`**: everything goes into `internal/`, which doesn't import Wails
  ([ADR 0008](docs/adr/0008-backend-independent-of-wails.md)).
- **Structs exposed to the UI**: when you change one, update the Go struct, `frontend/src/lib/types.ts` and the mock.
- **Expired sign-in**: errors that need a new sign-in start with `LOGIN_REQUIRED` (`app.LoginRequired`).
- **Colors**: only through the tokens in `frontend/src/app.css`, each with its meaning ([design](docs/design.md)).
- **Secrets**: copied only with `SetSecretClipboard`, written only with `0600` and atomic writes.
- **`#nosec`**: every `gosec` suppression carries the rule and the reason: `// #nosec G304 -- reason`.
- **New decisions**: when a change alters a lasting decision, include a new ADR
  ([ADR 0001](docs/adr/0001-record-architecture-decisions.md)).

### Known, harmless warnings

- The Wails bindings generator prints `Not found: time.Time`. It only affects `frontend/wailsjs`, which isn't used.
- Opening `http://localhost:34115` in a regular browser during `wails dev` shows an error in `wails/ipc.js`. Inside
  the app window it doesn't.
- Vite empties `frontend/dist` on build and deletes the committed `.gitkeep`. The build scripts restore it; if you run
  `wails build` directly, run `git checkout -- frontend/dist/.gitkeep` before committing.

## Build and release

| Platform | Command | Output |
|---|---|---|
| Linux, webkit2gtk-4.0 | `scripts/build-linux.sh`, in an EL8 environment | `build/bin/pickrole` |
| Linux, webkit2gtk-4.1 | `WAILS_TAGS=webkit2_41 scripts/build-linux.sh` | `build/bin/pickrole` |
| Windows | `scripts\build-windows.ps1` | `build\bin\pickrole.exe` |

The scripts embed the version, commit and date (`-ldflags`), shown in the About dialog and, on Windows, in the `.exe`
file properties. `scripts/package-linux.sh <el8|webkit41> <version>` turns a Linux build into its packages, and the
**Linux packages** workflow (Actions tab → Run workflow) builds, packages and install-tests every Linux package without
a release.

To move to a new Go release, update the `toolchain` line in `go.mod` and `GO_VERSION`/`GO_SHA256` in
`scripts/setup-el8.sh` (checksum from https://go.dev/dl/). Do it for every Go security release: `govulncheck` in CI and
in the release fails when the binaries contain a known vulnerability.

To publish a version:

1. In the [changelog](CHANGELOG.md), rename "Unreleased" to `[X.Y.Z] - YYYY-MM-DD` and start a new "Unreleased"
   section. That section becomes the release notes, so write it for people who use the app.
2. Push the `vX.Y.Z` tag on `main`. The release workflow first checks the changelog section
   (`scripts/release-notes.sh`), then builds and install-tests the Linux packages, builds the Windows `.zip`, and
   publishes everything with `SHA256SUMS`.

Versions with a suffix, such as `v0.2.0-beta.1`, are published as pre-releases: GitHub marks them as such and keeps
the latest stable version as "Latest". Their changelog section is `[0.2.0-beta.1]`. In the packages the suffix becomes
`~beta.1` (`0.2.0~beta.1`), so the final `0.2.0` counts as newer; the Windows file properties show `0.2.0`, and the
About dialog shows the full version.
