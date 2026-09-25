#!/usr/bin/env bash
# Dependency vulnerability gate (#487). One gate, run identically by the
# Dependency gate workflow (PRs, master, daily) and by release.yml before
# anything is published.
#
#   scripts/security/dependency-gate.sh <image-ref>...
#
# Scans, with HIGH and CRITICAL kept:
#   - backend Go modules          trivy fs backend/            (go.mod)
#   - frontend production deps    trivy fs frontend/           (package-lock.json;
#                                 Trivy skips devDependencies by default)
#   - every image given           trivy image <image-ref>      (OS packages + Go binary)
# plus govulncheck on the backend, whose reachable findings block regardless of
# severity. security/vulnerability-exceptions.yaml is applied by `vulngate`.
#
# Needs trivy, govulncheck and go on PATH; the images must be in the local
# Docker daemon. Reports go to $GATE_REPORTS (default: gate-reports/).
# Exits non-zero when the gate fails.
set -euo pipefail

[ "$#" -gt 0 ] || { echo "usage: dependency-gate.sh <image-ref>..." >&2; exit 2; }
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
mkdir -p "${GATE_REPORTS:=gate-reports}"
OUT="$(cd "${GATE_REPORTS}" && pwd)"

cd "${ROOT}"
scan=(--quiet --scanners vuln --severity "HIGH,CRITICAL" --format json --exit-code 0)
reports=()

echo "== trivy: backend Go modules"
trivy fs "${scan[@]}" --output "${OUT}/trivy-backend.json" backend
reports+=(-trivy "${OUT}/trivy-backend.json")

echo "== trivy: frontend production dependencies"
trivy fs "${scan[@]}" --skip-dirs node_modules --output "${OUT}/trivy-frontend.json" frontend
reports+=(-trivy "${OUT}/trivy-frontend.json")

for image in "$@"; do
  echo "== trivy: image ${image}"
  out="${OUT}/trivy-image-$(printf '%s' "${image}" | tr -c 'A-Za-z0-9._-' '_').json"
  trivy image "${scan[@]}" --image-src docker --output "${out}" "${image}"
  reports+=(-trivy "${out}")
done

echo "== govulncheck: backend call graph"
(cd backend && govulncheck -format json ./... > "${OUT}/govulncheck.json")
reports+=(-govulncheck "${OUT}/govulncheck.json")

echo "== vulngate"
(cd backend && go build -o "${OUT}/vulngate" ./cmd/vulngate)
"${OUT}/vulngate" -exceptions security/vulnerability-exceptions.yaml "${reports[@]}"
