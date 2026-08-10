#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 1 || $# -gt 2 ]]; then
  echo "usage: $0 vMAJOR.MINOR.PATCH [OUTPUT_DIRECTORY]" >&2
  exit 2
fi

release_tag=$1
repository_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
output_directory=${2:-"$repository_root/dist"}
release_version=${release_tag#v}

"$repository_root/scripts/check-release-version.sh" "$release_tag"
cd "$repository_root"

if [[ -e $output_directory ]]; then
  echo "output directory already exists: $output_directory" >&2
  exit 2
fi
mkdir -p "$output_directory/release" "$output_directory/npm"

targets=(
  "darwin arm64"
  "darwin amd64"
  "linux arm64"
  "linux amd64"
  "windows amd64"
)

for target in "${targets[@]}"; do
  read -r goos goarch <<<"$target"
  node_arch=$goarch
  [[ $node_arch == amd64 ]] && node_arch=x64
  node_os=$goos
  [[ $node_os == windows ]] && node_os=win32
  package_target="$node_os-$node_arch"
  executable=tw-doctor
  [[ $goos == windows ]] && executable=tw-doctor.exe

  package_directory="$output_directory/npm/@tw-doctor/$package_target"
  mkdir -p "$package_directory/bin"
  cp "$repository_root/npm/platforms/$package_target/package.json" "$package_directory/package.json"
  cp "$repository_root/LICENSE" "$package_directory/LICENSE"
  cp "$repository_root/npm/platforms/README.md" "$package_directory/README.md"

  CGO_ENABLED=0 GOOS=$goos GOARCH=$goarch go build \
    -trimpath \
    -ldflags "-s -w -X github.com/Cyberlane/tailwind-doctor/internal/audit.Version=$release_version" \
    -o "$package_directory/bin/$executable" \
    ./cmd/tw-doctor

  archive_root="$output_directory/archive-$package_target"
  mkdir -p "$archive_root"
  cp "$package_directory/bin/$executable" "$archive_root/$executable"
  cp "$repository_root/LICENSE" "$repository_root/README.md" "$archive_root/"
  if [[ $goos == windows ]]; then
    (cd "$archive_root" && zip -q -X "$output_directory/release/tw-doctor_${release_version}_${goos}_${node_arch}.zip" "$executable" LICENSE README.md)
  else
    tar -czf "$output_directory/release/tw-doctor_${release_version}_${goos}_${node_arch}.tar.gz" -C "$archive_root" "$executable" LICENSE README.md
  fi
  rm -r "$archive_root"
done

for package in tw-doctor tailwind-doctor; do
  mkdir -p "$output_directory/npm/$package"
  cp -R "$repository_root/npm/$package/bin" "$output_directory/npm/$package/bin"
  if [[ -d $repository_root/npm/$package/lib ]]; then
    cp -R "$repository_root/npm/$package/lib" "$output_directory/npm/$package/lib"
  fi
  cp "$repository_root/npm/$package/package.json" "$repository_root/npm/$package/README.md" \
    "$repository_root/LICENSE" "$output_directory/npm/$package/"
done

(cd "$output_directory/release" && shasum -a 256 ./* | LC_ALL=C sort -k2 > SHA256SUMS)

echo "Built release packages in $output_directory."
