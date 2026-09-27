# Changelog

What changed in each version. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and versions
follow [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- The CodeArtifact token goes to several Maven `<server>` entries at once, one per repository, plugin repository or
  mirror. **Settings → CodeArtifact → Detect in settings.xml** finds them: repositories whose URL is CodeArtifact and
  servers whose password reads `${env.CODEARTIFACT_AUTH_TOKEN}`. It also fills the domain, owner account and region
  from the repository URL. Configs with a single `serverId` keep working.
- README: step-by-step command-line install (download, verify the checksum, install) and a ready-made prompt for
  installing with an AI coding assistant.

## [0.2.0-beta.3] - 2026-09-27

### Fixed

- In production, roles whose names only contained "read", "view", "audit" or "billing" (such as `DataReadWrite` or
  `OverviewAdmin`) loaded without confirmation. Only names that say the role only reads, as whole words (`ReadOnly`,
  `ViewOnly`, `SecurityAudit`, `Billing`), skip it now, and any word such as `Write` or `Admin` asks for confirmation.
- The comment PickRole writes in `~/.aws/credentials` and the version suffix of modified local builds are in English.

### Security

- "Copy export" validates the credentials itself and quotes every value before they reach the clipboard.
- The build scripts refuse a version with characters outside letters, digits, `.`, `-` and `_`.
- Dependabot also watches the frontend packages.

## [0.2.0-beta.2] - 2026-09-26

First public release, and a beta: everything planned for 0.2.0 is in, and it needs testing on real networks and AWS
organizations before the final release. Report problems in the [issues](https://github.com/pickrole/pickrole/issues).

### Security

- The binaries are built with Go 1.27.1, and a build fails when a binary contains a known vulnerability
  (`govulncheck`). `0.1.0` and `0.2.0-beta.1`, published before this repository became public, were built with Go 1.25,
  whose standard library has known vulnerabilities fixed since.

### Added

- Linux packages for current distributions: an RPM for Fedora and a `.deb` for Ubuntu 22.04+ and Debian 12+, built
  against webkit2gtk-4.1. The RHEL, AlmaLinux and Rocky Linux 8 and 9 RPM is now named `pickrole_<version>_el8_x86_64.rpm`.
  Every release installs each package on its target distribution and checks that the binary finds all its libraries.
- The interface is now in English and Brazilian Portuguese. It follows the system language by default, with English
  as the fallback, and can be changed in **Settings → Preferences → Language**.
- **Settings → Network**: choose the proxy (automatic, manual with exceptions, or none) and test the connection to AWS
  before saving.
- On Linux, the automatic proxy now includes the manual proxy from GNOME's network settings, so PickRole works behind a
  proxy when started from the application menu.
- Documentation: architecture, stack, design, architecture decision records, contributing guide and security policy.

### Changed

- The header shows the active profile (account / role, PROD tag in production) and opens its account on click; it
  says when the role credentials have expired. The footer keeps only errors and the cache time.
- The sign-in screen has a back button: to the accounts or, on first use, to the settings.
- Documentation is now in English, with a Portuguese summary in `README.pt-BR.md`.
- The README screenshot shows the current look.

### Fixed

- Sign-in no longer fails with `InvalidClientException` when AWS stops accepting the saved OIDC client registration:
  PickRole registers again and retries.
- Windows: the `.exe` file properties (Details tab) show the version, description and product name, and the version
  follows the release instead of staying at 0.1.0.
- Release notes now come from this changelog instead of pull request titles.

## [0.1.0] - 2026-09-26

First release, published before this repository became public.

### Features

- Browser sign-in to AWS IAM Identity Center with a device code, and session renewal without the browser.
- Cached account and role list, search (`Ctrl K`), favorites and recent profiles.
- Credentials written to `~/.aws/credentials`, in the `[default]` profile or in one profile per account and role.
- CodeArtifact token written to Maven's `settings.xml`.
- Production accounts highlighted, with confirmation for write roles.
- Configuration that can be exported and imported by a team; detection of an `sso-session` in `~/.aws/config`.
- Proxy from environment variables and, on Windows, from the system proxy (PAC included).
- Opening screen, About dialog with version and build details, and light and dark themes.

### Platforms

- RHEL 8 and 9 (RPM and `.tar.gz`) and Windows 10 and 11 (portable `.exe`).

### Known limitations

- The interface hasn't been checked on Linux (WebKitGTK) or against a real AWS yet; the flow was tested against the
  project's fake AWS.
- On Linux, the GNOME proxy settings aren't read: use `HTTPS_PROXY`.
- The binaries aren't signed.

[Unreleased]: https://github.com/pickrole/pickrole/compare/v0.2.0-beta.3...HEAD
[0.2.0-beta.3]: https://github.com/pickrole/pickrole/compare/v0.2.0-beta.2...v0.2.0-beta.3
[0.2.0-beta.2]: https://github.com/pickrole/pickrole/releases/tag/v0.2.0-beta.2
