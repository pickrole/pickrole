# 0024. New-version notice and updates from the app

- Status: accepted, amended by [0025](0025-update-notice-in-about.md)
- Date: 2026-09-28

## Context

Releases come out often during the beta, and nothing told users about them: they had to watch the releases page,
download the right package for their system, check it against `SHA256SUMS` and install it by hand.

The two platforms install differently. On Windows PickRole is a portable `.exe` in a folder the user owns. On Linux it
is an RPM or `.deb` in `/usr/bin`, which only root can replace, in two builds (webkit2gtk-4.0 for EL8, webkit2gtk-4.1
for current distributions, [ADR 0019](0019-second-linux-build-for-webkit2gtk-4-1.md)).

## Decision

- **Check** (`internal/update`): at start and twice a day, PickRole lists the releases through the GitHub API, over the
  same HTTP client as AWS, so the configured proxy applies ([ADR 0020](0020-proxy-settings-and-gnome.md)). A beta is
  offered newer betas and final releases; a final release only final releases. Local builds (`dev`) never check.
  **Settings → Preferences → Check for new versions** turns it off.
- **Notice**: a "Version X" badge in the header opens a dialog with the versions, a link to the release notes and
  **Update now**.
- **Download**: only from `https://github.com/pickrole/pickrole/releases/download/`, only the package this
  installation needs, with size limits. It is kept only if its SHA-256 matches the release's `SHA256SUMS`.
- **Install**:
  - Windows: the `.exe` from the `.zip` replaces the running one, which is renamed to `pickrole.exe.old` (a running
    `.exe` can be renamed but not overwritten) and removed on the next start.
  - Linux, packaged install in `/usr/bin`: `pkexec dnf install -y` or `pkexec apt-get install -y`, so the desktop
    asks for the password. The package matches the build (`_el8_` for the EL8 build; `_fedora_` or `.deb`, by the
    package manager, for the webkit2gtk-4.1 one). When `pkexec` is missing, refused or not allowed, the dialog shows the
    `sudo dnf install …` command for the already checked file.
  - Anything else (a `.tar.gz`, another architecture) only gets the link to the release page.
- **Restart**: the new version starts with `--replaces-pid=<old pid>` and waits for the old one to exit, since PickRole
  runs a single instance.

## Alternatives considered

- **Notice only**: simpler, but the manual steps it would leave (pick the package, verify, install) are the ones people
  skip or get wrong.
- **A self-contained updater that overwrites `/usr/bin` without the package manager**: the RPM database would no
  longer match the file, and it would still need root.

## Consequences

- One click on Windows; on Linux, one click and the password, or one command to paste.
- The checksum proves the download is intact and matches what the release lists, not who published it: a compromised
  GitHub account could replace both. Signing the binaries is the next step (see [SECURITY.md](../../SECURITY.md)).
- GitHub's unauthenticated API allows 60 requests an hour per address; two checks a day per user are far below that.
