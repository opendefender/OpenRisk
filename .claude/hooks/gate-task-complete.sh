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
# It checks the checkouts the agent actually edited, read from its transcript
# (see _roots.sh), not the session's cwd: /work puts agents in a sibling
# worktree, and the cwd is the main checkout.
#
# The previous version tested for go.mod and package.json at the repo root.
# Neither is the app (the module is in backend/, the root package.json is the
# Playwright harness), so it checked nothing at all (#958).
set -uo pipefail
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=/dev/null
. "$DIR/_json.sh"

INPUT=$(cat)
# shellcheck source=/dev/null
. "$DIR/_roots.sh"

AGENT=$(json_get "$INPUT" '.agent_type')
[ -n "$AGENT" ] || AGENT=$(json_get "$INPUT" '.agent_id')
AGENT="${AGENT:-main}"

# On SubagentStop only the agents that write code are gated. A reviewer or an
# explorer stopping in a shared, dirty checkout must not be blocked for
# changes that are not its own. TaskCompleted (team mode) is always gated.
if [ "$(json_get "$INPUT" '.hook_event_name')" = "SubagentStop" ]; then
  case "$AGENT" in
    backend-go|frontend-react|qa-automation|performance-engineer|devops-sre|cloud-sysadmin|website-dev|motion-designer) ;;
    *) exit 0 ;;
  esac
fi

