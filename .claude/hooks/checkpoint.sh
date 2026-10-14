#!/usr/bin/env bash
# SubagentStop: append a machine-written checkpoint so a fresh session can
# resume without exploring the repository. This is the usage-limit safety net.
#
# 167 of the first 174 lines said agent=unknown: built-in agents
# (general-purpose, Explore) send no agent_type. It now falls back to the
# agent id, and when even that is missing it writes the payload's keys, so a
# gap shows up instead of hiding behind "unknown" (#956).
set -uo pipefail
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=/dev/null
. "$DIR/_json.sh"

# shellcheck source=/dev/null
. "$DIR/_roots.sh"

INPUT=$(cat)
# Branch and sha come from the checkout the agent worked in (its worktree,
# read from the transcript), not the session's cwd. The file itself stays in
# the project the session runs from, where /resume reads it.
ROOT=$(edited_roots "$INPUT" | grep -vx '!unreadable' | head -1)
PROJECT="${CLAUDE_PROJECT_DIR:-${ROOT:-$PWD}}"
{ [ -n "$ROOT" ] && cd "$ROOT"; } || cd "$PROJECT" || exit 0

AGENT=$(json_get "$INPUT" '.agent_type')
if [ -z "$AGENT" ]; then
  ID=$(json_get "$INPUT" '.agent_id')
  if [ -n "$ID" ]; then
    AGENT="built-in:${ID:0:12}"
  else
    KEYS=$(printf '%s' "$INPUT" | python3 -c 'import json,sys
try: print(",".join(sorted(json.load(sys.stdin))))
except Exception: print("unparsed")' 2>/dev/null)
    AGENT="?(keys:${KEYS:-none})"
  fi
fi

CP=".claude/CHECKPOINT.md"
BRANCH=$(git branch --show-current 2>/dev/null); BRANCH="${BRANCH:-detached}"
SHA=$(git rev-parse --short HEAD 2>/dev/null || echo "-")
DIRTY=$(git status --porcelain 2>/dev/null | wc -l | tr -d ' ')
ISSUE=$(printf '%s' "$BRANCH" | grep -oE '[0-9]{2,}' | head -1)
# A branch with no number (agent, main, a scratch name): take the last issue
# number the recent commits mention, and mark it as inferred.
if [ -z "$ISSUE" ]; then
  ISSUE=$(git log -20 --format=%s 2>/dev/null | grep -oE '\(#[0-9]+\)' | head -1 | tr -d '()#')
  [ -n "$ISSUE" ] && ISSUE="$ISSUE(from commits)"
fi

mkdir -p "$PROJECT/.claude"
CP="$PROJECT/$CP"
[ -f "$CP" ] || printf '# Checkpoints — machine written. Read by /resume.\n\n' > "$CP"
printf -- '- %s | agent=%s | branch=%s | sha=%s | uncommitted=%s | issue=#%s\n' \
  "$(date -u +%FT%TZ)" "$AGENT" "$BRANCH" "$SHA" "$DIRTY" "${ISSUE:-?}" >> "$CP"

# Keep it small — it is read on every resume.
if [ "$(wc -l < "$CP" 2>/dev/null || echo 0)" -gt 200 ]; then
  { head -2 "$CP"; tail -100 "$CP"; } > "$CP.tmp" && mv "$CP.tmp" "$CP"
fi
exit 0
