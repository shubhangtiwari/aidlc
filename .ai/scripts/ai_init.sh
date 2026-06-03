#!/usr/bin/env bash
# Generate IDE/assistant configuration from `.ai/` (and optional project manifest).
#
# The `.ai/` directory is portable governance. Project-specific facts come from an
# optional manifest when present; otherwise generic repo defaults are used.

set -euo pipefail

README_INIT_MARKER="<!-- INIT:BEGIN -->"

REPO=""
IDE=""

HAS_MANIFEST=0
PROJECT_NAME=""
LANGUAGE=""
MANIFEST_PATH=""
SOURCE_ROOT=""
PACKAGE_NAME=""
RUNTIME=""
BUILD_TOOL=""
MODULE_PATH=""

PERSONA_NAMES=()
PERSONA_DESCRIPTIONS=()
PERSONA_BODIES=()

SKILL_NAMES=()
SKILL_DESCRIPTIONS=()
SKILL_BODIES=()

shopt -s nullglob

die() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

usage() {
  printf 'usage: %s [--repo PATH] <claude|codex|cursor|copilot|windsurf|all>\n' "$0"
}

trim() {
  sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//' <<<"$1"
}

strip_yaml_string() {
  local value="$1"
  local first=""
  local last=""

  if [ "${#value}" -ge 2 ]; then
    first="${value:0:1}"
    last="${value:$((${#value} - 1)):1}"
    if { [ "$first" = "'" ] || [ "$first" = '"' ]; } && [ "$first" = "$last" ]; then
      value="${value:1:$((${#value} - 2))}"
    fi
  fi

  printf '%s' "$value"
}

strip_blank_edges() {
  awk '
    {
      lines[NR] = $0
      if ($0 ~ /[^[:space:]]/) {
        if (!first) {
          first = NR
        }
        last = NR
      }
    }
    END {
      if (!first) {
        exit
      }
      for (i = first; i <= last; i++) {
        print lines[i]
      }
    }
  '
}

append_doc() {
  local kind="$1"
  local name="$2"
  local description="$3"
  local body="$4"

  if [ "$kind" = "persona" ]; then
    PERSONA_NAMES+=("$name")
    PERSONA_DESCRIPTIONS+=("$description")
    PERSONA_BODIES+=("$body")
  else
    SKILL_NAMES+=("$name")
    SKILL_DESCRIPTIONS+=("$description")
    SKILL_BODIES+=("$body")
  fi
}

load_doc() {
  local kind="$1"
  local path="$2"
  local first_line=""
  local frontmatter=""
  local body=""
  local name=""
  local description=""
  local line=""
  local key=""
  local value=""

  first_line="$(sed -n '1p' "$path")"
  [ "$first_line" = "---" ] || die "$path: missing YAML frontmatter"

  awk 'NR > 1 && $0 == "---" { found = 1; exit } END { exit found ? 0 : 1 }' "$path" \
    || die "$path: unterminated frontmatter"

  frontmatter="$(awk 'NR == 1 { next } $0 == "---" { exit } { print }' "$path")"
  body="$(
    awk '
      NR == 1 && $0 == "---" {
        frontmatter = 1
        next
      }
      frontmatter && $0 == "---" {
        frontmatter = 0
        body = 1
        next
      }
      body {
        print
      }
    ' "$path" | strip_blank_edges
  )"

  while IFS= read -r line; do
    [ -n "$(trim "$line")" ] || continue
    case "$line" in
      *:*) ;;
      *) die "$path: frontmatter line must be key: value" ;;
    esac

    key="$(trim "${line%%:*}")"
    value="$(trim "${line#*:}")"
    [ -n "$key" ] || die "$path: empty frontmatter key"
    value="$(strip_yaml_string "$value")"

    case "$key" in
      name) name="$value" ;;
      description) description="$value" ;;
    esac
  done <<<"$frontmatter"

  [ -n "$name" ] || name="$(basename "$path" .md)"
  [ -n "$description" ] || die "$path: frontmatter missing required 'description'"

  append_doc "$kind" "$name" "$description" "$body"
}

load_docs() {
  local kind="$1"
  local dir="$2"
  local files=("$dir"/*.md)
  local path=""

  [ "${#files[@]}" -gt 0 ] || die "$dir: no ${kind} files found"
  for path in "${files[@]}"; do
    load_doc "$kind" "$path"
  done
}

toml_get() {
  local file="$1"
  local section="$2"
  local key="$3"

  awk -v want_section="$section" -v want_key="$key" '
    function trim_value(value) {
      gsub(/^[[:space:]]+|[[:space:]]+$/, "", value)
      return value
    }

    /^[[:space:]]*($|#)/ {
      next
    }

    /^[[:space:]]*\[/ {
      line = $0
      sub(/[[:space:]]+#.*/, "", line)
      gsub(/^[[:space:]]*\[+/, "", line)
      gsub(/\]+[[:space:]]*$/, "", line)
      current_section = trim_value(line)
      next
    }

    current_section == want_section {
      line = $0
      sub(/[[:space:]]+#.*/, "", line)
      equals = index(line, "=")
      if (!equals) {
        next
      }
      found_key = trim_value(substr(line, 1, equals - 1))
      if (found_key == want_key) {
        print trim_value(substr(line, equals + 1))
        exit
      }
    }
  ' "$file"
}

