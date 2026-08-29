#!/usr/bin/env bash
# SubagentStop: append a machine-written checkpoint so a fresh session can
# resume without exploring the repository. This is the usage-limit safety net.
set -uo pipefail
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=/dev/null
. "$DIR/_json.sh"

INPUT=$(cat)
AGENT=$(json_get "$INPUT" '.agent_type'); AGENT="${AGENT:-unknown}"
CP=".claude/CHECKPOINT.md"
BRANCH=$(git branch --show-current 2>/dev/null || echo "-")
SHA=$(git rev-parse --short HEAD 2>/dev/null || echo "-")
DIRTY=$(git status --porcelain 2>/dev/null | wc -l | tr -d ' ')
ISSUE=$(printf '%s' "$BRANCH" | grep -oE '[0-9]+' | head -1)

mkdir -p .claude
[ -f "$CP" ] || printf '# Checkpoints — machine written. Read by /resume.\n\n' > "$CP"
printf -- '- %s | agent=%s | branch=%s | sha=%s | uncommitted=%s | issue=#%s\n' \
  "$(date -u +%FT%TZ)" "$AGENT" "$BRANCH" "$SHA" "$DIRTY" "${ISSUE:-?}" >> "$CP"

# Keep it small — it is read on every resume.
if [ "$(wc -l < "$CP" 2>/dev/null || echo 0)" -gt 200 ]; then
  { head -2 "$CP"; tail -100 "$CP"; } > "$CP.tmp" && mv "$CP.tmp" "$CP"
fi
exit 0
