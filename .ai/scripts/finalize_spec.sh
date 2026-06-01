#!/usr/bin/env bash
# Post-merge spec finalization: remove from in-flight tracker, set status implemented, commit.
# Usage: finalize_spec.sh [--dry-run] [--spec PATH] [--branch NAME] [--push]

set -euo pipefail

REPO=""
SPEC_PATH=""
BRANCH_NAME=""
DRY_RUN=0
DO_PUSH=0

die() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

usage() {
  cat <<'EOF'
usage: finalize_spec.sh [options]

  --dry-run       Print actions without writing or committing
  --spec PATH     Governing spec (default: resolve from branch / in-flight / sole spec)
  --branch NAME   Merged feature branch (CI passes merged PR head ref)
  --push          Push commit to origin after local finalize
  -h, --help      Show this help

Runs after a spec's PR is merged: removes the entry from docs/spec/.in-flight.yaml,
sets spec frontmatter status to implemented, and commits on the current branch.
In CI, set CI=true (or use --push) to push the cleanup commit to the default branch.
EOF
}

frontmatter_value() {
  local file="$1"
  local key="$2"

  awk -v want="$key" '
    NR == 1 && $0 == "---" { in_fm = 1; next }
    in_fm && $0 == "---" { exit }
    in_fm {
      line = $0
      sub(/[[:space:]]+#.*/, "", line)
      colon = index(line, ":")
      if (!colon) next
      k = substr(line, 1, colon - 1)
      gsub(/^[[:space:]]+|[[:space:]]+$/, "", k)
      if (k != want) next
      v = substr(line, colon + 1)
      gsub(/^[[:space:]]+|[[:space:]]+$/, "", v)
      if (v ~ /^".*"$/) {
        sub(/^"/, "", v)
        sub(/"$/, "", v)
      }
      print v
      exit
    }
  ' "$file"
}

normalize_spec_path() {
  local raw="$1"
  local rel=""

  [ -n "$raw" ] || return 1
  if [ -f "$raw" ]; then
    rel="${raw#"$REPO"/}"
    rel="${rel#/}"
    printf '%s\n' "$rel"
    return 0
  fi
  if [ -f "$REPO/$raw" ]; then
    rel="${raw#/}"
    printf '%s\n' "$rel"
    return 0
  fi
  return 1
}

spec_from_in_flight_branch() {
  local tracker="$REPO/docs/spec/.in-flight.yaml"
  local branch="$1"
  local spec=""

  [ -n "$branch" ] || return 0
  [ -f "$tracker" ] || return 0

  spec="$(awk -v branch="$branch" '
    BEGIN { in_list = 0; current_spec = "" }
    /^in-flight:/ { in_list = 1; next }
    in_list && /^[[:space:]]*-[[:space:]]*spec:/ {
      current_spec = $0
      sub(/^[[:space:]]*-[[:space:]]*spec:[[:space:]]*/, "", current_spec)
      sub(/#.*/, "", current_spec)
      gsub(/^[[:space:]]+|[[:space:]]+$/, "", current_spec)
      gsub(/^["'\'']|["'\'']$/, "", current_spec)
      next
    }
    in_list && /^[[:space:]]+branch:/ {
      line = $0
      sub(/^[[:space:]]+branch:[[:space:]]*/, "", line)
      sub(/#.*/, "", line)
      gsub(/^[[:space:]]+|[[:space:]]+$/, "", line)
      gsub(/^["'\'']|["'\'']$/, "", line)
      if (line == branch && current_spec != "") {
        print current_spec
        exit
      }
      next
    }
  ' "$tracker")"

  [ -n "$spec" ] && printf '%s\n' "$spec"
}

detect_spec_from_branch() {
  local branch=""

  branch="$(git -C "$REPO" rev-parse --abbrev-ref HEAD 2>/dev/null || true)"
  [ -n "$branch" ] && [ "$branch" != "HEAD" ] || return 0
  spec_from_in_flight_branch "$branch"
}

resolve_spec_path() {
  local candidate=""
  local rel=""
  local count=0

  if [ -n "$SPEC_PATH" ]; then
    rel="$(normalize_spec_path "$SPEC_PATH")" || die "spec not found: $SPEC_PATH"
    printf '%s\n' "$REPO/$rel"
    return 0
  fi

  if [ -n "$BRANCH_NAME" ]; then
    candidate="$(spec_from_in_flight_branch "$BRANCH_NAME" || true)"
    if [ -n "$candidate" ]; then
      rel="$(normalize_spec_path "$candidate")" || die "in-flight spec not found: $candidate"
      printf '%s\n' "$REPO/$rel"
      return 0
    fi
  fi

  candidate="$(detect_spec_from_branch || true)"
  if [ -n "$candidate" ]; then
    rel="$(normalize_spec_path "$candidate")" || die "in-flight spec not found: $candidate"
    printf '%s\n' "$REPO/$rel"
    return 0
  fi

  for candidate in "$REPO"/docs/spec/*.md; do
    [ -f "$candidate" ] || continue
    case "$(basename "$candidate")" in
      README.md) continue ;;
    esac
    count=$((count + 1))
    SPEC_PATH="$candidate"
  done

  [ "$count" -eq 1 ] || die "could not resolve spec; pass --spec docs/spec/<epoch>-<slug>.md"
  printf '%s\n' "$SPEC_PATH"
}

remove_from_in_flight() {
  local tracker="$REPO/docs/spec/.in-flight.yaml"
  local spec_rel="$1"
  local branch="$2"
  local tmp=""

  [ -f "$tracker" ] || return 0

  tmp="$(mktemp)"
  awk -v want_spec="$spec_rel" -v want_branch="$branch" '
    function trim_val(s,    t) {
      t = s
      sub(/#.*/, "", t)
      gsub(/^[[:space:]]+|[[:space:]]+$/, "", t)
      gsub(/^["'\'']|["'\'']$/, "", t)
      return t
    }
    /^in-flight:/ { print; next }
    /^[[:space:]]*-[[:space:]]*spec:/ {
      pending_spec_line = $0
      pending_spec = trim_val(substr($0, index($0, "spec:") + 5))
      pending_branch_line = ""
      pending_branch = ""
      expect_branch = 1
      next
    }
    expect_branch && /^[[:space:]]+branch:/ {
      pending_branch_line = $0
      pending_branch = trim_val(substr($0, index($0, "branch:") + 7))
      omit = 0
      if (want_spec != "" && pending_spec == want_spec) {
        if (want_branch == "" || pending_branch == want_branch) {
          omit = 1
        }
      }
      if (!omit) {
        print pending_spec_line
        print pending_branch_line
      }
      expect_branch = 0
      next
    }
    expect_branch {
      print pending_spec_line
      expect_branch = 0
    }
    { print }
  ' "$tracker" >"$tmp"

  if ! cmp -s "$tracker" "$tmp"; then
    if [ "$DRY_RUN" -eq 1 ]; then
      printf '[dry-run] would update %s (remove in-flight entry)\n' "${tracker#"$REPO"/}"
      rm -f "$tmp"
    else
      mv "$tmp" "$tracker"
      printf '==> updated %s\n' "${tracker#"$REPO"/}"
    fi
  else
    rm -f "$tmp"
    printf 'in-flight: no matching entry to remove\n'
  fi
}

set_spec_status_implemented() {
  local spec_file="$1"
  local status=""
  local tmp=""

  status="$(frontmatter_value "$spec_file" "status")"
  if [ "$status" = "implemented" ]; then
    printf 'spec already implemented: %s\n' "${spec_file#"$REPO"/}"
    return 0
  fi

  if [ "$DRY_RUN" -eq 1 ]; then
    printf '[dry-run] would set status: implemented in %s\n' "${spec_file#"$REPO"/}"
    return 0
  fi

  tmp="$(mktemp)"
  awk '
    BEGIN { in_fm = 0; done = 0 }
    NR == 1 && $0 == "---" { in_fm = 1; print; next }
    in_fm && $0 == "---" { in_fm = 0; print; next }
    in_fm {
      if ($0 ~ /^[[:space:]]*status:[[:space:]]*/) {
        print "status: implemented"
        done = 1
        next
      }
      print
      next
    }
    { print }
    END {
      if (!done) exit 2
    }
  ' "$spec_file" >"$tmp" || die "could not update status in ${spec_file#"$REPO"/}"

  mv "$tmp" "$spec_file"
  printf '==> set status: implemented in %s\n' "${spec_file#"$REPO"/}"
}

commit_finalize() {
  local spec_rel="$1"
  local msg=""

  msg="chore: finalize spec ${spec_rel}"

  if [ "$DRY_RUN" -eq 1 ]; then
    if ! git -C "$REPO" diff --quiet -- docs/spec/ 2>/dev/null || \
       ! git -C "$REPO" diff --cached --quiet -- docs/spec/ 2>/dev/null; then
      printf '[dry-run] would git add docs/spec/ and commit: %s\n' "$msg"
    else
      printf '[dry-run] nothing to commit\n'
    fi
    return 0
  fi

  git -C "$REPO" add docs/spec/
  if git -C "$REPO" diff --cached --quiet; then
    printf 'nothing to commit\n'
    return 0
  fi
  git -C "$REPO" commit -m "$msg"
  printf '==> commit: %s\n' "$(git -C "$REPO" rev-parse --short HEAD)"
}

push_if_requested() {
  if [ "$DRY_RUN" -eq 1 ]; then
    if [ "$DO_PUSH" -eq 1 ]; then
      printf '[dry-run] would git push\n'
    fi
    return 0
  fi

  if [ "$DO_PUSH" -eq 0 ]; then
    return 0
  fi

  git -C "$REPO" push origin HEAD
  printf '==> pushed to origin\n'
}

parse_args() {
  REPO="$(cd "$(dirname "$0")/../.." && pwd)"

  if [ "${CI:-}" = "true" ] || [ "${CI:-}" = "1" ]; then
    DO_PUSH=1
  fi

  while [ "$#" -gt 0 ]; do
    case "$1" in
      --dry-run) DRY_RUN=1; shift ;;
      --push) DO_PUSH=1; shift ;;
      --branch)
        [ "$#" -ge 2 ] || die "--branch requires a name"
        BRANCH_NAME="$2"
        shift 2
        ;;
      --branch=*) BRANCH_NAME="${1#--branch=}"; shift ;;
      --spec)
        [ "$#" -ge 2 ] || die "--spec requires a path"
        SPEC_PATH="$2"
        shift 2
        ;;
      --spec=*) SPEC_PATH="${1#--spec=}"; shift ;;
      -h|--help) usage; exit 0 ;;
      *) die "unknown option '$1'" ;;
    esac
  done
}

main() {
  local spec_file=""
  local spec_rel=""
  local branch_for_removal=""

  parse_args "$@"

  if [ ! -d "$REPO/.git" ]; then
    printf 'skip: not a git repository (%s)\n' "$REPO" >&2
    exit 0
  fi

  spec_file="$(resolve_spec_path)"
  spec_rel="${spec_file#"$REPO"/}"
  branch_for_removal="$BRANCH_NAME"
  if [ -z "$branch_for_removal" ]; then
    branch_for_removal="$(git -C "$REPO" rev-parse --abbrev-ref HEAD 2>/dev/null || true)"
  fi

  printf '==> spec: %s\n' "$spec_rel"
  if [ -n "$branch_for_removal" ] && [ "$branch_for_removal" != "HEAD" ]; then
    printf '==> branch: %s\n' "$branch_for_removal"
  fi

  remove_from_in_flight "$spec_rel" "$branch_for_removal"
  set_spec_status_implemented "$spec_file"
  commit_finalize "$spec_rel"
  push_if_requested

  printf 'done\n'
}

main "$@"
