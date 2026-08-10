#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "usage: $0 RELEASE_OUTPUT_DIRECTORY" >&2
  exit 2
fi

repository_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
output_directory=$1
if [[ $output_directory != /* ]]; then
  output_directory="$repository_root/$output_directory"
fi
release_version=$(node -p "require(process.argv[1]).version" "$output_directory/npm/tw-doctor/package.json")

packages=()
while IFS= read -r package; do
  packages+=("$package")
done < <(find "$output_directory/npm/@tw-doctor" -mindepth 1 -maxdepth 1 -type d -print | sort)
packages+=("$output_directory/npm/tw-doctor" "$output_directory/npm/tailwind-doctor")

for package in "${packages[@]}"; do
  package_name=$(node -p "require(process.argv[1]).name" "$package/package.json")
  npm pack --dry-run --json "$package" >/dev/null
  if npm view "$package_name@$release_version" version >/dev/null 2>&1; then
    echo "$package_name@$release_version already exists; leaving the immutable version untouched."
    continue
  fi
  npm publish "$package" --access public
done
