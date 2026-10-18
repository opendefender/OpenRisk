---
name: reviewer-routing-posture
description: #1005 route-reviewers.sh 2026-10-18 — r1 BLOCK, r2 (a73d5ff4) PASS; r1 issues were: fails open on diff errors (orphan/shallow), INFRA regex misses agent/, security/, scripts/, Makefile, npm manifests, agent prompts/memory
metadata:
  type: project
---

Review of `.claude/hooks/route-reviewers.sh` (issue #1005), 2026-10-18: round 1 (e0e92c78) BLOCK; round 2 (a73d5ff4) PASS — merge-base required, git diff status checked, INFRA widened, agent prompts/memory routed, --literal-pathspecs, here-strings. Residual LOW: the name-list diff runs twice and the second run's status is not checked. Branch-name label MEDIUM carried to #1008 (runner must pass --labels).

Round 1 findings:

- No code execution from diff content found: --no-ext-diff/--no-textconv, NUL names, option-like base refused; glob-shaped filenames still match themselves (git exact-match first).
- Fail-open: `git diff BASE...HEAD` errors go to /dev/null, so an orphan branch or a shallow clone (no merge base) prints nothing and exits 0.
- INFRA regex misses security-relevant paths that CI/release/users execute: agent/ (shipped endpoint agent with self-update), security/vulnerability-exceptions.yaml (suppresses dependency gate), scripts/, Makefile, install.sh, bootstrap-openrisk-agents.sh, package.json/package-lock.json, frontend/scripts, nginx.conf (headers), vite.config.ts, index.html.
- `.md` files skip all review: .claude/agents/*.md, .claude/agent-memory/** (incl. this directory), CLAUDE.md — test r25 codifies "no reviewer". Prompt-injection persistence path.
- MEDIUM non-blocking: labels derived from branch name are agent-controlled; runner should pass the issue/labels itself.

**Why:** the router decides whether devsecops ever sees a PR; every under-call is a silent bypass of this review.
**How to apply:** on re-review, re-run the orphan/shallow probe and the unlisted-path probe; see [[agent-hooks-posture]], [[ci-security-gates-posture]].
