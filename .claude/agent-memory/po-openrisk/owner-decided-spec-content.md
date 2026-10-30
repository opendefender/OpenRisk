---
name: owner-decided-spec-content
description: When a D-entry fixes what a spec contains (D-030 "17 entities"), narrowing or growing it is an owner call even if an audit calls it "sequencing"; refine everything else and park only that criterion
metadata:
  type: feedback
---

If an owner decision states the content of a deliverable (D-030: the #541 ADR has "17 entities"),
a proposal to narrow it, even when framed as "sequencing, not a cut", goes to DECISIONS.md. #541
on 2026-10-30 became D-083. Refine the rest of the issue fully, write the scope-dependent criteria
for the recommended option, mark them "pending D-xxx", and keep `status:needs-refinement` so
`/decide` only has to flip the label.

**Why:** the owner overrides scope-cut recommendations ([[owner-overrides-on-scope-and-polish]]),
so a PO-side narrowing of an owner-decided scope would be reversed, and it would have been
presented as decided when it was not.

**How to apply:** check the slice against the decided list. The 2026-09-25 audit slice added two
entities that were not in it (`ControlImplementation`, `ControlTest`). Also check that "spec +
code in one issue" does not conflict with a "no code before ADR accepted" decision. If it does,
split the code into a follow-up issue, which is hygiene and not an escalation. See
[[verify-agent-claims-before-drafting]].
