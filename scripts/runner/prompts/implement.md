/work {{N}}

You run unattended in the agent runner (#1252). Your turn budget for this issue is {{TURNS}} (size {{SIZE}}).
- Follow CLAUDE.md and docs/AGENT_RULES.md.
- Work only in the issue's worktree, never in the runner's checkout.
- Open the PR against `develop` with `Closes #{{N}}`.
- If you are about to run out of turns: commit and push what exists, post the mandatory progress comment on #{{N}} with `**Blocked on** — turn budget`, and stop. Never leave work only on disk.
- An action the harness refuses is a blocker: post it in your report and stop. Never work around a refusal.
