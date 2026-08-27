---
name: resume
description: Rebuild working context after a usage limit, a crash, a /clear or a new day, without re-reading the repository. Reconstructs state from GitHub and the last checkpoint in seconds. Use as the first command of any recovery session.
argument-hint: "[issue-number — optional, defaults to whatever is in progress]"
---

# Resume work

Your context is empty. **Do not explore the repository.** State does not live in
the conversation — it lives in GitHub and in `.claude/CHECKPOINT.md`. Read those.

## 1. Where were we — exactly three commands

```
cat .claude/CHECKPOINT.md 2>/dev/null | tail -30
gh issue list --label status:in-progress --json number,title,labels,url
git status --short && git branch --show-current
```

## 2. Load the one issue

Target issue: `${ARGUMENTS:-the single issue labelled status:in-progress}`.
If several are in progress, list them and ask me which one. Do not guess.

```
gh issue view <n> --comments
```

The **last agent comment is your instruction set.** Its `Next` field is your
task. Its `Done` field tells you what NOT to redo. Its `Verified` field tells
you which commands already passed.

## 3. Confirm the branch, then continue

```
git switch <the branch from gh issue develop>
git log --oneline -5
```

## 4. Report before working — three lines maximum

```
Resuming #<n> — <title>
Last checkpoint: <date> by <agent> — <the Next field>
Continuing with: <what you will do now>
```

Then continue with `/work <n>`.

## Rules that make this cheap

- Never `find` or `ls -R` the repository.
- Never read `docs/JOURNAL.md`.
- Never re-read files the last checkpoint already summarized.
- Never re-run a verification the checkpoint recorded as green, unless you have
  since changed the code it covered.
- If the checkpoint is missing or unreadable, say so and read only the issue.
