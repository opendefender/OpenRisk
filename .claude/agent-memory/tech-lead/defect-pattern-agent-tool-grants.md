---
name: defect-pattern-agent-tool-grants
description: Agent frontmatter traps seen reviewing #1004 — Bash(pattern) in tools is undocumented, memory grants Write/Edit, subagent cwd is the session's, committed memory is public
metadata:
  type: project
---

Seen reviewing #1004 (2026-10-16, head 181023f6), agent stack v2 roster.

- **`tools: Bash(git diff:*)` is not a documented narrowing.** The sub-agents doc
  lists tool names only; specifiers are documented for `disallowedTools` (where
  they remove the whole tool) and for `permissions.deny`. Whether a specifier in
  `tools` narrows Bash, grants all of it, or drops it was not proven (live probe
  failed on an expired OAuth session). devsecops, issue-triage, product-verifier,
  scout, code-reviewer and market-na all rely on it.
- **`memory: project` auto-enables Read, Write and Edit.** "Read-only on code" in
  a reviewer's description is then policy, not enforcement.
- **A subagent starts in the session cwd.** `/work` creates `../OpenRisk-wt-<n>`,
  so a reviewer limited to `git diff` (no `git -C`, no `cd`) diffs whatever the
  session checkout holds. tech-lead used to have full Bash and could `git -C`.
- **Agent memory is committed to a public repo.** devsecops posture files carry
  exploit-level detail of open defects (#944 MFA bypass); acceptable only when
  the issue is already public or fixed, else `.claude/agent-memory-local/`.

**Why:** these all fail silently: a review of the wrong checkout still ends PASS.
**How to apply:** on any agent-config diff, check where the agent's commands run,
what `memory:` adds to its tools, and whether a new memory file discloses an
unfixed defect. See [[defect-pattern-agent-hooks]].
