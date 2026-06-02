#!/usr/bin/env sh
set -eu

cd "$(dirname "$0")/.."

test -f go.mod
test ! -f ../go.mod
test ! -f ../go.sum
test -f scripts/build-release-assets.sh
test -f scripts/install.sh
test -f scripts/install.ps1

go test ./...

tmp_dir="$(mktemp -d)"
cleanup() {
  rm -rf "$tmp_dir"
}
trap cleanup EXIT INT TERM

AIDLC_VERSION=release-check AIDLC_DIST_DIR="$tmp_dir/dist" scripts/build-release-assets.sh

test -f "$tmp_dir/dist/aidlc_Darwin_x86_64.tar.gz"
test -f "$tmp_dir/dist/aidlc_Darwin_arm64.tar.gz"
test -f "$tmp_dir/dist/aidlc_Linux_x86_64.tar.gz"
test -f "$tmp_dir/dist/aidlc_Linux_arm64.tar.gz"
test -f "$tmp_dir/dist/aidlc_Windows_x86_64.zip"
test -f "$tmp_dir/dist/aidlc_Windows_arm64.zip"
test -f "$tmp_dir/dist/checksums.txt"

echo "aidlc release check passed"
