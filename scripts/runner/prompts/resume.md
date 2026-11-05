You are resuming the fix of PR #{{PR}} (issue #{{N}}), which an earlier run did not finish. It was cut by the usage window or by the machine stopping. Its work was committed (`wip(#{{N}}): …`) and pushed to the PR's branch.

1. You are in the PR's worktree. Read the bot's review comments on the PR (they start with `<!-- agent-review:verdict`), the `wip` commits and the issue's last progress comment. Continue from there; do not redo what is done.
2. Your worktree is detached on the tip of `{{BRANCH}}`. Fix **every blocking finding**, and only those. Commit with `git commit -s` and push with `git push origin HEAD:{{BRANCH}}`. Never force push, never start a new branch.
3. Answer each blocking finding in one PR comment: *fixed in `<sha>`*, or *disputed: <reason>*. A dispute goes to tech-lead's single arbitration.
4. Post the mandatory progress comment on #{{N}}.

Turn budget: {{TURNS}}. The rules are CLAUDE.md and docs/AGENT_RULES.md. A refused action is a blocker: report it and stop.

**Before any push, verify the change (rule 8.2, #1294).**
- Run the unit tests, the type check and lint for what you touched.
- If the change can be seen through the UI or the API, verify it **live** on your own throwaway stack:
  `scripts/runner/live-stack.sh up {{N}}`, then exercise the change against the URLs it prints, then `scripts/runner/live-stack.sh test {{N}}` (the E2E PR gate), then `scripts/runner/live-stack.sh down {{N}}`.
  Never use, stop or rebuild the owner's stacks (`openrisk-dev`, `openrisk-staging`).
- Paste the commands and their output in the PR under **Verified**. A UI or API change without a live run is not done, and the reviewer fails it.
