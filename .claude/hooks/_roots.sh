#!/usr/bin/env bash
# The git checkouts an agent actually worked in. Usage:
#   ROOTS=$(edited_roots "$INPUT")
#
# Subagent hooks receive the SESSION's cwd, which is the main checkout, while
# /work has agents edit a sibling worktree (../OpenRisk-wt-<n>). Gating the cwd
# passed broken work in the very flow we mandate (#958 review). So the roots
# come from the agent's transcript: every file it wrote with Edit/Write/
# MultiEdit/NotebookEdit, plus every directory a Bash command entered with
# `cd <dir>` or `git -C <dir>`. Agent memory and the checkpoint file are
# bookkeeping, not work, and never make a checkout a root.
# Falls back to the payload cwd when the transcript yields nothing.

edited_roots() {
  local input="$1" transcript cwd
  transcript=$(json_get "$input" '.agent_transcript_path')
  [ -n "$transcript" ] || transcript=$(json_get "$input" '.transcript_path')
  cwd=$(json_get "$input" '.cwd')
  {
    if [ -n "$transcript" ] && [ -f "$transcript" ] && command -v python3 >/dev/null 2>&1; then
      python3 - "$transcript" "$cwd" <<'PY'
import json, os, re, shlex, sys
path, cwd = sys.argv[1], sys.argv[2] or os.getcwd()
seen = set()
def emit(p):
    if not p:
        return
    p = os.path.expanduser(p)
    if not os.path.isabs(p):
        p = os.path.join(cwd, p)
    if "/.claude/agent-memory/" in p or p.endswith("/.claude/CHECKPOINT.md"):
        return
    d = p if os.path.isdir(p) else os.path.dirname(p)
    if d and d not in seen:
        seen.add(d)
        print(d)
for line in open(path, errors="replace"):
    try:
        msg = json.loads(line).get("message") or {}
    except ValueError:
        continue
    content = msg.get("content")
    if not isinstance(content, list):
        continue
    for item in content:
        if not isinstance(item, dict) or item.get("type") != "tool_use":
            continue
        name, inp = item.get("name"), item.get("input") or {}
        if name in ("Edit", "Write", "MultiEdit", "NotebookEdit"):
            emit(inp.get("file_path") or inp.get("notebook_path"))
        elif name == "Bash":
            cmd = inp.get("command") or ""
            for m in re.finditer(r"(?:^|[;&|(]\s*)cd\s+(\S+)|git\s+-C\s+(\S+)", cmd):
                target = m.group(1) or m.group(2)
                try:
                    target = shlex.split(target)[0]
                except ValueError:
                    pass
                emit(target)
PY
    fi
  } | while IFS= read -r d; do
        [ -d "$d" ] && git -C "$d" rev-parse --show-toplevel 2>/dev/null
      done | sort -u | {
        out=$(cat)
        if [ -n "$out" ]; then printf '%s\n' "$out"
        elif [ -n "$cwd" ] && [ -d "$cwd" ]; then git -C "$cwd" rev-parse --show-toplevel 2>/dev/null
        else git rev-parse --show-toplevel 2>/dev/null
        fi
      }
}
