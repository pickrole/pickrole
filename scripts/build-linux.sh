#!/usr/bin/env bash
# Builds the PickRole binary for Linux, in one of two variants:
#
#   default                  links webkit2gtk-4.0. Build it in an AlmaLinux/Rocky/
#                            RHEL 8 environment (glibc 2.28) so it runs on RHEL 8
#                            and 9; CI uses an almalinux:8 container.
#   WAILS_TAGS=webkit2_41    links webkit2gtk-4.1, for distributions that dropped
#                            4.0 (Fedora 40+, Ubuntu 24.04+, Debian 13). CI builds
#                            it in an ubuntu:22.04 container (glibc 2.35).
#
# See .github/workflows and docs/adr/0010 and 0019.
set -euo pipefail

VERSION="${VERSION:-dev}"
WAILS_VERSION="${WAILS_VERSION:-v2.16.0}"
WAILS_TAGS="${WAILS_TAGS:-}"

# The version goes into -ldflags: letters, digits, dots, hyphens and
# underscores only (0.2.0, 0.2.0-beta.1, a commit hash, dev).
if [[ ! "${VERSION}" =~ ^[0-9A-Za-z._-]+$ ]]; then
  echo "invalid VERSION: ${VERSION}" >&2
  exit 1
fi

cd "$(dirname "$0")/.."

if ! command -v wails >/dev/null 2>&1; then
  go install "github.com/wailsapp/wails/v2/cmd/wails@${WAILS_VERSION}"
  export PATH="$(go env GOPATH)/bin:$PATH"
fi

COMMIT="$(git rev-parse --short=7 HEAD 2>/dev/null || true)"
COMMIT="${COMMIT:-${GITHUB_SHA:0:7}}"
BUILD_DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

args=(build -clean -trimpath
  -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.buildDate=${BUILD_DATE}")
if [[ -n "${WAILS_TAGS}" ]]; then
  args+=(-tags "${WAILS_TAGS}")
fi
wails "${args[@]}"

echo "Binary: build/bin/pickrole (version ${VERSION}, commit ${COMMIT}, ${BUILD_DATE}, tags '${WAILS_TAGS}')"