toml_scalar() {
  local value="$1"
  local first=""
  local last=""

  value="$(trim "$value")"
  case "$value" in
    \[*\])
      value="${value#\[}"
      value="${value%%,*}"
      value="${value%\]}"
      value="$(trim "$value")"
      ;;
  esac

  if [ "${#value}" -ge 2 ]; then
    first="${value:0:1}"
    last="${value:$((${#value} - 1)):1}"
    if { [ "$first" = "'" ] || [ "$first" = '"' ]; } && [ "$first" = "$last" ]; then
      value="${value:1:$((${#value} - 2))}"
    fi
  fi

  printf '%s' "$value"
}

detect_python_package() {
  local root="$1"
  local candidates=()
  local child=""
  local base=""

  [ -d "$root" ] || return 0
  for child in "$root"/*; do
    [ -d "$child" ] || continue
    base="$(basename "$child")"
    case "$base" in
      .*) continue ;;
    esac
    [ -f "$child/__init__.py" ] && candidates+=("$base")
  done

  if [ "${#candidates[@]}" -eq 1 ]; then
    printf '%s' "${candidates[0]}"
  fi
}

detect_python() {
  local manifest="$REPO/pyproject.toml"
  local name=""
  local requires=""
  local where=""
  local package_root=""

  name="$(toml_scalar "$(toml_get "$manifest" "project" "name")")"
  [ -n "$name" ] || name="$(toml_scalar "$(toml_get "$manifest" "tool.poetry" "name")")"
  [ -n "$name" ] || name="$(basename "$REPO")"

  requires="$(toml_scalar "$(toml_get "$manifest" "project" "requires-python")")"
  [ -n "$requires" ] || requires="$(toml_scalar "$(toml_get "$manifest" "tool.poetry.dependencies" "python")")"

  where="$(toml_scalar "$(toml_get "$manifest" "tool.setuptools.packages.find" "where")")"
  where="${where%/}"
  if [ -n "$where" ]; then
    SOURCE_ROOT="$where"
  elif [ -d "$REPO/src" ]; then
    SOURCE_ROOT="src"
  else
    SOURCE_ROOT="."
  fi

  if [ "$SOURCE_ROOT" = "." ]; then
    package_root="$REPO"
  else
    package_root="$REPO/$SOURCE_ROOT"
  fi

  PROJECT_NAME="$name"
  LANGUAGE="Python"
  MANIFEST_PATH="pyproject.toml"
  PACKAGE_NAME="$(detect_python_package "$package_root")"
  RUNTIME="$requires"
  BUILD_TOOL="pyproject"
  MODULE_PATH=""
}

json_top_string() {
  local file="$1"
  local key="$2"

  sed -nE 's/^[[:space:]]*"'"$key"'"[[:space:]]*:[[:space:]]*"([^"]*)".*/\1/p' "$file" | head -n 1
}

json_engines_node() {
  local file="$1"

  awk '
    /"engines"[[:space:]]*:/ {
      in_engines = 1
    }
    in_engines {
      print
      if ($0 ~ /}/) {
        exit
      }
    }
  ' "$file" | sed -nE 's/.*"node"[[:space:]]*:[[:space:]]*"([^"]*)".*/\1/p' | head -n 1
}

detect_node() {
  local manifest="$REPO/package.json"
  local name=""
  local runtime=""
  local package_manager=""

  name="$(json_top_string "$manifest" "name")"
  [ -n "$name" ] || name="$(basename "$REPO")"

  runtime="$(json_engines_node "$manifest")"
  package_manager="$(json_top_string "$manifest" "packageManager")"

  PROJECT_NAME="$name"
  PACKAGE_NAME="$name"
  if [ -f "$REPO/tsconfig.json" ] || grep -Eq '"typescript"[[:space:]]*:' "$manifest"; then
    LANGUAGE="TypeScript / Node"
  else
    LANGUAGE="JavaScript / Node"
  fi
  RUNTIME="$runtime"
  SOURCE_ROOT="."
  [ -d "$REPO/src" ] && SOURCE_ROOT="src"
  MANIFEST_PATH="package.json"
  BUILD_TOOL="${package_manager:-npm-compatible}"
  MODULE_PATH=""
}

detect_go() {
  local manifest="$REPO/go.mod"
  local line=""
  local stripped=""
  local module_path=""
  local runtime=""

  while IFS= read -r line; do
    stripped="$(trim "$line")"
    case "$stripped" in
      module\ *) module_path="${stripped#module }" ;;
      go\ *) runtime="${stripped#go }" ;;
    esac
  done <"$manifest"

  PROJECT_NAME="${module_path##*/}"
  [ -n "$PROJECT_NAME" ] || PROJECT_NAME="$(basename "$REPO")"
  PACKAGE_NAME=""
  MODULE_PATH="$module_path"
  LANGUAGE="Go"
  RUNTIME="$runtime"
  SOURCE_ROOT="."
  MANIFEST_PATH="go.mod"
  BUILD_TOOL="go"
}

