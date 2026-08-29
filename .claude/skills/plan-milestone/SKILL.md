---
name: plan-milestone
description: Plan or replan an OpenRisk milestone — select ready issues, assess the load against the date, name what gets cut, and sequence the work. Use at the start of a milestone or when one starts slipping.
argument-hint: <milestone title>
---

# Plan milestone: $ARGUMENTS

Delegate to `po-openrisk`, with `tech-lead` consulted on sequencing and
dependencies.

## Produce

1. **Candidate table** — issue · title · estimate · dependencies · risk.
2. **Load versus date** — total S/M/L against the due date, honestly. If it
   does not fit, say so in the first sentence, not the last.
3. **The cut** — which issues leave the milestone and why. Scope is cut by
   removing issues, never by lowering the Definition of Done.
4. **Sequence** — the dependency-ordered list, marking which pairs can run in
   parallel under `/sprint`.
5. **The one risk** most likely to blow this milestone, and the mitigation.
6. **Apply it** — `gh issue edit <n> --milestone "$ARGUMENTS"` for the keepers,
   `--remove-milestone` for the cuts.

End with any item that needs my decision, appended to `docs/DECISIONS.md`.
