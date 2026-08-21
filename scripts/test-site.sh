#!/usr/bin/env bash
set -euo pipefail

if [[ $# -gt 1 ]]; then
  echo "usage: $0 [vMAJOR.MINOR.PATCH]" >&2
  exit 2
fi

repository_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
release_version=$(node -p "require(process.argv[1]).version" "$repository_root/npm/tw-doctor/package.json")
release_tag=${1:-v$release_version}
temporary_root=$(mktemp -d "${TMPDIR:-/tmp}/tw-doctor-site-test.XXXXXX")
trap 'rm -r "$temporary_root"' EXIT

output_directory="$temporary_root/site"
"$repository_root/scripts/build-site.sh" "$release_tag" "$output_directory"

if [[ $(<"$output_directory/release-version.txt") != "$release_tag" ]]; then
  echo "generated release-version.txt does not match $release_tag" >&2
  exit 1
fi
if ! grep -Fq "data-release-version=\"$release_tag\">$release_tag</span>" "$output_directory/index.html"; then
  echo "generated index.html does not identify $release_tag" >&2
  exit 1
fi
if grep -Fq '__TW_DOCTOR_RELEASE_TAG__' "$output_directory/index.html"; then
  echo "generated index.html still contains the release-tag placeholder" >&2
  exit 1
fi

while IFS= read -r relative_file; do
  cmp "$repository_root/site/$relative_file" "$output_directory/$relative_file"
done < <(cd "$repository_root/site" && find . -type f ! -name index.html -print | LC_ALL=C sort)

if "$repository_root/scripts/build-site.sh" "${release_tag#v}" "$temporary_root/invalid" >/dev/null 2>&1; then
  echo "site build accepted a release version without the v prefix" >&2
  exit 1
fi
if "$repository_root/scripts/build-site.sh" "$release_tag" "$output_directory" >/dev/null 2>&1; then
  echo "site build overwrote an existing output directory" >&2
  exit 1
fi

echo "Documentation site build passed for $release_tag."