xml_project_child() {
  local file="$1"
  local tag="$2"

  if command -v xmllint >/dev/null 2>&1; then
    xmllint --xpath "string(/*[local-name()='project']/*[local-name()='$tag'][1])" "$file" 2>/dev/null || true
    return 0
  fi

  sed '/<parent>/,/<\/parent>/d' "$file" \
    | sed -nE 's|.*<'"$tag"'[^>]*>([^<]+)</'"$tag"'>.*|\1|p' \
    | head -n 1
}

xml_parent_child() {
  local file="$1"
  local tag="$2"

  if command -v xmllint >/dev/null 2>&1; then
    xmllint --xpath "string(/*[local-name()='project']/*[local-name()='parent']/*[local-name()='$tag'][1])" "$file" 2>/dev/null || true
    return 0
  fi

  sed -n '/<parent>/,/<\/parent>/p' "$file" \
    | sed -nE 's|.*<'"$tag"'[^>]*>([^<]+)</'"$tag"'>.*|\1|p' \
    | head -n 1
}

xml_property() {
  local file="$1"
  local tag="$2"
  local tag_regex=""

  if command -v xmllint >/dev/null 2>&1; then
    xmllint --xpath "string(/*[local-name()='project']/*[local-name()='properties']/*[local-name()='$tag'][1])" "$file" 2>/dev/null || true
    return 0
  fi

  tag_regex="${tag//./[.]}"
  sed -nE 's|.*<'"$tag_regex"'[^>]*>([^<]+)</'"$tag_regex"'>.*|\1|p' "$file" | head -n 1
}

detect_maven() {
  local artifact_id=""
  local group_id=""
  local runtime=""
  local package_name=""

  artifact_id="$(xml_project_child "$REPO/pom.xml" "artifactId")"
  [ -n "$artifact_id" ] || artifact_id="$(basename "$REPO")"

  group_id="$(xml_project_child "$REPO/pom.xml" "groupId")"
  [ -n "$group_id" ] || group_id="$(xml_parent_child "$REPO/pom.xml" "groupId")"

  if [ -n "$group_id" ]; then
    package_name="${group_id}.${artifact_id}"
  else
    package_name="$artifact_id"
  fi
  package_name="${package_name//-/.}"

  runtime="$(xml_property "$REPO/pom.xml" "java.version")"
  [ -n "$runtime" ] || runtime="$(xml_property "$REPO/pom.xml" "maven.compiler.release")"

  PROJECT_NAME="$artifact_id"
  PACKAGE_NAME="$package_name"
  MODULE_PATH=""
  LANGUAGE="Java / Kotlin"
  RUNTIME="$runtime"
  SOURCE_ROOT="src/main/java"
  [ -d "$REPO/src/main/kotlin" ] && SOURCE_ROOT="src/main/kotlin"
  MANIFEST_PATH="pom.xml"
  BUILD_TOOL="maven"
}

first_regex_group() {
  local file="$1"
  local regex="$2"

  sed -nE 's/.*'"$regex"'.*/\1/p' "$file" | head -n 1
}

detect_gradle() {
  local manifest_name="build.gradle"
  local build_file=""
  local settings_file=""
  local name=""
  local group=""
  local language="Java"
  local runtime=""
  local package_name=""
  local candidate=""

  [ -f "$REPO/build.gradle.kts" ] && manifest_name="build.gradle.kts"
  build_file="$REPO/$manifest_name"

  for candidate in "$REPO/settings.gradle.kts" "$REPO/settings.gradle"; do
    if [ -f "$candidate" ]; then
      settings_file="$candidate"
      break
    fi
  done

  if [ -n "$settings_file" ]; then
    name="$(first_regex_group "$settings_file" "rootProject[.]name[[:space:]]*=[[:space:]]*['\\\"]([^'\\\"]+)['\\\"]")"
  fi
  [ -n "$name" ] || name="$(basename "$REPO")"

  group="$(first_regex_group "$build_file" "group[[:space:]]*=[[:space:]]*['\\\"]([^'\\\"]+)['\\\"]")"
  if grep -Eq 'org[.]jetbrains[.]kotlin|kotlin\(' "$build_file"; then
    language="Kotlin"
  fi

  runtime="$(first_regex_group "$build_file" "JavaVersion[.]VERSION_([0-9]+)")"
  [ -n "$runtime" ] || runtime="$(first_regex_group "$build_file" "jvmToolchain[(]([0-9]+)[)]")"

  if [ -n "$group" ]; then
    package_name="${group}.${name}"
  else
    package_name="$name"
  fi
  package_name="${package_name//-/.}"

  PROJECT_NAME="$name"
  PACKAGE_NAME="$package_name"
  MODULE_PATH=""
  LANGUAGE="$language / Gradle"
  RUNTIME="$runtime"
  SOURCE_ROOT="src/main/java"
  [ "$language" = "Kotlin" ] && SOURCE_ROOT="src/main/kotlin"
  MANIFEST_PATH="$manifest_name"
  BUILD_TOOL="gradle"
}

