#!/usr/bin/env bash
set -euo pipefail

repository_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
temporary_root=$(mktemp -d "${TMPDIR:-/tmp}/tw-doctor-neovim-test.XXXXXX")
trap 'rm -r "$temporary_root"' EXIT

go build -trimpath -o "$temporary_root/tw-doctor" "$repository_root/cmd/tw-doctor"

TW_DOCTOR_TEST_BINARY="$temporary_root/tw-doctor" nvim --clean --headless \
  --cmd "set noswapfile" \
  --cmd "lua dofile([[$repository_root/editors/neovim/test.lua]])" \
  --cmd quit
