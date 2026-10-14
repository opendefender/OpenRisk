#!/usr/bin/env bash
# TaskCompleted + SubagentStop: an agent cannot finish on work CI would reject.
#
# It checks what the branch changed, with the commands CI runs on it:
#   backend/  gofmt, go build, go vet, go test and golangci-lint on the
#             changed packages
#   frontend/ tsc -b, the ESLint ratchet, Prettier on the changed files
# An agent that changed nothing (a reviewer, an explorer) passes at once.
#
# Exit 2 blocks the stop and hands the failing output back to the agent. After
# three blocks for the same agent it lets the agent stop, so a gate it cannot
# satisfy never loops forever, and records GATE RED in the checkpoint so the
# next session sees it.
#
# The previous version tested for go.mod and package.json at the repo root.
# Neither is the app (the module is in backend/, the root package.json is the
# Playwright harness), so it checked nothing at all (#958).
set -uo pipefail
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=/dev/null
. "$DIR/_json.sh"

INPUT=$(cat)
ROOT=$(json_get "$INPUT" '.cwd')
{ [ -n "$ROOT" ] && [ -d "$ROOT" ]; } || ROOT="${CLAUDE_PROJECT_DIR:-$PWD}"
ROOT=$(cd "$ROOT" 2>/dev/null && git rev-parse --show-toplevel 2>/dev/null) || exit 0
cd "$ROOT" || exit 0

AGENT=$(json_get "$INPUT" '.agent_type')
[ -n "$AGENT" ] || AGENT=$(json_get "$INPUT" '.agent_id')
AGENT="${AGENT:-main}"

BASE=$(git merge-base HEAD origin/master 2>/dev/null || git merge-base HEAD master 2>/dev/null || echo HEAD)
CHANGED=$({ git diff --name-only "$BASE"; git ls-files --others --exclude-standard; } 2>/dev/null | sort -u)
[ -n "$CHANGED" ] || exit 0

existing() { while IFS= read -r f; do [ -f "$f" ] && printf '%s\n' "$f"; done; }
GO_FILES=$(printf '%s\n' "$CHANGED" | grep -E '^backend/.*\.go$' | existing)
FE_FILES=$(printf '%s\n' "$CHANGED" | grep -E '^frontend/.*\.(ts|tsx|js|jsx|mjs|css|json)$' | grep -v 'package-lock.json' | existing)
[ -n "$GO_FILES$FE_FILES" ] || exit 0

FAIL=""
run() { # run <label> <dir> <cmd...>
  local label="$1" dir="$2" out
  shift 2
  if ! out=$(cd "$dir" && "$@" 2>&1); then
    FAIL+=$'\n'"✗ $label"$'\n'"$(printf '%s\n' "$out" | grep -vE 'sqlite3-binding|zTail|^[[:space:]]*[|^]' | tail -25)"$'\n'
  fi
}

if [ -n "$GO_FILES" ] && command -v go >/dev/null 2>&1; then
  REL=$(printf '%s\n' "$GO_FILES" | sed 's|^backend/||')
  PKGS=$(printf '%s\n' "$REL" | xargs -n1 dirname | sort -u | sed 's|^|./|')
  # shellcheck disable=SC2086
  UNFORMATTED=$(cd backend && gofmt -l $REL 2>/dev/null)
  [ -z "$UNFORMATTED" ] || FAIL+=$'\n'"✗ gofmt — not formatted:"$'\n'"$UNFORMATTED"$'\n'
  run "go build ./..." backend go build ./...
  # shellcheck disable=SC2086
  run "go vet (changed packages)" backend go vet $PKGS
  # shellcheck disable=SC2086
  run "go test (changed packages)" backend go test -count=1 $PKGS
  LINT=$(command -v golangci-lint || true)
  [ -n "$LINT" ] || { [ -x "$(go env GOPATH)/bin/golangci-lint" ] && LINT="$(go env GOPATH)/bin/golangci-lint"; }
  if [ -n "$LINT" ]; then
    # shellcheck disable=SC2086
    run "golangci-lint (changed packages)" backend "$LINT" run --timeout=5m --max-issues-per-linter=0 --max-same-issues=0 $PKGS
  else
    FAIL+=$'\n'"✗ golangci-lint is not installed, and CI runs it. Install the pinned version:"$'\n'"  go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8"$'\n'
  fi
fi

if [ -n "$FE_FILES" ]; then
  if [ -d frontend/node_modules ]; then
    REL=$(printf '%s\n' "$FE_FILES" | sed 's|^frontend/||')
    run "tsc -b --noEmit" frontend npx --no-install tsc -b --noEmit
    run "ESLint ratchet (npm run lint:ceiling)" frontend npm run --silent lint:ceiling
    # shellcheck disable=SC2086
    run "prettier --check (changed files)" frontend npx --no-install prettier --check $REL
  else
    FAIL+=$'\n'"✗ frontend/node_modules is missing, so nothing frontend was checked. Run npm ci in frontend/, or symlink the main checkout's node_modules when the lockfiles match."$'\n'
  fi
fi

STATE="${TMPDIR:-/tmp}/openrisk-gate-$(printf '%s' "$ROOT|$AGENT" | cksum | cut -d' ' -f1)"
if [ -z "$FAIL" ]; then
  rm -f "$STATE"
  exit 0
fi

COUNT=$(( $(cat "$STATE" 2>/dev/null || echo 0) + 1 ))
echo "$COUNT" > "$STATE"
if [ "$COUNT" -gt 3 ]; then
  rm -f "$STATE"
  mkdir -p .claude
  printf -- '- %s | GATE RED | agent=%s | branch=%s | stopped after 3 blocked attempts\n' \
    "$(date -u +%FT%TZ)" "$AGENT" "$(git branch --show-current)" >> .claude/CHECKPOINT.md 2>/dev/null
  echo "Gate still red after 3 attempts; letting $AGENT stop. Recorded in .claude/CHECKPOINT.md. Report this work as NOT done." >&2
  exit 0
fi

{
  echo "Not done: CI would reject this branch (attempt $COUNT of 3)."
  printf '%s\n' "$FAIL"
  echo "Fix these, then finish. A failure that was already on master goes in your report with its issue number."
} >&2
exit 2
