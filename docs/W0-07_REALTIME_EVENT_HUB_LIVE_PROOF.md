# W0-07 — Real-Time Event Hub Live Proof

Everything below was produced by running the code against a real Postgres, a
real Redis and two real tenants. Where something was **not** exercised live, it
says so and says what covers it instead.

## Environment

| | |
| --- | --- |
| Backend | Go 1.25.12, binary built from this branch |
| API | `http://127.0.0.1:8107` (`PORT=8107`) |
| Postgres | 16, `openrisk_db` container, `localhost:5434` |
| Redis | `openrisk_redis` container, `localhost:6379` |
| Migrations | `migrations: applied successfully (version=59 dirty=false)` |
| Replay window | 24 h (default `DefaultReplayRetention`) |
| Date | 2026-08-24 |

Boot log, realtime lines:

```
Realtime: event hub started (durable log + per-tenant fanout + domain relay, replay window 24h0m0s)
{"level":"info","retention":86400000,"interval":3600000,"message":"realtime retention worker started"}
Realtime: domain relay subscribed to [risk.score_updated vulnerability.detected asset.criticality_changed mitigation.auto_completed]
Realtime: relay subscribed to openrisk:realtime:events:*
Realtime: GET /realtime/events mounted (SSE, session-authenticated, tenant-scoped)
⚡ OpenRisk API listening on port 8107
```

## Commit SHA

`2525ba7583254b1ced3463e08b25099744a884ce` — branch `feat/w0-07-realtime-event-hub`.

## Transport

**Server-Sent Events.** `GET /api/v1/realtime/events`. The decision and its
alternatives are in `docs/W0-07_REALTIME_EVENT_HUB.md` § Architectural Decision.

Response headers observed live:

```
HTTP 200
Content-Type: text/event-stream
Cache-Control: no-cache
X-Accel-Buffering: no
```

## Authentication

The ordinary session gate. The browser sends its HttpOnly cookie, other callers
a bearer token. **There is no `?token=` fallback** — a credential in a URL lands
in access logs, proxy logs and browser history.

Both proof tenants were registered through `POST /auth/register` and signed in
through `POST /auth/login`, completing the mandatory MFA enrolment
(`/auth/mfa/setup` → `/auth/mfa/verify`) exactly as a real operator would.

## Test Tenants

| | Tenant A | Tenant B |
| --- | --- | --- |
| Organization | `W0-07 Tenant A <ts>` | `W0-07 Tenant B <ts>` |
| Tenant id (final run) | `3d9986f1-aef6-45b3-a5ab-210e269a619d` | `292e0aca-8f3b-4b00-afed-b15b3af42b15` |
| Role | org admin (`permissions: ["*"]`) | org admin (`permissions: ["*"]`) |

Created fresh by the driver on every run, so the proof is repeatable and both
accounts start from the same state.

## Test Users

`w007-a-<ts>@example.com` and `w007-b-<ts>@example.com`. Passwords are supplied
by the driver at runtime and appear nowhere in this repository.

## Event Catalog

`GET /realtime/catalog` → **HTTP 200, 25 event types**, `transport: sse`,
`delivery.semantics: at-least-once`, ordering documented in the response.

Every descriptor carries the permission required to receive it — asserted by the
contract test and re-checked live.

## Event Publication

Real mutations through the real domain flow, not a script poking the publisher:

```
MUTATION A create risk   :: HTTP 201 id=22b8dd96-6ef6-4337-9c6d-a41ca5226c12
MUTATION B create asset  :: HTTP 201 id=b214a5e2-bb8a-4a00-8bac-a8d0191df977
```

Delivered on the streams that were already open:

```
STREAM A received :: [{"type":"risk.created","seq":"1","aggregate":{"type":"risk","id":"22b8dd96-…"}},
                      {"type":"risk.score_changed","seq":"2","aggregate":{"type":"risk","id":"22b8dd96-…"}}]
STREAM B received :: [{"type":"asset.created","seq":"1","aggregate":{"type":"asset","id":"b214a5e2-…"}}]
```

Both publication paths are visible in that output: `risk.created` and
`asset.created` come from the **mutation bridge**, `risk.score_changed` from the
**domain relay** picking up the score worker's internal event. Confirmed by the
metric labels:

```
openrisk_realtime_events_published_total{event_type="risk.created",origin="mutation"}       6
openrisk_realtime_events_published_total{event_type="asset.created",origin="mutation"}      4
openrisk_realtime_events_published_total{event_type="risk.score_changed",origin="domain"}   6
```

