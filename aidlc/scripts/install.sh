#!/usr/bin/env sh
set -eu

repo="${AIDLC_REPO:-shubhangtiwari/aidlc}"
version="${AIDLC_VERSION:-latest}"
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

usage_failure() {
  echo "aidlc install: $1" >&2
  echo "aidlc install: set AIDLC_INSTALL_DIR to a writable directory on PATH, or create one of:" >&2
  echo "aidlc install:   /usr/local/bin" >&2
  echo "aidlc install:   \$HOME/.local/bin" >&2
  echo "aidlc install: this installer does not invoke sudo; create system directories before rerunning if needed" >&2
  exit 2
}

ensure_writable_dir() {
  dir="$1"
  if [ -z "$dir" ]; then
    return 1
  fi
  if ! mkdir -p "$dir" 2>/dev/null; then
    return 1
  fi
  [ -d "$dir" ] && [ -w "$dir" ]
}

choose_install_dir() {
  if [ "${AIDLC_INSTALL_DIR+x}" = "x" ]; then
    if [ -z "$AIDLC_INSTALL_DIR" ]; then
      usage_failure "AIDLC_INSTALL_DIR is set but empty"
    fi
    if ! ensure_writable_dir "$AIDLC_INSTALL_DIR"; then
      usage_failure "AIDLC_INSTALL_DIR is not a writable directory: $AIDLC_INSTALL_DIR"
    fi
    printf '%s\n' "$AIDLC_INSTALL_DIR"
    return
  fi

  if ensure_writable_dir "/usr/local/bin"; then
    printf '%s\n' "/usr/local/bin"
    return
  fi

  if [ -n "${HOME:-}" ]; then
    user_dir="$HOME/.local/bin"
    if ensure_writable_dir "$user_dir"; then
      printf '%s\n' "$user_dir"
      return
    fi
  fi

  usage_failure "could not create or write to a standard install directory"
}

dir_on_path() {
  dir="$1"
  old_ifs="$IFS"
  IFS=:
  for path_dir in ${PATH:-}; do
    IFS="$old_ifs"
    if [ "$path_dir" = "$dir" ]; then
      return 0
    fi
    IFS=:
  done
  IFS="$old_ifs"
  return 1
}

print_path_guidance() {
  installed_path="$1"
  install_dir="$2"
  echo "aidlc installed to $installed_path"
  if dir_on_path "$install_dir"; then
    echo "Verify with: aidlc --version"
    return
  fi

  echo "aidlc install: warning: $install_dir is not on PATH" >&2
  echo "aidlc install: next steps:" >&2
  echo "aidlc install:   rerun with AIDLC_INSTALL_DIR set to a directory already on PATH" >&2
  echo "aidlc install:   add this line to your shell configuration: export PATH=\"$install_dir:\$PATH\"" >&2
  echo "aidlc install:   or run Make helpers with: AIDLC_BIN=$installed_path make <target>" >&2
}

install_dir="$(choose_install_dir)"

os="$(uname -s)"
arch="$(uname -m)"
case "$os" in
  Darwin) os="darwin" ;;
  Linux) os="linux" ;;
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

download() {
  url="$1"
  output="$2"
  label="$3"
  if ! curl -fsSL "$url" -o "$output"; then
    echo "aidlc install: failed to download $label from $url" >&2
    echo "aidlc install: release assets are required; check AIDLC_REPO=$repo and AIDLC_VERSION=$version" >&2
    echo "aidlc install: for unreleased source checkouts, run: cd aidlc && go install ./cmd/aidlc" >&2
    exit 2
  fi
}

download "$base_url/$archive" "$tmp_dir/$archive" "$archive"
download "$base_url/$checksums" "$tmp_dir/$checksums" "$checksums"

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

print_path_guidance "$install_dir/aidlc" "$install_dir"
