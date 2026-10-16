---
name: defect-pattern-transcript-metrics
description: Pitfalls when measuring tokens/PRs from Claude Code transcripts (scripts/agents/token_usage.py, #1003) — streamed subagent usage, gh api implicit POST, /pulls sub-paths
metadata:
  type: project
---

Facts checked on the owner's transcripts on 2026-10-15 while reviewing #1003 (agent stack v2 baseline, epic #1002):

- Main-session transcripts: every line of one message.id carries identical usage. Taking the first line is exact.
- Subagent transcripts: usage is streamed. Input/cache are identical across lines, output_tokens grows line by line (560 of 2650 messages differ, never decreasing, so max == last). Taking the first line undercounts ~4.5x overall; max per id is the best available floor (fixed in 38fdde11). It is NOT "8 or 16" — first-chunk values range widely.
- PR openings in this repo are often `gh api repos/.../pulls -F ... ` with NO `-X POST` (gh defaults to POST when fields are given). A regex requiring `-X POST` misses them (#753, #851, #852, #943 historically).
- `-X POST .../pulls/N/reviews` returns a pull/N URL — a regex not anchored at `/pulls` end counts reviews as openings, which inflates PR counts exactly where v2 adds reviewers.
- Transcripts start 2026-08-14 (not 08-20). Only one project folder exists (worktree sessions do not get their own folder so far).
- PRs are also opened through ad-hoc shell functions (`mk(){ gh pr create ...}; mk ...`, #967–#971) — any "gh at command start" anchor must accept `{`.
- Coverage vs GitHub (re-review 38fdde11, 2026-10-15): Aug 7/38, Sep 67/151, Oct 56/56; zero openings outside their session window. Old transcripts age out — the v2 comparison must run soon after v2 sessions.
- Still open after PASS (suggestions): detector evaluates flags over the whole Bash command, not per segment (a pulls GET followed by `... -F x` counts every listed PR); `repos/{owner}/{repo}/pulls` placeholder and `timeout gh ...` wrappers are missed.

**Why:** the v2 before/after comparison reuses the same script; detection errors bias the comparison, not just the baseline.
**How to apply:** when #1003 or its v2 re-run comes back, re-check the regex anchoring and the per-id max, and re-verify report claims against data rather than the script's docstring. See [[defect-pattern-agent-hooks]] for the other agent-tooling class.
