# Agent rulebook

Every rule an agent follows on OpenRisk, with what makes it hold. A rule that nothing enforces
is a wish, so each line names its enforcement point and its status. CLAUDE.md stays the
constitution; this file is its operating detail (#1277).

**Enforcement points:**
- `hook`: `.claude/hooks/`, runs before or after the agent's tool call and can refuse it;
- `CI`: a required check (D-082);
- `ruleset`: branch rules on `develop`, `staging` and `master` (D-091);
- `bot`: a workflow that runs as the delivery bot (D-092): the daily brief, the weekly planning (#1326);
- `review`: the PR review gate (#1271);
- `harness`: the agent CLI's own safety check.

**Status:**
- ✅ enforced today, with the file;
- 🔧 enforced once the linked issue lands;
- 📝 review only, with the reason.

## 1. Honesty and proof

| # | Rule | Enforced by | Status |
|---|---|---|---|
| 1.1 | Nothing is called done without a file path and the output of the command that proves it. "Should work" is not a status. | review, `gate-task-complete.sh` | ✅ hook · 🔧 #1271 |
| 1.2 | A failure is reported as a failure, with its output. A skipped step is reported as skipped. | review | 🔧 #1271 |
| 1.3 | No capability is documented or announced unless the code exists and a user can reach it (CLAUDE.md rule 12). | review, `product-verifier` before a release | 🔧 #1271, #1256 |
| 1.4 | Every report ends with what is not done and why. | review | 🔧 #1271 |

## 2. Scope

| # | Rule | Enforced by | Status |
|---|---|---|---|
| 2.1 | No work without an issue that passed the Ready gate **and** the automatic refinement (D-094), and **the owner starts it** with `/work` (D-101, D-102). No agent and no timer starts an implementation; the briefs only propose. | no agent runs outside a `/work` session or a bot workflow (D-102); the brief ranks only refined Ready issues (`scripts/brief/rank.mjs`); Ready gate script | ✅ #1320, #1325 · 🔧 #1249, #1326 |
| 2.2 | One issue → one branch `<type>/<n>-<slug>` → one PR with `Closes #<n>`. | CI check on the PR (title, branch name, closing reference) | 🔧 #1251 |
| 2.3 | Smallest correct change. No refactor or cleanup outside the issue. | review | 📝 needs judgement |
| 2.4 | An issue found on the way becomes a new issue, never extra commits on the current PR. | review | 📝 needs judgement |
| 2.5 | Read every file of a module before writing in it (CLAUDE.md rule 1). | review | 📝 needs judgement |
| 2.6 | A criterion that can only be checked once merged (a measurement later, a run on the next release) ends with `(after merge)` in the issue, at refinement (#1304). It does not hold the merge only if that same line was already there when the issue was first set `status:in-progress`, at least one criterion stays unmarked, and the PR body names an open follow-up issue (`After merge: #<m>`) that the PR does not close and whose body refers to the issue. The merge closes the issue; the follow-up stays open until the criterion is met. Limit: the anchor proves the marker came before the work, not that another role approved it, since the agent that claims an issue often wrote it; the bot's agent-review still reads the marker. | `scripts/merge/conditions.mjs` (`exemptCriteria`, `bodyAtClaim`, `followUpRefs`), fed from GitHub's edit history by `scripts/merge/enqueue.mjs` | ✅ merge gate · 📝 closing the follow-up needs judgement |

## 3. Irreversible and owner-only actions

| # | Rule | Enforced by | Status |
|---|---|---|---|
| 3.1 | No force push, no branch deletion on `develop` / `staging` / `master`. | hook `guard-bash.sh`, ruleset | ✅ hook · 🔧 #1244 ruleset |
| 3.2 | No merge by hand; merges go through the `develop` merge queue (ADR 0009). | hook `guard-bash.sh`, ruleset | ✅ hook · 🔧 #1251 |
| 3.2b | Only the owner merges into `master`, through the PR `staging → master` that the bot opens after the soak. No agent and no bot merges or pushes to `master` (D-101). | ruleset on `master` (PR required, no bypass actor), hook `guard-bash.sh` | 🔧 #1256 |
| 3.3 | No release, tag or deletion of a release by an agent. | hook `guard-bash.sh` | ✅ |
| 3.4 | No `rm -rf` on `/`, `~` or `$HOME`. No destructive DDL in a shell. No `terraform apply/destroy`, no `kubectl delete`. | hook `guard-bash.sh` | ✅ |
| 3.5 | No pipe-to-shell (`curl … \| sh`). | hook `guard-bash.sh` | ✅ |
| 3.6 | Money and irreversible matters go to `docs/DECISIONS.md` under **Open** and wait (D-089). | review | 📝 needs judgement |

## 4. Secrets

| # | Rule | Enforced by | Status |
|---|---|---|---|
| 4.1 | No reading of `.env*`, `*.pem`, `*.key` with the file tools. | `settings.json` deny | ✅ |
| 4.2 | No reading of `.env*` (except `*.example`) through the shell either (`cat`, `grep`, `head`…). | hook `guard-bash.sh` | 🔧 patch on #1277 |
| 4.3 | No secret file sent to GitHub (`gh … @.env`, key files, `secrets/`). | hook `guard-bash.sh` | ✅ |
| 4.4 | No secret in logs, comments, PRs or commits. | CI `Secret Detection` | ✅ |
| 4.5 | A secret the owner must create is asked for by name and scope, never received in the chat. | review | 📝 |

## 5. Self-modification

| # | Rule | Enforced by | Status |
|---|---|---|---|
| 5.1 | An agent never edits its own rules: `CLAUDE.md`, `.claude/settings*.json`, `.claude/hooks/`, `.claude/agents/`, `.claude/skills/`. It prepares a patch and the owner applies it. `.claude/agent-memory/` stays writable. | harness · hook `guard-edit.sh` (file tools) and `guard-bash.sh` (shell writes) | ✅ harness · 🔧 patch on #1277 |
| 5.2 | No change to the gates that judge a PR (`.github/workflows/`, risk rules, `scripts/board/`) without the stricter review of ADR 0009 D3. | `Risk class` check, review | 🔧 #1250, #1271 |

## 6. The owner's machine

| # | Rule | Enforced by | Status |
|---|---|---|---|
| 6.1 | Work happens in a git worktree of the issue's branch, never in the owner's main checkout. | review | 📝 |
| 6.2 | Never stop, remove or rebuild the owner's running stacks. Verification uses throwaway names, ports and volumes (`OPENRISK_DEV_VOLUMES`, an env file in the scratchpad). | hook `guard-bash.sh`: `down -v`, `docker volume rm/prune`, `docker system prune` refused unless the env file is in the scratchpad | 🔧 patch on #1277 |
| 6.3 | Temporary files go to the session scratchpad, never inside the repository. | `gate-task-complete.sh`: no untracked file outside the allowed paths | 🔧 #1277 follow-up |

## 7. Shared GitHub state

| # | Rule | Enforced by | Status |
|---|---|---|---|
| 7.1 | Board fields are changed only by `scripts/board/setup.mjs`, which keeps option ids. A raw `singleSelectOptions` mutation is refused: it recreates the options and clears every value (2026-11-04). | hook `guard-bash.sh` | 🔧 patch on #1277 |
| 7.2 | Labels and milestones are never deleted from the repository. On an issue, a label is removed only for a `status:` transition or by refinement, with the reason (D-003, #1270). | hook `guard-bash.sh` (`gh label delete`, `DELETE repos/…/labels/…`, `DELETE repos/…/milestones/…`) · review | 🔧 patch on #1277 |
| 7.3 | Issues are never closed by hand. A merge closes them; refinement closes duplicates and obsolete issues with proof (#1270). | hook `guard-bash.sh` (`gh issue close`) | 🔧 patch on #1277 |
| 7.4 | A body posted from a file uses `-F body=@file` and is read back (`--jq '.body\|length'`). | hook `guard-bash.sh` blocks `-f body=@` | ✅ |
| 7.5 | A script that writes shared state is run with `--dry-run` first, and the dry run is pasted. | review | 📝 |

## 8. Tests and quality

| # | Rule | Enforced by | Status |
|---|---|---|---|
| 8.1 | CLAUDE.md's absolute rules: tenant filter, typed errors, three tests per use case, no `any`, three UI states, Zod, transactions. | CI (tenant isolation gate, lint, typecheck), review | ✅ CI · 🔧 #1271 |
| 8.2 | A UI change is checked in a real browser, with figures (`always-verify-live`). | review | 🔧 #1271 |
| 8.3 | A test that fails then passes is flaky. The second flaky failure opens a bug and gets fixed; it is not rerun a third time. Quarantine never covers tenant isolation or authz tests. | #1254 | 🔧 #1254 |
| 8.4 | Never weaken or delete an assertion to make a test pass. | review | 🔧 #1271 |
| 8.5 | Format only the files you changed; no tree-wide formatter. | `format-and-lint.sh` | ✅ |

## 9. Budgets and models

| # | Rule | Enforced by | Status |
|---|---|---|---|
| 9.1 | Every agent run in a workflow has a pinned model and a `max-turns` budget, and prints the tokens it used. | bot workflows (`agent-review.yml`, #1326) | ✅ review gate · 🔧 #1326 |
| 9.2 | Model by role: `haiku` mechanical, `sonnet` by default, `opus` only for L design, arbitration and ADRs. | agent frontmatter | 🔧 #1253 |
| 9.3 | No model call to poll: waiting is a workflow trigger or a schedule. | bot workflows | ✅ |
| 9.4 | At most 3 review rounds per PR, then `status:blocked`. | review gate | 🔧 #1271 |

## 10. Reporting and decisions

| # | Rule | Enforced by | Status |
|---|---|---|---|
| 10.1 | Before finishing, post the issue comment in the CLAUDE.md format (Done, Verified, Criteria, Next, Blocked on). | `checkpoint.sh`, review | ✅ hook · 🔧 #1271 |
| 10.2 | A choice that used to need the owner is decided and written as `Decided (agent, <date>)` in `docs/DECISIONS.md` (D-089). | review | 🔧 #1271 |
| 10.3 | Commits: `git commit -s`, Conventional Commits, English, no mention of an AI or an assistant. | CI `Signed-off-by trailer` | ✅ DCO · 📝 wording |

## Violations

The weekly figures (#1257) count violations per rule and per agent:
- a hook refusal;
- a review finding citing a rule number;
- a reverted PR.

Two violations of the same rule by the same agent in a week lead the retrospective to propose a change to that agent's prompt. The owner applies it (rule 5.1).

## Changing this file

This file is documentation, so an agent may propose changes to it in a PR. The enforcement it describes lives in `.claude/` and changes only by an owner-applied patch (rule 5.1).