detect_rust() {
  local manifest="$REPO/Cargo.toml"
  local name=""
  local edition=""

  name="$(toml_scalar "$(toml_get "$manifest" "package" "name")")"
  [ -n "$name" ] || name="$(basename "$REPO")"
  edition="$(toml_scalar "$(toml_get "$manifest" "package" "edition")")"

  PROJECT_NAME="$name"
  PACKAGE_NAME="${name//-/_}"
  MODULE_PATH=""
  LANGUAGE="Rust"
  RUNTIME=""
  [ -n "$edition" ] && RUNTIME="edition $edition"
  SOURCE_ROOT="src"
  MANIFEST_PATH="Cargo.toml"
  BUILD_TOOL="cargo"
}

detect_generic_defaults() {
  HAS_MANIFEST=0
  PROJECT_NAME="$(basename "$REPO")"
  LANGUAGE=""
  MANIFEST_PATH=""
  PACKAGE_NAME=""
  RUNTIME=""
  BUILD_TOOL=""
  MODULE_PATH=""
  if [ -d "$REPO/src" ]; then
    SOURCE_ROOT="src"
  else
    SOURCE_ROOT="."
  fi
}

detect_project_facts() {
  if [ -f "$REPO/pyproject.toml" ]; then
    detect_python
    HAS_MANIFEST=1
  elif [ -f "$REPO/package.json" ]; then
    detect_node
    HAS_MANIFEST=1
  elif [ -f "$REPO/go.mod" ]; then
    detect_go
    HAS_MANIFEST=1
  elif [ -f "$REPO/pom.xml" ]; then
    detect_maven
    HAS_MANIFEST=1
  elif [ -f "$REPO/build.gradle.kts" ] || [ -f "$REPO/build.gradle" ]; then
    detect_gradle
    HAS_MANIFEST=1
  elif [ -f "$REPO/Cargo.toml" ]; then
    detect_rust
    HAS_MANIFEST=1
  else
    detect_generic_defaults
  fi
}

load_sources() {
  local ai_dir="$REPO/.ai"

  [ -d "$ai_dir" ] || die "$REPO: missing .ai directory"
  detect_project_facts
  load_docs "persona" "$ai_dir/personas"
  load_docs "skill" "$ai_dir/skills"
}

announce_written() {
  local path="$1"
  local rel="${path#"$REPO"/}"

  printf 'wrote %s\n' "$rel"
}

ensure_parent_dir() {
  mkdir -p "$(dirname "$1")"
}

json_string() {
  local value="$1"

  value="${value//\\/\\\\}"
  value="${value//\"/\\\"}"
  value="${value//$'\n'/\\n}"
  printf '"%s"' "$value"
}

selected_ides_json() {
  case "$IDE" in
    all)
      printf '["claude","codex","cursor","copilot","windsurf"]'
      ;;
    claude|codex|cursor|copilot|windsurf)
      printf '['
      json_string "$IDE"
      printf ']'
      ;;
    *)
      die "unknown IDE '$IDE'. Choose from: claude, codex, cursor, copilot, windsurf, all"
      ;;
  esac
}

record_workspace_lock() {
  local lock_path="$REPO/aidlc.lock.json"
  local legacy_path="$REPO/.aidlc/manifest.json"
  local source_path=""
  local temp_path=""
  local python_bin=""
  local status=0

  if [ -f "$lock_path" ]; then
    source_path="$lock_path"
  elif [ -f "$legacy_path" ]; then
    source_path="$legacy_path"
  fi

  if command -v python3 >/dev/null 2>&1; then
    python_bin="python3"
  elif command -v python >/dev/null 2>&1; then
    python_bin="python"
  else
    die "python3 is required to record aidlc.lock.json"
  fi

  temp_path="$(mktemp "$REPO/.aidlc-lock.XXXXXX")"
  set +e
  AIDLC_LOCK_SOURCE="$source_path" \
    AIDLC_LOCK_TARGET="$lock_path" \
    AIDLC_LOCK_TEMP="$temp_path" \
    AIDLC_LOCK_IDES="$(selected_ides_json)" \
    "$python_bin" - <<'PY'
import json
import os
import sys

source = os.environ.get("AIDLC_LOCK_SOURCE", "")
target = os.environ["AIDLC_LOCK_TARGET"]
temp = os.environ["AIDLC_LOCK_TEMP"]
ides = json.loads(os.environ["AIDLC_LOCK_IDES"])

data = {}
if source:
    with open(source, "r") as handle:
        loaded = json.load(handle)
    if not isinstance(loaded, dict):
        sys.stderr.write("error: target lock must contain a JSON object\n")
        sys.exit(1)
    data = loaded

if not data.get("schema_version"):
    data["schema_version"] = 1

workspace = data.get("workspace")
if not isinstance(workspace, dict):
    workspace = {}
workspace["ides"] = ides
data["workspace"] = workspace

with open(temp, "w") as handle:
    json.dump(data, handle, indent=2, sort_keys=False)
    handle.write("\n")
if hasattr(os, "replace"):
    os.replace(temp, target)
else:
    os.rename(temp, target)
PY
  status="$?"
  set -e
  if [ "$status" -ne 0 ]; then
    rm -f "$temp_path"
    exit "$status"
  fi
  announce_written "$lock_path"
}

