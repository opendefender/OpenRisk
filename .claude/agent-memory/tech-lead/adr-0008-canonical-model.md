---
name: adr-0008-canonical-model
description: ADR 0008 (canonical model, #541) review 2026-11-03 — approve with changes; the contradictions found and the design points to re-check on the next draft
metadata:
  type: project
---

ADR 0008 (`docs/adr/0008-openrisk-canonical-model.md`, worktree OpenRisk-wt-541) reviewed 2026-11-03 as proposed. Verdict: approve with changes. Score Engine / SmartScore sign-off: not touched (score read-only, confidence never an input, test pinned).

Required changes raised (check they landed on the next draft):
- Edge shape in D5 has no id/tenant_id/updated_at/provenance, yet D7 maps crosswalk/RCM/link columns to an "edge envelope". The edge needs its own envelope defined.
- Edge uniqueness `(source, verb, target)` collides with `evidenced_by.result_id`: one piece of evidence cannot back two runs of the same test.
- `threat_intelligence` "rises to 0.80 by becoming derived" contradicts `derived = min(inputs)`.
- `derived` requires `provenance.derived_from`, but `starter`/`ai`/`vendor` risks have no canonical input record.
- The canonical Control id is "fresh" while nothing is persisted, so identity changes between reads. Needs a deterministic UUIDv5 from compliance_controls.id until #553.
- The D3 provenance closed-key rule plus required `method`/`actor`/`confidence_rule` silently supersedes ADR 0004 D3a's provenance (channel/template_id/...). D-044 requires naming the superseded field.
- The `canonical` package allowlist forbids `internal/domain`, so it cannot return `domain.ErrValidation`. The ADR must say where the sentinel lives.

**Why:** these are the contract-internal contradictions that #542 would freeze under semver.
**How to apply:** on re-review, grep the draft for each of these items before reading anything else. Related: [[adr-0001-entity-registry]], [[adr-0005-router-matrix]].
