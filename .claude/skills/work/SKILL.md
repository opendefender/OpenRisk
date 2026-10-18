---
name: work
description: Work a single OpenRisk issue end to end — read it, route it to the right agents, implement, test, secure, verify, open the PR. This is the main daily loop. One issue per session.
argument-hint: <issue-number>
---

# Work issue #$ARGUMENTS

One issue, one session, one branch, one PR. Do not start a second issue in this
session — start a new session instead. This keeps context small and resumable.

## 1. Load (cheap)

```
gh api repos/opendefender/OpenRisk/issues/$ARGUMENTS --jq '.title, .body, [.labels[].name], .milestone.title'
gh api repos/opendefender/OpenRisk/issues/$ARGUMENTS/comments --jq '.[] | .user.login + " " + .created_at + "\n" + .body'
```
If `status:ready` is absent: stop. Report what is missing and delegate to
`po-openrisk` to refine it. Do not implement an unready issue.

If the issue already has agent comments, **resume from the last one's `Next`
field**. Do not redo work. Do not re-read files the comment already describes.

## 2. Claim, in a worktree of your own

Several sessions run at once on this repository, and a shared checkout has
carried one issue's commits onto another's branch. Work in a dedicated
worktree, branched from the latest master:

```
git fetch origin
git worktree add ../OpenRisk-wt-$ARGUMENTS -b <type>/$ARGUMENTS-<slug> origin/master
gh issue edit $ARGUMENTS --add-label status:in-progress --remove-label status:ready
```
When `frontend/package-lock.json` matches the main checkout, symlink its
`frontend/node_modules` instead of reinstalling.

## 3. Route by `area:` label, then review

| Label | Implementation, in order | Review, always |
|---|---|---|
| `area:backend` | tech-lead (design, if structural) → backend-go → qa-automation | code-reviewer + devsecops |
| `area:frontend` | ux-designer (if no spec) → frontend-react → qa-automation | code-reviewer |
| `area:infra` | devops-sre → cloud-sysadmin | code-reviewer + devsecops |
| `area:design` | art-director → ux-designer → motion-designer | art-director, then code-reviewer if code changed |
| `area:marketing` | brand-strategist → copywriter | product-verifier |
| `type:security` | devsecops (finding) → owning agent → qa-automation | code-reviewer + devsecops |

Add devsecops to the review of any change that touches auth, sessions, tenant
data, uploads, outbound requests or secrets, whatever its label.

Skip an implementation stage only when it is genuinely irrelevant, and say
which. **Never skip the review.**

### Review loop

1. The reviewer reads the whole diff, not a summary:
   `git diff origin/master...HEAD`. It ends with `REVIEW: PASS` or
   `REVIEW: BLOCK — <n> critical` (devsecops: `SECURITY: PASS|FAIL`).
2. BLOCK or FAIL goes back to the implementing agent with the findings
   verbatim. It fixes them, then the same reviewer reads the new diff.
3. A PASS covers the diff it read. Any later change, a CI fix included, goes
   back to the same reviewer before the PR is handed over.
4. After two blocked rounds, `tech-lead` arbitrates. What it cannot settle
   goes to `docs/DECISIONS.md` with both positions and the reviewer's
   findings, and is reported. Do not open the PR over a block.

### Live check (frontend and any user-visible change)

Run the branch for real (see the live boot recipe in agent memory or
`dev/`), exercise the screen in a browser, and record in the issue comment
which screen, which states (loading, error, empty, filled) and which role
were checked. Unit tests alone do not close a UI issue.

## 4. Checkpoint — mandatory, before you finish or run low

Post the issue comment in the exact format from CLAUDE.md, including the
**Review** line. This comment is the resume anchor: a fresh session with zero
context must be able to continue from it alone. Write it as if the reader
knows nothing.

## 5. Ship the branch

Stage by explicit path, never `git add -A`, and look at what you stage:
```
git status --short
git add <path> <path>
git commit -s -m "type(scope): subject (#$ARGUMENTS)"
git log --no-merges --format='%h %s' origin/master..HEAD   # every line must carry #$ARGUMENTS
git push -u origin HEAD
gh pr create --base master --title "type(scope): subject (#$ARGUMENTS)" --body-file pr.md
```
`pr.md` holds `Closes #$ARGUMENTS`, the verification output, the review
verdicts, and the honest remainders. If `gh pr create` fails with the
Projects classic error, use the REST fallback in CLAUDE.md.

### Wait for CI before handing over

```
gh pr checks <pr> --watch
```
A check your branch turned red is yours: fix it and push. A check that is
also red on master is named in the PR body with its tracking issue (open
one if none exists). Only then:
```
gh issue edit $ARGUMENTS --add-label status:in-review --remove-label status:in-progress
```

Never merge. Merging is the owner's decision.

## 6. Close out

One paragraph: what shipped, what was verified, the review verdicts, the CI
state, what remains, and whether anything needs the owner. Nothing else.
