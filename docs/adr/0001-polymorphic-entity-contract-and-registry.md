# 0001 — Polymorphic entity contract, entity registry and tenant gating

Status: proposed
Blocked on: owner decision D-005 (docs/DECISIONS.md) — acceptance is the owner's
call, and D-005 is still OPEN. This file stays `proposed` until it is resolved.
Implemented by: #410 (partially — see "Implementation status" below)

## Implementation status — as merged on #410

This ADR is NOT yet accepted — D-005 is open — but its D2 recommendation
(panic at boot on an incomplete registration) is already implemented, because it
is reversible and is the recommended option. The parts that are BUILT are listed
here so that a reader can tell the decision from the delivery. Nothing below is aspirational:
each row was checked against the code on 2026-08-28.

| Decision | Status | Where |
|---|---|---|
| `TenantScope` — unexported id, one constructor, refuses `uuid.Nil` | **built** | `entity/isolation.go`, `TestTenantScope_RefusesTheNilTenant` |
| `TenantGate` closed union with unexported `isTenantGate()` marker | **built** | `entity/isolation.go` |
| `OwnColumn` / `ThroughParent` as its only members | **built** | `entity/isolation.go` |
| `IDShape`, with `IDShapeSequential` requiring an `EnumerationMitigation` | **built** | `TestRegistry_SequentialIDRequiresAnEnumerationMitigation` |
| `Registration` struct; `Register` takes one and panics when incomplete | **built** | `entity/registry.go`, `TestRegistry_IncompleteRegistrationPanics` |
| Cross-tenant coverage derived from `Registry.Registrations()` | **built** | `TestRegistry_EveryRegisteredTypeIsTenantIsolated` (service), `TestEntityDrawer_CrossTenant` (HTTP) |
| `IsolationTest` checked against the source by an AST scan | **built** | `TestRegistry_IsolationTestNamesAnExistingFunction` |
| A tenantless caller answers 401, not 403 | **built** | `TestGet_TenantlessCallerIs401` |
| **Ports take `TenantScope` rather than a bare `uuid.UUID`** | **NOT built** | `RelationReader` and `EventSource` still take `tenantID uuid.UUID`. The transposition this ADR set out to make impossible is still expressible. Tracked as #410 criterion 14. |
| The two vocabulary switches folded into one derived table | **NOT built** | `auditTypes` and `typeForAuditEntity` are still two hand-written switches (`entity/timeline.go`). |
| Audit-trail drop observability | **NOT built** | ADR D6c; #410 criteria 40-41. |

The gate table below describes the three timeline sources as designed. Only
`AssetSnapshotSource`'s row is currently expressed as a `TenantGate` value in
code; the two `ThroughParent` rows are enforced by their queries but are not yet
declared through the registry, because no journal source is a registered entity
type. They are listed because the ADR's purpose is to state the strategy, and
because a future source added without one must be caught by review until the
declaration is mechanical.

## Context

