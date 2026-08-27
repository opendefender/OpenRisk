---
name: brief
description: The OpenRisk morning brief — milestone state, what shipped, what is in flight, what needs the owner's decision, and today's three highest-leverage issues. Run this first every day. Cheap by design.
---

# Morning brief

Delegate to `po-openrisk`. Give it this instruction verbatim:

> Produce the daily brief. Read only: `gh issue list --state open --json
> number,title,labels,milestone,assignees`, the milestones API, `gh pr list`,
> and `docs/DECISIONS.md`. Do not read source files. Do not read JOURNAL.md.
> Output the daily brief format from your definition, under 25 lines.

Then, without further reading, end with exactly one line:

`START: /work <issue-number>` — the issue you recommend starting now, and why
in under 15 words.

## Token budget

This whole command should cost under 15k tokens. If you find yourself reading
source files, you have misunderstood it. The brief reads GitHub, nothing else.
