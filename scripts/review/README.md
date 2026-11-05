# PR review gate (#1271)

Who reviews a PR is computed, not chosen:
- the reviewers `.claude/hooks/route-reviewers.sh` prints for the diff;
- `tech-lead` on every PR;
- `devsecops` on every red-class PR (#1250);
- `qa-automation` when tests change.

The `Agent review` workflow records them on each head commit and sets the commit status `agent-review` to pending.

## Where reviewers run (D-095)

Reviewers run **in GitHub Actions**, in the `Agent review` workflow, never on an agent's
machine: every local agent acts through the owner's account, so a verdict sent from there
could not prove who wrote it.

When `CI Status` is green on the head commit, the workflow starts one job per required role:
1. It checks out the PR head as **data**. `.claude/` and `CLAUDE.md` are renamed and `.mcp.json` is removed, so the PR cannot configure its reviewer. HOME is empty.
2. It builds the prompt from the default branch (`prompt.mjs`): the role's agent definition, the constitution and the rulebook. The PR's title, body, issue and diff are quoted as data.
3. It runs the agent CLI (pinned version) with **Read, Grep and Glob only**. The CLI never sees the bot's token.
4. `record.mjs` validates the final JSON verdict:
   `{"verdict": "pass"|"fail", "blocking": n, "findings": "…"}`.
   The **bot** then posts the comment and the `agent-review/<role>` status. An invalid or missing answer becomes `error`: it is rerun, and it is neither an approval nor a round.

The gates read **only the bot's statuses and comments**. A status or a marker comment written by anyone else is ignored. Fork PRs are never reviewed this way.

## What happens next

| Result | What follows |
|---|---|
| All required roles pass on the head commit | `agent-review` is success, a condition of the merge (#1280) |
| A fix pushed after a failure | Only the roles that refused, errored or are newly required review the new commit. A role that approved the previous commit keeps its approval: the bot writes it as *carried over, not re-reviewed*, and the routing comment lists both groups (D-096). |
| A failure | The issue goes back to `status:in-progress`, and the runner gives the blocking findings to an implementer, who pushes fixes and answers each finding. The new commit is routed again. |
| A dispute | tech-lead arbitrates once; its verdict is final |
| Failures on 3 different commits | The issue is `status:blocked` and the PR gets a summary |

Reviewers never push code. Nothing outside the workflow can record a verdict that counts.

## Commands

```
node scripts/review/route.mjs --pr <n> --worktree <dir> --dry-run   # who would review
node scripts/review/plan.mjs --pr <n>                                # which reviews are due
node scripts/review/aggregate.mjs --pr <n> --dry-run                 # current verdict
node --test scripts/review/*.test.mjs
```
