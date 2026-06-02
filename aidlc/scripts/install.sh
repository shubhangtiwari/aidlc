#!/usr/bin/env sh
set -eu

repo="${AIDLC_REPO:-shubhangtiwari/aidlc}"
version="${AIDLC_VERSION:-latest}"
install_dir="${AIDLC_INSTALL_DIR:-$HOME/.local/bin}"
tmp_dir="$(mktemp -d)"

cleanup() {
  rm -rf "$tmp_dir"
}
trap cleanup EXIT INT TERM

need() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "aidlc install: required command not found: $1" >&2
    exit 2
  fi
}

need curl
need tar

os="$(uname -s)"
arch="$(uname -m)"
case "$os" in
  Darwin) os="Darwin" ;;
  Linux) os="Linux" ;;
  *) echo "aidlc install: unsupported OS: $os" >&2; exit 2 ;;
esac
case "$arch" in
  x86_64|amd64) arch="x86_64" ;;
  arm64|aarch64) arch="arm64" ;;
  *) echo "aidlc install: unsupported architecture: $arch" >&2; exit 2 ;;
esac

if [ "$version" = "latest" ]; then
  base_url="https://github.com/$repo/releases/latest/download"
else
  base_url="https://github.com/$repo/releases/download/$version"
fi

archive="aidlc_${os}_${arch}.tar.gz"
checksums="checksums.txt"

curl -fsSL "$base_url/$archive" -o "$tmp_dir/$archive"
curl -fsSL "$base_url/$checksums" -o "$tmp_dir/$checksums"

expected="$(awk -v file="$archive" '$2 == file || $2 == "*" file { print $1 }' "$tmp_dir/$checksums" | head -n 1)"
if [ -z "$expected" ]; then
  echo "aidlc install: checksum for $archive not found" >&2
  exit 2
fi

if command -v sha256sum >/dev/null 2>&1; then
  actual="$(sha256sum "$tmp_dir/$archive" | awk '{print $1}')"
elif command -v shasum >/dev/null 2>&1; then
  actual="$(shasum -a 256 "$tmp_dir/$archive" | awk '{print $1}')"
else
  echo "aidlc install: sha256sum or shasum is required" >&2
  exit 2
fi

if [ "$actual" != "$expected" ]; then
  echo "aidlc install: checksum mismatch for $archive" >&2
  exit 2
fi

mkdir -p "$install_dir"
tar -xzf "$tmp_dir/$archive" -C "$tmp_dir" aidlc
install -m 0755 "$tmp_dir/aidlc" "$install_dir/aidlc"

echo "aidlc installed to $install_dir/aidlc"
