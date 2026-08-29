#!/usr/bin/env bash
# PreToolUse(Bash): block irreversible or owner-only commands, whoever asks.
set -uo pipefail
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=/dev/null
. "$DIR/_json.sh"

INPUT=$(cat)

if ! json_available; then
  echo "OpenRisk guard: neither jq nor python3 is available; command guards" >&2
  echo "cannot be evaluated reliably. Install jq: sudo apt install -y jq" >&2
  exit 2   # fail closed — a guard that cannot read its input blocks.
fi

CMD=$(json_get "$INPUT" '.tool_input.command')
[ -z "$CMD" ] && exit 0
block() { echo "BLOCKED by OpenRisk policy: $1" >&2; exit 2; }

printf '%s' "$CMD" | grep -qE 'git[[:space:]]+push[[:space:]]+(-f|--force)'   && block "force push"
printf '%s' "$CMD" | grep -qE 'gh[[:space:]]+pr[[:space:]]+merge'             && block "merging is the owner's decision"
printf '%s' "$CMD" | grep -qE 'gh[[:space:]]+release[[:space:]]+(create|delete)' && block "releases are the owner's decision"
printf '%s' "$CMD" | grep -qE 'rm[[:space:]]+-[a-z]*r[a-z]*f?[[:space:]]+(/|~|\$HOME)([[:space:]]|$)' && block "destructive rm"
printf '%s' "$CMD" | grep -qE 'terraform[[:space:]]+apply'                    && block "terraform apply needs human approval"
printf '%s' "$CMD" | grep -qE 'kubectl[[:space:]]+delete'                     && block "kubectl delete needs human approval"
printf '%s' "$CMD" | grep -qiE 'DROP[[:space:]]+(TABLE|DATABASE|SCHEMA)'      && block "destructive DDL"
printf '%s' "$CMD" | grep -qE '(curl|wget)[^|]*\|[[:space:]]*(ba)?sh'         && block "pipe-to-shell"
printf '%s' "$CMD" | grep -qE '^[[:space:]]*(ls[[:space:]]+-[a-zA-Z]*R|find[[:space:]]+/[[:space:]])' && block "full-tree scan wastes context — use rg or gh instead"
exit 0
