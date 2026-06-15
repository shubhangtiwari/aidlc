#!/usr/bin/env sh
set -eu

cd "$(dirname "$0")/.."

test -f go.mod
test ! -f ../go.mod
test ! -f ../go.sum
test -f scripts/build-release-assets.sh
test -f scripts/install.sh
test -f scripts/install.ps1
test -f ../.ai/Makefile.inc

if grep -Eq '^[[:space:]]*sudo([[:space:]]|$)' scripts/install.sh; then
  echo "aidlc release check: Unix installer must not invoke sudo internally" >&2
  exit 2
fi
if ! grep -Fq 'install_dir="$(choose_install_dir)"' scripts/install.sh; then
  echo "aidlc release check: Unix installer must choose install dir through discovery helper" >&2
  exit 2
fi
if ! grep -Fq 'ensure_writable_dir "/usr/local/bin"' scripts/install.sh; then
  echo "aidlc release check: Unix installer must prefer writable /usr/local/bin" >&2
  exit 2
fi
if ! grep -Fq 'user_dir="$HOME/.local/bin"' scripts/install.sh; then
  echo "aidlc release check: Unix installer must fall back to user-local ~/.local/bin" >&2
  exit 2
fi
if ! grep -Fq 'dir_on_path "$install_dir"' scripts/install.sh; then
  echo "aidlc release check: Unix installer must check whether the install dir is on PATH" >&2
  exit 2
fi
if ! grep -Fq 'AIDLC_BIN=$installed_path make <target>' scripts/install.sh; then
  echo "aidlc release check: Unix installer must print AIDLC_BIN Make helper guidance" >&2
  exit 2
fi

if ! grep -Fq 'Join-Path (Join-Path $DefaultLocalAppData "Programs") "aidlc\bin"' scripts/install.ps1; then
  echo "aidlc release check: Windows installer must default to a user-local app bin directory" >&2
  exit 2
fi
if ! grep -Fq 'SetEnvironmentVariable("Path", $NewUserPath, "User")' scripts/install.ps1; then
  echo "aidlc release check: Windows installer must update the user PATH when possible" >&2
  exit 2
fi
if ! grep -Fq 'open a new terminal or restart your IDE' scripts/install.ps1; then
  echo "aidlc release check: Windows installer must print restart guidance for PATH changes" >&2
  exit 2
fi
if ! grep -Fq '$env:AIDLC_BIN = $ExecutableLiteral' scripts/install.ps1; then
  echo "aidlc release check: Windows installer must print AIDLC_BIN fallback guidance" >&2
  exit 2
fi

if ! grep -Fq 'AIDLC_RESOLVE = aidlc_bin=' ../.ai/Makefile.inc; then
  echo "aidlc release check: Make helper must use shared aidlc resolution" >&2
  exit 2
fi
if ! grep -Fq 'command -v aidlc' ../.ai/Makefile.inc; then
  echo "aidlc release check: Make helper must resolve aidlc from PATH" >&2
  exit 2
fi
if ! grep -Fq '$$HOME/.local/bin/aidlc' ../.ai/Makefile.inc; then
  echo "aidlc release check: Make helper must check user-local aidlc locations" >&2
  exit 2
fi
if ! grep -Fq '$$LOCALAPPDATA/Programs/aidlc/bin/aidlc.exe' ../.ai/Makefile.inc; then
  echo "aidlc release check: Make helper must check the Windows installer default aidlc.exe location" >&2
  exit 2
fi
if ! grep -q '^ai-doctor:' ../.ai/Makefile.inc; then
  echo "aidlc release check: Make helper must expose ai-doctor" >&2
  exit 2
fi

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

doctor_help="$("$tmp_dir/run/aidlc" doctor --help)"
case "$doctor_help" in
  *"Usage: aidlc doctor [flags]"* ) ;;
  * ) echo "aidlc release check: generated binary did not expose doctor help" >&2; echo "$doctor_help" >&2; exit 2 ;;
esac
case "$doctor_help" in
  *"--dir DIR"* ) ;;
  * ) echo "aidlc release check: doctor help did not document --dir" >&2; echo "$doctor_help" >&2; exit 2 ;;
esac

echo "aidlc release check passed"
