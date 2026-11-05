/work {{N}}

You run unattended in the agent runner (#1252). Your turn budget for this issue is {{TURNS}} (size {{SIZE}}).
- Follow CLAUDE.md and docs/AGENT_RULES.md.
- Work only in the issue's worktree, never in the runner's checkout.
- Open the PR against `develop` with `Closes #{{N}}`.
- If you are about to run out of turns: commit and push what exists, post the mandatory progress comment on #{{N}} with `**Blocked on** — turn budget`, and stop. Never leave work only on disk.
- An action the harness refuses is a blocker: post it in your report and stop. Never work around a refusal.

**Before any push, verify the change (rule 8.2, #1294).**
- Run the unit tests, the type check and lint for what you touched.
- If the change can be seen through the UI or the API, verify it **live** on your own throwaway stack:
  `scripts/runner/live-stack.sh up {{N}}`, then exercise the change against the URLs it prints, then `scripts/runner/live-stack.sh test {{N}}` (the E2E PR gate), then `scripts/runner/live-stack.sh down {{N}}`.
  Never use, stop or rebuild the owner's stacks (`openrisk-dev`, `openrisk-staging`).
- Paste the commands and their output in the PR under **Verified**. A UI or API change without a live run is not done, and the reviewer fails it.
