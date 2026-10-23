#!/usr/bin/env bash
# Runs one ZAP baseline scan for .github/workflows/dast.yml (#649) and fails
# unless it really covered the API.
#
#   .zap/scan.sh <name> <spec> <target url> <api base url>
#
# Reads ZAP_IMAGE, and ZAP_AUTH_HEADER_VALUE when the scan is signed in (ZAP
# then sends it as the Authorization header on every request).
# ZAP_DOCKER_ARGS replaces "--network host" for a local run under Docker
# Desktop, e.g. "--add-host host.docker.internal:host-gateway". Writes
# zap-report/<name>.{html,json,md,log}.
#
# --autooff keeps zap-baseline on its classic path: every alert on the host
# counts against rules.tsv, informational ones excepted. The automation
# framework path also counts informational classifiers (10049, 10050), which
# describe a response rather than flag a defect.
set -euo pipefail

name=$1 spec=$2 target=$3 api=$4
report=zap-report

# A spec the import silently fails on leaves ZAP with the handful of URLs its
# spider finds, and a green scan of almost nothing. Every documented GET path
# is reached by the import, so that count is the floor.
expected=$(yq '[.paths[] | select(has("get"))] | length' "$report/$spec")

read -ra net <<< "${ZAP_DOCKER_ARGS:---network host}"
auth_env=()
if [ -n "${ZAP_AUTH_HEADER_VALUE:-}" ]; then
  auth_env=(-e ZAP_AUTH_HEADER_VALUE)
fi

set +e
docker run --rm "${net[@]}" "${auth_env[@]}" -v "$PWD/$report:/zap/wrk:rw" "$ZAP_IMAGE" \
  zap-baseline.py --autooff -t "$target" -c rules.tsv -I \
    -z "-openapifile /zap/wrk/$spec -openapitargeturl $api" \
    -r "$name.html" -J "$name.json" -w "$name.md" 2>&1 | tee "$report/$name.log"
rc=${PIPESTATUS[0]}
set -e

urls=$(grep -oE 'Total of [0-9]+ URLs' "$report/$name.log" | grep -oE '[0-9]+' || echo 0)
echo "$name: ZAP exit $rc, $urls URLs scanned, $expected documented GET paths"
if [ "$urls" -lt "$expected" ]; then
  echo "❌ $name: ZAP saw $urls URLs, fewer than the $expected GET paths in $spec; the OpenAPI import did not take"
  exit 1
fi
# 0 clean, 1 a FAIL rule fired, 2 WARN only (turned into 0 by -I), 3 ZAP failed.
exit "$rc"
