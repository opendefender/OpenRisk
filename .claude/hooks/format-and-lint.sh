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
    # Tenant-isolation tripwire (ABSOLUTE RULE 2), on every non-test Go file
    # under backend/internal and backend/pkg: handlers and use cases call
    # database.DB directly too, not only repositories. The logic lives in
    # tenant-tripwire.sh, which the completion gate also runs on the whole
    # branch, so a query added through Bash does not escape it.
    case "$FILE" in
      *_test.go) ;;
      */backend/internal/*|*/backend/pkg/*)
        if ! FOUND=$(bash "$DIR/tenant-tripwire.sh" "$FILE"); then
          {
            echo "TENANT TRIPWIRE — $FILE"
            printf '%s\n' "$FOUND" | head -5
            echo ""
            echo "ABSOLUTE RULE 2: every DB query filters by tenant_id, in the same"
            echo "statement. A tenantID parameter in the func signature does not count."
            echo "If this table has no tenant column, gate it through the parent"
            echo "entity with an ownsX helper and say so explicitly in your report."
            echo "If the query is genuinely tenant-agnostic (migrations, health"
            echo "checks), put the reason right above it: // tenant: <why>"
          } >&2
          exit 2
        fi ;;
    esac ;;
  *.ts|*.tsx|*.js|*.jsx|*.mjs|*.css|*.json)
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
