# 0010. Linux build in an EL8 container

- Status: accepted
- Date: 2026-09-26

## Context

RHEL 8 and 9 are the main Linux targets. A binary built on a newer system needs a newer glibc and won't run on RHEL 8,
which also only has webkit2gtk-4.0.

## Decision

Build the Linux binary in an `almalinux:8` container (glibc 2.28, webkit2gtk-4.0), in CI and for releases. Go is
downloaded and checked against a SHA-256 pinned in `scripts/setup-el8.sh`.

## Consequences

- One binary and one RPM serve RHEL 8 and 9, and other distributions that still ship webkit2gtk-4.0.
- Distributions that only ship webkit2gtk-4.1 (such as Fedora 40+ and Ubuntu 24.04) can't open this binary. Serving
  them needs a second build with the `webkit2_41` tag.
