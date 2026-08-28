# W0-07 — Real-Time Event Hub

## Executive Summary

OpenRisk now has one real-time backbone instead of three hand-rolled streams and
two clients pointing at endpoints that never existed.

A business change becomes a **canonical domain event** in a versioned envelope,
is **appended to a durable per-tenant ordered log**, is **fanned out** to the
other API instances, and reaches the browser over **one authenticated,
tenant-scoped SSE connection per tab**. A client that disconnects resumes from a
cursor; a client that falls too far behind is told to resynchronise rather than
left believing it is current.

Two guarantees are load-bearing, and both are tested and proved live:

> **Tenant A never receives tenant B's events** — not through the stream, not
> through a filter, not through a replay cursor, not by naming B in a header.

> **A reconnect or a redelivery does not cause the same business event to be
> processed twice** — every event has a server-minted id, and every consumer
> deduplicates on it.

Delivery is **at-least-once**. Exactly-once is not claimed, because the system
cannot provide it.

## Current Architecture

Before this wave (full inventory in `W0-07_REALTIME_EVENT_HUB_AUDIT.md`):

* `pkg/events` held four Redis channel names with bare payload structs — no
  event id, no version, no ordering, no correlation. One of the four
  (`risk.score_updated`) was published on every score recomputation and consumed
  by nobody, the clearest evidence that the "SSE hub" its comments referenced
  had never been built.
* Three SSE endpoints (mitigations, scanner, report progress) each
  re-implemented headers, subscription, keepalive and teardown. Two filtered by
  tenant *after* subscribing to a global channel.
* No durability: an event published while a client was reconnecting was gone.
  The scanner worked around this with a job-specific replay read back from
  Postgres — the only reconnect-safety in the codebase.
* No backpressure anywhere: a client that stopped reading held a goroutine and a
  Redis subscription until the kernel buffer filled.
* No stream authorization, no metrics, no structured logging.
* The frontend opened a connection per feature, plus two WebSocket clients
  pointing at `ws://localhost:8080/ws/notifications` and `/api/v1/ws/dashboard`,
  neither of which the backend has ever served.

What was worth keeping, and was kept: the `<aggregate>.<action>` naming, the
per-tenant Redis channel pattern the scanner already used, the audit chain's
sequencing technique, and `promauto` registration at import time.

## Architectural Decision

```
Business mutation
      ↓  (observed where the audit trail already observes it)
Canonical domain event      pkg/realtime.Envelope, validated against the catalog
      ↓
Durable append              realtime_events, per-tenant monotonic sequence
      ↓
Local hub dispatch  ─────┐  tenant-keyed routing table, bounded buffers
      ↓                  │
Redis fanout             │  openrisk:realtime:events:<tenant>
      ↓                  │
Relay on every instance ─┘
      ↓
SSE                         GET /api/v1/realtime/events
      ↓
Frontend client             one per tab: dedup, ordering, gap detection, resync
      ↓
Query invalidation          the API remains the source of truth
```

Three decisions are worth stating because the alternatives were real:

**Validate → persist → deliver, in that order.** Delivering before persisting
would produce events that whoever happened to be online saw and that no
reconnecting client could ever replay. That is the failure that makes a
real-time feature untrustworthy rather than merely incomplete.

**Derive events from the audit observation, not from a call per use case.**
There are several hundred mutating use cases. A publish call added to each is a
call somebody eventually forgets, and the one they forget is the one that
mattered. The audit middleware already sees every successful mutation with the
tenant, the actor, the resource that changed and the changed fields.

**Keep it a closed catalog.** The bridge can only emit names the catalog
defines. That is what stops "derived from HTTP" turning into UI-shaped events:
`risk.status_changed` is expressible, `risk-card-refresh` is not.

## Transport

**SSE.** Not WebSocket, and not both.

* The stream is one-directional. Every command in OpenRisk already travels as an
  HTTP request with its own authorization, validation and audit trail; a second
  command channel would need all three again.
