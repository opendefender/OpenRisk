#!/usr/bin/env bash
# SubagentStop: rewrite the line of the checkout the agent worked in, in
# .claude/STATE.md, so a fresh session can resume without exploring the
# repository. This is the usage-limit safety net. It used to append to
# CHECKPOINT.md, whose last 30 lines were mostly the same branch and sha
# repeated (#1007).
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
# shellcheck source=/dev/null
. "$DIR/_state.sh"

INPUT=$(cat)
# Branch and sha come from the checkout the agent worked in (its worktree,
# read from the transcript), not the session's cwd.
ROOT=$(edited_roots "$INPUT" | grep -vx '!unreadable' | head -1)
SESSION=$(json_get "$INPUT" '.cwd')
PROJECT="${CLAUDE_PROJECT_DIR:-${SESSION:-$PWD}}"
PROJECT=$(git -C "$PROJECT" rev-parse --show-toplevel 2>/dev/null || printf '%s' "$PROJECT")
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

state_write "$PWD" "$AGENT"
exit 0
