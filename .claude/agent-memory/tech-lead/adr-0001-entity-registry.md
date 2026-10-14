---
name: adr-0001-entity-registry
description: ADR 0001 proposes the polymorphic entity contract, fail-closed registry and TenantScope type; docs/adr was empty before it
metadata:
  type: project
---

`docs/adr/` was **empty** until 2026-08-28 — no ADR had ever existed on any branch,
despite CLAUDE.md naming `docs/adr/` as the source of truth for decisions. ADR 0001
(`docs/adr/0001-polymorphic-entity-contract-and-registry.md`) is the first, status
`proposed`, awaiting the owner.

Its two decisions that need the owner's call:
- **D2** — `Registry.Register` panics at DI-wiring time on a registration that omits its
  `TenantGate`, `IDShape` or `IsolationTest`. A deployment that cannot express how a type
  is gated does not boot.
- **D4** — sequential integer ids (incidents) stay raw through the polymorphic route.
  Opaque handles were rejected because the first link-emitting site that forgets falls
  back to the raw id, giving the illusion of a control. Mitigation is 404-parity plus a
  repeated-not-found security signal.

**Why:** Go cannot prove a hand-written GORM query filters by tenant. The ADR states
explicitly what is mechanical (gate declaration, tenant/entity-id transposition,
`uuid.Nil`) and what remains a test obligation, so no later reader mistakes the second
for the first.

**How to apply:** ADR numbering starts at 0001 and the format is the one in the tech-lead
agent brief (Status / Context / Decision / Consequences / Alternatives rejected). Check
`docs/adr/` for the next free number before writing another.

See [[w1-02-entity-drawer-boundaries]] and [[defect-pattern-tenant-isolation]].