* Resumption from a cursor is a protocol feature of SSE (`Last-Event-ID`), not
  something each application has to invent.
* The codebase already runs three SSE endpoints and no WebSocket at all, so this
  consolidates rather than adds.

A bidirectional transport would earn its place with collaborative editing,
presence, or frequent client→server messages. None exists today. When one does,
the envelope, the log, the hub and the authorization model are all transport-
independent; only the handler would change.

## Authentication

The route sits behind the ordinary session gate (`middleware.Protected`), so:

* a browser authenticates with its **HttpOnly session cookie** — `EventSource`
  with `withCredentials` sends it, and GET is a safe method so CSRF does not
  apply;
* a non-browser caller authenticates with a **bearer token**.

There is deliberately **no `?token=` fallback**, unlike the older
`/mitigations/events`. A credential in a URL lands in access logs, proxy logs,
browser history and any leaked `Referer`. Cookie sessions arrived in W0-03 and
made that fallback unnecessary.

The tenant comes from the resolved session (`middleware.GetContext`) and from
nowhere else. No header and no query parameter can name it.

## Tenant Isolation

Four layers, in this order:

1. **The session** decides the tenant. A client cannot supply one.
2. **The hub's routing table is keyed by tenant.** `Dispatch` looks up that
   tenant's bucket and never walks another. A subscription carries the tenant it
   was created with and cannot be moved. Isolation is a property of the data
   structure, not of a conditional somebody has to remember to write.
3. **The relay demands agreement.** A fanout message is dropped unless the
   channel it arrived on and the envelope's own tenant match; a mismatch is
   logged as a security event.
4. **The durable log fails closed.** `Replay` and `Bounds` return an error
   without a tenant rather than an unscoped result — the replay cursor is
   client-supplied, so it is exactly the input someone would use to try to reach
   another tenant's history.

Subscription filters are consulted only *after* all of this. A filter can narrow
what a client receives; it can never widen it.

## Event Envelope

```json
{
  "id": "b0a3…",                       // server-minted, the dedup key
  "envelopeVersion": 1,                // the outer contract's version
  "type": "risk.status_changed",       // <aggregate>.<action>, from the catalog
  "version": 1,                        // the PAYLOAD schema version for this type
  "occurredAt": "2026-08-24T12:44:29Z",
  "tenantId": "3d9986f1-…",
  "actorId": "f71f4c90-…",             // absent for system-originated events
  "aggregate": { "type": "risk", "id": "22b8dd96-…" },
  "sequence": 3,                       // per-tenant monotonic position
  "correlationId": "d5f47f85-…",       // shared with the audit entry
  "causationId": "e4e3b069-…",         // the audit entry that caused it
  "payload": { "changedFields": ["status"], "action": "transition" }
}
```

`envelopeVersion` and `version` are separate on purpose: the first says how to
find the fields, the second says what the payload for this type looks like. The
first is expected to change roughly never.

**Payload minimisation.** Events carry references and changed-field *names*, not
entities. `SanitizePayload` drops nested structures and any field whose name
looks like a secret (`password`, `token`, `mfa`, `secret`, `credential`,
`hash`, …), and the validator refuses an envelope that still carries one — so a
publisher cannot put a secret on the wire even by accident. A consumer that
needs the object reads it back from the API, which is also the only way it can
be sure it holds the current state rather than one that was already stale when
it was serialised.

Payloads are capped at 16 KiB: an unbounded payload is an unbounded write into
every connected client's buffer.

## Event Identity

`id` is a server-minted UUID, assigned by the publisher. A caller cannot supply
one — two different events sharing an id would collide in every consumer's
deduplication.

It is the deduplication key everywhere: in the hub's recent-id window, in the
handler's replay/live overlap suppression, and in the frontend client.

## Ordering

