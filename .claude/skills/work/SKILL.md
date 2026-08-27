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
gh issue view $ARGUMENTS --comments
```
If `status:ready` is absent: stop. Report what is missing and delegate to
`po-openrisk` to refine it. Do not implement an unready issue.

If the issue already has agent comments, **resume from the last one's `Next`
field**. Do not redo work. Do not re-read files the comment already describes.

## 2. Claim

```
gh issue develop $ARGUMENTS --checkout
gh issue edit $ARGUMENTS --add-label status:in-progress --remove-label status:ready
```

## 3. Route by `area:` label

| Label | Agents, in order |
|---|---|
| `area:backend` | tech-lead (design only if structural) → backend-go → qa-automation → devsecops |
| `area:frontend` | ux-designer (if no spec) → frontend-react → qa-automation |
| `area:infra` | devops-sre → cloud-sysadmin |
| `area:design` | art-director → ux-designer → motion-designer |
| `area:marketing` | brand-strategist → copywriter → product-verifier |
| `type:security` | devsecops → owning agent → qa-automation |

Skip a stage only when it is genuinely irrelevant, and say which you skipped.

## 4. Checkpoint — mandatory, before you finish or run low

Post the issue comment in the exact format from CLAUDE.md. This comment is the
resume anchor: a fresh session with zero context must be able to continue from
it alone. Write it as if the reader knows nothing.

## 5. Ship the branch

```
git add -A && git commit -m "type(scope): subject (#$ARGUMENTS)"
gh pr create --fill --body "Closes #$ARGUMENTS

## Verification
<paste the command output>

## Honest remainders
<what is not done and why>"
gh issue edit $ARGUMENTS --add-label status:in-review --remove-label status:in-progress
```

Never merge. Merging is the owner's decision.

## 6. Close out

One paragraph: what shipped, what was verified, what remains, and whether
anything needs the owner. Nothing else.
