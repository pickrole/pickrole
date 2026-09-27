# 0021. Pinned Go toolchain, and vulnerability checks on the binaries

- Status: accepted
- Date: 2026-09-26

## Context

The standard library is compiled into the binary, so its vulnerabilities ship with PickRole. The builds took their Go
version from the `go` line in `go.mod` (the minimum the code needs, `1.25.0`) and from `scripts/setup-el8.sh`
(`1.25.1`), so `0.2.0-beta.1` was built with Go versions a year old. `govulncheck` on its Windows binary reports 45
known vulnerabilities, all in the standard library and all fixed in later Go releases. The `govulncheck` runs before
that were done on a development machine with a current Go, so they didn't show it.

`actions/setup-go` v6+ also stopped letting Go download a newer toolchain by itself (`GOTOOLCHAIN=local`), so the build
must name the version it wants.

## Decision

- `go.mod` keeps `go 1.25.0` as the minimum and adds a `toolchain` line with the Go release the binaries are built
  with. `actions/setup-go` and local builds (through `GOTOOLCHAIN=auto`) use it; `scripts/setup-el8.sh` pins the same
  version with its official SHA-256.
- CI and the release run `govulncheck -mode=binary` on every binary they build. It covers the standard library and the
  dependencies actually linked in, and fails the job on any known vulnerability.
- The toolchain moves to each Go security release.

## Consequences

- Release binaries carry the current standard library.
- A newly published Go vulnerability fails CI and blocks a release until the toolchain is updated, which is the point.
- Updating Go is a two-file change (`go.mod`, `scripts/setup-el8.sh`), described in `CONTRIBUTING.md`.
