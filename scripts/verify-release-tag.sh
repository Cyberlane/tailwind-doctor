#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "usage: $0 vMAJOR.MINOR.PATCH" >&2
  exit 2
fi

release_tag=$1
: "${GITHUB_REPOSITORY:?GITHUB_REPOSITORY is required}"
: "${GITHUB_SHA:?GITHUB_SHA is required}"

reference=$(gh api "repos/$GITHUB_REPOSITORY/git/ref/tags/$release_tag")
if [[ $(jq -r '.object.type' <<<"$reference") != tag ]]; then
  echo "$release_tag is not an annotated tag" >&2
  exit 1
fi

tag_sha=$(jq -r '.object.sha' <<<"$reference")
tag_object=$(gh api "repos/$GITHUB_REPOSITORY/git/tags/$tag_sha")
if [[ $(jq -r '.verification.verified' <<<"$tag_object") != true ]]; then
  echo "$release_tag does not have a GitHub-verified signature" >&2
  jq -r '.verification.reason' <<<"$tag_object" >&2
  exit 1
fi
if [[ $(jq -r '.object.sha' <<<"$tag_object") != "$GITHUB_SHA" ]]; then
  echo "$release_tag does not target workflow commit $GITHUB_SHA" >&2
  exit 1
fi

echo "$release_tag is annotated, signed, GitHub-verified, and targets $GITHUB_SHA."
