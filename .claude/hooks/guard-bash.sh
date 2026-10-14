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
# gh api -f (long form --raw-field) never expands @file: it posts the literal
# path, answers 200, and has overwritten issue bodies and checkpoint comments
# that way. -F (long form --field) reads the file and stays allowed.
# Covers -fkey=@x, --raw-field=key=@x and array keys.
printf '%s' "$CMD" | grep -qE 'gh[[:space:]]+api.*(^|[[:space:]])(-f|--raw-field)([[:space:]]+|=)?["'"'"']?[^[:space:]=]+=@' \
  && block "gh api -f key=@file posts the literal path — use -F key=@file"

# Never send a secret file to GitHub. -F body=@file, --body-file and -F <file>
# are the normal way to post, so an instruction planted in an issue could
# otherwise get .env, a key or the generated admin password published, around
# the Read deny rules (#959 security reviews).
# This matches paths, so it is an aid, not a control: --body "$(cat .env)" or
# --body-file - still get past it. Review and the deny rules are the control.
SECRET_PATH='(\.env(rc)?([.][A-Za-z0-9_]+)*|\.pem|\.key|\.p12|\.pfx|id_rsa[^[:space:]]*|id_ed25519[^[:space:]]*|/\.ssh/[^[:space:]]*|secrets/[^[:space:]]*|initial_admin_password[^[:space:]]*|\.netrc|\.git-credentials|gh/hosts\.yml|\.npmrc|\.pgpass|kubeconfig[^[:space:]]*|\.kube/config)'
if printf '%s' "$CMD" | grep -qE "gh[[:space:]]+(api|issue|pr|gist|release)"; then
  if printf '%s' "$CMD" | grep -oE "(=@|(--body-file|-F)[[:space:]=]+|gist[[:space:]]+create[^|;&]*[[:space:]])[\"']?[^[:space:]\"']+" \
     | grep -vE '\.env\.example' | grep -qE "${SECRET_PATH}([\"']|$)"; then
    block "sending a secret file (.env, key, credentials, secrets/) to GitHub"
  fi
fi

printf '%s' "$CMD" | grep -qE '(curl|wget)[^|]*\|[[:space:]]*(ba)?sh'         && block "pipe-to-shell"
printf '%s' "$CMD" | grep -qE '^[[:space:]]*(ls[[:space:]]+-[a-zA-Z]*R|find[[:space:]]+/[[:space:]])' && block "full-tree scan wastes context — use rg or gh instead"
exit 0