**Guaranteed:** a total order per tenant *in the durable log*. `sequence` is a
monotonic per-tenant counter assigned inside the append transaction, protected
by a per-tenant mutex, a `SELECT … FOR UPDATE` on Postgres, and a unique index on
`(tenant_id, sequence)` as the final backstop. A racing publisher fails loudly
rather than silently forking the order.

**Guaranteed:** replay is delivered oldest-first.

**Not guaranteed:** that live delivery across multiple API instances observes
that order exactly. Local delivery is ordered; the Redis fanout can interleave
between instances. Clients order by `sequence` and detect gaps, which is why
`sequence` is on the wire and why the SSE `id` is the same number.

**Not guaranteed:** any ordering *between* tenants. There is no global sequence,
and a client cannot infer another tenant's activity from gaps in its own.

## Replay / Reconnect

The SSE `id:` of every event is its sequence, so `Last-Event-ID` — which the
browser sends automatically — is the resume cursor. A non-browser client can
pass `?last_event_id=`.

```
connect → subscribe → replay from cursor → go live
```

The subscription is registered **before** the catch-up read. That ordering is
the whole trick: an event published during the replay is buffered rather than
lost in the gap between "read history" and "start listening". The overlap is
then suppressed by id.

Bounds and their answers:

| Situation | Answer |
| --- | --- |
| No cursor (first connection) | Nothing replayed. A fresh page loads from the API; replaying a day into it is work nobody asked for. |
| Cursor inside the window | Replayed, oldest first, in pages of 500, up to 10 pages. |
| Cursor at or beyond the head | Nothing to replay; stream goes live. |
| Cursor older than the retained window | `stream.resync` with `reason=cursor_expired` and the real `oldest_retained`. |
| More than 5000 events missed | `stream.resync` with `reason=replay_too_large`. Refetching beats a replay that takes longer than the refetch. |
| Replay read fails | `stream.resync` with `reason=replay_failed`. |

A partial replay is never served silently. That would leave a client believing
it is current, which is the one outcome the design does not allow.

## Duplicate Events

**At-least-once delivery with idempotent consumption.** Suppression happens at
three points, deliberately:

* **the hub**, on a bounded recent-id window — this absorbs the same event
  arriving from the local publisher and from the cross-instance fanout;
* **the handler**, over the replay/live overlap of one connection;
* **the client**, on its own id window — a client that trusts a distributed
  system to deliver exactly once has misunderstood the system it is in.

And the effect is bounded anyway: an event triggers a query invalidation, which
is idempotent by construction. Applying the same invalidation three times costs
two refetches and changes nothing.

## Delivery Semantics

```
At-least-once delivery
+ server-minted event ids
+ idempotent consumers
+ reconnect replay from a cursor
```

Not exactly-once. Not at-most-once. `GET /realtime/catalog` states
`"semantics": "at-least-once"` in the contract itself, because a consumer that
believes exactly-once stops writing the idempotency it actually needs.

## Backpressure

Delivery is non-blocking. Each subscriber has a bounded buffer (256 events);
when it is full:

1. the envelope is dropped **for that subscriber only**;
2. the drop is counted (`openrisk_realtime_events_dropped_total`, plus a
   per-subscription counter surfaced in the heartbeat);
3. a **coalescing** resync instruction is raised — repeated overflow leaves one
   pending instruction, not a queue of them.

Blocking instead would let one stalled browser tab stop every other tenant's
events, which is how a real-time feature takes a product down.

Other bounds, all enforced and all documented in `GET /realtime/catalog`:

| Bound | Value | Why it exists |
| --- | --- | --- |
| Subscriber buffer | 256 events | Sized for a burst, not for a client that stopped reading |
| Connections per instance | 2000 | An instance-wide ceiling |
| Connections per tenant | 100 | One tenant cannot consume the whole budget |
| Payload | 16 KiB | An oversized event would evict everyone else's buffered events |
| Filter entries | 64 | A filter is held for the life of a connection |
| Replay page | 500 rows | A day of history in one burst is worse than paging |
| Replay pages | 10 | Beyond that, refetching is faster than replaying |
| Connection lifetime | 2 h | A connection must not outlive a deploy or a rotated session indefinitely |

