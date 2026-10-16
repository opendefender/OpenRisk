---
name: transcript-tooling-posture
description: Review of scripts/agents/token_usage.py (#1003, 2026-10-15) that reads the owner's private Claude Code transcripts; printed channels and what to re-check on revisions
metadata:
  type: project
---

#1003 (2026-10-15) added scripts/agents/token_usage.py. It reads ~/.claude/projects/<slug>/ transcripts and prints aggregates. The first review (c34d3a1d) passed with two LOWs. Both were fixed in 38fdde11 and re-reviewed: PASS with no findings.

- Printed channels: the session id (from the filename), the top-level `timestamp`[:10], `message.model` (any string, not pattern-checked), the agent type from .meta.json (must match `[\w.-]{1,64}`), PR numbers, the PR state (only merged/open/closed/?), `--repo` (validated) and ints. Tracebacks carry no data values.
- .meta.json also holds `description` and `name`, which are the model-written task text. The test now plants a sentinel there, in a timestamp suffix, and in prompts, commands and results. It checks stdout and stderr in 4 modes.
- PRs now count only for `--repo` (default opendefender/OpenRisk). Creation commands must start a shell command. The gh api call is `repos/<validated>/pulls/<int>`, and its output is reduced to a three-value set.
- Residual risk, not filed: `model` is the one printed transcript string with no pattern check. It is written by Claude Code, not by the model.

**Why:** transcripts hold secrets and private text. Every later revision of agent tooling that reads them must be checked against these channels.
**How to apply:** on a change to scripts/agents/*, grep for print/stderr, diff the set of keys the script reads, and run the script on the real data against a strict line regex. See [[agent-hooks-posture]].
