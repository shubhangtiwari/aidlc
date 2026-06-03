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

test -f "$tmp_dir/dist/aidlc_darwin_x86_64.tar.gz"
test -f "$tmp_dir/dist/aidlc_darwin_arm64.tar.gz"
test -f "$tmp_dir/dist/aidlc_linux_x86_64.tar.gz"
test -f "$tmp_dir/dist/aidlc_linux_arm64.tar.gz"
test -f "$tmp_dir/dist/aidlc_windows_x86_64.zip"
test -f "$tmp_dir/dist/aidlc_windows_arm64.zip"
test -f "$tmp_dir/dist/checksums.txt"

os="$(uname -s)"
arch="$(uname -m)"
case "$os" in
  Darwin) os="darwin" ;;
  Linux) os="linux" ;;
  *) echo "aidlc release check: unsupported OS: $os" >&2; exit 2 ;;
esac
case "$arch" in
  x86_64|amd64) arch="x86_64" ;;
  arm64|aarch64) arch="arm64" ;;
  *) echo "aidlc release check: unsupported architecture: $arch" >&2; exit 2 ;;
esac

mkdir -p "$tmp_dir/run"
tar -xzf "$tmp_dir/dist/aidlc_${os}_${arch}.tar.gz" -C "$tmp_dir/run" aidlc
if [ "$("$tmp_dir/run/aidlc" version)" != "aidlc release-check" ]; then
  echo "aidlc release check: generated binary did not report release-check version" >&2
  "$tmp_dir/run/aidlc" version >&2
  exit 2
fi

upgrade_help="$("$tmp_dir/run/aidlc" upgrade --help)"
case "$upgrade_help" in
  *"Usage: aidlc upgrade [flags]"* ) ;;
  * ) echo "aidlc release check: generated binary did not expose upgrade help" >&2; echo "$upgrade_help" >&2; exit 2 ;;
esac
case "$upgrade_help" in
  *"--repo owner/repo"* ) ;;
  * ) echo "aidlc release check: upgrade help did not document --repo" >&2; echo "$upgrade_help" >&2; exit 2 ;;
esac
case "$upgrade_help" in
  *"--version latest|TAG"* ) ;;
  * ) echo "aidlc release check: upgrade help did not document --version" >&2; echo "$upgrade_help" >&2; exit 2 ;;
esac
case "$upgrade_help" in
  *"--install-dir DIR"* ) ;;
  * ) echo "aidlc release check: upgrade help did not document --install-dir" >&2; echo "$upgrade_help" >&2; exit 2 ;;
esac
case "$upgrade_help" in
  *"--dry-run"* ) ;;
  * ) echo "aidlc release check: upgrade help did not document --dry-run" >&2; echo "$upgrade_help" >&2; exit 2 ;;
esac

echo "aidlc release check passed"