## Event Durability

`realtime_events` (migration `0059`): id, tenant, sequence, type, both versions,
aggregate type and id, actor, correlation, causation, payload, occurred-at and
created-at.

Retention is instance-wide, 24 h by default, overridable with
`REALTIME_REPLAY_RETENTION_HOURS` (0 disables the sweep). The
`RealtimeRetentionWorker` sweeps hourly and once at boot. Without it the replay
window would be "whatever the disk allows" — a promise nobody made and nobody
can keep.

Unlike the audit trail's retention this is not per tenant: the log is a replay
buffer, not evidence. The state its events describe lives in the tables they
were derived from.

## Outbox / Publishing Strategy

**This is an event log, not a transactional outbox, and the difference is
stated rather than glossed over.**

The row is appended *after* the business transaction commits. A crash in that
window loses the event: the database says the risk changed, the stream says
nothing.

Why that was accepted here:

* the stream is **never** the source of truth. A client that misses an event and
  later resyncs reads exactly the state the API would have given it;
* every client already has a resync path, exercised on cursor expiry, buffer
  overflow and gap detection;
* a true outbox means threading a transaction handle through every mutating use
  case in the codebase — a change far larger than this wave, and one that would
  touch code paths this wave otherwise leaves alone.

What closing it would take, if a future consumer needs it: give the repositories
a transaction-scoped variant, have the mutation bridge enlist in the caller's
transaction, and add a publisher worker that drains unpublished rows. The table
is already shaped for it — it only lacks a `published_at` column and the worker.

## Canonical Domain Events

Two publication paths, because changes come from two different places.

**The mutation bridge** rides the audit middleware's observation. Every
successful `POST/PUT/PATCH/DELETE` already yields the tenant, the actor, the
resource that actually changed (from the row layer, not guessed from the URL)
and the changed field names. An explicit map from (entity, action) onto the
catalog decides what is published; an unmapped mutation is silent.

Three subtleties it handles:

* **A trailing route verb beats the HTTP method.** `POST /risks/:id/transition`
  is a status change, and only the verb knows that — Risk is deliberately not
  audited at row level.
* **A sub-resource POST is ambiguous** and both readings are real:
  `POST /risks/:id/mitigations` creates a mitigation,
  `POST /mitigations/:id/sub-actions` updates one. What separates them is whose
  id was reported: if the aggregate's id is a path parameter it already existed.
* **A create can leave the trail without an id** — no path parameter, no audited
  row. The id is then read from the response, for creates only. This was found
  by running the system live, not by reading it: `risk.created` never fired
  until it was fixed.

**The domain relay** covers what never passes through an authenticated mutation:
a vulnerability arriving on a webhook with no session, a score recomputed by the
worker minutes later, a finding the scanner stopped seeing. It subscribes to the
existing internal channels and republishes through the canonical publisher. Those
channels and their existing consumers are untouched, and the relay reads only the
fields the catalog declares — forwarding whole internal payloads would make every
field added to an internal struct silently part of a public contract.

## Event Catalog

25 types, version 1, all tenant-scoped. `Status` is Implemented unless stated.

