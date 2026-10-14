---
name: comments-claiming-guarantees
description: Backend files in this repo carry doc comments asserting guarantees (atomicity, tenant safety) the code never implements — read the function body before any criterion cites the guarantee
metadata:
  type: project
---

Several backend use cases in this repo document a guarantee they do not implement.
The comment reads as a spec that was written first and never satisfied.

**Confirmed instance:** `backend/internal/application/risk/bulk_action.go` says
*"MANDATORY: All operations must be atomic within a transaction"* and *"Operation
is atomic (all succeed or all fail)"*. The code is a per-item loop with no
transaction that returns `BulkActionResult{Success, Failed, Errors}` — it is
*built* to partially succeed. Same file: `performedBy` is threaded through all
four helpers and read by none of them (no audit record at all), and
`BulkActionRemoveTags` is declared at `:23` but has no `case` in the switch, so
`"remove_tags"` returns *"unknown action type"*. There is no `bulk_action_test.go`,
which is how all three survived. Found 2026-09-07 refining #235; raised as D-036
(the response contract is a breaking change) and as child issue B of that split.

**Why:** this is the same failure mode as the `finding` alias incident in
[[verify-agent-claims-before-drafting]] — two of three wrong claims there came
from reading a comment instead of the function under it. Here the risk is sharper:
an aspirational comment about *tenant safety* or *atomicity* becomes a governance
capability we report to a regulated customer under ABSOLUTE RULE 12.

**How to apply:** never let a doc comment's guarantee become an acceptance
criterion or a claim-matrix row. Grep the function body for what would implement
it (`Transaction`, `Tx`, an audit append, the `case` arm) and cite line numbers.
A use case with no `_test.go` is where to look first — the test is what would have
caught the divergence, so its absence predicts the divergence.
