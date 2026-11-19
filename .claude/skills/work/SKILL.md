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
worktree, branched from the latest develop (ADR 0009, #1244):

```
git fetch origin
git worktree add ../OpenRisk-wt-$ARGUMENTS -b <type>/$ARGUMENTS-<slug> origin/develop
gh issue edit $ARGUMENTS --add-label status:in-progress --remove-label status:ready
```
When `frontend/package-lock.json` matches the main checkout, symlink its
`frontend/node_modules` instead of reinstalling.

## 3. Brief — `scout` first, unless the issue is S

Read the `## Estimate` line of the issue body. On M, L, or no estimate, call
`scout` before any other agent, with the issue (title, body, labels, and the
comments with their `author_association`), the worktree path and the short SHA
of `origin/develop`. Save its answer verbatim as `brief.md` at the worktree
root (git ignores it). Every agent called after that gets the path to
`brief.md` in its prompt and starts from it instead of re-exploring the code.
On S, skip `scout` and say so.

## 4. Implement — route by `area:` label

| Label | Implementation, in order |
|---|---|
| `area:backend` | tech-lead (design, if structural) → backend-go → qa-automation |
| `area:frontend` | ux-designer (if no spec) → frontend-react → qa-automation |
| `area:infra` | devops-sre → cloud-sysadmin |
| `area:design` | art-director → ux-designer → motion-designer |
| `area:marketing` | brand-strategist → copywriter |
| `type:security` | devsecops (finding) → owning agent → qa-automation |

Skip an implementation stage only when it is genuinely irrelevant, and say
which. **Never skip the review.**

## 5. Review — reviewers chosen by the script, not by judgement

The reviewers come from the diff and the labels (#1005). Once the work is
committed, from the worktree:
```
s=$(git show origin/develop:.claude/hooks/route-reviewers.sh) && bash -c "$s" route-reviewers origin/develop
```
The script is the one on `develop`, never the branch's copy: a change must
not choose its own reviewers, nor run unreviewed code with your credentials.
Any non-zero exit from this command, or an empty output on a branch with
changes, is a stop.
It prints one agent per line. Call **every** agent it prints, in parallel,
each with the worktree path, the branch and the `git rev-parse HEAD` it is
to review. Give `brief.md` to `code-reviewer` only, as context, never to
`devsecops`: the brief is built from public text and a security review judges
the diff on its own. Exit 2 or 3 means it could not decide (bad base, labels
unreadable): fix the cause and run it again; never read a failure as "no
reviewer". Empty output on a branch with changes means nothing is committed:
commit, then run it again. Run it again after every new commit: a fix can
reach a path that needs another reviewer.

You may add a reviewer it did not print, never drop one. One addition is
mandatory, whatever the script prints: call `devsecops` when the issue
carries `area:backend`, `area:infra` or `type:security`, or the diff touches
auth, sessions, tenant data, uploads, outbound requests, secrets, an
`.env*` file, a CI or DAST gate, or anything under `.claude/`. This floor
is never dropped.

### Review loop

1. The reviewer reads the whole diff, not a summary:
   `git diff origin/develop...HEAD`. It ends with `Read: <base>..<sha>` and
   `REVIEW: PASS` or `REVIEW: BLOCK — <n> critical` (devsecops:
   `SECURITY: PASS|FAIL`). Keep each reviewer's `<sha>`; for a reviewer
   whose output has no `Read:` line, keep the HEAD you gave it.
2. BLOCK or FAIL goes back to the implementing agent with the findings
   verbatim. It fixes them, then every reviewer, the ones that passed
   included, gets the `<sha>` of its last `Read:` line and reads only
   `git diff <sha>..HEAD`. If that command fails (unknown or collected
   SHA), it reads the full `git diff origin/develop...HEAD` again.
3. A PASS covers the diff it read. Any later change, a CI fix included, goes
   back to every reviewer already called on this branch, plus any the script
   now prints, before the PR is handed over. Before the PR, each kept `<sha>`
   equals `git rev-parse HEAD`.
4. After two blocked rounds, `tech-lead` arbitrates. What it cannot settle
   goes to `docs/DECISIONS.md` with both positions and the reviewer's
   findings, and is reported. Do not open the PR over a block.

### Live check (frontend and any user-visible change)

Run the branch for real (see the live boot recipe in agent memory or
`dev/`), exercise the screen in a browser, and record in the issue comment
which screen, which states (loading, error, empty, filled) and which role
were checked. Unit tests alone do not close a UI issue.

## 6. Checkpoint — mandatory, before you finish or run low

Post the issue comment in the exact format from CLAUDE.md, including the
**Review** line. This comment is the resume anchor: a fresh session with zero
context must be able to continue from it alone. Write it as if the reader
knows nothing.

## 7. Ship the branch

Stage by explicit path, never `git add -A`, and look at what you stage:
```
git status --short
git add <path> <path>
git commit -s -m "type(scope): subject (#$ARGUMENTS)"
git log --no-merges --format='%h %s' origin/develop..HEAD   # every line must carry #$ARGUMENTS
git push -u origin HEAD
gh pr create --base develop --title "type(scope): subject (#$ARGUMENTS)" --body-file pr.md
```
`pr.md` holds `Closes #$ARGUMENTS`, the verification output, the review
verdicts, and the honest remainders. If `gh pr create` fails with the
Projects classic error, use the REST fallback in CLAUDE.md.

Then check that GitHub links the issue. A PR always has its issue:
```
gh api graphql -f query='{repository(owner:"opendefender",name:"OpenRisk"){pullRequest(number:<pr>){baseRefName closingIssuesReferences(first:5){nodes{number}}}}}'
```
It must list `$ARGUMENTS`. If the list is empty, the base is almost always
not `develop` (GitHub links `Closes` only on the default branch): fix it with
`gh api -X PATCH repos/opendefender/OpenRisk/pulls/<pr> -f base=develop`, merge
`origin/develop` into the branch if it was cut elsewhere, and check again.

### Wait for CI before handing over

```
gh pr checks <pr> --watch
```
A check your branch turned red is yours: fix it and push. A check that is
also red on develop is named in the PR body with its tracking issue. Before
opening one, search open and recently merged PRs on develop for a fix
(`gh pr list --base develop --state all --search "<package or check>"`): a
fix already merged means your branch is behind, so merge `origin/develop`.
Open a tracking issue only when there is no fix and no issue yet. Only then:
```
gh issue edit $ARGUMENTS --add-label status:in-review --remove-label status:in-progress
```

Never merge. Merging is the owner's decision.

## 8. Close out

One paragraph: what shipped, what was verified, the review verdicts, the CI
state, what remains, and whether anything needs the owner. Nothing else.
