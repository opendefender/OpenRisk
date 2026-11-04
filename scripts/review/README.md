# PR review gate (#1271)

Who reviews a PR is computed, not chosen:
- the reviewers `.claude/hooks/route-reviewers.sh` prints for the diff;
- `tech-lead` on every PR;
- `devsecops` on every red-class PR (#1250);
- `qa-automation` when tests change.

The `Agent review` workflow records them on each head commit and sets the commit status `agent-review` to pending.

## A reviewer agent's run

1. Read the diff (`git diff origin/develop...HEAD`), the closing issue's acceptance criteria, and only the files the diff touches.
2. Check:
   - every criterion is met, with its proof;
   - CLAUDE.md's absolute rules hold;
   - no claim is made without code;
   - the rulebook (`docs/AGENT_RULES.md`) is respected.
3. Write numbered findings to a file. Mark each one `blocking` or `nit`, with file and line, and cite the rule number when one applies. On a pass, still say what was checked.
4. Record the verdict:
   ```
   node scripts/review/verdict.mjs --pr <n> --role <role> --verdict pass|fail --blocking <k> --file findings.md
   ```
   The script refuses:
   - a role that is not required;
   - a stale commit;
   - `pass` with blocking findings;
   - `fail` with none.

   Nits never fail a review.

## What happens next

| Result | What follows |
|---|---|
| All required roles pass on the head commit | `agent-review` is success, a condition of the merge (#1280) |
| A failure | The issue goes back to `status:in-progress`, and the runner gives the blocking findings to an implementer, who pushes fixes and answers each finding. The new commit is routed again. |
| A dispute | tech-lead arbitrates once; its verdict is final |
| Failures on 3 different commits | The issue is `status:blocked` and the PR gets a summary |

Reviewers never push code. Implementers never call `verdict.mjs`: the runner (#1252) hands it to reviewer runs only.

## Commands

```
node scripts/review/route.mjs --pr <n> --worktree <dir> --dry-run   # who would review
node scripts/review/aggregate.mjs --pr <n> --dry-run                 # current verdict
node --test scripts/review/*.test.mjs
```