### A defect this found

The first live run produced `risk.score_changed` and **no `risk.created`**.
`POST /risks` has no path parameter and Risk is deliberately excluded from the
audit plugin, so the audit entry carried no entity id and the bridge skipped the
event for naming no subject. Fixed in `2525ba7` by reading the created id from
the response, for creates only. Re-proved above. The unit tests had passed:
every other covered aggregate happens to have a path parameter or an audited
row.

## Tenant Isolation

The mandatory test, over HTTP, with both streams open at once:

```
ISOLATION A :: tenants seen on A's stream = {3d9986f1-…} — only A: True
ISOLATION B :: tenants seen on B's stream = {292e0aca-…} — only B: True
ISOLATION A misses B's asset :: True
ISOLATION B misses A's risk  :: True
```

Forged tenant identifiers — `X-Organization-ID`, `X-Tenant-ID`, `?tenant_id=`
and `?organization_id=` all naming tenant B, on tenant A's session:

```
SECURITY forged tenant :: server bound the stream to 3d9986f1-… (= A: True)
SECURITY forged tenant delivery :: B created asset e008140a-…; forged stream received 0 events
```

At the database, each tenant keeps its own sequence and no sequence is ever
issued twice:

```
        tenant_id               | events | oldest | newest
--------------------------------+--------+--------+--------
 01dde5c7-8315-…                |      6 |      1 |      6
 292e0aca-8f3b-…                |      2 |      1 |      2
 3d9986f1-aef6-…                |      6 |      1 |      6
 …

select tenant_id, sequence, count(*) … having count(*) > 1;   →  (0 rows)
```

## Reconnect Behavior

The mandatory test — connect, receive, disconnect, produce, reconnect with the
cursor:

```
RECONNECT                          :: stream A disconnected at cursor 2
RECONNECT missed event             :: HTTP 201 risk 79185f7e-… created while A was disconnected
RECONNECT replay                   :: [{"type":"risk.created","seq":"3","aggregate_id":"79185f7e-…"},
                                        {"type":"risk.score_changed","seq":"4","aggregate_id":"79185f7e-…"}]
RECONNECT missed event replayed         :: True
RECONNECT already-seen events not replayed :: True
```

Cursor edge cases:

```
REPLAY cursor 0 (first connection)  :: 0 events — by design, a first connection replays nothing
REPLAY cursor beyond the head       :: 0 events, 0 resync frames (connection healthy)
REPLAY malformed cursor             :: HTTP 200, stream opened (a bad cursor must not lock a client out)
```

## Replay

Served live, in order, from the durable log, using the SSE `id:` the client sent
back as `Last-Event-ID`. The metric distinguishes the two delivery modes:

```
openrisk_realtime_events_delivered_total{aggregate="risk",mode="live"}    6
openrisk_realtime_events_delivered_total{aggregate="risk",mode="replay"}  6
openrisk_realtime_events_delivered_total{aggregate="asset",mode="live"}   6
```

**Expired-cursor resync was not reproduced live** — it needs a cursor older than
the 24 h retention window, which this session could not age. It is covered by
`TestRealtimeStream_ExpiredCursorAsksForAResync`, which deletes the head of the
log and asserts the client receives `stream.resync` with
`reason=cursor_expired` and the real `oldest_retained`.

## Duplicate Event Handling

```
DUPLICATES :: 2 events delivered, 2 distinct ids — no repeat: True
```

The re-presentation case (the same envelope offered three times, as a
cross-instance echo would be) is covered by
`TestRealtimeStream_ADuplicatedEventIsDeliveredOnce` and
`TestHub_DuplicateDispatchIsSuppressedByEventID`. The live run exercises the
overlap that arises naturally between replay and live delivery, which is the
duplicate a real client actually meets.

## Ordering

```
ORDERING :: sequences delivered [3, 4] — strictly increasing: True
```

Per tenant, in the durable log, the order is total: the unique index on
`(tenant_id, sequence)` is the guarantee, and the query above shows no duplicate
sequence for any tenant.

## Backpressure

**Not reproduced live** — it needs a client that stops reading while events keep
arriving, which this driver has no way to stage against a real server without
fabricating the condition.

Covered by `TestHub_SlowSubscriberIsBoundedDroppedAndToldToResync`: with a
4-slot buffer and 20 events, the stalled subscriber's buffer never exceeds 4, it
records exactly 16 drops, it receives a resync instruction, the healthy
subscriber beside it receives all 20 in order, and dispatch never blocks.
`TestHub_ResyncSignalCoalesces` asserts 50 overflows leave one pending
instruction.

