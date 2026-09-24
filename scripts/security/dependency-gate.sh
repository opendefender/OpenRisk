#!/usr/bin/env bash
# Dependency vulnerability gate (#487). One gate, run identically by the
# Dependency gate workflow (PRs, master, daily) and by release.yml before
# anything is published.
#
#   scripts/security/dependency-gate.sh <image-ref> [report-dir]
#
# Scans, with HIGH and CRITICAL kept:
#   - backend Go modules          trivy fs backend/            (go.mod)
#   - frontend production deps    trivy fs frontend/           (package-lock.json;
#                                 Trivy skips devDependencies by default)
#   - the image that ships        trivy image <image-ref>      (OS packages + Go binary)
# plus govulncheck on the backend, whose reachable findings block regardless of
# severity. security/vulnerability-exceptions.yaml is applied by `vulngate`.
#
# Needs trivy, govulncheck and go on PATH; the image must be in the local
# Docker daemon. Exits non-zero when the gate fails.
set -euo pipefail

IMAGE="${1:?usage: dependency-gate.sh <image-ref> [report-dir]}"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
OUT="$(mkdir -p "${2:-${ROOT}/gate-reports}" && cd "${2:-${ROOT}/gate-reports}" && pwd)"

cd "${ROOT}"
scan=(--quiet --scanners vuln --severity HIGH,CRITICAL --format json --exit-code 0)

echo "== trivy: backend Go modules"
trivy fs "${scan[@]}" --output "${OUT}/trivy-backend.json" backend

echo "== trivy: frontend production dependencies"
trivy fs "${scan[@]}" --skip-dirs node_modules --output "${OUT}/trivy-frontend.json" frontend

echo "== trivy: image ${IMAGE}"
trivy image "${scan[@]}" --image-src docker --output "${OUT}/trivy-image.json" "${IMAGE}"

echo "== govulncheck: backend call graph"
(cd backend && govulncheck -format json ./... > "${OUT}/govulncheck.json")

echo "== vulngate"
(cd backend && go build -o "${OUT}/vulngate" ./cmd/vulngate)
"${OUT}/vulngate" \
  -exceptions security/vulnerability-exceptions.yaml \
  -trivy "${OUT}/trivy-backend.json" \
  -trivy "${OUT}/trivy-frontend.json" \
  -trivy "${OUT}/trivy-image.json" \
  -govulncheck "${OUT}/govulncheck.json"
