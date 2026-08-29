# W1-02 — Audit: entity detail surfaces and timelines before the universal drawer

Read before implementing. This is the inventory the universal drawer replaces or
absorbs, established by reading the code — not by assuming.

## 1. Detail surfaces that exist today

| Surface | File | Shape | Data |
| --- | --- | --- | --- |
| Risk detail | `frontend/src/features/risks/RiskRegisterPage.tsx` (inline, ~1465 lines) | Drawer inline in the page, 6 tabs | Real API |
| Incident detail | `frontend/src/features/incidents/IncidentDrawer.tsx` | Drawer | Real API |
| Control detail | `frontend/src/features/compliance/ControlDrawer.tsx` | Drawer | Real API |
| Evidence detail | `frontend/src/features/evidence/EvidenceDrawer.tsx` | Drawer | Real API |
| Asset history | `frontend/src/features/assets/AssetHistoryDrawer.tsx` | Drawer (history only) | Real API |
| CVE detail | `frontend/src/features/cti/CveDetailDrawer.tsx` | Drawer | Real API |
| Vulnerability detail | `frontend/src/features/vulnerabilities/VulnerabilitiesPage.tsx` (inline) | Drawer inline | Real API |
| Mitigation detail | `frontend/src/features/mitigations/MitigationDetailDrawer.tsx` | Drawer | Real API |

Findings:

- **Eight independent drawer implementations**, each with its own header, tab
  model, loading/empty/error handling and close behaviour. None is URL-driven:
  every one holds the selected entity in local `useState`, so a drawer cannot be
  deep-linked, survives no refresh, and Back does not close it.
- **No relation navigation.** A risk's asset is text, not a link; opening it
  means leaving the page and losing filters.
- **No fixture-backed drawer.** A grep for `mock|fixture|fake|dummy|sample|hardcoded`
  across `src/features` (excluding tests) returns only `placeholder=` form props,
  a `RiskRulePage` preview of real matched rows, an "insert sample payload"
  button in the vulnerability ingest form, and the War Room banner that states
  in prose that its composer is not backed by a service. The rule in §60 is
  therefore already satisfied at the starting line and must stay satisfied.

## 2. Deep-link convention already in place

`frontend/src/shared/useFocusParam.ts` (UX-1) reads `?focus=<id>&tab=<tab>` and
**clears it** with `{ replace: true }` once consumed. That is deliberate for a
search-palette hand-off but it is the opposite of what W1-02 requires: the URL
must *hold* the drawer state so refresh, Back and a copied link all work. W1-02
introduces `?drawer=<type>&entity=<id>&tab=<tab>` as the durable state and keeps
`?focus=` working by translating it.

## 3. Timeline / history sources (candidate sources of truth)

| Source | Table | Scope | Covers |
| --- | --- | --- | --- |
| **`domain.AuditEvent`** | `audit_events` | tenant, hash-chained, append-only | **Every** entity type: `entity_type` + `entity_id`, actor, before/after, changed fields, IP, request id |
| `domain.RiskHistory` | `risk_histories` | child of a risk (no tenant column) | Risk score/status changes, including those made by background workers |
| `domain.IncidentTimeline` | `incident_timelines` | child of an incident | Incident lifecycle entries |
| `domain.AssetSnapshot` | `asset_snapshots` | tenant | Asset change snapshots with `changed_by` |

`audit_events` is the canonical universal source: it is the only one keyed by
`(entity_type, entity_id)` for all eight types, the only one with an actor and a
before→after diff, and it is written automatically for every successful mutating
request by `middleware.AuditMutations`. §18 forbids introducing a parallel
journal, so the timeline reads `audit_events` and **merges** the domain journals
that record events the HTTP audit trail cannot see — a risk score recomputed by
`ScoreWorker`, an incident entry appended by the incident service.

`ListAuditEventsUseCase` already filters by `EntityType`/`EntityID` and resolves
actor emails, so the entity timeline and audit tab reuse it rather than a new
query.

## 4. Entity-type mapping to the real domain

The mission names eight types. Two of them do not exist under that name and are
mapped honestly rather than invented:

| Mission type | Real model | Identifier | Note |
| --- | --- | --- | --- |
| Asset | `domain.Asset` | uuid | |
| Risk | `domain.Risk` | uuid | |
| Vulnerability | `domain.Vulnerability` | uuid | |
| Finding | `domain.Vulnerability` with a scanner/assessment source | uuid | **No `Finding` table exists.** Scanner `FindingDiscovery` is a transient preview struct; a finding becomes persistent only as a `Vulnerability`. The drawer exposes findings as the scanner-sourced projection of a vulnerability. |
| Control | `domain.ComplianceControl` | uuid | |
| Incident | `domain.Incident` | **uint** (sequential) | The only non-uuid id; the drawer's id is a string and the resolver parses it. |
| Vendor | `domain.Asset` with `category = "vendor"` | uuid | **No `Vendor` table exists.** `domain.CategoryVendor` is a first-class asset category with its own attribute schema; vendors are assets. |
| Evidence | `domain.Evidence` | uuid | |

## 5. Detail endpoints that already exist (reused, not replaced)

`GET /risks/:id`, `/assets/:id`, `/vulnerabilities/:id`,
`/compliance/controls/:controlId`, `/incidents/:id`, `/evidence/:evidenceId`,
plus relation reads `/risks/:id/mitigations`, `/risks/:id/incidents`,
`/risks/:id/control-mappings`, `/asset-dependencies`,
`/compliance/controls/:controlId/evidences`, `/assets/:id/history`,
`/risks/:id/timeline`.

The drawer must not fan out to a dozen of these per open (§45). W1-02 therefore
adds one small generic contract over the existing repositories rather than
wiring the frontend to N per-module endpoints, and rather than the
17-endpoints-times-8-entities explosion §42 forbids.

## 6. Guardrails this work must satisfy

- `internal/security/isolation/registry.go` fails the build for any parameterised
  route with no recorded isolation decision. Every new `:type`/`:id` route needs
  an entry.
- RULE #2: every query filters by `tenant_id`. Relations cross entity families,
  so each side must be re-authorised, not just the entry point.