Connection limits were exercised live in the handler suite
(`TestRealtimeStream_CapacityIsRefusedNotQueued` → HTTP 503).

## Observability

18 realtime series on `GET /metrics`:

```
openrisk_realtime_active_connections            openrisk_realtime_events_dropped_total
openrisk_realtime_active_tenants                openrisk_realtime_events_published_total
openrisk_realtime_buffered_events               openrisk_realtime_publish_latency_seconds_{bucket,count,sum}
openrisk_realtime_connection_duration_seconds_{bucket,count,sum}
openrisk_realtime_delivery_lag_seconds_{bucket,count,sum}
openrisk_realtime_duplicates_suppressed_total   openrisk_realtime_replays_total
openrisk_realtime_events_delivered_total        openrisk_realtime_subscriptions_total
```

Cardinality, checked on the label keys actually emitted:

```
METRICS cardinality :: label keys in use: ['aggregate','event_type','le','mode','origin','outcome']
                       — unbounded keys: none
```

Structured logs carry the connection id, tenant, user, filter and cursor, and
never a credential:

```
realtime: stream open conn=3f0a0896-… tenant=01dde5c7-… user=f71f4c90-… filter=aggregates=… cursor=0
realtime: stream closed conn=0defbd1b-… tenant=01dde5c7-… dropped=0 duration=20s

grep -ciE "realtime.*(bearer|password|secret|token=|or_access|jwt)" server.log  →  0
```

Eight Prometheus alert rules are in `deployment/monitoring/alerts.yml`
(`openrisk_realtime` group). They are **written, not fired**: no Prometheus was
run against this instance.

## Frontend Consumption

**Proved by typecheck, build and unit tests, not by browser driving.**
`tsc -b --force` clean, `vite build` clean, 20 new tests in
`src/lib/__tests__/realtime.test.ts` covering single-connection behaviour, no
credential in the URL, dedup, gap detection, cursor resumption, jittered
backoff, the liveness watchdog, resync handling and tenant-switch teardown.

Interactive browser driving remains blocked in this sandbox, as recorded for
previous waves.

## Security Validation

| Check | Result | Evidence |
| --- | --- | --- |
| Unauthenticated stream | Refused | live, HTTP 401 |
| Forged tenant (2 headers + 3 query params) | Ignored | live, stream bound to session tenant, 0 foreign events |
| Cross-tenant delivery | None | live, both directions |
| Cross-tenant replay | None | `TestRealtimeStream_ReplayCannotReachAnotherTenantsHistory`; repository fails closed without a tenant |
| No readable category | Refused 403 | `TestRealtimeStream_RefusesWhenNoCategoryIsReadable` |
| Per-aggregate authorization | Enforced | `TestRealtimeStream_DeliversOnlyTheCategoriesTheCallerMayRead` |
| Unknown subscription token | Refused 400 | live |
| Event injection from a client | Impossible | the stream is receive-only; publication has no client-facing route |
| Secrets in payloads | None stored | live SQL: `payload::text ~* '(password\|secret\|token\|mfa\|credential\|api_key)'` → 0 rows |
| Credentials in logs | None | live grep → 0 |
| Payload size limit | 16 KiB | `TestValidate_RefusesAnOversizedPayload` |
| Subscription filter limit | 64 entries | `TestParseFilter_RefusesAnAbusivelyLongList` |
| Connection limits | Enforced | live 503 in the handler suite |

**Not exercised live**: expired session mid-stream and revoked session
mid-stream. The stream is authorised once at connect, so a session revoked
afterwards keeps its stream until the connection ends (max 2 h) or the client
reconnects. Stated as a limitation below rather than claimed as covered.

## Accessibility

The connection state is exposed in one polite live region
(`role="status" aria-live="polite"`) whose text changes only when the state
changes. Individual events are deliberately **not** announced: narrating every
risk update in a busy tenant would make the page unusable. The header dot
carries `data-state` for tests and a `title` for sighted users. Reviewed in
code; no assistive-technology session was run.

## Performance

Measured incidentally, not benchmarked. `openrisk_realtime_publish_latency_seconds`
and `openrisk_realtime_delivery_lag_seconds` are both exported and populated;
during the run every publish landed in the lowest histogram buckets and
end-to-end delivery was visibly sub-second (streams received each mutation
within the 4 s the driver waits before asserting).

## Load Test

