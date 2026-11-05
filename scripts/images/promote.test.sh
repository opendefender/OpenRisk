#!/usr/bin/env bash
# Tests scripts/images/promote.sh against a throwaway registry (#1245).
#
#   REGISTRY=localhost:5000 scripts/images/promote.test.sh
#
# Needs a registry listening at $REGISTRY (CI runs registry:2 as a service) and
# a buildx builder that can push to it. Covers the two manifest shapes the
# images workflow can produce: an index (buildx with attestations, the default)
# and a single manifest, the one imagetools would rewrite without
# --prefer-index=false.
set -euo pipefail

registry="${REGISTRY:-localhost:5000}"
here="$(cd "$(dirname "$0")" && pwd)"
promote="${here}/promote.sh"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

printf 'FROM scratch\nCOPY f /f\n' > "${work}/Dockerfile"
date +%s%N > "${work}/f"

sha="$(printf '%040d' 0 | tr 0 a)"
fail=0
check() { # <name> <expected exit> <command...>
  local name="$1" want="$2"; shift 2
  local got=0
  "$@" > "${work}/out" 2>&1 || got=$?
  if [ "$got" -eq "$want" ]; then
    echo "ok   ${name}"
  else
    echo "FAIL ${name}: exit ${got}, want ${want}"; sed 's/^/     /' "${work}/out"; fail=1
  fi
}
digest_of() { docker buildx imagetools inspect "$1" --format '{{.Manifest.Digest}}'; }

for shape in index manifest; do
  image="${registry}/promote-test-${shape}"
  extra=()
  [ "$shape" = manifest ] && extra=(--provenance=false --sbom=false)
  docker buildx build --quiet "${extra[@]}" --label "org.opencontainers.image.revision=${sha}" \
    --tag "${image}:sha-${sha}" --push "$work" > /dev/null
  built="$(digest_of "${image}:sha-${sha}")"

  check "${shape}: promote to staging" 0 "$promote" "$image" "$sha" staging
  check "${shape}: staging digest equals sha- digest" 0 test "$(digest_of "${image}:staging")" = "$built"
  check "${shape}: promote to master" 0 "$promote" "$image" "$sha" master
  check "${shape}: master digest equals sha- digest" 0 test "$(digest_of "${image}:master")" = "$built"
done

image="${registry}/promote-test-index"
check "refuses a commit that was never built" 1 "$promote" "$image" "$(printf '%040d' 0 | tr 0 b)" staging
other="$(printf '%040d' 0 | tr 0 d)"
docker buildx build --quiet --label "org.opencontainers.image.revision=${sha}" \
  --tag "${image}:sha-${other}" --push "$work" > /dev/null
check "refuses an image labelled with another commit" 1 "$promote" "$image" "$other" staging
check "refuses a short sha" 2 "$promote" "$image" abc1234 staging
check "refuses to overwrite a sha- tag" 2 "$promote" "$image" "$sha" "sha-$(printf '%040d' 0 | tr 0 c)"
check "refuses a malformed tag" 2 "$promote" "$image" "$sha" 'stag ing'

exit "$fail"
