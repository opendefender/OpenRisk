---
name: defect-pattern-tenant-isolation
description: The three recurring shapes of cross-tenant leak in OpenRisk, and where each one hid
metadata:
  type: project
---

OpenRisk has shipped cross-tenant reads three times in three distinct shapes. Treat any
new query as guilty until its WHERE clause is read.

**Shape 1 — parent-gated child read with no ownership check.** `/incidents/:id/timeline`,
`/incidents/:id/actions` read and wrote by sequential integer id without checking the
parent incident's tenant. Fixed 2026-07-23 with `IncidentService.ownsIncident`.

**Shape 2 — handler never reads the tenant context at all.** `RiskTimelineHandler` served
every `/risks/:id/timeline*` by UUID with no tenant. `risk_histories` has no `tenant_id`
column, so the fix is a JOIN onto the parent `risks`.

**Shape 3 — unparameterised collection route.** `GET /timeline/recent` returned every
tenant's changes. `internal/security/isolation/registry.go` derives its route surface by
AST and only demands a decision for routes with an id *in the path*, so it never asked
about this one. Same blind spot still applies to `GET /timeline` (W1-02).

**Why:** every one of these was found by a human reading code, not by CI. The isolation
gate closes shape 1 and 2 automatically over new code; shape 3 is still open.

**How to apply:** when reviewing any new read, ask (a) does the table have `tenant_id`,
(b) if not, which parent gates it and is the gate function named `ownsX`, (c) is the route
parameterised — if not, the isolation gate did not check it and you must. Also check
`internal/handler/context_helpers.go:safeGetUUID`, which returns `uuid.Nil` rather than
failing closed; any handler using it for a tenant is a latent shape-2 defect.

See [[w1-02-entity-drawer-boundaries]] and [[adr-0001-entity-registry]].
