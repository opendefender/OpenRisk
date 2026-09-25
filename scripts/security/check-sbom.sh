#!/usr/bin/env bash
# Refuse to publish an SBOM nobody can use (#487): each file must be CycloneDX
# JSON listing at least one component, and `trivy sbom` must be able to read it.
#
#   scripts/security/check-sbom.sh <file.cdx.json>...
set -euo pipefail
[ "$#" -gt 0 ] || { echo "usage: check-sbom.sh <file.cdx.json>..." >&2; exit 2; }

status=0
for f in "$@"; do
  if ! count="$(python3 -c '
import json, sys
try:
    d = json.load(open(sys.argv[1]))
except (OSError, ValueError) as e:
    sys.exit(f"unreadable: {e}")
if not isinstance(d, dict) or d.get("bomFormat") != "CycloneDX":
    sys.exit("bomFormat is not CycloneDX")
print(len(d.get("components") or []))
' "$f")"; then
    echo "::error title=SBOM::${f} is not a CycloneDX JSON document"
    status=1
    continue
  fi
  if [ "${count}" -eq 0 ]; then
    echo "::error title=SBOM::${f} lists zero components"
    status=1
    continue
  fi
  if ! trivy sbom --quiet --format json --output /dev/null "$f"; then
    echo "::error title=SBOM::trivy sbom cannot read ${f}"
    status=1
    continue
  fi
  echo "OK ${f}: ${count} components"
done
exit "${status}"
