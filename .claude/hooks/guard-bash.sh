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

# Options may sit before the subcommand (git -C dir push, kubectl -n prod
# delete, terraform -chdir=x apply) and after it (git push origin main --force).
# OPTS only accepts dash options with an optional value between the tool and
# its verb, so "git commit -m 'push -f'" is not a push. ARGS stops at ; & |
# so a later "&& rm -f tmp" is not read as a push flag (#1224).
OPTS='([[:space:]]+-[^[:space:]]+([[:space:]]+[^-;&|[:space:]][^;&|[:space:]]*)?)*'
ARGS='[^;&|]*'

printf '%s' "$CMD" | grep -qE "git${OPTS}[[:space:]]+push${ARGS}[[:space:]](--force|-[a-zA-Z]*f[a-zA-Z]*([[:space:]]|\$)|[\"']?\+[^[:space:]])" \
  && block "force push"
printf '%s' "$CMD" | grep -qE 'gh[[:space:]]+pr[[:space:]]+merge'             && block "merging is the owner's decision"
printf '%s' "$CMD" | grep -qE 'gh[[:space:]]+release[[:space:]]+(create|delete)' && block "releases are the owner's decision"
printf '%s' "$CMD" | grep -qE 'rm[[:space:]]+-[a-z]*r[a-z]*f?[[:space:]]+(/|~|\$HOME)([[:space:]]|$)' && block "destructive rm"
printf '%s' "$CMD" | grep -qE "terraform${OPTS}[[:space:]]+(apply|destroy)"   && block "terraform apply/destroy needs human approval"
printf '%s' "$CMD" | grep -qE "kubectl${OPTS}[[:space:]]+delete"              && block "kubectl delete needs human approval"
# TRUNCATE must be followed by a name: coreutils truncate always takes -s or -r.
printf '%s' "$CMD" | grep -qiE 'DROP[[:space:]]+(TABLE|DATABASE|SCHEMA)|TRUNCATE[[:space:]]+["[:alpha:]_]' && block "destructive DDL"
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

# --- #1277 rulebook (docs/AGENT_RULES.md) -------------------------------------
# 4.2 Secrets are not read through the shell either (the file tools are denied in
# settings.json). *.example files are templates and stay readable.
printf '%s' "$CMD" | grep -qE '(^|[;&|[:space:]])(cat|less|more|head|tail|grep|rg|awk|sed|bat|strings|xxd|od|cut|sort)[[:space:]]([^;&|]*[/[:space:]])?\.env([.a-zA-Z0-9_-]*)?([[:space:]]|$)' \
  && ! printf '%s' "$CMD" | grep -qE '\.env[.a-zA-Z0-9_-]*\.example([[:space:]]|$)' \
  && block "reading a .env file through the shell (rule 4.2)"
# 5.1 Agents never rewrite their own rules from the shell.
printf '%s' "$CMD" | grep -qE '(>>?|tee([[:space:]]+-a)?|sed[[:space:]]+-i[^[:space:]]*|mv|cp|rm|truncate|git[[:space:]]+(apply|checkout[[:space:]]+[^;&|]*--|restore))[[:space:]]+([^;&|]*[/[:space:]])?(CLAUDE\.md|\.claude/(settings[^/[:space:]]*\.json|hooks/|agents/|skills/))' \
  && block "agents do not edit their own rules (CLAUDE.md, .claude/settings, hooks, agents, skills): prepare a patch for the owner (rule 5.1)"
# 6.2 The owner's running stacks and their data are not stopped or wiped. A
# throwaway stack whose env file lives in the session scratchpad is allowed.
printf '%s' "$CMD" | grep -qE 'docker[[:space:]]+(volume[[:space:]]+(rm|prune)|system[[:space:]]+prune|compose[^;&|]*[[:space:]]down[^;&|]*[[:space:]](-v|--volumes))' \
  && ! printf '%s' "$CMD" | grep -qE -- '--env-file[[:space:]=]+[^[:space:]]*/scratchpad/' \
  && block "removing Docker volumes outside a scratchpad throwaway stack (rule 6.2)"
# 7.1 Board select options are changed only by scripts/board/setup.mjs, which keeps
# their ids; a raw singleSelectOptions mutation clears every value (2026-11-04).
printf '%s' "$CMD" | grep -qE 'gh[[:space:]]+api[[:space:]]+graphql[^;&|]*singleSelectOptions' \
  && block "raw singleSelectOptions mutation; use scripts/board/setup.mjs (rule 7.1)"
# 7.2 Labels and milestones are never deleted from the repository. Removing a
# label from an issue stays allowed: the status: transitions need it.
printf '%s' "$CMD" | grep -qE 'gh[[:space:]]+label[[:space:]]+delete|gh[[:space:]]+api[^;&|]*-X[[:space:]]*DELETE[^;&|]*repos/[^/[:space:]]+/[^/[:space:]]+/(labels|milestones)/|gh[[:space:]]+api[^;&|]*repos/[^/[:space:]]+/[^/[:space:]]+/(labels|milestones)/[^;&|]*-X[[:space:]]*DELETE' \
  && block "deleting a label or a milestone (rule 7.2)"
# 7.3 Issues are closed by a merge, or by refinement with proof (#1270), not by hand.
printf '%s' "$CMD" | grep -qE 'gh[[:space:]]+issue[[:space:]]+close|gh[[:space:]]+api[^;&|]*/issues/[0-9]+[^;&|]*state=closed' \
  && block "closing an issue by hand; a merge closes it (rule 7.3)"
# 3.2 Merges go through the queue, enqueued by scripts/merge/enqueue.mjs run as the bot
# (#1280, D-092). No agent merges or enqueues from the shell, by any API.
printf '%s' "$CMD" | grep -qE '(enqueuePullRequest|mergePullRequest|enablePullRequestAutoMerge|gh[[:space:]]+api[^;&|]*/pulls/[0-9]+/merge([[:space:]]|$))' \
  && block "merging or enqueuing from the shell; the merge workflow does it as the bot (rule 3.2, #1280)"
printf '%s' "$CMD" | grep -qE '^[[:space:]]*(ls[[:space:]]+-[a-zA-Z]*R|find[[:space:]]+/[[:space:]])' && block "full-tree scan wastes context — use rg or gh instead"
exit 0