| Event | Aggregate | Ver | Trigger | Origin | Permission | Ordering | Retention |
| --- | --- | --: | --- | --- | --- | --- | --- |
| `risk.created` | risk | 1 | Risk created via API | mutation | `risks:read` | tenant seq | 24 h |
| `risk.updated` | risk | 1 | Risk updated via API | mutation | `risks:read` | tenant seq | 24 h |
| `risk.deleted` | risk | 1 | Risk deleted via API | mutation | `risks:read` | tenant seq | 24 h |
| `risk.status_changed` | risk | 1 | `POST /risks/:id/transition` | mutation | `risks:read` | tenant seq | 24 h |
| `risk.score_changed` | risk | 1 | Score worker recomputation | domain | `risks:read` | tenant seq | 24 h |
| `asset.created` | asset | 1 | Asset created via API | mutation | `assets:read` | tenant seq | 24 h |
| `asset.updated` | asset | 1 | Asset updated via API | mutation | `assets:read` | tenant seq | 24 h |
| `asset.deleted` | asset | 1 | Asset deleted via API | mutation | `assets:read` | tenant seq | 24 h |
| `asset.criticality_changed` | asset | 1 | Criticality change | domain | `assets:read` | tenant seq | 24 h |
| `vulnerability.detected` | vulnerability | 1 | Ingest records a new vulnerability | domain | `vulnerabilities:read` | tenant seq | 24 h |
| `vulnerability.updated` | vulnerability | 1 | Remediation status changed | mutation | `vulnerabilities:read` | tenant seq | 24 h |
| `vulnerability.deleted` | vulnerability | 1 | Deleted via API | mutation | `vulnerabilities:read` | tenant seq | 24 h |
| `incident.created` | incident | 1 | Incident declared | mutation | `incidents:read` | tenant seq | 24 h |
| `incident.updated` | incident | 1 | Incident updated (status, severity) | mutation | `incidents:read` | tenant seq | 24 h |
| `incident.deleted` | incident | 1 | Incident deleted | mutation | `incidents:read` | tenant seq | 24 h |
| `control.created` | control | 1 | Control created | mutation | `compliance:controls:read` | tenant seq | 24 h |
| `control.updated` | control | 1 | Control updated (incl. status) | mutation | `compliance:controls:read` | tenant seq | 24 h |
| `control.deleted` | control | 1 | Control deleted | mutation | `compliance:controls:read` | tenant seq | 24 h |
| `assessment.created` | compliance_audit | 1 | Compliance audit planned | mutation | `compliance:audits:read` | tenant seq | 24 h |
| `assessment.updated` | compliance_audit | 1 | Audit updated; completion is an update carrying `status` | mutation | `compliance:audits:read` | tenant seq | 24 h |
| `assessment.deleted` | compliance_audit | 1 | Audit deleted | mutation | `compliance:audits:read` | tenant seq | 24 h |
| `mitigation.created` | mitigation | 1 | Mitigation plan created | mutation | `mitigations:read` | tenant seq | 24 h |
| `mitigation.updated` | mitigation | 1 | Plan or sub-action updated | mutation | `mitigations:read` | tenant seq | 24 h |
| `mitigation.deleted` | mitigation | 1 | Plan deleted | mutation | `mitigations:read` | tenant seq | 24 h |
| `mitigation.auto_completed` | mitigation | 1 | Scanner auto-completes a sub-action | domain | `mitigations:read` | tenant seq | 24 h |

Notes on what is deliberately absent:

* **`assessment.*` is the compliance audit's.** OpenRisk has no aggregate named
  "assessment"; what the product calls one is `ComplianceAudit`, whose
  `internal` type the domain documents as a self-assessment. Inventing an
  aggregate with no mutations behind it would have been worse.
* **There is no `assessment.failed`.** No mutation produces that state; an audit
  is planned, in progress, completed or cancelled, and completion arrives as an
  update carrying `status` in `changedFields`.
* **There is no `vulnerability.created`.** Ingest publishes
  `vulnerability.detected`, which is what the domain already calls it.
* **Deferred:** notification, automation-execution, approval and governance
  events. Their mutations exist, so they can be added by extending the map — but
  they have no consumer today, and publishing events nobody reads is how a
  catalog fills up with fiction.

The live source of truth is `GET /realtime/catalog`, generated from the same
structure the publisher validates against, so it cannot describe an event that
cannot be published.

## Event Consumers

