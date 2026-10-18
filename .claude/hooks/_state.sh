#!/usr/bin/env bash
# One line per worktree in .claude/STATE.md of the main checkout. Usage:
#   state_write <checkout> <agent> [gate]
#
# The line says where the work in that checkout stands: branch, issue, sha,
# uncommitted count, the last agent that stopped there, the gate state and the
# time. A new write replaces the checkout's line instead of appending, and
# drops the lines of worktrees git no longer lists (#1007).
#
# gate is "green" or "GATE RED". Left empty, the line keeps the gate state it
# had, so a checkpoint written after the gate gave up does not hide GATE RED.
#
# The file lives in the main checkout whichever worktree the agent was in, so
# one file lists them all. Two hooks writing at once can still lose a line:
# the write goes through a temp file and mv, so the file is never half written.

state_write() {
  local wt="$1" agent="$2" gate="${3:-}" live main file tmp branch sha dirty issue
  wt=$(git -C "$wt" rev-parse --show-toplevel 2>/dev/null) || return 0
  live=$(git -C "$wt" worktree list --porcelain 2>/dev/null | sed -n 's/^worktree //p')
  [ -n "$live" ] || live="$wt"
  main=$(printf '%s\n' "$live" | head -1)
  file="$main/.claude/STATE.md"

  branch=$(git -C "$wt" branch --show-current 2>/dev/null); branch="${branch:-detached}"
  sha=$(git -C "$wt" rev-parse --short HEAD 2>/dev/null || echo "-")
  dirty=$(git -C "$wt" status --porcelain 2>/dev/null | wc -l | tr -d ' ')
  issue=$(printf '%s' "$branch" | grep -oE '[0-9]{2,}' | head -1)
  # A branch with no number (agent, main, a scratch name): take the last issue
  # number the recent commits mention, and mark it as inferred.
  if [ -z "$issue" ]; then
    issue=$(git -C "$wt" log -20 --format=%s 2>/dev/null | grep -oE '\(#[0-9]+\)' | head -1 | tr -d '()#')
    [ -n "$issue" ] && issue="$issue(from commits)"
  fi

  if [ -z "$gate" ] && [ -f "$file" ]; then
    gate=$(STATE_WT="$wt" awk -F ' [|] ' '$1 == "- " ENVIRON["STATE_WT"] { for (i = 2; i <= NF; i++) if ($i ~ /^gate=/) g = substr($i, 6) } END { print g }' "$file")
  fi

  mkdir -p "$main/.claude" || return 0
  tmp=$(mktemp "$file.XXXXXX" 2>/dev/null) || return 0
  local line
  line=$(printf -- '- %s | branch=%s | issue=#%s | sha=%s | uncommitted=%s | agent=%s | gate=%s | %s' \
    "$wt" "$branch" "${issue:-?}" "$sha" "$dirty" "$agent" "${gate:--}" "$(date -u +%FT%TZ)")
  {
    printf '# Work state, one line per worktree. Machine written, read by /resume.\n\n'
    [ -f "$file" ] && STATE_WT="$wt" STATE_LIVE="$live" STATE_LINE="$line" awk -F ' [|] ' '
      BEGIN { n = split(ENVIRON["STATE_LIVE"], a, "\n"); for (i = 1; i <= n; i++) ok["- " a[i]] = 1 }
      /^- / {
        if ($1 == "- " ENVIRON["STATE_WT"]) { if (!done) print ENVIRON["STATE_LINE"]; done = 1 }
        else if ($1 in ok) print
      }
      END { if (!done) print ENVIRON["STATE_LINE"] }' "$file"
    [ -f "$file" ] || printf '%s\n' "$line"
  } > "$tmp" || { rm -f "$tmp"; return 0; }
  mv -f "$tmp" "$file" || rm -f "$tmp"
}
