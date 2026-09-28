# Architecture decision records (ADRs)

Each file records one decision: the context, what was decided, the alternatives and the consequences. To
propose a new one, copy [`template.md`](template.md) with the next number. An accepted ADR isn't edited: if the
decision changes, a new ADR supersedes it ([0001](0001-record-architecture-decisions.md)).

| ADR | Decision | Status |
|---|---|---|
| [0001](0001-record-architecture-decisions.md) | Record architecture decisions | accepted |
| [0002](0002-credentials-in-files.md) | Credentials in files, never in environment variables | accepted |
| [0003](0003-wails-with-system-webview.md) | Desktop app with Wails v2 and the system webview | accepted |
| [0004](0004-svelte-frontend.md) | UI in Svelte 5, TypeScript and Tailwind, developable without a backend | accepted |
| [0005](0005-surgical-file-edits.md) | Surgical edits to the credentials file and settings.xml | accepted |
| [0006](0006-sso-cache-in-aws-cli-format.md) | SSO cache in the AWS CLI format | accepted |
| [0007](0007-device-code-sign-in.md) | Device-code sign-in, renewed with the refresh token | accepted |
| [0008](0008-backend-independent-of-wails.md) | Backend independent of Wails | accepted |
| [0009](0009-fake-aws-for-tests.md) | A local fake AWS for tests | accepted |
| [0010](0010-linux-build-on-el8.md) | Linux build in an EL8 container | accepted |
| [0011](0011-portable-windows-exe.md) | Windows as a portable .exe, with no installer | accepted |
| [0012](0012-proxy.md) | Proxy from environment variables and, on Windows, from the system | accepted, extended by 0020 |
| [0013](0013-maven-settings-inside-home.md) | Maven's settings.xml only inside the home folder | accepted |
| [0014](0014-secrets-out-of-clipboard-history.md) | Secrets out of the clipboard history | accepted |
| [0015](0015-actions-pinned-by-commit.md) | GitHub Actions pinned by commit | accepted |
| [0016](0016-languages.md) | Portuguese UI, English code | superseded by 0018 |
| [0017](0017-colors-with-one-meaning.md) | Colors as tokens, one meaning each | accepted |
| [0018](0018-english-first-localized-ui.md) | English-first project with a localized UI | accepted |
| [0019](0019-second-linux-build-for-webkit2gtk-4-1.md) | A second Linux build for webkit2gtk-4.1 | accepted |
| [0020](0020-proxy-settings-and-gnome.md) | Proxy settings in the app, and the GNOME proxy on Linux | accepted |
| [0021](0021-pinned-go-toolchain.md) | Pinned Go toolchain, and vulnerability checks on the binaries | accepted |
| [0022](0022-several-maven-servers.md) | Several Maven servers, detected from settings.xml | accepted |
| [0023](0023-automatic-renewal.md) | Automatic renewal of the active profile | accepted |
| [0024](0024-updates-from-the-app.md) | New-version notice and updates from the app | accepted, amended by 0025 and 0026 |
| [0025](0025-update-notice-in-about.md) | The new-version notice lives in About | accepted |
| [0026](0026-updates-with-pbrun.md) | Updates on machines that elevate through pbrun | accepted |
