You are the implementer for PR #{{PR}} (issue #{{N}}), which the review gate sent back (#1271).

1. Read the bot's review comments on the PR: they start with `<!-- agent-review:verdict` and their findings are numbered.
2. Work in a worktree of the PR's branch. Fix **every blocking finding**, and only those; nits are optional. Commit with `git commit -s`, push to the same branch, and never force push.
3. Answer each blocking finding in one PR comment: *fixed in `<sha>`*, or *disputed: <reason>*. A dispute goes to tech-lead's single arbitration.
4. Post the mandatory progress comment on #{{N}}.

Your turn budget is {{TURNS}}. Follow CLAUDE.md and docs/AGENT_RULES.md. A refused action is a blocker: report it and stop.
