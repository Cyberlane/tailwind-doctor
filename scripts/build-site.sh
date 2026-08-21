#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 2 || ! $1 =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "usage: $0 vMAJOR.MINOR.PATCH OUTPUT_DIRECTORY" >&2
  exit 2
fi

release_tag=$1
output_directory=$2
repository_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
site_source="$repository_root/site"
version_placeholder=__TW_DOCTOR_RELEASE_TAG__
expected_placeholder_count=3

if [[ $output_directory != /* ]]; then
  output_directory="$repository_root/$output_directory"
fi
case "$output_directory/" in
  "$site_source/"*)
    echo "output directory must be outside the site source: $output_directory" >&2
    exit 2
    ;;
esac
if [[ -e $output_directory ]]; then
  echo "output directory already exists: $output_directory" >&2
  exit 2
fi

placeholder_count=$({ grep -oF "$version_placeholder" "$site_source/index.html" || true; } | wc -l | tr -d '[:space:]')
if [[ $placeholder_count != "$expected_placeholder_count" ]]; then
  echo "site/index.html must contain exactly $expected_placeholder_count $version_placeholder placeholders; found $placeholder_count" >&2
  exit 1
fi

mkdir -p "$output_directory"
cp -R "$site_source/." "$output_directory/"
sed "s/$version_placeholder/$release_tag/g" "$site_source/index.html" >"$output_directory/index.html"
printf '%s\n' "$release_tag" >"$output_directory/release-version.txt"

if grep -Fq "$version_placeholder" "$output_directory/index.html"; then
  echo "generated site still contains the release-tag placeholder" >&2
  exit 1
fi
if ! grep -Fq "data-release-version=\"$release_tag\">$release_tag</span>" "$output_directory/index.html"; then
  echo "generated site does not expose release version $release_tag" >&2
  exit 1
fi

echo "Built documentation site for $release_tag in $output_directory."
