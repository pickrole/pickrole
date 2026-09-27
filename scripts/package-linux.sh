#!/usr/bin/env bash
# Packages build/bin/pickrole into dist/, for the variant it was built as
# (see scripts/build-linux.sh):
#
#   el8        pickrole_<v>_el8_x86_64.rpm            RHEL, AlmaLinux, Rocky 8 and 9
#              pickrole_<v>_linux_amd64_webkit2gtk-4.0.tar.gz
#   webkit41   pickrole_<v>_fedora_x86_64.rpm         Fedora
#              pickrole_<v>_amd64.deb                 Ubuntu 22.04+, Debian 12+
#              pickrole_<v>_linux_amd64_webkit2gtk-4.1.tar.gz
#
# Usage: scripts/package-linux.sh <el8|webkit41> <version>
set -euo pipefail

variant="${1:?usage: scripts/package-linux.sh <el8|webkit41> <version>}"
version="${2:?usage: scripts/package-linux.sh <el8|webkit41> <version>}"

cd "$(dirname "$0")/.."
test -x build/bin/pickrole || { echo "build/bin/pickrole not found: run scripts/build-linux.sh first" >&2; exit 1; }

if ! command -v nfpm >/dev/null 2>&1; then
  go install github.com/goreleaser/nfpm/v2/cmd/nfpm@v2.47.0
  export PATH="$(go env GOPATH)/bin:$PATH"
fi

export VERSION="${version}"
mkdir -p dist
case "${variant}" in
  el8)
    nfpm package -p rpm -f build/linux/nfpm-el8.yaml -t "dist/pickrole_${version}_el8_x86_64.rpm"
    tar -C build/bin -czf "dist/pickrole_${version}_linux_amd64_webkit2gtk-4.0.tar.gz" pickrole
    ;;
  webkit41)
    nfpm package -p rpm -f build/linux/nfpm-webkit41.yaml -t "dist/pickrole_${version}_fedora_x86_64.rpm"
    nfpm package -p deb -f build/linux/nfpm-webkit41.yaml -t "dist/pickrole_${version}_amd64.deb"
    tar -C build/bin -czf "dist/pickrole_${version}_linux_amd64_webkit2gtk-4.1.tar.gz" pickrole
    ;;
  *)
    echo "unknown variant: ${variant} (use el8 or webkit41)" >&2
    exit 1
    ;;
esac
ls -l dist