toml_basic_string() {
  local value="$1"

  value="${value//\\/\\\\}"
  value="${value//\"/\\\"}"
  printf '"%s"' "$value"
}

toml_multiline_string() {
  local value="$1"

  value="${value//\"\"\"/\\\"\"\"}"
  printf '"""\n%s\n"""' "$value"
}

render_project_facts_block() {
  printf '## Project facts\n\n'
  printf '%s\n' "- Project: \`$PROJECT_NAME\`"
  if [ "$HAS_MANIFEST" -eq 1 ]; then
    [ -z "$LANGUAGE" ] || printf '%s\n' "- Language: $LANGUAGE"
    printf '%s\n' "- Manifest: \`$MANIFEST_PATH\`"
  else
    printf '%s\n' "- Manifest: not detected (optional — re-run \`make init <ide>\` after adding one)"
  fi
  if [ "$SOURCE_ROOT" = "." ]; then
    printf '%s\n' "- Source root: repository root"
  else
    printf '%s\n' "- Source root: \`$SOURCE_ROOT/\`"
  fi
  [ -z "$PACKAGE_NAME" ] || printf '%s\n' "- Package/import namespace: \`$PACKAGE_NAME\`"
  [ -z "$MODULE_PATH" ] || printf '%s\n' "- Module path: \`$MODULE_PATH\`"
  [ -z "$RUNTIME" ] || printf '%s\n' "- Runtime/version constraint: \`$RUNTIME\`"
  [ -z "$BUILD_TOOL" ] || printf '%s\n' "- Build tool: \`$BUILD_TOOL\`"
  printf '%s\n' "- Architecture and layer rules: see \`docs/ARCHITECTURE.md\` and \`docs/architecture/\`."
  printf '%s\n' "- Module contracts and read-only paths: see \`docs/blueprints/\`."
  printf '%s\n\n' "- Execute via \`Makefile\` only."
}

render_readme_shared_body() {
  local readme="$REPO/.ai/README.md"

  grep -Fq "$README_INIT_MARKER" "$readme" || die "$readme: missing $README_INIT_MARKER"
  awk -v marker="$README_INIT_MARKER" '
    !found {
      if (index($0, marker)) {
        found = 1
      }
      next
    }

    found {
      if (!started && $0 == "") {
        next
      }
      if (!started && $0 ~ /^<!--/) {
        in_comment = 1
        if ($0 ~ /-->/) {
          in_comment = 0
        }
        next
      }
      if (in_comment) {
        if ($0 ~ /-->/) {
          in_comment = 0
        }
        next
      }
      if (!started && $0 == "") {
        next
      }
      started = 1
      print
    }
  ' "$readme"
}

render_intro() {
  local ide="$1"

  if [ "$HAS_MANIFEST" -eq 1 ]; then
    printf '<!-- generated from .ai/ + %s -- do not edit by hand. Run `make init %s` to regenerate. -->\n\n' "$MANIFEST_PATH" "$ide"
  else
    printf '<!-- generated from .ai/ -- do not edit by hand. Run `make init %s` to regenerate. -->\n\n' "$ide"
  fi
  printf '# AI Governance — %s\n\n' "$PROJECT_NAME"
  printf 'Source of truth: `.ai/` for portable guidance, `docs/` for architecture and contracts, and optional project manifests for toolchain facts. This file is generated.\n\n'
  render_project_facts_block
}

MODEL_DEFAULTS=""

load_model_defaults() {
  MODEL_DEFAULTS="$REPO/.ai/models.defaults.toml"
  [ -f "$MODEL_DEFAULTS" ] || MODEL_DEFAULTS=""
}

persona_model() {
  local ide="$1"
  local persona="$2"

  [ -n "$MODEL_DEFAULTS" ] || return 0
  toml_scalar "$(toml_get "$MODEL_DEFAULTS" "${ide}.${persona}" "model")"
}

persona_reasoning() {
  local ide="$1"
  local persona="$2"

  [ -n "$MODEL_DEFAULTS" ] || return 0
  toml_scalar "$(toml_get "$MODEL_DEFAULTS" "${ide}.${persona}" "reasoning")"
}

write_codex_agent() {
  local index="$1"
  local name="${PERSONA_NAMES[$index]}"
  local description="${PERSONA_DESCRIPTIONS[$index]}"
  local body="${PERSONA_BODIES[$index]}"
  local path="$REPO/.codex/agents/$name.toml"
  local model=""
  local reasoning=""

  model="$(persona_model codex "$name")"
  reasoning="$(persona_reasoning codex "$name")"

  ensure_parent_dir "$path"
  {
    printf 'name = %s\n' "$(toml_basic_string "$name")"
    printf 'description = %s\n' "$(toml_basic_string "$description")"
    [ -z "$model" ] || printf 'model = %s\n' "$(toml_basic_string "$model")"
    [ -z "$reasoning" ] || printf 'model_reasoning_effort = %s\n' "$(toml_basic_string "$reasoning")"
    case "$name" in
      architect|reviewer) printf 'sandbox_mode = "read-only"\n' ;;
    esac
    printf 'developer_instructions = %s\n' "$(toml_multiline_string "$body")"
  } >"$path"
  announce_written "$path"
}

