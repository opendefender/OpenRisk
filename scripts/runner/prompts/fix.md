You are the implementer for PR #{{PR}} (issue #{{N}}), which the review gate sent back (#1271).

1. Read the bot's review comments on the PR: they start with `<!-- agent-review:verdict` and their findings are numbered.
2. You are in the runner's worktree of the PR, detached on the tip of `{{BRANCH}}` (the branch itself is checked out in the owner's `/work` worktree). Fix **every blocking finding**, and only those; nits are optional. Commit with `git commit -s` and push with `git push origin HEAD:{{BRANCH}}`. Never force push.
3. Answer each blocking finding in one PR comment: *fixed in `<sha>`*, or *disputed: <reason>*. A dispute goes to tech-lead's single arbitration.
4. Post the mandatory progress comment on #{{N}}.

Your turn budget is {{TURNS}}. Follow CLAUDE.md and docs/AGENT_RULES.md. A refused action is a blocker: report it and stop.

**Before any push, verify the change (rule 8.2, #1294).**
- Run the unit tests, the type check and lint for what you touched.
- If the change can be seen through the UI or the API, verify it **live** on your own throwaway stack:
  `scripts/runner/live-stack.sh up {{N}}`, then exercise the change against the URLs it prints, then `scripts/runner/live-stack.sh test {{N}}` (the E2E PR gate), then `scripts/runner/live-stack.sh down {{N}}`.
  Never use, stop or rebuild the owner's stacks (`openrisk-dev`, `openrisk-staging`).
- Paste the commands and their output in the PR under **Verified**. A UI or API change without a live run is not done, and the reviewer fails it.