PINNED_LINT="v1.64.8"
PINNED_RE=${PINNED_LINT#v}; PINNED_RE=${PINNED_RE//./\\.}
FAIL=""
run() { # run <label> <dir> <cmd...>
  local label="$1" dir="$2" out
  shift 2
  if ! out=$(cd "$dir" && "$@" 2>&1); then
    FAIL+=$'\n'"✗ $label"$'\n'"$(printf '%s\n' "$out" | grep -vE 'sqlite3-binding|zTail|^[[:space:]]*[|^]' | tail -25)"$'\n'
  fi
}
existing() { while IFS= read -r f; do [ -f "$f" ] && printf '%s\n' "$f"; done; }

# Check one checkout: what its branch changed against master, with what CI runs.
check_root() {
  local ROOT="$1" BASE CHANGED GO_FILES FE_FILES REL PKGS UNFORMATTED TRIP LINT f out
  cd "$ROOT" || return
  if ! BASE=$(git merge-base HEAD origin/master 2>/dev/null || git merge-base HEAD master 2>/dev/null); then
    FAIL+=$'\n'"✗ $ROOT: no origin/master or master to compare against, so the branch cannot be checked. Run git fetch origin."$'\n'
    return
  fi
  CHANGED=$({ git diff --name-only "$BASE"; git ls-files --others --exclude-standard; } 2>/dev/null | sort -u)
  [ -n "$CHANGED" ] || return
  GO_FILES=$(printf '%s\n' "$CHANGED" | grep -E '^backend/.*\.go$' | existing)
  FE_FILES=$(printf '%s\n' "$CHANGED" | grep -E '^frontend/.*\.(ts|tsx|js|jsx|mjs|css|json)$' | grep -v 'package-lock.json' | existing)

  if [ -n "$GO_FILES" ]; then
    if ! command -v go >/dev/null 2>&1; then
      FAIL+=$'\n'"✗ go is not installed, so the Go changes in $ROOT were not checked."$'\n'
    else
      REL=$(printf '%s\n' "$GO_FILES" | sed 's|^backend/||')
      PKGS=$(printf '%s\n' "$REL" | xargs -n1 dirname | sort -u | sed 's|^|./|')
      # shellcheck disable=SC2086
      UNFORMATTED=$(cd backend && gofmt -l $REL 2>/dev/null)
      [ -z "$UNFORMATTED" ] || FAIL+=$'\n'"✗ gofmt — not formatted:"$'\n'"$UNFORMATTED"$'\n'
      # The tenant tripwire on the whole branch, not only on Edit/Write: a
      # query added through sed or a heredoc, or a tenant filter deleted.
      TRIP=""
      while IFS= read -r f; do
        case "$f" in *_test.go) continue ;; backend/internal/*|backend/pkg/*) ;; *) continue ;; esac
        out=$(bash "$DIR/tenant-tripwire.sh" "$ROOT/$f" "$BASE") || TRIP+="$out"$'\n'
      done <<< "$GO_FILES"
      [ -z "$TRIP" ] || FAIL+=$'\n'"✗ tenant tripwire — add the tenant filter, or // tenant: <why> right above:"$'\n'"$TRIP"
      run "go build ./..." backend go build ./...
      # shellcheck disable=SC2086
      run "go vet (changed packages)" backend go vet $PKGS
      # shellcheck disable=SC2086
      run "go test (changed packages)" backend go test -count=1 $PKGS
      LINT=$(command -v golangci-lint || true)
      [ -n "$LINT" ] || { [ -x "$(go env GOPATH)/bin/golangci-lint" ] && LINT="$(go env GOPATH)/bin/golangci-lint"; }
      if [ -z "$LINT" ]; then
        FAIL+=$'\n'"✗ golangci-lint is not installed, and CI runs it. Install the pinned version:"$'\n'"  go install github.com/golangci/golangci-lint/cmd/golangci-lint@$PINNED_LINT"$'\n'
      elif ! "$LINT" --version 2>/dev/null | grep -qE "version v?${PINNED_RE}( |$)"; then
        FAIL+=$'\n'"✗ golangci-lint is not the version CI pins ($PINNED_LINT): $("$LINT" --version 2>&1 | head -1)"$'\n'"  go install github.com/golangci/golangci-lint/cmd/golangci-lint@$PINNED_LINT"$'\n'
      else
        # shellcheck disable=SC2086
        run "golangci-lint (changed packages)" backend "$LINT" run --timeout=5m --max-issues-per-linter=0 --max-same-issues=0 $PKGS
      fi
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
      FAIL+=$'\n'"✗ $ROOT/frontend/node_modules is missing, so nothing frontend was checked. Run npm ci in frontend/, or symlink the main checkout's node_modules when the lockfiles match."$'\n'
    fi
  fi
}

ROOTS=$(edited_roots "$INPUT")
if printf '%s\n' "$ROOTS" | grep -qx '!unreadable'; then
  FAIL+=$'\n'"✗ could not read the agent transcript, so the gate does not know which checkout to check; nothing was checked."$'\n'
  ROOTS=$(printf '%s\n' "$ROOTS" | grep -vx '!unreadable')
  [ -n "$ROOTS" ] || ROOTS="${CLAUDE_PROJECT_DIR:-$PWD}"
fi
[ -n "$ROOTS" ] || exit 0
while IFS= read -r r; do [ -d "$r" ] && check_root "$r"; done <<< "$ROOTS"
ROOT=$(printf '%s\n' "$ROOTS" | head -1)
cd "$ROOT" || exit 0

# Per-user state, read as a number only: a predictable file in a shared /tmp
# fed to $(( )) could run commands (#958 security review).
STATE_DIR="${XDG_RUNTIME_DIR:-$(git rev-parse --git-common-dir 2>/dev/null || echo "$ROOT/.git")}"
[ "${OPENRISK_GATE_STATE_DIR:-}" ] && STATE_DIR="$OPENRISK_GATE_STATE_DIR"
STATE="$STATE_DIR/openrisk-gate-$(printf '%s' "$ROOTS|$AGENT" | cksum | cut -d' ' -f1)"
if [ -z "$FAIL" ]; then
  rm -f "$STATE"
  exit 0
fi

PREV=$(cat "$STATE" 2>/dev/null)
[[ "$PREV" =~ ^[0-9]+$ ]] || PREV=0
COUNT=$((PREV + 1))
echo "$COUNT" > "$STATE"
if [ "$COUNT" -gt 3 ]; then
  rm -f "$STATE"
  CP_DIR="${CLAUDE_PROJECT_DIR:-$ROOT}/.claude"
  mkdir -p "$CP_DIR"
  printf -- '- %s | GATE RED | agent=%s | roots=%s | stopped after 3 blocked attempts\n' \
    "$(date -u +%FT%TZ)" "$AGENT" "$(printf '%s' "$ROOTS" | tr '\n' ' ')" >> "$CP_DIR/CHECKPOINT.md" 2>/dev/null
  echo "Gate still red after 3 attempts; letting $AGENT stop. Recorded in .claude/CHECKPOINT.md. Report this work as NOT done." >&2
  exit 0
fi

{
  echo "Not done: CI would reject this branch (attempt $COUNT of 3)."
  printf '%s\n' "$FAIL"
  echo "Fix these, then finish. A failure that was already on master goes in your report with its issue number."
} >&2
exit 2
