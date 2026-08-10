#!/usr/bin/env bash
set -euo pipefail

repository_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
temporary_root=$(mktemp -d "$repository_root/../tw-doctor-release-test.XXXXXX")
trap 'rm -r "$temporary_root"' EXIT

output_directory="$temporary_root/output"
relative_output_directory="../$(basename "$temporary_root")/output"
pack_directory="$temporary_root/packs"
install_directory="$temporary_root/install"
mkdir -p "$pack_directory" "$install_directory"
release_version=$(node -p "require(process.argv[1]).version" "$repository_root/npm/tw-doctor/package.json")
release_tag="v$release_version"

"$repository_root/scripts/build-release.sh" "$release_tag" "$relative_output_directory"
(cd "$output_directory/release" && shasum -a 256 -c SHA256SUMS)

fake_bin="$temporary_root/fake-bin"
mkdir -p "$fake_bin"
cat >"$fake_bin/npm" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

case ${1:-} in
  pack | view) exit 0 ;;
  publish)
    echo "publish should be skipped when the registry version exists" >&2
    exit 1
    ;;
  *) exit 2 ;;
esac
EOF
chmod +x "$fake_bin/npm"
PATH="$fake_bin:$PATH" "$repository_root/scripts/publish-npm.sh" "$relative_output_directory"

case "$(uname -s):$(uname -m)" in
  Darwin:arm64) package_target=darwin-arm64 ;;
  Darwin:x86_64) package_target=darwin-x64 ;;
  Linux:aarch64 | Linux:arm64) package_target=linux-arm64 ;;
  Linux:x86_64) package_target=linux-x64 ;;
  *)
    echo "release launcher smoke is not supported on $(uname -s)/$(uname -m)" >&2
    exit 2
    ;;
esac

binary="$output_directory/npm/@tw-doctor/$package_target/bin/tw-doctor"
if [[ $($binary --version) != "tw-doctor $release_version" ]]; then
  echo "release binary did not report tw-doctor $release_version" >&2
  exit 1
fi

for package in \
  "$output_directory/npm/@tw-doctor/$package_target" \
  "$output_directory/npm/tw-doctor" \
  "$output_directory/npm/tailwind-doctor"; do
  npm pack --silent --pack-destination "$pack_directory" "$package" >/dev/null
done

(cd "$install_directory" && \
  npm init -y >/dev/null && \
  npm install --ignore-scripts --omit=optional "$pack_directory"/*.tgz >/dev/null && \
  [[ $(./node_modules/.bin/tw-doctor --version) == "tw-doctor $release_version" ]] && \
  [[ $(./node_modules/.bin/tailwind-doctor --version) == "tw-doctor $release_version" ]])

echo "Release archives and npm launchers passed."
