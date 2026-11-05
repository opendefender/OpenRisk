#!/usr/bin/env bash
# Promote an image that was already built: point <image>:<tag> at the exact
# manifest of <image>:sha-<sha>, then prove the digest did not change (#1245,
# ADR 0009 D1). It never builds. An image that was never built for <sha> is an
# error, not a reason to build one.
#
#   scripts/images/promote.sh <image> <full sha> <tag>
#   scripts/images/promote.sh ghcr.io/opendefender/openrisk-backend "$GITHUB_SHA" staging
#
# The copy is by digest, so a newer push of sha-<sha> between the read and the
# write cannot slip in. --prefer-index=false makes imagetools copy a single
# manifest as it is: without it a single-platform manifest is wrapped in a new
# index, whose digest differs. The final comparison catches that and any other
# rewrite.
set -euo pipefail

usage() { echo "usage: $0 <image> <40-hex sha> <tag>" >&2; exit 2; }
[ "$#" -eq 3 ] || usage
image="$1" sha="$2" tag="$3"

[[ "$sha" =~ ^[0-9a-f]{40}$ ]] || { echo "::error::'$sha' is not a full commit SHA" >&2; exit 2; }
# A tag of the registry grammar, and never one that could pass for a build tag.
[[ "$tag" =~ ^[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}$ && "$tag" != sha-* ]] || { echo "::error::'$tag' is not a promotion tag" >&2; exit 2; }

digest_of() {
  docker buildx imagetools inspect "$1" --format '{{.Manifest.Digest}}'
}

src="${image}:sha-${sha}"
if ! before="$(digest_of "$src" 2>/dev/null)" || [ -z "$before" ]; then
  echo "::error::${src} does not exist. Promotion never builds: the commit must have been built on develop first." >&2
  exit 1
fi

# The sha- tag is mutable. Refuse an image that does not say it was built from
# this commit; it is a floor, not provenance (signed attestations: #1323).
# .Image reads one config: images.yml builds a single platform (linux/amd64).
revision="$(docker buildx imagetools inspect "${image}@${before}" --format '{{json .Image.Config.Labels}}' \
  | jq -r '."org.opencontainers.image.revision" // empty')"
if [ "$revision" != "$sha" ]; then
  echo "::error::${src} is labelled revision '${revision}', not ${sha}" >&2
  exit 1
fi

docker buildx imagetools create --prefer-index=false --tag "${image}:${tag}" "${image}@${before}"

after="$(digest_of "${image}:${tag}")"
echo "${src} ${before}"
echo "${image}:${tag} ${after}"
if [ "$before" != "$after" ]; then
  echo "::error::${image}:${tag} is ${after}, not the promoted ${before}" >&2
  exit 1
fi
echo "OK: ${image}:${tag} is the digest built for ${sha}"
