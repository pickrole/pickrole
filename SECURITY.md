# Security

PickRole handles AWS credentials. Vulnerability reports are very welcome.

## Reporting

**Don't open a public issue.** Use GitHub's [private vulnerability
reporting](https://github.com/pickrole/pickrole/security/advisories/new) (*Security* tab → *Report a vulnerability*).
Include the version (About → "Copy details"), the operating system and the steps to reproduce.

The project is maintained on a volunteer basis, so a reply may take a few days. Once fixed, the report is published as
an advisory, crediting the reporter if they want.

## Supported versions

Security fixes go into the latest version. Older versions don't get fixes.

## Scope

In scope: leaking credentials, tokens or secrets; writing files where they shouldn't go; injection into the edited
files (`credentials`, `settings.xml`); opening unsafe URLs; and the build and release pipeline.

Out of scope: the fake AWS (`internal/fakeaws`, `cmd/fakeaws`), which is for local testing only, and attacks that
already require control of the user's account on the machine.

## Protections in place

- Credentials and tokens only in files, with `0600` permissions and atomic writes
  ([ADR 0002](docs/adr/0002-credentials-in-files.md)).
- `settings.xml`, which receives the CodeArtifact token, can only be inside the home folder, even following links and
  junctions ([ADR 0013](docs/adr/0013-maven-settings-inside-home.md)).
- Credentials from AWS are validated before reaching a file or the clipboard. On Windows, copies stay out of the
  clipboard history and cloud sync ([ADR 0014](docs/adr/0014-secrets-out-of-clipboard-history.md)).
- The browser only opens `https` URLs without embedded credentials, or `http` on loopback.
- The proxy setting, which travels in shared config files, can't hold a user or password
  ([ADR 0020](docs/adr/0020-proxy-settings-and-gnome.md)).
- CI actions are pinned by commit, and the Go toolchain for the Linux build is checked against a SHA-256
  ([ADR 0015](docs/adr/0015-actions-pinned-by-commit.md)).
- Release binaries are built with the Go toolchain pinned in `go.mod` (`toolchain`) and in `scripts/setup-el8.sh`,
  kept at the latest patch release. CI and the release run `govulncheck` on every binary they build, standard library
  included, and fail on any known vulnerability ([ADR 0021](docs/adr/0021-pinned-go-toolchain.md)).

## Known limitations

- On Linux, copying credentials ("Copy export", "Copy token") is a plain copy: a clipboard manager may keep it.
- On Windows, `0600` doesn't change the file's ACL, which is inherited from the home folder (by default only the owner
  can read it).
- The binaries aren't signed (Authenticode on Windows, GPG for the RPM). Check downloads against the release's
  `SHA256SUMS`. Updates from the app do that check themselves, but the checksum proves the file is intact, not who
  published it: a compromised GitHub account could replace both
  ([ADR 0024](docs/adr/0024-updates-from-the-app.md)).
