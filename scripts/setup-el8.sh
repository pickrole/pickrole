#!/usr/bin/env bash
# Installs the build dependencies in an AlmaLinux/Rocky 8 container.
set -euo pipefail

GO_VERSION="${GO_VERSION:-1.27.1}"
# Official checksum from https://go.dev/dl/ for the default version. Pinned
# here, not fetched at build time, so a tampered download cannot also supply
# a matching checksum. Pass GO_SHA256 together with any other GO_VERSION.
if [[ "${GO_VERSION}" == "1.27.1" ]]; then
  GO_SHA256="${GO_SHA256:-63d339f0da5ab53635a56f2490a7984dfe12dfcff22ad749f63edaf590168445}"
fi
: "${GO_SHA256:?set GO_SHA256 for GO_VERSION=${GO_VERSION}}"

dnf install -y dnf-plugins-core
dnf config-manager --set-enabled powertools || dnf config-manager --set-enabled crb || true
dnf install -y git gcc gcc-c++ make tar gzip curl pkgconf-pkg-config gtk3-devel webkit2gtk3-devel

tarball="$(mktemp)"
curl -fsSL -o "${tarball}" "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz"
echo "${GO_SHA256}  ${tarball}" | sha256sum -c -
tar -C /usr/local -xzf "${tarball}"
rm -f "${tarball}"
echo "/usr/local/go/bin" >> "${GITHUB_PATH:-/dev/null}"
echo "$HOME/go/bin" >> "${GITHUB_PATH:-/dev/null}"