| Consumer | Status | Idempotent because |
| --- | --- | --- |
| Frontend query cache | Implemented | events invalidate queries; invalidation is idempotent |
| Mitigation board | Implemented | reads back from the API; the toast is per event id |
| Connection indicator | Implemented | renders state, holds none |
| Notifications | Deferred | the notification store is server-side already; wiring it to the stream is follow-up |
| Automation engine | Unchanged | still consumes the internal channels directly, deliberately: putting SOAR behind the realtime hub would add a hop to a path that already works and is separately audited |
| Analytics | Deferred | consumes on refetch today |

## Frontend Realtime Client

`src/lib/realtime.ts` — one connection per tab. Components subscribe; they never
connect.

States: `INITIALIZING`, `CONNECTED`, `RECONNECTING`, `DISCONNECTED`,
`RESYNCING`, `FORBIDDEN`, `ERROR`. There is deliberately no state meaning
"degraded but pretending"; a client that cannot hold a stream says so.

* **Backoff** 1/2/4/8/16/30 s with **full jitter**. Without jitter, every tab
  that lost the same deploy reconnects at the same instant — the storm is caused
  by the retry policy, not the outage.
* **Offline aware**: going offline does not burn backoff attempts; coming back
  online reconnects immediately.
* **Liveness watchdog** at 70 s (three missed 20 s heartbeats). This covers the
  case nothing else can: a suspended laptop or a middlebox that drops the socket
  without a FIN, where `EventSource` sits in a state that looks open forever.
* **Dedup** on a bounded id window; **gap detection** on sequence.
* Events **invalidate queries**; they are never written into the cache. The
  event says an aggregate changed, the API says what it changed to.

## Tenant Switching

`clearSessionScope()` — the one place a session boundary is enforced — tears the
stream down **first**, before the caches it feeds:

```
switchTenant()  → close the socket
                → drop the cursor and the dedup window
                → emit resync('tenant_changed')
                → reconnect under the new session
```

A stream left open across a switch would keep delivering the previous identity's
events into the new one's cache. And the cursor is a position in the previous
tenant's sequence — a number that means something different in the next tenant's
log, so replaying it would be asking for events by an index that does not
address them.

As a second line, the server names the tenant in its `stream.hello`; if it
differs from the one the previous connection served, the client drops its cursor
and resyncs.

## Observability

Metrics (all on `GET /metrics`, registered at import time):

*Counters* — events published (by type and origin), publish failures (by
reason), events delivered (by aggregate and live/replay), events dropped,
duplicates suppressed, subscriptions (by outcome), replays (by outcome), resyncs
(by reason).
*Gauges* — active connections, active tenants, buffered events.
*Histograms* — publish latency, delivery lag (occurredAt → socket), connection
duration.

**Cardinality**: not one series is labelled by tenant, user, connection or event
id. The label keys in use are `aggregate`, `event_type`, `mode`, `origin`,
`outcome` and `le` — all finite. The per-tenant question belongs to the logs,
which are not indexed by a time-series database.

**Structured logs** carry connection id, tenant, user, filter, cursor, drop
count and duration, and never a credential.

**Tracing**: `correlationId` (the request id) and `causationId` (the audit entry
id) travel on every mutation-derived event, so "why did this event fire?" is
answered by joining to the audit trail. Full OpenTelemetry span propagation into
the consumer is not implemented.

Eight alert rules in `deployment/monitoring/alerts.yml`, `openrisk_realtime`
group, built around the hub's actual failure mode: silence.

## Failure Handling

| Failure | Behaviour |
| --- | --- |
| Redis unavailable | Publication succeeds; the durable append and local delivery both work. Other instances are blind until it returns; counted as `publish_failures{reason="fanout"}`. |
| Database unavailable | Publication fails and delivers nothing — an event nobody could replay is worse than none. Counted as `reason="log"`. |
| Invalid event | Refused before persistence. Counted as `reason="invalid"`; a code defect, not an operational one. |
| Consumer crash (browser) | The connection ends; the client reconnects with its cursor. |
| Slow consumer | Bounded drop plus a resync instruction. |
| Relay receives a mismatched tenant | Dropped and logged as a security event. |
| Relay receives an unknown type | Dropped and logged. |
| Retention sweep fails | Logged and retried on the next tick; never fatal. |
| Retention exhausted for a cursor | `stream.resync` with `cursor_expired`. |