**Not run.** No load-testing pass was performed for this wave, so no throughput,
concurrency or percentile figure is claimed. The bounds that would govern one
are in place and unit-tested: 256-event buffer per subscriber, 2000 connections
per instance, 100 per tenant, 500-row replay pages, 10 pages maximum, 16 KiB
payloads.

## Commands Executed

```bash
# Toolchain
go build ./... && go vet ./...
go test ./...                                   # full backend suite
cd frontend && npx tsc -b --force && npx vite build && npx vitest run

# Live stack
DB_HOST=localhost DB_PORT=5434 PORT=8107 DATABASE_URL=…?sslmode=disable ./openrisk-w007

# Live proof driver (registers two tenants, enrols MFA, opens both streams,
# performs real mutations, asserts what each stream received)
python3 liveproof.py <timestamp>

# Database verification
psql -c "select tenant_id, count(*), min(sequence), max(sequence) from realtime_events group by tenant_id;"
psql -c "select tenant_id, sequence, count(*) from realtime_events group by tenant_id, sequence having count(*)>1;"
psql -c "select count(*) from realtime_events where payload::text ~* '(password|secret|token|mfa|credential|api_key)';"
psql -c "select r.type, r.correlation_id, r.causation_id, a.action, a.entity_type
           from realtime_events r join audit_events a on a.id::text = r.causation_id;"

# Metrics
curl -s http://127.0.0.1:8107/metrics | grep '^openrisk_realtime_'
```

## Results

| Test suite | Result |
| --- | --- |
| `go build ./...` | PASS |
| `go vet ./...` | PASS |
| `go test ./...` (full backend) | PASS — 0 failures |
| `pkg/realtime` | PASS — envelope, catalog, filter, authorization |
| `internal/application/realtime` | PASS — hub, publisher, relays, bridge (incl. `-race`) |
| `internal/infrastructure/repository` | PASS — sequencing, isolation, replay, purge, concurrency |
| `internal/handler` (17 stream tests) | PASS — over a real HTTP listener |
| `internal/infrastructure/workers` | PASS — retention |
| `tsc -b --force` | PASS |
| `vite build` | PASS |
| `vitest run` | 203 passed, **1 pre-existing failure** (`App.integration.test.tsx`, reproduced on a clean tree before this branch's changes) |
| Live cross-tenant isolation | PASS |
| Live reconnect + replay | PASS |
| Live duplicate suppression | PASS |
| Live ordering | PASS |
| Live audit correlation | PASS |
| Load test | NOT RUN |
| Browser driving | NOT AVAILABLE (sandbox) |

## Screenshots / Evidence

No screenshots: interactive browser driving is blocked in this sandbox. The
evidence is the transcript above — HTTP statuses, SSE frames as received, SQL
results and Prometheus samples, all reproducible by re-running the driver.

Audit correlation, joined live:

```
     type      |     corr     |    cause     | action | entity_type
---------------+--------------+--------------+--------+-------------
 risk.created  | d5f47f85-6ba | e4e3b069-8f0 | create | risk
 asset.created | d8e5586f-1e8 | 7f5b61ae-a14 | create | asset
```

Every realtime event's `causation_id` resolves to a real audit entry for the
same action, and both carry the same request id.

## Known Limitations

1. **Not a transactional outbox.** The event row is appended after the business
   transaction commits, not inside it. A crash in that window loses the event.
   Survivable because the stream is never the source of truth — a client that
   misses an event and later resyncs reads the same state the API would have
   given it — but it is a real gap, not a rounding error.
2. **Delivery is at-least-once, ordering is total only in the log.** Live
   delivery across instances is best-effort ordered; clients order by sequence
   and detect gaps. Exactly-once is not claimed anywhere.
3. **Expired-cursor resync was not aged live.** Covered by an integration test
   that deletes the head of the log.
4. **Backpressure was not staged live.** Covered by unit tests with exact drop
   counts.
5. **A session revoked mid-stream keeps its stream** until it ends (2 h ceiling)
   or the client reconnects. Authorization happens at connect.
6. **No load test was run**, so no capacity figure is claimed.
7. **Frontend proved by build and unit tests**, not by browser driving.
8. **Alert rules are written, not fired.** No Prometheus ran against this
   instance.
9. **The three legacy SSE endpoints remain** (`/mitigations/events`,
   `/scanner/events`, `/reports/:id/progress`). Nothing in the frontend uses the
   mitigation one any more; retiring them is follow-up work.
10. **`assessment.*` events are the compliance audit's**, because that is what
    the domain calls a self-assessment. There is no separate assessment
    aggregate to publish for.
