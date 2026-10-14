#!/usr/bin/env bash
# PostToolUse(Edit|Write): format, then trip the tenant-isolation wire.
# Exit 2 returns the message to the agent as a correction it must act on.
set -uo pipefail
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=/dev/null
. "$DIR/_json.sh"

INPUT=$(cat)
FILE=$(json_get "$INPUT" '.tool_input.file_path')
[ -z "$FILE" ] && exit 0
[ -f "$FILE" ] || exit 0

case "$FILE" in
  *.go)
    command -v gofmt     >/dev/null 2>&1 && gofmt -s -w "$FILE"
    command -v goimports >/dev/null 2>&1 && goimports -w "$FILE"
    # Tenant-isolation tripwire on persistence code (ABSOLUTE RULE 2).
    case "$FILE" in
      */infrastructure/*|*repository*|*repo.go)
        # Only lines this edit ADDED are checked (git diff against HEAD; a new
        # file counts as all added). Code that was already there and already
        # reviewed never re-trips the wire, so the escape hatch can be strict.
        #
        # Reported, on an added line, with no tenant reference within three
        # lines either side:
        #   .Where(  .Raw(  .Exec(
        #   .First/.Find/.Take/.Last called with an inline condition, as in
        #   First(&x, id) or Find(&xs, "owner_id = ?", id)
        # The escape hatch is a comment in the two lines above that contains
        # "tenant:" and a reason, e.g. `// tenant: gated by ownsRisk above`.
        # Before #958 any comment at all silenced it, and Raw/Exec/First were
        # never looked at.
        if git -C "$(dirname "$FILE")" ls-files --error-unmatch "$FILE" >/dev/null 2>&1; then
          ADDED=$(git -C "$(dirname "$FILE")" diff -U0 HEAD -- "$FILE" 2>/dev/null \
            | awk '/^@@/ { split($3, a, ","); s = substr(a[1], 2); n = (a[2] == "" ? 1 : a[2]);
                           for (i = 0; i < n; i++) print s + i }')
        else
          ADDED=$(awk '{ print NR }' "$FILE")
        fi

        UNJUSTIFIED=""
        if [ -n "$ADDED" ]; then
          UNJUSTIFIED=$(printf '%s\n' "$ADDED" | awk -v file="$FILE" '
            FNR == NR { added[$1] = 1; next }
            { line[FNR] = $0 }
            END {
              q = "\\.(Where|Raw|Exec)\\(|\\.(First|Find|Take|Last)\\([^,()]+,"
              t = "tenant_id|TenantID|tenantID"
              for (n = 1; n <= FNR; n++) {
                if (!(n in added) || line[n] !~ q || line[n] ~ /^[[:space:]]*\/\//) continue
                ok = 0
                for (k = n - 3; k <= n + 3; k++) if (k >= 1 && line[k] ~ t) ok = 1
                for (k = n - 2; k < n; k++) if (k >= 1 && tolower(line[k]) ~ /^[[:space:]]*\/\/.*tenant:/) ok = 1
                if (!ok) printf "%d:%s\n", n, line[n]
              }
            }' - "$FILE")
        fi

        if [ -n "$UNJUSTIFIED" ]; then
          {
            echo "TENANT TRIPWIRE — $FILE"
            echo "A query you just added has no tenant_id near it and no tenant: comment:"
            printf '%s\n' "$UNJUSTIFIED" | head -5
            echo ""
            echo "ABSOLUTE RULE 2: every DB query filters by tenant_id."
            echo "If this table has no tenant column, gate it through the parent"
            echo "entity with an ownsX helper and say so explicitly in your report."
            echo "If the query is genuinely tenant-agnostic (migrations, health"
            echo "checks) or chained onto a base that already filters by tenant,"
            echo "put the reason on the line above: // tenant: <why>"
          } >&2
          exit 2
        fi ;;
    esac ;;
  *.ts|*.tsx|*.css)
    # Prettier lives in frontend/node_modules. This used to run from the repo
    # root, where `npx --no-install prettier` finds nothing and fails silently,
    # so no agent edit was ever formatted; 62 unformatted files reached master
    # that way (#957, #958).
    FE="${FILE%%/frontend/*}/frontend"
    if [ "$FE" != "$FILE/frontend" ] && [ -x "$FE/node_modules/.bin/prettier" ]; then
      (cd "$FE" && ./node_modules/.bin/prettier --write "$FILE" >/dev/null 2>&1) || true
    fi ;;
esac
exit 0
