#!/usr/bin/env bash
set -euo pipefail

mode=${1:?usage: verify-release-boundary.sh setting|release TAG}
repo=${GITHUB_REPOSITORY:-kimjooyoon/gooo-evolution-transaction-coordinator}
api_root=${GITHUB_API_URL:-https://api.github.com}
api_version='X-GitHub-Api-Version: 2026-03-10'

case "$mode" in
  setting)
    response=$(gh api --header "$api_version" "$api_root/repos/$repo/immutable-releases")
    enabled=$(jq -er '.enabled' <<<"$response")
    test "$enabled" = true
    ;;
  release)
    tag=${2:?release mode requires TAG}
    response=$(gh api --header "$api_version" "$api_root/repos/$repo/releases/tags/$tag")
    immutable=$(jq -er '.immutable' <<<"$response")
    test "$immutable" = true
    asset_count=$(jq -er '.assets | length' <<<"$response")
    test "$asset_count" -gt 0
    jq -e 'all(.assets[]; (.digest | type == "string" and startswith("sha256:")))' <<<"$response" >/dev/null
    ;;
  *)
    echo "unknown mode: $mode" >&2
    exit 2
    ;;
esac

printf 'release boundary: %s closed\n' "$mode"
