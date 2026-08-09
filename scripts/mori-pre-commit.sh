#!/usr/bin/env bash
set -euo pipefail

repository_root=$(git rev-parse --show-toplevel)
cd "$repository_root"

if ! command -v mori >/dev/null 2>&1; then
  echo "Mori is required before committing source changes; install the version in .mori-version." >&2
  exit 1
fi

required_version=$(tr -d '[:space:]' < .mori-version)
reported_version=$(mori version)
installed_version="v$(awk '{print $2}' <<<"$reported_version")"
if [[ "$installed_version" != "$required_version" ]]; then
  echo "Mori version mismatch: project requires $required_version, but '$reported_version' is active." >&2
  exit 1
fi

is_supported_source() {
  local path=$1
  case "$path" in
    *.bash|*.cjs|*.cts|*.go|*.js|*.jsx|*.mjs|*.mts|*.py|*.pyi|*.rs|*.sh|*.swift|*.ts|*.tsx|*.zsh)
      return 0
      ;;
  esac

  local first_line
  if [[ -f "$path" ]] && IFS= read -r first_line < "$path"; then
    case "$first_line" in
      '#!'*'/bash'|'#!'*'/dash'|'#!'*'/sh'|'#!'*'/zsh'|'#!'*'/env bash'|'#!'*'/env dash'|'#!'*'/env sh'|'#!'*'/env zsh'|'#!'*'/env node'|'#!'*'/env nodejs'|'#!'*'/env python'|'#!'*'/env python3')
        return 0
        ;;
    esac
  fi
  return 1
}

staged_sources=()
while IFS= read -r -d '' path; do
  if is_supported_source "$path"; then
    staged_sources+=("$path")
  fi
done < <(git diff --cached --name-only --diff-filter=ACMR -z)

if (( ${#staged_sources[@]} == 0 )); then
  exit 0
fi

for path in "${staged_sources[@]}"; do
  if ! git diff --quiet -- "$path"; then
    echo "Mori cannot review partially staged source: $path" >&2
    echo "Stage the complete file or restore its unstaged changes before committing." >&2
    exit 1
  fi
done

report=$(mktemp "${TMPDIR:-/tmp}/tw-doctor-mori.XXXXXX")
trap 'rm -f "$report"' EXIT

arguments=(scan --format text --fail-on-focused-match)
for path in "${staged_sources[@]}"; do
  arguments+=(--focus-path "$path")
done

set +e
mori "${arguments[@]}" . >"$report" 2>&1
status=$?
set -e

if (( status == 0 )); then
  echo "Mori pre-commit review passed for ${#staged_sources[@]} staged source file(s)."
  exit 0
fi

sed -n '1,2400p' "$report" >&2
if (( status == 3 )); then
  echo "Mori found focused structural matches. Inspect both locations and reuse, refactor, or document why the similarity is intentional." >&2
fi
exit "$status"
