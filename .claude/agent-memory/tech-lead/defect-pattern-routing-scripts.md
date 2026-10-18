---
name: defect-pattern-routing-scripts
description: Bash decision scripts (route-reviewers.sh, #1005) fail open — pipefail+grep -q SIGPIPE, cwd-relative pathspecs, swallowed git errors
metadata:
  type: project
---

Shell scripts that decide *who reviews* or *whether to block* fail open in three
ways, all reproduced on #1005 (2026-10-18, route-reviewers.sh):

1. `set -o pipefail` + `producer | grep -q` — grep exits at first match, the
   producer takes SIGPIPE (141), the pipeline reports failure, the `&& add`
   never runs. Only shows on big diffs (~3000 lines); small fixtures pass.
   Fix: capture to a variable and `grep -q <<<"$var"`, or drop pipefail there.
2. `git diff --name-only` prints repo-root paths, but pathspecs after `--` are
   cwd-relative. Run from a subdir and the content checks see an empty diff.
   Fix: `cd "$(git rev-parse --show-toplevel)"` or `--literal-pathspecs` + `:(top)`.
3. `git diff A...B 2>/dev/null` inside `< <(...)` — no merge base (shallow CI
   clone, orphan) gives an empty file list and exit 0 = "no reviewers".

All three fixed in round 2 (a73d5ff4); the fix uses `local -n` (bash >= 4.3),
which on macOS /bin/bash 3.2 would silently leave the diff empty — watch for it.

**Why:** the issue's stated risk was "a rule too narrow silently skips a
reviewer"; these are the mechanical versions of that, and fixture tests miss them.
**How to apply:** when reviewing any gate/routing script, test a 5000-line
fixture, a run from a subdirectory, and a range with no merge base.
Related: [[defect-pattern-agent-hooks]], [[defect-pattern-agent-tool-grants]].
