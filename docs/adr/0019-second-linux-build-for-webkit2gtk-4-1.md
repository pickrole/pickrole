# 0019. A second Linux build for webkit2gtk-4.1

- Status: accepted
- Date: 2026-09-26

## Context

The EL8 build ([ADR 0010](0010-linux-build-on-el8.md)) links webkit2gtk-4.0, which RHEL 8 and 9 ship. Current
distributions dropped webkit2gtk-4.0 and only ship webkit2gtk-4.1 (Fedora 40+, Ubuntu 24.04+, Debian 13), so that
binary doesn't start there. The code itself runs on any of them: the limit is only what the binary links against.
Wails v2 supports webkit2gtk-4.1 with the `webkit2_41` build tag.

## Decision

Build Linux twice:

- **webkit2gtk-4.0**, in an `almalinux:8` container (glibc 2.28): an RPM for RHEL, AlmaLinux and Rocky Linux 8 and 9,
  and a `.tar.gz`.
- **webkit2gtk-4.1**, with `-tags webkit2_41` in an `ubuntu:22.04` container (glibc 2.35): an RPM for Fedora, a `.deb`
  for Ubuntu 22.04+ and Debian 12+, and a `.tar.gz`.

Package names say which distribution they're for (`_el8_`, `_fedora_`, `_amd64.deb`). The **Linux packages** workflow
installs every package in a container of its target distribution (AlmaLinux 8, Rocky Linux 9, Fedora, Ubuntu 22.04 and
24.04, Debian 12) and fails if the binary can't find a library. It runs on every release and on demand.

## Alternatives considered

- **Only webkit2gtk-4.1**: drops RHEL 8, which has no webkit2gtk-4.1.
- **Flatpak or AppImage**: one package for every distribution, but a bigger download and a second packaging system to
  maintain. Worth revisiting if the distribution list keeps growing.

## Consequences

- PickRole installs with the native package manager on RHEL-like systems, Fedora, Ubuntu and Debian.
- The install check proves that packages install and that libraries resolve, not that the window opens; that still
  needs a person on each distribution.
- Two Linux builds per release, and the install checks, cost CI minutes, so they don't run on every pull request.
  Pull requests only compile the webkit2gtk-4.1 variant.
