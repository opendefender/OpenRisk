#!/usr/bin/env bash
# PreToolUse(Edit|Write|MultiEdit|NotebookEdit): agents never edit their own rules
# (docs/AGENT_RULES.md rule 5.1, #1277). CLAUDE.md, .claude/settings*.json and the
# hooks, agents and skills change only by a patch the owner applies.
# .claude/agent-memory/ and everything else stay writable.
set -uo pipefail
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=/dev/null
. "$DIR/_json.sh"

INPUT=$(cat)
if ! json_available; then
  echo "OpenRisk guard: neither jq nor python3 is available; edit guards cannot run." >&2
  exit 2   # fail closed
fi

FILE=$(json_get "$INPUT" '.tool_input.file_path')
[ -z "$FILE" ] && FILE=$(json_get "$INPUT" '.tool_input.notebook_path')
[ -z "$FILE" ] && exit 0

case "$FILE" in
  */.claude/agent-memory/*|.claude/agent-memory/*) exit 0 ;;
  */CLAUDE.md|CLAUDE.md|*/.claude/settings*.json|.claude/settings*.json|\
  */.claude/hooks/*|.claude/hooks/*|*/.claude/agents/*|.claude/agents/*|\
  */.claude/skills/*|.claude/skills/*)
    echo "BLOCKED by OpenRisk policy: agents do not edit their own rules ($FILE)." >&2
    echo "Prepare a patch in the scratchpad and ask the owner to apply it (docs/AGENT_RULES.md rule 5.1)." >&2
    exit 2 ;;
esac
exit 0