W1-02 (#200) replaces eight per-module detail drawers with one universal drawer
over eight business objects — assets, risks, vulnerabilities, findings,
controls, incidents, vendors, evidence — plus one tenant-wide activity feed.
That collapses roughly thirty-two per-type endpoints (detail, relations,
timeline, audit × 8) onto five routes.

The forces:

1. **Concentration cuts both ways.** Thirty-two routes are thirty-two chances
   for one type to answer a permission question differently from its neighbour.
   Five routes are one place to get it right — and one place to get it
   catastrophically wrong. The drawer is now the broadest id-forging surface in
   the product: a single URL shape reaches every business object we own.

2. **We have already shipped this exact defect, twice.** From the isolation
   sweep of 2026-07-23:
   - `GET /incidents/:id/timeline`, `GET|POST /incidents/:id/actions`,
     `PUT /incidents/:id/actions/:actionId` and
     `POST /incidents/:id/risks/:riskId` read *and wrote* by sequential integer
     id with no check that the parent incident belonged to the caller's tenant.
     Read and write IDOR. Fixed with `IncidentService.ownsIncident`.
   - `RiskTimelineHandler` never read the tenant context at all. Every
     `/risks/:id/timeline*` returned any risk's history by UUID, and
     `GET /timeline/recent` returned **the changes of every tenant in the
     deployment**. `risk_histories` has no `tenant_id` column, so the fix had to
     gate through the parent risk and, for the tenant-wide feed, join `risks`.

   Both are precisely the shapes a universal drawer and a global timeline
   generalise. Building them without deciding the gating strategy in writing
   would be re-running a known incident at eight times the blast radius.

3. **The ids are not homogeneous.** Risks, assets, vulnerabilities, controls
   and evidence use UUID primary keys. Incidents use a sequential integer. A
   polymorphic `{type, id}` route is exactly where an enumerable-integer entity
   gets walked by an attacker who only has to guess a type name and count.

4. **Some history tables have no tenant column.** `risk_histories` and
   `incident_timelines` are children of their parent entity. `asset_snapshots`
   does carry `tenant_id`. A contract that treats all three alike will get one
   of them wrong.

5. **CLAUDE.md ABSOLUTE RULE #2** requires a `tenant_id` filter on every query,
   and requires that a table without one says explicitly which parent gates it.
   Today that rule is enforced by review discipline. Review discipline is what
   produced the six leaks fixed on 2026-07-23.

6. **Go cannot prove tenancy.** No type system available to us can assert that
   a hand-written GORM query contains a predicate. Whatever we decide, part of
   the guarantee will be a test obligation. The design question is how much can
   be moved from *discipline* to *compiler or build failure*, and how loudly the
   remainder announces itself.

## Decision

### D1 — Five routes, the type as a path parameter, the gate in the service

```
GET /api/v1/entities                          catalogue
GET /api/v1/entities/:type/:id                summary + actions + sections
GET /api/v1/entities/:type/:id/relations      edges, grouped by target type
GET /api/v1/entities/:type/:id/timeline       merged, user-facing history
GET /api/v1/entities/:type/:id/audit          raw audit records
```

No `RequirePermission` middleware is mounted on these routes. The required
permission depends on `:type`, so a fixed route guard would either lock out
every non-admin or gate a control behind the risk permission. The gate lives in
`entity.Service.access`, where the type is known, and reads the same descriptor
table the relation filter and the catalogue read.

**Envelopes.** `GET /entities/:type/:id` answers
`{summary, actions, sections}`; relations answer `{groups: [...]}`; timeline
answers `{events, next_cursor, sources}`; audit answers
`{events, total, limit, offset}`. Collections are always a JSON array, never
`null` — a denied or empty relation group carries `items: []`, because a `null`
reaching a client that types the field as an array throws in the browser on the
one path hardest to notice in testing.

**Error shape.** Typed domain errors only, mapped by `writeAppError`:

| Situation | Status |
|---|---|
| unknown `:type`, malformed cursor or filter | 400 |
| type known, caller lacks the type's read permission | 403 |
| id absent, id malformed, id in another tenant | **404, all three identical** |
| type known but no resolver wired on this deployment | 400 |

The three-way 404 collapse is the load-bearing part. A forged id, an
ill-formed id and a real-but-foreign id must be indistinguishable in status,
body and timing class, or the difference is an oracle for enumerating another
tenant's records. The permission check runs **before** the lookup for the same
reason: were it to run after, a caller without permission would receive 404 for
a fabricated id and 403 for a real one, which tells them which ids exist.

### D2 — A registration is a declaration, and it cannot omit its tenant gate

Registering a type is not "hand a resolver to a map". A registration is a value
whose fields cannot be left unset, built through a constructor that refuses an
incomplete one:

```go
type IDShape string
const (
    IDShapeUUID       IDShape = "uuid"
    IDShapeSequential IDShape = "sequential"
)

// TenantGate is a closed union. There is no zero value that means "ungated".
type TenantGate interface{ isTenantGate() }

// OwnColumn: the table carries tenant_id and the repository filters on it.
type OwnColumn struct{ Table string }

// ThroughParent: the table has no tenant_id. Parent names the gating entity and
// GateFunc names the ownsX function or the JOIN predicate that enforces it.
// This is the RiskHistory / IncidentTimeline case, stated rather than implied.
type ThroughParent struct {
    Parent   Type
    Table    string
    GateFunc string // e.g. "JOIN risks r ON r.id = h.risk_id AND r.tenant_id = ?"
}

type Registration struct {
    Type          Type
    Resolver      Resolver
    IDShape       IDShape
    Gate          TenantGate
    IsolationTest string // the test function that proves it
    // Required only when IDShape == IDShapeSequential.
    EnumerationMitigation string
}
```

`Registry.Register` takes a `Registration` and **panics at construction time**
(in `main.go`'s DI wiring, i.e. at process start, not on a request) when
`Resolver`, `IDShape`, `Gate` or `IsolationTest` is unset, or when
`IDShape == IDShapeSequential` and `EnumerationMitigation` is empty. A
deployment that cannot express how a type is gated does not boot. This is
deliberately louder than returning an error: an error would be logged and
ignored, and the failure mode of "type quietly unregistered" is a 400, which is
survivable — whereas the failure mode of "type registered without a gate" is a
cross-tenant read, which is not.

A type with no registration at all remains *unsupported* rather than broken:
`access` answers a typed validation error naming the type, so the platform
still starts with a resolver missing.

### D3 — The tenant id is a distinct, unforgeable type

Today a relation query reads:

```go
RisksForAsset(ctx context.Context, tenantID, assetID uuid.UUID, limit int)
```

Two adjacent `uuid.UUID` values with no type-level distinction. Transposing them
compiles, passes vet, and produces a query that filters an asset table by a
tenant id. We adopt:

```go
// TenantScope is the only thing a repository will accept as a tenant. Its field
// is unexported, so it cannot be built by literal — the sole constructor takes
// a Caller and refuses one whose TenantID is uuid.Nil.
type TenantScope struct{ id uuid.UUID }

func NewTenantScope(c Caller) (TenantScope, error) // error when c.TenantID == uuid.Nil
func (s TenantScope) UUID() uuid.UUID
func (s TenantScope) String() string
```

Every method of `RelationReader`, `EventSource` and every resolver port takes a
`TenantScope` in first position after `ctx`. Three consequences follow
mechanically rather than by review:

- A tenant and an entity id can no longer be transposed — different types.
- `uuid.Nil` cannot enter a query as a tenant: the only constructor rejects it.
  There is no fallback to `uuid.Nil` and no "query across all tenants" path.
- Grepping for `TenantScope` enumerates every tenant-sensitive port in the
  package, which is what an auditor needs and a comment cannot guarantee.

`Caller.Valid()` stays as the outer gate. A caller with `TenantID == uuid.Nil`
or a nil permission source is refused before any query runs.

**Correction to current behaviour:** `access` presently answers *403
"authentication required"* for a tenant-less caller. Per the constitution a
missing tenant context is fail-closed **401**. The drawer routes sit behind the
auth middleware so this is not reachable today, but the code should not depend
on that; the service returns a typed unauthorized error and the handler maps it
to 401.

### D4 — Sequential integer ids stay raw; enumeration is mitigated, not hidden

Incidents may be addressed as `/entities/incident/1`. We do **not** introduce
opaque handles.

The reasoning: the tenant predicate is the actual access control, and it is
asserted per type by test. An opaque handle (an HMAC over
`tenant_id || type || id`) would be defence layered on top of a control that
either works or does not. Worse, it would have to be minted in every place that
emits a link — relations, timeline targets, deep links, the catalogue — and the
first place that forgets falls back to the raw id, which produces the *illusion*
of opacity while leaving the enumerable path open. Illusory controls are more
dangerous than absent ones because they stop people testing the real one.

What we do instead, and what `EnumerationMitigation` on a sequential
registration must name:

1. The 404 collapse of D1 makes a foreign incident indistinguishable from a
   nonexistent one — asserted by test, not asserted by comment.
2. A repeated-not-found counter on the polymorphic routes, keyed by
   (actor, type), emits a security event past a threshold. Enumeration on an
   integer keyspace is cheap and currently *silent*; making it noisy is the
   control that actually applies to it. This is the piece not yet built — see
   the child issue criteria.
3. New entity types added to the drawer SHOULD use UUID primary keys. Incidents
   are grandfathered; migrating them is out of scope for W1-02 and is not
   pretended otherwise.

### D5 — History tables without `tenant_id` are gated through the parent, and say so

`EventSource` implementations declare their gate in the registration (D2):

| Source | Table | `tenant_id`? | Gate |
|---|---|---|---|
| `RiskHistorySource` | `risk_histories` | no | `ThroughParent{Parent: risk}` — `JOIN risks r ON r.id = h.risk_id AND r.tenant_id = ?` |
| `IncidentTimelineSource` | `incident_timelines` | no | `ThroughParent{Parent: incident}` — `JOIN incidents i ON i.id = t.incident_id AND i.tenant_id = ?` |
| `AssetSnapshotSource` | `asset_snapshots` | yes | `OwnColumn{Table: "asset_snapshots"}` |

Two layers, both required:

- **Ownership first.** `Service.Timeline` and `Service.Audit` re-resolve the
  entity's `Summary` before reading any journal. Not redundant: each is a
  separate HTTP request, and a request that names an entity must prove the
  caller may read *that* entity, in *this* tenant, at the time of the call.
  Trusting that a previous call succeeded is how a relation endpoint becomes an
  IDOR — it is what happened to `/incidents/:id/timeline`.
- **The predicate anyway.** The source re-applies the join even though ownership
  was just proven, because the source is reachable from any future caller and
  the join costs an index lookup.

### D5b — The two aliases have DIFFERENT contracts, and the narrowing is one-way

`finding` and `vendor` are both aliases over another type's table. They are not
symmetric, and the package documentation currently claims they are:

| Alias | Over | Narrows? | Enforced by | Tested by |
|---|---|---|---|---|
| `vendor` | `assets` where `category = vendor` | **yes** | `AssetResolver.vendorOnly` (`resolver_asset.go:44,75`) | `TestVendor_RefusesNonVendorAsset` |
| `finding` | `vulnerabilities`, **all rows** | **no** | nothing — `self: TypeFinding` changes the label and the 404 wording only | `TestFinding_IsAnAliasOfVulnerability` pins the non-narrowing |

`types.go:48-52` states that the finding resolver "refuses an id whose row was
not produced by a scan." **It does not.** `NewFindingResolver`
(`resolver_vulnerability.go:55`) differs from `NewVulnerabilityResolver` in one
field. `load()` has no source predicate.

**`resolver_vulnerability.go:29-41` is normative; `types.go:48-52` is wrong and
is corrected to match it.** The resolver's reasoning is the stronger argument
and it is the one the code implements: `domain.Vulnerability` is the register of
what has been *observed* on the estate, and every `VulnSource` member —
including `manual` — is a tool or a person reporting presence. There is no
subset of that table that is "findings but not vulnerabilities". Narrowing to
non-manual sources would fabricate a distinction the domain does not make *and*
would hide manually-reported vulnerabilities from the scanner-facing screens
whose links the alias exists to resolve. Intelligence about CVEs that exist in
the world lives in `cti_vulnerabilities` and is not drawer-addressable at all.

Consequence to state rather than hide: the narrowing is **one-way**. The vendor
resolver refuses a non-vendor asset, but the asset resolver accepts a vendor.
`/entities/asset/<vendor-id>` renders the vendor under an asset heading, by
design. Only the vendor direction is restricted.

### D6 — The global feed reads only the canonical trail, scoped by the Caller

`GET /timeline` reads `audit_events` and nothing else. `audit_events` carries
`tenant_id` of its own; the supplementary journals do not, and a tenant-wide
read of them would mean a join per source for events the trail already carries
for anything done through the API. Stating that the feed has exactly one source
is what keeps it auditable.

Why it cannot regress to the `/timeline/recent` leak:

- The tenant comes from `Caller`, which comes from the signed token. There is no
  parameter, header or body field that can influence it, and after D3 the tenant
  cannot be `uuid.Nil` because `NewTenantScope` refuses one.
- Row-level authorisation: an event is emitted only if the caller may read the
  type it is about. Events about entities with no drawer — automation rules,
  delegations, approval workflows — are governance surface and reach only a
  holder of `governance:audit:read`. A page therefore may come back shorter than
  its limit; that is the filter working, and the cursor still advances past what
  was filtered.
- The feed carries no before/after snapshots. Those stay behind
  `governance:audit:read` on the audit endpoint. A timeline says a field moved;
  it does not say what it moved to.

**Known gap this ADR closes explicitly:** `internal/security/isolation` derives
its route surface by AST and only demands a decision for routes with an id *in
the path*. `GET /timeline` has no path parameter and is therefore invisible to
the gate — which is the same blind spot class that let `/timeline/recent` ship.
The isolation registry gains an explicit entry for `/api/v1/timeline` and
`/api/v1/entities`, and the gate is extended to demand a decision for any route
returning a tenant-scoped **collection**, not only for parameterised routes.

### D6b — Two vocabularies, one mapped pair. The inverse is lossy BY CONSTRUCTION

The audit trail and the entity registry name things differently, and they must,
because they are populated by different writers:

- **The trail's `entity_type`** is derived heuristically from the route by
  `middleware.resourceFromRoute` → `normaliseResource` → `singular`, or from a
  model's `AuditEntityType()` when the GORM plugin observed the row. Its
  vocabulary is therefore *route collections*: `assets` → `asset`,
  `vulnerabilities` → `vulnerability`.
- **The registry's `Type`** has eight members, two of which — `finding` and
  `vendor` — **have no routes of their own**. They are aliases resolved in the
  drawer layer. `grep -in vendor cmd/server/main.go` returns only the two
  registry-wiring lines; a vendor is created and updated through `/assets`.

The mapping is an explicitly declared pair, not an accident:

- **Forward — `auditTypes(Type) []string` (`timeline.go:105`).** Total and
  correct for all eight types. It maps each alias onto its base
  (`TypeAsset, TypeVendor → ["asset"]`; `TypeVulnerability, TypeFinding →
  ["vulnerability"]`) and it handles the two-writer disagreement: a control is
  `compliance_control` when the GORM plugin named it and `control` when the HTTP
  middleware did, so it queries both. Evidence queries `["evidence",
  "control_evidence"]`. Both evidence route shapes (`/evidence` and
  `/compliance/evidences`) normalise to `evidence` — verified. **An entity's own
  audit tab and timeline therefore resolve correctly for all eight types,
  aliases included.**
- **Inverse — `typeForAuditEntity(string) (Type, bool)` (`timeline.go:129`).**
  **Structurally lossy, and not fixable.** It can never return `TypeVendor` or
  `TypeFinding`: a trail row records `entity_type: "asset"`, and whether that
  asset is a vendor is knowable only by loading the row, which the tenant-wide
  feed deliberately does not do (that is what makes it one query instead of N).

**Decision: the global feed's type vocabulary is SIX types** — asset, risk,
vulnerability, control, incident, evidence — and this is declared, not
discovered. A vendor mutation surfaces in the feed as an `asset` and deep-links
to the asset drawer; a scanner-sourced vulnerability surfaces as a
`vulnerability`. Both links resolve to the same row through the same tenant
predicate, so neither is a leak; the vendor case renders under an asset heading,
which is the one-way narrowing of D5b showing through.

**The asymmetry is intended, and it is contradictory in front of a user.** State
it rather than let two defensible decisions collide silently:

- `/entities/vendor/<a-server-id>` is a **deliberate 404**. The drawer refuses
  to render a server under a vendor heading (D5b).
- A vendor mutation in the global feed is journalled as `"asset"` and
  **deep-links into the asset drawer**. The feed renders a vendor as an asset.

So the drawer narrows in one direction and the feed widens in the other, and a
user who clicks a vendor's row in the activity feed lands on an asset heading
for the object whose vendor drawer they could have opened from the register. The
resolution is *not* to relax the drawer's 404 — that guard is the only thing
making the vendor type mean anything. It is to accept that the feed addresses
objects by the type the trail recorded, which is the coarser of the two, and to
say so in the UI copy rather than leave the user to notice the heading changed.

Two more things follow, and both are criteria:

1. **The feed must never offer `vendor` or `finding` as an entity-type filter.**
   Such a chip could never match a row — a W0-05-class control that lies. *As
   built this is not violated:* `GlobalTimelinePage.tsx`'s `KINDS` are change
   verbs (all / created / updated / deleted), and there is no entity-type filter
   in the UI at all. The rule is recorded so that adding one later cannot
   silently introduce eight chips over a six-type vocabulary.
2. Loading enough to distinguish an alias would cost a row read per event. We do
   not do it, and the cost of not doing it is the heading in case 2 above.

### D6bis — Three hand-maintained vocabularies, agreeing by care. Same fix as D2

There are three independent naming authorities for "what kind of thing is this",
and nothing connects them:

| Function | Location | Produces | Maintained by |
|---|---|---|---|
| `resourceFromRoute` → `normaliseResource` → `singular` | `middleware/audit_mutations.go` | the trail's `entity_type`, from a URL | string heuristics |
| `auditTypes(Type) []string` | `entity/timeline.go:105` | drawer type → trail strings | a hand-written switch |
| `typeForAuditEntity(string) (Type, bool)` | `entity/timeline.go:129` | trail string → drawer type | a second hand-written switch |

They agree today. They agree **by care**, not by construction: a new module route
whose collection name pluralises unusually, or a new drawer type, changes one of
the three and nothing tells the author about the other two. That is the same
defect class as `Registry.Register` accepting an ungated resolver (D2) — a
declaration nobody is forced to keep complete — and it gets the same treatment.

Decision: the pair `auditTypes` / `typeForAuditEntity` becomes **one derived
structure**, not two switches. A single table maps each drawer `Type` to its
trail strings and marks which mapping is the canonical inverse; both functions
are generated from it. The inverse remains lossy for the two aliases (D6b) —
that is data loss in the trail, not a table problem — but the loss becomes
*declared in one place* rather than implied by a switch that happens to omit two
cases.

`resourceFromRoute` cannot be folded in: it runs before any drawer type is
known and must answer for routes that have no drawer at all. It is bridged
instead by a test that walks the registered route surface, applies the
heuristic, and asserts every produced string is either claimed by the table or
explicitly listed as drawer-less. A new route that journals under a name no
drawer claims then fails the build rather than going quietly missing from a
timeline — which is how `POST /evidence/:id/links` came to journal as `"link"`.

### D6c — The trail write is best-effort; the timeline inherits that

`middleware.AuditMutations` writes with `_ = appender.Append(...)`
(`audit_mutations.go:78`), deliberately: a trail write must never fail the
business call the user just completed. Its other guards are sound and are not
changed — it skips when `mw.OrganizationID == uuid.Nil` rather than
mis-attributing a tenant, skips non-2xx, and `createdIDFromResponse` is bounded
to 64 KB, JSON only, accepting a string or a number so a nested relation object
is never mistaken for identity.

The consequence reaches this ADR because the drawer's timeline and the global
feed both read that trail: **a dropped write means the timeline is incomplete
and nothing currently says so.** For a product whose audit trail is the
ISO 27001 A.8.15 / A.5.28 artefact, silent incompleteness is the wrong default.

Decision: the write stays best-effort — failing a user's successful mutation to
record it would be worse — but it stops being silent. An `Append` failure logs
at error level with the tenant, actor, entity type and entity id, and increments
a counter that alerting can watch. The timeline's `sources` field already tells
a reader which journals were consulted; it does not and cannot tell them a row
was lost, so the signal has to be operational.

This middleware arrived in `7f2b8fa`, a fix commit, and was never issue-tracked.
It sits upstream of both 200b and 200c. Recording it here is the tracking.

### D7 — What remains a test obligation, stated honestly

D2 and D3 make three things mechanical: a registration cannot omit its gate
declaration, a tenant cannot be transposed with an entity id, and `uuid.Nil`
cannot become a tenant. None of them proves a resolver's SQL actually filters.

That last step is a **conformance test driven off the registry itself**:
`TestRegistry_EveryRegisteredTypeIsTenantIsolated` iterates
`Registry.Registrations()` and, for each, requires a fixture row in tenant A and
tenant B and asserts a cross-tenant read is 404. Adding a type without adding
its two-tenant fixture fails the build. A second test asserts every
registration's `IsolationTest` names a function that exists in the package, by
AST scan — the same technique `internal/security/isolation` already uses.

The guarantee is therefore: *nobody can register a type without declaring how it
is gated, and nobody can register a type without a test that the gate holds.*
Not: *the compiler proves the gate holds.* That distinction is stated here so no
later reader mistakes the second for the first.

## Consequences

**Easier**

- One permission table. A type is gated identically in the drawer, in a relation
  list and in the catalogue, because all three read `descriptors`.
- One place to audit cross-module joins: `gorm_entity_relation_repository.go`.
  Previously the same joins were scattered across five module repositories.
- Adding a ninth entity type is a `Registration` plus a `Resolver`, and the
  build tells the author what they forgot.
- The global feed answers "what changed here today, and who changed it" across
  every module — a question no single register can answer.
- Every relation chip and every timeline row deep-links to its subject's drawer,
  because `DeepLink` is built server-side from one table rather than assembled
  in three clients that drift.

**Harder**

- Every existing `RelationReader` and `EventSource` signature changes when D3
  lands. That is a mechanical but wide edit, and it is the cost of making
  transposition a compile error.
- Opening a drawer is up to four HTTP requests (summary, relations, timeline,
  audit). Deliberate: they are separately slow, separately permissioned and
  separately allowed to fail, and folding them into one read would mean a single
  failing relation query blanks the whole drawer.
- The service re-resolves the summary on every sub-read. One extra indexed
  lookup per relations/timeline/audit call, paid on purpose.
- `Registry.Register` panicking at boot means a wiring mistake is a failed
  deployment rather than a degraded one. That is the intent; it must be covered
  by a startup smoke test so it is caught in CI and not in production.
- Incidents remain enumerable by integer. Mitigated (D4), not eliminated.

- **Prose in this wave has asserted behaviour the code does not have, three
  times.** This is a pattern, not three unrelated slips, and it is named here
  because the wave's stated purpose is preventing exactly it (RULE #12):
  1. `types.go:48-52` claims the finding resolver refuses non-scan rows. It does
     not, and `resolver_vulnerability.go:29-41` argues the opposite in the same
     package (D5b).
  2. Commit `7f499ab` describes the global feed as spanning "risks, assets,
     findings, controls, incidents and evidence" — it names *findings*, which
     the feed's vocabulary cannot produce, and omits *vendors* entirely.
  3. The same sentence is in the code, at `GlobalTimelinePage.tsx:8`, so it will
     be read by everyone who opens the file and not only by whoever runs
     `git log`.

  Consequence: a doc-vs-code check belongs in review for this package
  specifically. Comments here are load-bearing — several of them are the only
  record of *why* a tenant predicate is shaped the way it is — which is exactly
  why a false one is expensive.

**Neutral but recorded**

- `finding` and `vendor` have no tables of their own. A finding is a projection
  of the vulnerability register; a vendor is an asset of category `vendor`. Both
  are resolvers over an existing table. Only the vendor one narrows (D5b).
  Inventing storage to satisfy a drawer is the failure mode this wave exists to
  prevent.
- Linking evidence to a control (`POST /evidence/:id/links`) journals under
  `entity_type: "link"`, which no drawer type claims. Such events reach only a
  holder of `governance:audit:read` in the feed and do not appear in the
  evidence drawer's own timeline. Recorded rather than fixed; it is a gap in
  coverage, not in isolation.
- Legacy `/risks/:id/timeline*` and `/timeline/recent` stay mounted for now.
  Retiring them is tracked separately; both were tenant-threaded in the
  2026-07-23 sweep and are not a live leak.

## Alternatives rejected

**A — One detail/relations/timeline/audit endpoint per type (~32 routes).**
Rejected: thirty-two independent guards is thirty-two independent chances to
answer a permission question differently, and it is the arrangement that
produced the incident and risk-timeline leaks. It also gives no single place to
audit cross-module relation joins.

**B — Opaque, signed entity handles for all types.**
Rejected: see D4. Opacity would have to be minted at every link-emitting site;
the first omission silently falls back to a raw id, leaving the appearance of a
control without the control. It also breaks operator ergonomics (support cannot
correlate a URL to a record) and makes every relation row a signing operation.

**C — A tenant-scoped `*gorm.DB` handle injected per request, with a global
scope callback.**
Rejected: it would make the filter invisible in the query text, which is exactly
where reviewers and the isolation gate look. A predicate applied by a callback
that a `Table()`-based raw query silently bypasses — and this package uses
`Table()` for its projections — is worse than an explicit one. It would also
make `ThroughParent` joins impossible to express.

**D — `Registration.Gate` as a free-text string documenting the strategy.**
Rejected: a string cannot be enumerated by the conformance test, and a comment
that lies is the exact artefact of the 2026-07-23 sweep. The closed union of D2
can be switched on, tested, and rendered into the isolation registry.

**E — Merge the supplementary journals into the tenant-wide feed.**
Rejected: `risk_histories` and `incident_timelines` have no tenant column, so a
tenant-wide read of them is a join per source across the whole table — the exact
query whose absence let `/timeline/recent` leak. The trail already carries
everything done through the API; the journals are merged only in the per-entity
timeline, where the parent is known and ownership has already been proven.

**F — Give `finding` and `vendor` their own permissions and tables.**
Rejected: it would invent an authorisation concept and storage the product does
not have. `finding` is gated by `vulnerabilities:read`, `vendor` by
`assets:read`.

**G — Narrow the `finding` alias to non-manual `VulnSource` values, to make
`types.go:48-52` true.**
Rejected: it would hide manually-reported vulnerabilities from the
scanner-facing screens whose links the alias exists to resolve, and it would
fabricate a distinction the domain does not make. The documentation is corrected
to match the code, not the reverse (D5b).

**H — Require a round-trip criterion: every object mutated through its module
route produces a trail entry the feed finds under the type the drawer addresses
it by, for all eight types.**
Rejected as specified: it would fail by design for `vendor` and `finding`,
because the inverse map is lossy by construction (D6b) and no amount of testing
makes a `entity_type: "asset"` row remember it was a vendor. The criterion that
*is* adopted is narrower and achievable: the feed's vocabulary is declared as
six types, the forward map `auditTypes` round-trips for all eight in the
per-entity timeline, and the UI never offers a filter chip the vocabulary cannot
match.
