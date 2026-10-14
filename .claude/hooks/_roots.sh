#!/usr/bin/env bash
# The checkouts of THIS project an agent worked in. Usage:
#   ROOTS=$(edited_roots "$INPUT")
#
# Subagent hooks receive the session's cwd (the main checkout) while /work has
# agents edit a sibling worktree, so the roots come from the agent's
# transcript (#958 review).
#
# Only worktrees of this project are ever returned. A path in the transcript
# is matched against `git worktree list` by string prefix after normalising
# it; no git command, build or test ever runs in a directory that is not one
# of those worktrees. Following any `cd` the transcript mentions ran the code
# of whatever repo an agent had cloned or merely named (second security
# review on #967).
#
# Matches come from Edit/Write/MultiEdit/NotebookEdit paths and from every
# absolute path (and `cd <relative>`) inside Bash commands, so an edit made
# with sed on /abs/worktree/file, or inside bash -c "cd …", still counts.
# Agent memory and the checkpoint file are bookkeeping and never count.
# Order: worktrees other than the main checkout first, most-touched first.
# With no match, the session's own checkout. "!unreadable" when a transcript
# was named but could not be read, so the caller can refuse to guess.

edited_roots() {
  local input="$1" transcript cwd project allowed
  transcript=$(json_get "$input" '.agent_transcript_path')
  [ -n "$transcript" ] || transcript=$(json_get "$input" '.transcript_path')
  cwd=$(json_get "$input" '.cwd')
  project="${CLAUDE_PROJECT_DIR:-${cwd:-$PWD}}"
  project=$(git -C "$project" rev-parse --show-toplevel 2>/dev/null) || return 0
  allowed=$(git -C "$project" worktree list --porcelain 2>/dev/null | sed -n 's/^worktree //p')
  [ -n "$allowed" ] || allowed="$project"

  # No transcript path at all: the session's own checkout. A transcript that
  # is named but cannot be read is reported, not papered over with the cwd.
  if [ -z "$transcript" ]; then
    printf '%s\n' "$project"
    return 0
  fi
  if [ ! -r "$transcript" ] || ! command -v python3 >/dev/null 2>&1; then
    printf '!unreadable\n'
    return 0
  fi
  OPENRISK_ALLOWED_ROOTS="$allowed" python3 - "$transcript" "${cwd:-$project}" "$project" <<'PY'
import json, os, re, sys
transcript, cwd, project = sys.argv[1], sys.argv[2], sys.argv[3]
allowed = [os.path.realpath(p) for p in os.environ.get("OPENRISK_ALLOWED_ROOTS", "").splitlines() if p.strip()]
allowed.sort(key=len, reverse=True)
edit_hits, bash_hits = {}, {}

def norm(p, base):
    p = os.path.expanduser(p.strip("\"'"))
    if not os.path.isabs(p):
        p = os.path.join(base, p)
    return os.path.realpath(os.path.normpath(p))

def count(p, hits, base=cwd):
    if not p or not isinstance(p, str) or "\n" in p:
        return
    p = norm(p, base)
    if "/.claude/agent-memory/" in p + "/" or p.endswith("/.claude/CHECKPOINT.md"):
        return
    for root in allowed:
        if p == root or p.startswith(root + os.sep):
            hits[root] = hits.get(root, 0) + 1
            return

try:
    lines = open(transcript, errors="replace").readlines()
except OSError:
    print("!unreadable")
    sys.exit(0)
for line in lines:
    try:
        msg = json.loads(line).get("message") or {}
        content = msg.get("content")
    except Exception:
        continue
    if not isinstance(content, list):
        continue
    for item in content:
        if not isinstance(item, dict) or item.get("type") != "tool_use":
            continue
        name, inp = item.get("name"), item.get("input") or {}
        if name in ("Edit", "Write", "MultiEdit", "NotebookEdit"):
            count(inp.get("file_path") or inp.get("notebook_path"), edit_hits)
        elif name == "Bash":
            cmd = inp.get("command") or ""
            # Absolute paths only: a relative cd resolves against wherever the
            # agent's shell was, which the transcript does not tell us.
            for p in re.findall(r"(?:(?<=[\s\"'=(:])|^)(/[^\s\"';|&()<>]+|~/[^\s\"';|&()<>]+)", cmd, re.M):
                count(p, bash_hits)

# Bash paths only when no file was edited directly: "git -C <main> log" or
# reading a file in the main checkout must not make it a root (review #967).
hits = edit_hits or bash_hits
real_project = os.path.realpath(project)
order = sorted(hits, key=lambda r: (r == real_project, -hits[r], r))
print("\n".join(order) if order else real_project)
PY
}
