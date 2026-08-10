#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "usage: $0 vMAJOR.MINOR.PATCH" >&2
  exit 2
fi
if [[ ! $1 =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "usage: $0 vMAJOR.MINOR.PATCH" >&2
  exit 2
fi

release_tag=$1
release_version=${release_tag#v}
repository_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
release_notes="$repository_root/docs/release-notes/$release_tag.md"

if [[ ! -f $release_notes ]]; then
  echo "missing release notes: $release_notes" >&2
  exit 1
fi

while IFS= read -r manifest; do
  package_name=$(node -p "require(process.argv[1]).name" "$manifest")
  package_version=$(node -p "require(process.argv[1]).version" "$manifest")
  if [[ $package_name == tailwind-doctor-packages ]]; then
    continue
  fi
  if [[ $package_version != "$release_version" ]]; then
    echo "$manifest: version $package_version does not match $release_tag" >&2
    exit 1
  fi
done < <(find "$repository_root/npm" -name package.json -not -path '*/test/*' -print | sort)

extension_manifest="$repository_root/editors/vscode/package.json"
extension_version=$(node -p "require(process.argv[1]).version" "$extension_manifest")
if [[ $extension_version != "$release_version" ]]; then
  echo "$extension_manifest: version $extension_version does not match $release_tag" >&2
  exit 1
fi
if ! grep -Fq "Version=\"$release_version\"" "$repository_root/editors/vscode/extension.vsixmanifest"; then
  echo "VS Code manifest version does not match $release_tag" >&2
  exit 1
fi

launcher="$repository_root/npm/tw-doctor/package.json"
alias_manifest="$repository_root/npm/tailwind-doctor/package.json"
node - "$launcher" "$alias_manifest" "$release_version" <<'NODE'
const launcher = require(process.argv[2]);
const alias = require(process.argv[3]);
const version = process.argv[4];
for (const [name, dependency] of Object.entries(launcher.optionalDependencies)) {
  if (dependency !== version) {
    throw new Error(`${name} is pinned to ${dependency}, want ${version}`);
  }
}
if (alias.dependencies["tw-doctor"] !== version) {
  throw new Error(`tailwind-doctor depends on ${alias.dependencies["tw-doctor"]}, want ${version}`);
}
NODE

echo "All npm packages and release notes match $release_tag."