write_project_skill() {
  local skills_root="$1"
  local index="$2"
  local name="${SKILL_NAMES[$index]}"
  local description="${SKILL_DESCRIPTIONS[$index]}"
  local body="${SKILL_BODIES[$index]}"
  local path="$REPO/$skills_root/$name/SKILL.md"

  ensure_parent_dir "$path"
  {
    printf -- '---\n'
    printf 'name: %s\n' "$name"
    printf 'description: %s\n' "$(json_string "$description")"
    printf -- '---\n\n'
    printf '%s\n' "$body"
  } >"$path"
  announce_written "$path"
}

gen_codex() {
  local path=""
  local i=0

  for ((i = 0; i < ${#PERSONA_NAMES[@]}; i++)); do
    write_codex_agent "$i"
  done

  for ((i = 0; i < ${#SKILL_NAMES[@]}; i++)); do
    write_project_skill ".codex/skills" "$i"
  done

  path="$REPO/AGENTS.md"
  ensure_parent_dir "$path"
  {
    render_intro "codex"
    render_readme_shared_body
    printf '\n## Codex Agents\n\n'
    printf 'Codex project instructions in `AGENTS.md` shape the main session. Delegable custom agents live under `.codex/agents/`.\n\n'
    for ((i = 0; i < ${#PERSONA_NAMES[@]}; i++)); do
      printf '%s\n' "- \`${PERSONA_NAMES[$i]}\` — \`.codex/agents/${PERSONA_NAMES[$i]}.toml\`"
    done
    printf '\n## Codex Skills\n\n'
    printf 'Agent Skills under `.codex/skills/<name>/SKILL.md` (from `.ai/skills/` via `make init codex`).\n\n'
    for ((i = 0; i < ${#SKILL_NAMES[@]}; i++)); do
      printf '%s\n' "- \`${SKILL_NAMES[$i]}\` — \`.codex/skills/${SKILL_NAMES[$i]}/SKILL.md\` — ${SKILL_DESCRIPTIONS[$i]}"
    done
    printf '\n## Persona Reference\n\n'
    for ((i = 0; i < ${#PERSONA_NAMES[@]}; i++)); do
      printf '### Persona — %s\n\n%s\n\n' "${PERSONA_NAMES[$i]}" "${PERSONA_BODIES[$i]}"
    done
    printf '## Skill Reference\n\n'
    for ((i = 0; i < ${#SKILL_NAMES[@]}; i++)); do
      printf '### Skill — %s\n\n%s\n\n' "${SKILL_NAMES[$i]}" "${SKILL_BODIES[$i]}"
    done
  } >"$path"
  announce_written "$path"
}

gen_unified() {
  local ide="$1"
  local path="$2"
  local i=0

  ensure_parent_dir "$path"
  {
    render_intro "$ide"
    render_readme_shared_body
    printf '\n## Personas\n\n'
    for ((i = 0; i < ${#PERSONA_NAMES[@]}; i++)); do
      printf '### Persona — %s\n\n%s\n\n' "${PERSONA_NAMES[$i]}" "${PERSONA_BODIES[$i]}"
    done
    printf '## Skills\n\n'
    for ((i = 0; i < ${#SKILL_NAMES[@]}; i++)); do
      printf '### Skill — %s\n\n%s\n\n' "${SKILL_NAMES[$i]}" "${SKILL_BODIES[$i]}"
    done
  } >"$path"
  announce_written "$path"
}

gen_copilot() {
  gen_unified "copilot" "$REPO/.github/copilot-instructions.md"
}

gen_windsurf() {
  gen_unified "windsurf" "$REPO/.windsurfrules"
}

gen_claude() {
  local path=""
  local i=0
  local model=""

  for ((i = 0; i < ${#PERSONA_NAMES[@]}; i++)); do
    path="$REPO/.claude/agents/${PERSONA_NAMES[$i]}.md"
    model="$(persona_model claude "${PERSONA_NAMES[$i]}")"
    ensure_parent_dir "$path"
    {
      printf -- '---\n'
      printf 'name: %s\n' "${PERSONA_NAMES[$i]}"
      printf 'description: %s\n' "$(json_string "${PERSONA_DESCRIPTIONS[$i]}")"
      [ -z "$model" ] || printf 'model: %s\n' "$model"
      printf -- '---\n\n'
      printf '%s\n' "${PERSONA_BODIES[$i]}"
    } >"$path"
    announce_written "$path"
  done

  for ((i = 0; i < ${#SKILL_NAMES[@]}; i++)); do
    write_project_skill ".claude/skills" "$i"
  done

  path="$REPO/CLAUDE.md"
  ensure_parent_dir "$path"
  {
    render_intro "claude"
    render_readme_shared_body
    printf '\n## Personas\n\nInvokable as Claude subagents under `.claude/agents/`.\n\n'
    for ((i = 0; i < ${#PERSONA_NAMES[@]}; i++)); do
      printf '%s\n' "- \`${PERSONA_NAMES[$i]}\` — ${PERSONA_DESCRIPTIONS[$i]}"
    done
    printf '\n## Skills\n\nInvokable as Claude skills under `.claude/skills/`.\n\n'
    for ((i = 0; i < ${#SKILL_NAMES[@]}; i++)); do
      printf '%s\n' "- \`${SKILL_NAMES[$i]}\` — ${SKILL_DESCRIPTIONS[$i]}"
    done
  } >"$path"
  announce_written "$path"
}

cursor_governance_globs() {
  if [ "$SOURCE_ROOT" = "." ]; then
    printf '{**/*,tests/**,docs/spec/**,docs/blueprints/**,docs/adr/**,docs/ARCHITECTURE.md,docs/architecture/**}'
  else
    printf '{%s/**,tests/**,docs/spec/**,docs/blueprints/**,docs/adr/**,docs/ARCHITECTURE.md,docs/architecture/**}' "$SOURCE_ROOT"
  fi
}

write_cursor_agent() {
  local index="$1"
  local name="${PERSONA_NAMES[$index]}"
  local description="${PERSONA_DESCRIPTIONS[$index]}"
  local body="${PERSONA_BODIES[$index]}"
  local path="$REPO/.cursor/agents/${name}.md"
  local model=""

  model="$(persona_model cursor "$name")"

  ensure_parent_dir "$path"
  {
    printf -- '---\n'
    printf 'name: %s\n' "$name"
    printf 'description: %s\n' "$(json_string "$description")"
    [ -z "$model" ] || printf 'model: %s\n' "$model"
    printf -- '---\n\n'
    printf '%s\n' "$body"
  } >"$path"
  announce_written "$path"
}

write_cursor_mdc() {
  local path="$1"
  local description="$2"
  local always_apply="$3"
  local globs="$4"
  local body="$5"

  ensure_parent_dir "$path"
  {
    printf -- '---\n'
    printf 'description: %s\n' "$(json_string "$description")"
    [ -z "$globs" ] || printf 'globs: %s\n' "$globs"
    printf 'alwaysApply: %s\n' "$always_apply"
    printf -- '---\n\n'
    printf '%s\n' "$body"
  } >"$path"
  announce_written "$path"
}

# Cursor Agent Skills: copy .ai/skills/*.md → .cursor/skills/<name>/SKILL.md
sync_cursor_project_skills() {
  local ai_skills="$REPO/.ai/skills"
  local cursor_skills="$REPO/.cursor/skills"
  local path=""
  local name=""

  [ -d "$ai_skills" ] || die "$ai_skills: skills directory missing"

  rm -rf "$cursor_skills"
  mkdir -p "$cursor_skills"

  for path in "$ai_skills"/*.md; do
    [ -f "$path" ] || continue
    name="$(basename "$path" .md)"
    ensure_parent_dir "$cursor_skills/$name/SKILL.md"
    cp "$path" "$cursor_skills/$name/SKILL.md"
    announce_written "$cursor_skills/$name/SKILL.md"
  done
}

remove_legacy_cursor_skill_rules() {
  local path=""
  for path in "$REPO/.cursor/rules/skill-"*.mdc; do
    [ -f "$path" ] || continue
    rm -f "$path"
    announce_written "$path (removed — use .cursor/skills/ instead)"
  done
}

remove_legacy_agents_skills_dir() {
  local legacy="$REPO/.agents/skills"
  if [ -d "$legacy" ]; then
    rm -rf "$REPO/.agents"
    announce_written "$REPO/.agents (removed — skills live under .cursor/skills/)"
  fi
}

gen_cursor() {
  local path=""
  local description=""
  local globs=""
  local i=0

  globs="$(cursor_governance_globs)"

  for ((i = 0; i < ${#PERSONA_NAMES[@]}; i++)); do
    write_cursor_agent "$i"
  done

  path="$REPO/.cursor/rules/core.mdc"
  ensure_parent_dir "$path"
  {
    render_intro "cursor"
    render_readme_shared_body
  } >"${path}.body"
  write_cursor_mdc "$path" "Core architecture, spec gate, and main-agent delegation" true "" "$(<"${path}.body")"
  rm -f "${path}.body"

  write_cursor_mdc \
    "$REPO/.cursor/rules/governance-spec-gate.mdc" \
    "Spec gate when editing source, tests, or contract docs — delegate before patching" \
    false \
    "$globs" \
    "$(cat <<EOF
# Governed paths — spec gate

Applies under the source root, \`tests/\`, \`docs/spec/\`, \`docs/blueprints/\`, \`docs/adr/\`,
\`docs/ARCHITECTURE.md\`, and \`docs/architecture/\`.

Portable rules: \`.ai/README.md\` (especially Scope resolution). Parallel waves: skill \`orchestrate-spec\`.

## Main session

- **Do not** edit governed paths when tier is medium, large, or uncertain without delegating.
- **Do** launch \`architect\` to draft scope-local spec file(s), then stop for approval.
  Scope-local specs live at \`docs/spec/<epoch>-<slug>.md\` relative to each resolved AIDLC scope root.
- **Do** apply skill \`classify-change\` in the main session before inline intent, spec, or
  implementer on governed paths (Hard Rule 7). Do not delegate triage to \`architect\`. When triage
  is medium/large/uncertain (\`next: draft-spec\`), delegate \`architect\` for planning.
- **Do** launch \`implementer\` for all governed edits: after \`status: approved\`, or after
  trivial/small intent is confirmed following triage (no spec). Main session does not patch governed source.
- **Do** expect implementer blueprint sanity on every run (update \`docs/blueprints/\` when needed).
- Check \`docs/spec/.in-flight.yaml\` in the owning scope for specs tied to the current branch.

## Review

| Tier | Reviewer |
| --- | --- |
| Trivial / small | Skip unless the user explicitly asks for a review |
| Medium / large (approved spec) | **Required** after implementer — diff vs spec; do not report complete or open a PR until \`reviewer\` runs |

Portable rules: Hard Rule 6 in \`.ai/README.md\`.
EOF
)"

  for ((i = 0; i < ${#PERSONA_NAMES[@]}; i++)); do
    description="Persona - ${PERSONA_NAMES[$i]}: ${PERSONA_DESCRIPTIONS[$i]}"
    write_cursor_mdc \
      "$REPO/.cursor/rules/persona-${PERSONA_NAMES[$i]}.mdc" \
      "$description" \
      false \
      "$globs" \
      "${PERSONA_BODIES[$i]}"
  done

  sync_cursor_project_skills
  remove_legacy_cursor_skill_rules
  remove_legacy_agents_skills_dir

  path="$REPO/AGENTS.md"
  ensure_parent_dir "$path"
  {
    render_intro "cursor"
    render_readme_shared_body
    printf '\n## Cursor agents\n\n'
    printf 'Delegable agents under `.cursor/agents/` (also exposed as Task `subagent_type` when rules are loaded):\n\n'
    for ((i = 0; i < ${#PERSONA_NAMES[@]}; i++)); do
      printf '%s\n' "- \`${PERSONA_NAMES[$i]}\` — \`.cursor/agents/${PERSONA_NAMES[$i]}.md\` — ${PERSONA_DESCRIPTIONS[$i]}"
    done
    printf '\n## Cursor rules\n\n'
    printf '%s\n' '- `core.mdc` — always applied (routing, spec gate, delegation)'
    printf '%s\n' "- \`governance-spec-gate.mdc\` — globs: \`${globs}\`"
    for ((i = 0; i < ${#PERSONA_NAMES[@]}; i++)); do
      printf '%s\n' "- \`persona-${PERSONA_NAMES[$i]}.mdc\` — same globs as governance rule"
    done
    printf '\n## Cursor skills\n\n'
    printf 'Agent Skills under `.cursor/skills/<name>/SKILL.md` (from `.ai/skills/` via `make init cursor`).\n'
    printf 'Invoke with `/` + skill name or let Agent decide from the skill description.\n\n'
    for ((i = 0; i < ${#SKILL_NAMES[@]}; i++)); do
      printf '%s\n' "- \`${SKILL_NAMES[$i]}\` — \`.cursor/skills/${SKILL_NAMES[$i]}/SKILL.md\` — ${SKILL_DESCRIPTIONS[$i]}"
    done
    printf '\nRegenerate after changing `.ai/`: `make init cursor`.\n'
  } >"$path"
  announce_written "$path"
}

run_generator() {
  case "$1" in
    claude) gen_claude ;;
    codex) gen_codex ;;
    cursor) gen_cursor ;;
    copilot) gen_copilot ;;
    windsurf) gen_windsurf ;;
    *) die "unknown IDE '$1'. Choose from: claude, codex, cursor, copilot, windsurf, all" ;;
  esac
}

parse_args() {
  local arg=""

  REPO="$(pwd)"
  while [ "$#" -gt 0 ]; do
    arg="$1"
    case "$arg" in
      --repo)
        [ "$#" -ge 2 ] || die "--repo requires a path"
        REPO="$2"
        shift 2
        ;;
      --repo=*)
        REPO="${arg#--repo=}"
        shift
        ;;
      -h|--help)
        usage
        exit 0
        ;;
      -*)
        die "unknown option '$arg'"
        ;;
      *)
        [ -z "$IDE" ] || die "unexpected argument '$arg'"
        IDE="$arg"
        shift
        ;;
    esac
  done

  [ -n "$IDE" ] || {
    usage >&2
    exit 2
  }
  REPO="$(cd "$REPO" && pwd -P)"
}

main() {
  parse_args "$@"

  case "$IDE" in
    claude|codex|cursor|copilot|windsurf|all) ;;
    *) die "unknown IDE '$IDE'. Choose from: claude, codex, cursor, copilot, windsurf, all" ;;
  esac

  load_sources
  load_model_defaults
  if [ "$IDE" = "all" ]; then
    run_generator "claude"
    run_generator "codex"
    run_generator "cursor"
    run_generator "copilot"
    run_generator "windsurf"
  else
    run_generator "$IDE"
  fi
  record_workspace_lock
}

main "$@"
