# 0009 — Autonomous delivery: develop → staging → master, merges without the owner on green, pull deploys

Status: **accepted**
Proposed: 2026-11-04, on #1242
Accepted: 2026-11-04 by the owner, with amendments — D-088 (`docs/DECISIONS.md`). D3 and
D5 below are written as amended; the proposed text of D3 (red PRs wait for the owner) was not
retained.
Amended: 2026-11-05 by D-101 (#1320) — the owner starts all work, and merges `staging → master`
themself. D1, D5 and D7 are written as amended; see "Amendment of 2026-11-05" at the end.
Amended again: 2026-11-05 by D-102 (#1325) — no local timer; see "Second amendment" at the end.
Builds on: D-082 (required checks on `master`), D-043 (E2E as the PR gate), D-015 (DCO), D-003 and
D-032 (labels), #1005 (`route-reviewers.sh`)
Milestone: Autonomous delivery (#1242 to #1258)

## Context

The owner wants the delivery cycle to run without them whenever the gates are green: keep the
board, pick work, implement, review, merge, release, and keep the dev and staging stacks on their
machine up to date. They also want the token bill to stay small and the agent team to be reusable
on other projects.

CLAUDE.md stands in the way on purpose. "**Never merge a PR.** Merging is the owner's decision"
(line 111), and any change to what agents may decide is an escalation. This ADR is that
escalation. Nothing in the milestone that merges, promotes or deploys may start before D-088 is
decided.

What was read rather than assumed, on origin/master `5d08e593` (2026-11-04):

1. **`deploy.yml` has never deployed anything.** It triggers on pushes to `stag` and `main`;
   the default branch is `master`. Its staging and production steps are `echo` lines with the
   `kubectl rollout` commands commented out (`.github/workflows/deploy.yml:112-113`, `165-166`,
   `203-204`). All its jobs run on `ubuntu-latest`.
2. **The two Compose stacks collide.** `docker-compose.yaml` and `docker-compose.staging.yaml`
   both publish `6380:6379` and `8080:8080`, and neither has a top-level `name:`. From the repo
   root they resolve to the same Compose project, so an `up` or a `down -v` on one acts on the
   other.
3. **No workflow listens to `merge_group`.** `ci.yml`, `e2e.yml`, `dependency-gate.yml`,
   `license-gate.yml`, `security-scanning.yml`, `security.yml` and `dco.yml` run on `push` and
   `pull_request` only. A merge queue would wait forever on the eight checks D-082 made
   required. `ci.yml`, `dependency-gate.yml` and `license-gate.yml` already list `develop` (and
   the dead `stag`) as push branches.
4. **Releases are already gated.** `release.yml` fires on `v*` tags, refuses a tag that disagrees
   with `VERSION` (today `1.1.0-rc.8`), runs the dependency gate on the exact image, checks
   reproducibility and SBOMs, and pushes last. `selfhost-install.yml` runs monthly and on demand.
5. **The routing of reviewers by diff already exists and fails closed.**
   `.claude/hooks/route-reviewers.sh` reads the diff and the issue labels and prints the
   reviewers; with no merge base or a failing `git diff` it exits 2 rather than print nothing.
6. **The agent team is heavy.** `.claude/agents/` holds 28 agents: 19 on `opus`, 7 on `sonnet`,
   2 on `haiku`.
7. **There is no production.** A release is a tag, GHCR images and notes. Self-hosters deploy
   it themselves. The owner's machine runs dev and staging only.

## Decision

### D1 — Three branches, one commit, one image

- `develop` receives every PR, through a GitHub merge queue. `staging` receives no PR and no
  push except the promotion below. `master` receives only the PR `staging → master` that the
  bot opens and **the owner merges** (D-101).
- A promotion is a fast-forward of the **same commit**. If a fast-forward is impossible, the
  promotion stops and opens a bug; it never forces and never merges.
- Each `develop` commit is built **once** into GHCR images tagged `sha-<full sha>`. Promotion
  adds the `staging` or `master` tag to that manifest; it never rebuilds. The digest that ran on
  dev is the digest that runs on staging.
- The versioned release image stays the one `release.yml` builds from the tag, with its gates
  unchanged. It is built from the same commit that soaked on staging; the reproducibility check
  already in `release.yml` is what ties the two.

As built by #1245 (`.github/workflows/images.yml`, `scripts/images/promote.sh`):

- The per-commit images are `ghcr.io/opendefender/openrisk-backend` and
  `ghcr.io/opendefender/openrisk-frontend`, from `backend/Dockerfile` and `frontend/Dockerfile`,
  the two images the Compose stacks run. Each carries `org.opencontainers.image.revision` and
  `.source`; the backend links the full SHA into `main.Commit`, which `/api/v1/health` reports.
- Promotion copies the manifest by digest (`imagetools create --prefer-index=false`) and fails
  unless the promoted tag has the digest of `sha-<sha>`. A commit with no `sha-` image fails its
  promotion; nothing builds it then. `ci.yml` builds the all-in-one image as a check only and
  pushes nothing.
- The release image `ghcr.io/opendefender/openrisk:X.Y.Z` is a different artifact: the
  all-in-one root `Dockerfile`, built by `release.yml` from the tag, gated, then pushed. It is
  not a retag of a `sha-` image and its digest differs. What ties them is the commit: the tag
  is cut on the `master` commit whose `sha-` images soaked on staging. The reproducibility check
  proves the release binary is the one that commit produces; it does not compare it with the
  binary inside the `sha-` backend image, which is linked with different flags.

Issues: #1244, #1245.

### D2 — "Green" is a contract, not a feeling

A PR into `develop` is green when all of these hold on its head commit and again in the queue
(`merge_group`):

1. The eight required contexts of D-082 pass.
2. A `Risk class` check, computed from the diff by a script (#1250), says `green`.
3. The reviews that `route-reviewers.sh` asks for have approved the head commit. A new push
   resets them.
4. The PR is not a draft, closes an issue (`Closes #<n>`), and its head is in this repository.

Every workflow that provides a required check also runs on `merge_group`. A flaky test may be
quarantined by the rule of #1254, never the tenant isolation suite or the authz matrix tests.

### D3 — The red class merges through a stricter gate (amended by D-088)

A PR is red when its diff touches any of: authentication, MFA, sessions or OAuth
(`backend/pkg/auth/`, `backend/pkg/otp/`, `backend/pkg/oauthpkce/`,
`backend/internal/middleware/auth*.go`), cryptography (`backend/pkg/crypto/`), the design of
tenant isolation (the scoping helpers named in the backend charter), a migration that drops or
renames, the Score Engine or SmartScore (`backend/pkg/scoring/engine.go`, `smart.go`),
licensing (`LICENSE`, `LICENSE.commercial`), pricing or entitlements (`docs/PRICING.md`,
`backend/pkg/entitlements/`), the workflows (`.github/workflows/`), the risk rules themselves,
`CLAUDE.md`, or the agents, skills and hooks (`.claude/`).

The list is the escalation list of CLAUDE.md turned into paths. It lives in one committed file
that is itself red. The classifier fails closed: anything it cannot read is red.

**A red PR does not wait for the owner** (D-088). It merges automatically when, on top of D2:
the security review (devsecops) and the tech-lead review approve the head commit; the full Go
suite on Postgres, the tenant isolation suite, the authz matrix and the full e2e suite pass; and
no review finding is open. A red PR that touches `.github/workflows/`, the risk rules or
`scripts/board/` must also pass a gate-integrity check: every required context still runs,
from the base branch's definition. Branch protection itself lives in the repository settings,
which no PR can change.

What needs a decision still needs one first: a change to the Score Engine or SmartScore formula,
a paid service, pricing, licensing or anything irreversible is raised in `docs/DECISIONS.md`
before the work. Once decided, its implementation merges like any other red PR.

**A PR from a fork is red whatever its diff.** It is never enqueued by automation, and no job
that holds a secret or acts on the owner's machine runs on it. The repository is public and
AGPL; this is the line that keeps it safe to automate.

Issues: #1250, #1251.

### D4 — Deploys are pulled by the owner's machine

There is no self-hosted runner and nothing listens on the machine. A systemd user timer runs a
script that reads which `sha-` image `develop` and `staging` point at, and if it differs from what
runs, pulls it and restarts that stack only. It keeps the previous tag, so a rollback is one
command, and it rolls back on its own when `/health` or the smoke suite fails after a switch. It
needs read access to GHCR at most, never a token that can write to the repository.

Dev and staging are separate Compose projects (`openrisk-dev`, `openrisk-staging`) with their own
ports, networks, volumes and env files.

`deploy.yml` is removed, with its `production` environment.

Issues: #1243, #1245, #1246.

### D5 — Promotion and release rules

- **develop → staging**, automatically, when the commit runs on dev and the full e2e suite and a
  smoke suite pass again against the dev stack, pulled from the merged commit (not the PR's CI
  run), and no `priority:P0-critical` issue is open.
- **staging → master**: when the commit has run on staging for **48 h** (D-088) with no rollback and no new P0/P1 bug, at least one `feat` or `fix` landed
  since the last tag, and no P0 or P1 issue is open in the active milestone, the bot **opens**
  the PR `staging → master` with the FR/EN notes, the `staging-verified` proof and the tested
  `rc`. It never merges it; the owner does (D-101). The final tag goes on the soaked `staging`
  commit once the owner has merged.
- **Tag**: version and changelog come from Conventional Commits since the last tag, and must
  update `VERSION` so `release.yml`'s guard passes. release-please does this, and it is a new
  third-party action, accepted by D-088 and pinned by SHA. `selfhost-install.yml` runs on the candidate
  before the tag. Release notes in `fr` and `en` are drafted by the release-notes role and
  checked by `product-verifier` against the claim matrix; a claim it cannot prove blocks the
  release (CLAUDE.md rule 12).

Issues: #1255, #1256.

### D6 — The board moves on events only

A GitHub Project (v2) with the statuses Inbox, Refinement, Ready, In progress, In review, On dev,
On staging, Released, Blocked, Needs owner, and the fields Priority, Tier, Size, Risk class,
Agent, Tokens. Every transition comes from a GitHub event (issue, label, push, PR, merge,
deployment, tag), handled by a workflow with no model call. "Ready" is set by a script that
checks the Ready definition of CLAUDE.md, not by an agent's reading. The `status:` labels stay in
sync so the existing skills keep working.

Issues: #1247, #1248, #1249.

### D7 — Agents: judgement only, budgeted, cheap by default

- Labels, transitions, versioning, merge and deploy are scripts and workflows. A model is called
  only where judgement is needed: refining an issue, implementing, reviewing, writing notes.
- The owner starts every implementation with `/work` (D-101); a daily brief from the bot tells
  them which three issues to start, and a Monday list gives the week (D-102, #1326). No agent
  runs on the owner's machine outside a session they opened: no local timer. On push the bot
  reviewers review and the queue merges on green; a red CI or a refused review comes back to
  the owner. Refinement, the sprint and the milestones are planned weekly by `po-openrisk` in
  GitHub Actions, with a pinned model and a `max-turns` budget. Nothing polls with a model.
- Model by role: `haiku` for mechanical work (triage, label hygiene, release-notes drafts, i18n
  parity), `sonnet` by default (implementation, review, tests, docs), `opus` only for designing
  L issues, arbitration and ADRs. Target: at most four agents on `opus`.
- Context is read once per issue (the `scout` brief), reviews are proportional to the routed
  risk, and tokens are recorded per issue.
- A failed deploy reverts its PR through the queue and opens a P1 bug, red PRs included (D-088).

Issues: #1252, #1253, #1254, #1257.

### D8 — The team becomes a private plugin

The generic core (about eight roles, the generic skills and hooks, workflow templates) moves to a
private repository `opendefender/agent-team`, distributed as a plugin marketplace for the agent
CLI. Each project carries `.claude/team.yaml` (stack, commands, risk paths, environments, ports,
budgets) and keeps its own charters and domain specialists. The licence of the private repository
is the owner's call before anything is copied out of this AGPL repository.

Issue: #1258.

## The CLAUDE.md amendment (applied by #1251, not before)

Line 111 today:

> - **Never merge a PR.** Merging is the owner's decision.

Accepted (D-088):

> - **Never merge a PR by hand.** The `develop` merge queue merges a PR once every acceptance
>   criterion of its issue is ✅, every required check is green and no review finding is open;
>   a red-class PR also needs the stricter gate of ADR 0009 D3. A PR from a fork is never
>   merged automatically.

The escalation list (line 167) is unchanged: it governs what needs a decision before the work,
not who merges.

Line 148, "Never close an issue. The merge closes it.", is unchanged. With `develop` as the
default branch, `Closes #<n>` closes the issue on the `develop` merge, and the board's On dev /
On staging / Released columns carry the rest.

## What this is not

- Not a production deployment. There is none, and this adds none.
- Not a change to the Score Engine, SmartScore, auth, crypto or tenant isolation design. It makes
  sure no change to them merges without the owner.
- Not a change to D-082's required checks or to `release.yml`'s gates. It adds checks; it removes
  none.
- Not a licence to run code from the public on the owner's machine. Fork PRs never run there.

## Consequences

- The owner is no longer on the merge path at all, except for fork PRs. The board's Needs owner
  column holds only issues waiting on a D-0xx entry.
- `develop` becomes the default branch. Anyone cloning without a tag gets `develop`; the install
  docs must point at tags or `master` (#1244 checks them).
- A bad green-class change reaches dev before anyone looks at it. The containment is the queue,
  the soak on staging, the automatic rollback and revert, and the fact that self-hosters only see
  tags cut from `master`.
- The quality of "green" now carries the project. A gap in a required check is no longer caught
  by the owner reading the PR. #1250 and the `merge_group` triggers are the first things to build
  after the Compose fix.
- One more third-party action (release-please), accepted by D-088.
- Running cost moves from the largest model to the default one for most work; the figures are
  measured in #1253 and #1257, not assumed here.

## Order of work

1. #1243 (no dependency, can start now), #1249, #1247, #1253.
2. On acceptance: #1244, #1250, #1245, #1246.
3. #1251, #1248, #1252, #1270 (automatic refinement, so the runner never runs out of Ready
   work), #1254.
4. #1255, #1256, #1257, #1258.

Added on 2026-11-04 at the owner's request: #1266 (sprints), #1267 (backlog triage), #1268 (team
operating model), #1270 (automatic refinement).

## Amendment of 2026-11-05 (D-101, #1320)

The owner wants to start every piece of work and to merge into `master` themself.

- **Start of work.** The runner has no implement step. The owner starts an issue with `/work`.
  `.github/workflows/brief.yml` posts each morning, as the bot, the three issues to start on the
  pinned issue « Brief du jour », which emails the owner through GitHub notifications.
- **`master`.** Its ruleset requires a PR and has no bypass actor. After the soak the bot opens
  the PR `staging → master`; only the owner merges it. #1256 implements this.
- **Consequences.** The owner is back on two paths: starting work and releasing. Merges into
  `develop` and promotion to `staging` stay automatic (D-088, D-100). "The owner is no longer on
  the merge path at all" (Consequences) no longer holds for `master`.

## Second amendment of 2026-11-05 (D-102, #1325)

No agent runs on the owner's machine outside a session they opened. The local runner and its
timer are removed. The owner works with `/work`; on push the bot reviewers review, and the
queue merges on green; a red CI or a refused review comes back to the owner. Refinement, the
sprint and the milestones are planned once a week by `po-openrisk` in GitHub Actions, as the
bot, which also posts the Monday list (#1326). D7's paragraph on the local timer no longer
applies.
