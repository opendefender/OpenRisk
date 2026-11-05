/resume

You are resuming OpenRisk issue #{{N}}, which an earlier run did not finish. It was cut by the usage window or by the machine stopping. Its work was committed (`wip(#{{N}}): …`) and pushed to the issue's branch.

1. Find the branch: `git branch -r --list 'origin/*/{{N}}-*'`. Work in its worktree (`../OpenRisk-wt-{{N}}`), creating it from that branch if it is missing. Never start a new branch.
2. Read the issue's last progress comment and the `wip` commits. Continue from there; do not redo what is done.
3. Then follow `/work {{N}}` to the end, live verification included (below), and open or update the PR against `develop`.

Turn budget: {{TURNS}}. The rules are CLAUDE.md and docs/AGENT_RULES.md. A refused action is a blocker: report it and stop.

**Before any push, verify the change (rule 8.2, #1294).**
- Run the unit tests, the type check and lint for what you touched.
- If the change can be seen through the UI or the API, verify it **live** on your own throwaway stack:
  `scripts/runner/live-stack.sh up {{N}}`, then exercise the change against the URLs it prints, then `scripts/runner/live-stack.sh test {{N}}` (the E2E PR gate), then `scripts/runner/live-stack.sh down {{N}}`.
  Never use, stop or rebuild the owner's stacks (`openrisk-dev`, `openrisk-staging`).
- Paste the commands and their output in the PR under **Verified**. A UI or API change without a live run is not done, and the reviewer fails it.
