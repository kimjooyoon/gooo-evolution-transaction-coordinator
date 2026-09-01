#!/usr/bin/env bash
set -euo pipefail

mode=${1:?usage: verify-release-boundary.sh setting|tag TAG COMMIT|release TAG [COMMIT]}
repo=${GITHUB_REPOSITORY:-kimjooyoon/gooo-evolution-transaction-coordinator}
api_version='X-GitHub-Api-Version: 2026-03-10'

case "$mode" in
  setting)
    response_file=$(mktemp)
    error_file=$(mktemp)
    trap 'rm -f "$response_file" "$error_file"' EXIT
    set +e
    gh api --header "$api_version" "repos/$repo/immutable-releases" >"$response_file" 2>"$error_file"
    status=$?
    set -e
    if [ "$status" -ne 0 ]; then
      error=$(tr '\n' ' ' <"$error_file")
      printf 'release boundary: setting unavailable reason=ADMINISTRATION_READ_UNAVAILABLE status=%s error=%s\n' "$status" "$error" >&2
      exit 1
    fi
    response=$(cat "$response_file")
    enabled=$(jq -er '.enabled' <<<"$response")
    test "$enabled" = true
    ;;
  tag)
    tag=${2:?tag mode requires TAG}
    expected_commit=${3:?tag mode requires COMMIT}
    ref=$(gh api --header "$api_version" "repos/$repo/git/ref/tags/$tag")
    test "$(jq -er '.object.type' <<<"$ref")" = tag
    tag_object=$(jq -er '.object.sha' <<<"$ref")
    tag_data=$(gh api --header "$api_version" "repos/$repo/git/tags/$tag_object")
    test "$(jq -er '.object.sha' <<<"$tag_data")" = "$expected_commit"
    ;;
  release)
    tag=${2:?release mode requires TAG}
    response_file=$(mktemp)
    error_file=$(mktemp)
    trap 'rm -f "$response_file" "$error_file"' EXIT
    set +e
    gh api --header "$api_version" "repos/$repo/releases/tags/$tag" >"$response_file" 2>"$error_file"
    status=$?
    set -e
    if [ "$status" -ne 0 ]; then
      error=$(tr '\n' ' ' <"$error_file")
      printf 'release boundary: release unavailable reason=RELEASE_API_UNAVAILABLE status=%s error=%s\n' "$status" "$error" >&2
      exit 1
    fi
    response=$(cat "$response_file")
    immutable=$(jq -er '.immutable' <<<"$response")
    test "$immutable" = true
    asset_count=$(jq -er '.assets | length' <<<"$response")
    test "$asset_count" -gt 0
    jq -e 'all(.assets[]; (.digest | type == "string" and startswith("sha256:")))' <<<"$response" >/dev/null
    if [ -n "${3:-}" ]; then
      expected_commit=$3
      ref=$(gh api --header "$api_version" "repos/$repo/git/ref/tags/$tag")
      test "$(jq -er '.object.type' <<<"$ref")" = tag
      tag_object=$(jq -er '.object.sha' <<<"$ref")
      tag_data=$(gh api --header "$api_version" "repos/$repo/git/tags/$tag_object")
      test "$(jq -er '.object.sha' <<<"$tag_data")" = "$expected_commit"
    fi
    ;;
  *)
    echo "unknown mode: $mode" >&2
    exit 2
    ;;
esac

printf 'release boundary: %s closed\n' "$mode"
