#!/usr/bin/env sh
set -eu

cd "$(dirname "$0")/.."

version="${AIDLC_VERSION:-dev}"
dist_dir="${AIDLC_DIST_DIR:-dist}"
tmp_dir="$(mktemp -d)"

cleanup() {
  rm -rf "$tmp_dir"
}
trap cleanup EXIT INT TERM

need() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "aidlc release: required command not found: $1" >&2
    exit 2
  fi
}

need go
need tar
need zip

rm -rf "$dist_dir"
mkdir -p "$dist_dir"
dist_abs="$(cd "$dist_dir" && pwd)"

checksum() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1"
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1"
  else
    echo "aidlc release: sha256sum or shasum is required" >&2
    exit 2
  fi
}

build_target() {
  goos="$1"
  goarch="$2"
  archive_os="$3"
  archive_arch="$4"
  binary="aidlc"
  archive="aidlc_${archive_os}_${archive_arch}.tar.gz"

  package_dir="$tmp_dir/${goos}-${goarch}"
  mkdir -p "$package_dir"
  if [ "$goos" = "windows" ]; then
    binary="aidlc.exe"
    archive="aidlc_${archive_os}_${archive_arch}.zip"
  fi

  echo "building $archive"
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" go build \
    -trimpath \
    -ldflags "-s -w -X github.com/shubhangtiwari/aidlc/aidlc/internal/commands.Version=$version" \
    -o "$package_dir/$binary" \
    ./cmd/aidlc

  if [ "$goos" = "windows" ]; then
    (cd "$package_dir" && zip -q "$dist_abs/$archive" "$binary")
  else
    tar -czf "$dist_abs/$archive" -C "$package_dir" "$binary"
  fi
}

build_target darwin amd64 Darwin x86_64
build_target darwin arm64 Darwin arm64
build_target linux amd64 Linux x86_64
build_target linux arm64 Linux arm64
build_target windows amd64 Windows x86_64
build_target windows arm64 Windows arm64

(
  cd "$dist_abs"
  for artifact in aidlc_*.tar.gz aidlc_*.zip; do
    checksum "$artifact"
  done
) >"$dist_abs/checksums.txt"

echo "aidlc release assets written to $dist_abs"
