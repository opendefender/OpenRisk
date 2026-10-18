---
name: agent-hooks-posture
description: Security posture of .claude/hooks (tenant tripwire, guard-bash, completion gate, checkpoint, _roots.sh) from the #956/#958/#959 reviews on 2026-10-14, round 1 and round 2
metadata:
  type: project
---

Two reviews on 2026-10-14 for #956/#958/#959 (branch chore/956-checkpoint-ignore). These hooks are developer aids. They are not security boundaries, and they should never be cited as the control for tenant isolation.

**Round 1 (FAIL, 1 blocking):** the tripwire's ±3-line rule was a regression on the IDOR shape. There was a guard-bash `-f`/`-F` exfil issue, and gate state in /tmp was subject to arithmetic injection.

**Round 2 (FAIL, 1 blocking):**
- Closed: the IDOR regression (tenant-tripwire.sh is now statement-scoped and ignores the func signature), the -f variants, `gh api -F body=@.env`, and the /tmp arithmetic issue (numeric regex check, per-user state dir).
- NEW HIGH: _roots.sh derives the gate/checkpoint roots from the agent transcript. That includes any `cd X` or `git -C X`, even inside a string, and any Edit/Write path, even a denied one. The hooks then run `git status/diff`, `go test` and `npm run` in that directory, unsandboxed and without a prompt. A plain `git clone` of an attacker repo, followed by `cd` into it, ran attacker test code (verified). A planted core.fsmonitor fired from checkpoint.sh for every agent type, Explore included (verified).
- MEDIUM: the builder rule accepts `q := db.Where(tenant)` followed by `q = r.db.Model(..)`, `q = r.db` or `q = r.db.WithContext(..)`, then `q.Where("id=?")`. The walk-back skips a non-decl reassignment.
- MEDIUM: a Bash-only edit by absolute path with no cd, `bash -c "cd .."`, or an `.claude/agent-memory/../..` path makes the gate skip the worktree. It falls back to the main checkout cwd.
- MEDIUM: `gh issue comment -F` and `gh pr comment -F` (short for --body-file, allow-listed) are not caught. Neither are .envrc, gh hosts.yml, .netrc and .git-credentials.
- LOW: any "tenant" word in the statement passes, including a trailing `// tenant` comment, a dest var `tenantX`, or `Preload("Tenant")`.

**Why:** the tripwire is the only automated tenant check at edit time, agents treat a green tripwire as evidence, and the hooks run outside the sandbox.
**How to apply:**
- When hooks change, rerun the IDOR shape and the builder-reassign shapes against both the old and new hook.
- For any hook that executes in transcript-derived dirs, test it with a malicious clone (default branch differing from master) and with core.fsmonitor.
- Never accept "the tripwire passed" as proof. Related: [[recurring-defect-areas]].