## Dead-Letter Strategy

There is **no dead-letter queue, and none is claimed.**

An event that cannot be dispatched to a subscriber is dropped for that
subscriber with a resync instruction; the event itself remains in the durable
log and is replayed on the next reconnect. An event that cannot be *published*
never enters the system — it fails the caller (log failure) or is refused
(invalid). So the poison-message problem a DLQ solves does not arise: there is
no queue in which one bad message can block the rest.

If a future server-side consumer processes events with real side effects — an
automation engine driven from the stream rather than from its own channel — it
will need one, and the log's `sequence` gives it a natural cursor to record.

## Security Model

| Control | Implementation |
| --- | --- |
| Authentication | Session cookie or bearer token; no credential in the URL |
| Stream authorization | `events:read` to hold a stream at all |
| Content authorization | Per aggregate, using the read permission that gates the same data over REST |
| Tenant isolation | Session-derived; hub routing keyed by tenant; relay demands channel/envelope agreement; log fails closed |
| Filter safety | Filters narrow only, and only within the permitted set |
| Payload safety | Nested structures dropped; secret-shaped names refused; 16 KiB cap |
| Client-side injection | Impossible — no route lets a client publish |
| Resource limits | Connections per instance and per tenant, buffer, filter entries, replay pages, connection lifetime |
| TLS | Deployment concern; cookies carry `Secure` in production (W0-03) |
| Origin | Same-origin by default; the CORS allowlist governs split-origin deployments |

**Known weakness, stated rather than buried:** authorization happens at connect.
A session revoked mid-stream keeps its stream until the connection ends (2 h
ceiling) or the client reconnects.

## API / Stream Contracts

### `GET /api/v1/realtime/events`

Auth: session cookie or `Authorization: Bearer`. Permission: `events:read`.

| Parameter | Where | Meaning |
| --- | --- | --- |
| `Last-Event-ID` | header | Resume cursor (the sequence of the last event received) |
| `last_event_id` | query | Same, for clients that cannot set the header |
| `types` | query | Comma-separated event types to narrow to |
| `aggregates` | query | Comma-separated aggregates to narrow to |

Responses: `200` (stream), `400` (unknown type or aggregate in the filter),
`401` (no session or no tenant), `403` (no readable event category),
`503` + `Retry-After` (instance at capacity).

Frames:

| Frame | Carries `id:` | Meaning |
| --- | --- | --- |
| `<event.type>` | yes (the sequence) | A domain event |
| `stream.hello` | no | Connection id, tenant, envelope version, effective filter, allowed aggregates |
| `stream.heartbeat` | no | Server time and this subscriber's drop count, every 20 s |
| `stream.resync` | no | `reason`, `detail`, and the relevant cursor bounds |
| `: keepalive` | — | Comment, so intermediaries do not time the connection out |

Control frames deliberately carry no `id:` — they are not positions in the
tenant's order, and letting one become a client's cursor would move it to a
number no event has.

### `GET /api/v1/realtime/catalog`

The machine-readable contract: envelope version, transport, endpoint, every
event descriptor with its aggregate, version, origin, trigger, payload fields
and required permission, plus the limits and the delivery semantics.

### `GET /api/v1/realtime/stats` (admin)

Open connections, distinct tenants, buffered events. No tenant's data.

## Test Strategy

