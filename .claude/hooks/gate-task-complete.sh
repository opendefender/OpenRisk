#!/usr/bin/env bash
# TaskCompleted: refuse to let a teammate close a task on a red build.
# Exit 2 blocks completion and returns the reason to the agent.
set -uo pipefail
cat >/dev/null

if command -v go >/dev/null 2>&1 && [ -f go.mod ]; then
  if ! OUT=$(go build ./... 2>&1); then
    echo "Task cannot be completed: 'go build ./...' fails." >&2
    printf '%s\n' "$OUT" | head -20 >&2
    exit 2
  fi
fi
if [ -f package.json ] && command -v npx >/dev/null 2>&1; then
  if ! OUT=$(npx --no-install tsc -b 2>&1); then
    echo "Task cannot be completed: 'tsc -b' fails." >&2
    printf '%s\n' "$OUT" | head -20 >&2
    exit 2
  fi
fi
exit 0
