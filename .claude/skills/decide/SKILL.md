---
name: decide
description: Review and resolve the pending owner decisions in docs/DECISIONS.md — each with its context, options, recommendation and cost of delay. Use when you are ready to unblock the team.
---

# Pending decisions

Delegate to `po-openrisk`. Read `docs/DECISIONS.md` and only the files each
decision explicitly references.

For each open decision present:

```
### D-<n> — <question>
**Context** — two sentences.
**Options** — A / B / C, each with its consequence.
**Recommendation** — one option, and the reason in one sentence.
**Cost of delay** — what is blocked and how much it grows per week.
**Reversible?** — yes | no
```

Order by cost of delay, highest first. Reversible decisions get a shorter
treatment — recommend and note that we can change course cheaply.

After I answer, record each decision in `docs/DECISIONS.md` with the date and
the rationale, unblock the affected issues by flipping their labels, and post
a comment on each explaining the decision so a future agent understands it.
