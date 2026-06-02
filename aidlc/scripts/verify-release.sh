#!/usr/bin/env sh
set -eu

cd "$(dirname "$0")/.."

test -f go.mod
test ! -f ../go.mod
test ! -f ../go.sum
test -f .goreleaser.yaml
test -f scripts/install.sh
test -f scripts/install.ps1

go test ./...

tmp_dir="$(mktemp -d)"
cleanup() {
  rm -rf "$tmp_dir"
}
trap cleanup EXIT INT TERM

for target in darwin/amd64 darwin/arm64 linux/amd64 linux/arm64 windows/amd64 windows/arm64; do
  goos="${target%/*}"
  goarch="${target#*/}"
  out="$tmp_dir/aidlc-$goos-$goarch"
  if [ "$goos" = "windows" ]; then
    out="$out.exe"
  fi
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" go build \
    -trimpath \
    -ldflags "-s -w -X github.com/aidlc/ai-dlc-template/aidlc/internal/commands.Version=release-check" \
    -o "$out" \
    ./cmd/aidlc
done

if command -v goreleaser >/dev/null 2>&1; then
  goreleaser check
fi

echo "aidlc release check passed"
