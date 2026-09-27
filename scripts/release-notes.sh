#!/usr/bin/env bash
# Prints the CHANGELOG.md section of a version: the release notes.
# Fails if the section is missing or empty.
#
# Usage: scripts/release-notes.sh <version>     (for example 0.2.0)
set -euo pipefail

version="${1:?usage: scripts/release-notes.sh <version>}"
cd "$(dirname "$0")/.."

notes=$(awk -v v="${version}" '
  index($0, "## [" v "]") == 1 { found = 1; next }
  found && (index($0, "## [") == 1 || $0 ~ /^\[.*\]: http/) { exit }
  found { print }
' CHANGELOG.md)

if ! grep -q '[^[:space:]]' <<<"${notes}"; then
  echo "CHANGELOG.md has no [${version}] section" >&2
  exit 1
fi
printf '%s\n' "${notes}"
