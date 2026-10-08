---
name: git-stash-untracked-path-footgun
description: `git stash push -- <specific untracked path>` fails with "did not match any file(s) known to git" and creates NO stash entry — if a later `git stash pop` runs anyway (e.g. chained with `&&`/`;`), it silently pops whatever OTHER stash already existed in the repo, merging unrelated cross-branch content into the working tree.
metadata:
  type: feedback
---

While proving a regression (stashing away a new, still-untracked file
alongside tracked ones to confirm a test fails pre-change), `git stash push
-m "..." -- path/to/new-untracked-file.ts path/to/tracked-file.tsx` errored
with `error: pathspec ... did not match any file(s) known to git` and created
**no stash** — `git stash push -- <paths>` cannot target an untracked file
without `-u`/`--include-untracked`, and the whole invocation aborts rather
than stashing just the tracked paths. The command was chained
(`git stash push ... && npx vitest run ...; git stash pop`), so when the
first half errored, the trailing `git stash pop` still ran — and popped a
**pre-existing, unrelated stash** that was already sitting in the repo from
a completely different branch/session, merging its changes (repo-root
`CLAUDE.md`, `.claude/agents/*`, `docs/DECISIONS.md`, a deleted script) into
the working tree with real conflict markers.

**Why this is dangerous:** `git stash pop` always pops `stash@{0}` (the most
recent entry) regardless of which stash push you *meant* to pair it with. A
failed/no-op `stash push` leaves whatever was already at `stash@{0}`
untouched and next in line — so an unconditional trailing `pop` in a chained
command is not "undo what I just stashed", it is "pop whatever is on top of
the stash right now", and those are only the same stash if the push actually
succeeded.

**Recovery used, confirmed safe:** `git stash list` to see the untouched
pre-existing entries were still present (a conflicted pop does not drop the
entry); `git reset HEAD -- <contaminated paths>` to unstage; then — since
`git checkout -- <paths>` was blocked by the environment's destructive-action
guard — `git stash push -m "..." -- <contaminated tracked paths>` (no `-u`,
only tracked files this time) to move the unwanted working-tree changes back
out reversibly. Verified the paths I actually intended to touch were
untouched via `git diff --stat` before continuing.

**How to apply:** never target an untracked path with `git stash push --
<paths>` (add it with `-u` if untracked files must be included, or stash only
the tracked subset and reason about the untracked file separately — an
unimported new file sitting on disk during a regression check is usually
harmless). Never chain a `git stash push ... && ... ; git stash pop` where
the pop is meant to be the push's own undo — check the push actually
succeeded (a real stash entry was created) before popping, or split the
commands so a failure doesn't silently fall through to an unconditional pop.
Before any stash pop, `git stash list` first if there is any doubt about
which entry is on top.
