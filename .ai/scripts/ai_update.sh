#!/usr/bin/env bash
# Pulls the upstream AIDLC repository and overwrites the local `.ai/` folder
# with the upstream copy. Use --dry-run to preview changes.

set -euo pipefail

UPSTREAM_URL="${AI_DLC_UPSTREAM:-https://github.com/shubhangtiwari/aidlc.git}"
REF="main"
DRY_RUN=0
ASSUME_YES=0

usage() {
  cat <<EOF
usage: $(basename "$0") [--ref <branch|tag|sha>] [--dry-run] [--yes]

Updates the local .ai/ directory from the upstream AIDLC repository.

Options:
  --ref REF    Upstream ref to pull (default: main)
  --dry-run    Show what would change without writing
  --yes, -y    Skip confirmation prompt
  --url URL    Override upstream URL (env: AI_DLC_UPSTREAM)
  -h, --help   Show this help

The upstream URL defaults to:
  $UPSTREAM_URL
EOF
}

while [ $# -gt 0 ]; do
  case "$1" in
    --ref) REF="$2"; shift 2 ;;
    --dry-run) DRY_RUN=1; shift ;;
    --yes|-y) ASSUME_YES=1; shift ;;
    --url) UPSTREAM_URL="$2"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown argument: $1" >&2; usage >&2; exit 2 ;;
  esac
done

command -v git >/dev/null || { echo "git is required" >&2; exit 1; }
command -v rsync >/dev/null || { echo "rsync is required" >&2; exit 1; }

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
cd "$REPO_ROOT"

TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

echo "→ fetching $UPSTREAM_URL @ $REF"
git clone --depth 1 --branch "$REF" --quiet "$UPSTREAM_URL" "$TMPDIR/upstream"

if [ ! -d "$TMPDIR/upstream/.ai" ]; then
  echo "upstream has no .ai/ directory at ref '$REF'" >&2
  exit 1
fi

UPSTREAM_SHA="$(git -C "$TMPDIR/upstream" rev-parse --short HEAD)"
echo "→ upstream commit: $UPSTREAM_SHA"

RSYNC_FLAGS=(-a --delete --itemize-changes)
if [ "$DRY_RUN" -eq 1 ]; then
  RSYNC_FLAGS+=(--dry-run)
  echo "→ dry run — no files will be written"
fi

if [ "$DRY_RUN" -eq 0 ] && [ "$ASSUME_YES" -eq 0 ]; then
  echo
  echo "This will overwrite $REPO_ROOT/.ai/ with the upstream copy ($UPSTREAM_SHA)."
  echo "Local edits inside .ai/ will be lost. Continue? [y/N]"
  read -r reply
  case "$reply" in
    y|Y|yes|YES) ;;
    *) echo "aborted."; exit 1 ;;
  esac
fi

mkdir -p "$REPO_ROOT/.ai"
rsync "${RSYNC_FLAGS[@]}" "$TMPDIR/upstream/.ai/" "$REPO_ROOT/.ai/"

echo
if [ "$DRY_RUN" -eq 1 ]; then
  echo "✓ dry run complete. Re-run without --dry-run to apply."
else
  echo "✓ .ai/ updated from upstream $UPSTREAM_SHA"
  echo "  next: make init <ide>  # regenerate IDE entrypoints"
fi