| Layer | Coverage |
| --- | --- |
| `pkg/realtime` | Envelope validation (each required field, unknown type, aggregate mismatch, version skew, forbidden field, oversize), wire-shape stability, forward-compatible decoding, catalog well-formedness and domain coverage, filter parsing and intersection, per-aggregate authorization, `Restrict` never widening |
| `internal/application/realtime` | Hub: cross-tenant isolation, concurrent sessions, nil-tenant refusal, filtering, backpressure with exact drop counts, resync coalescing, id dedup, connection limits, idempotent teardown, concurrency under `-race`. Publisher: persist-before-deliver, log failure delivers nothing, fanout failure does not fail publication, per-tenant channels, identity fields overridden, secret stripping, catalog refusal. Relays: per-channel translation, tenant/channel mismatch, undeclared internal fields, malformed input. Bridge: every covered aggregate, transition verb, sub-resource disambiguation, audit correlation, no snapshot copying, unmapped silence, created-id fallback |
| `internal/infrastructure/repository` | Monotonic per-tenant sequence, independence between tenants, replay isolation, fail-closed without tenant, ordering, limit cap, bounds, purge moving the window without reusing sequences, 25 concurrent appends |
| `internal/handler` | 17 tests over a **real HTTP listener**: isolation, forged identifiers, 401/403/400/503, permission-scoped delivery, reconnect replay, cross-tenant replay, expired cursor, first connection, malformed cursor, duplicate delivery, SSE headers, envelope contract, catalog |
| `internal/infrastructure/workers` | Retention: boot sweep, schedule, survives failure, stops with context, does nothing unconfigured |
| Frontend | 20 tests: single connection, no credential in URL, handshake, dedup, gap detection, cursor resumption, jittered backoff and escalation, liveness watchdog, resync handling, recycle exemption, tenant switch, teardown, malformed frames, listener isolation |
| Live | See `W0-07_REALTIME_EVENT_HUB_LIVE_PROOF.md` |

## Performance

Publish latency and delivery lag are exported as histograms and were populated
during the live run, with publishes in the lowest buckets and delivery visibly
sub-second. No benchmark was run, so no throughput number is claimed.

## Load Testing

**Not performed.** No concurrency, throughput or percentile figure is claimed
for this wave. The bounds a load test would exercise are in place and
unit-tested (buffer 256, 2000 connections per instance, 100 per tenant, 500-row
replay pages, 10 pages, 16 KiB payloads).

A meaningful test would drive N tenants × M connections at X events/s with
Y reconnects/s and measure p50/p95/p99 delivery lag, drop rate, replay volume
and memory. It is the first item of follow-up work.

## Live Proof

`docs/W0-07_REALTIME_EVENT_HUB_LIVE_PROOF.md` — two real tenants, real
mutations, both streams observed, database and metrics inspected. Cross-tenant
isolation, forged-identifier rejection, reconnect replay, duplicate suppression,
ordering and audit correlation all proved live. A defect the live run found
(`risk.created` never firing) is recorded there and fixed.

## Known Limitations

1. Not a transactional outbox — the append follows the commit, so a crash in
   between loses the event.
2. At-least-once, not exactly-once. Ordering is total only within the log.
3. Authorization at connect: a session revoked mid-stream keeps its stream until
   it ends.
4. No load test, so no capacity claim.
5. No dead-letter queue (none needed today — see that section).
6. Notification, automation and analytics consumers are not wired to the stream
   yet; automation deliberately keeps its own channel.
7. Frontend proved by typecheck, build and unit tests, not browser driving.
8. The three legacy SSE endpoints still exist; nothing in the frontend uses the
   mitigation one any more.
9. No OpenTelemetry span propagation — correlation and causation ids only.
10. One pre-existing frontend test failure (`App.integration.test.tsx`),
    reproduced on a clean tree before this branch.

## Follow-up Work

1. **Load test** the hub and publish real numbers.
2. **Close the outbox gap** for events where losing one matters, by enlisting
   the append in the caller's transaction and adding a drain worker.
3. **Wire the notification centre** to the stream so a notification appears
   without a refetch.
4. **Retire the three legacy SSE endpoints** once nothing calls them.
5. **Re-authorize long-lived streams** periodically, closing limitation 3.
6. **Extend the catalog** to governance and automation events when a consumer
   exists for them.
