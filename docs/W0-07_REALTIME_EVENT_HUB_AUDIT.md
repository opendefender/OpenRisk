# W0-07 — Audit: existing event and realtime infrastructure

Snapshot taken before any code was written, so the wave consolidates what exists
instead of building a parallel stack next to it.

Base commit: see `git log` for `audit(w0-07)`.

## 1. What already exists

### 1.1 Event vocabulary — `backend/pkg/events`

A single 86-line file holds four channel-name constants and their payload
structs:

| Constant | Channel | Payload | Published by | Consumed by |
| --- | --- | --- | --- | --- |
| `RiskUpdated` | `risk.updated` | `RiskUpdatedEvent` | `risk_handler.go` (create/update), `score_worker.go` (asset fan-out) | `ScoreWorker` |
| `RiskScoreUpdated` | `risk.score_updated` | `RiskScoreUpdatedEvent` | `score_worker.go` | **nobody** — the comment claims "SSE hub (Module 7), Notification service, dashboard cache invalidation"; none of the three subscribe |
| `AssetCriticalityChanged` | `asset.criticality_changed` | `AssetCriticalityChangedEvent` | `asset_handler.go` | `ScoreWorker` |
| `VulnerabilityDetected` | `vulnerability.detected` | `VulnerabilityDetectedEvent` | `automation/vuln_event_publisher.go` | `AutomationWorker` |

Findings:

* The names are already `<aggregate>.<action>` — the convention W0-07 needs
  exists, it is simply not applied outside these four.
* There is **no envelope**. Each payload is a bare struct: no event id, no
  version, no sequence, no correlation id, no occurred-at. Two of the four carry
  their own ad-hoc timestamp string.
* `risk.score_updated` is published on every score recomputation and read by
  nothing. It is the clearest evidence that the "SSE hub" referenced in the
  comments was never built.

### 1.2 Ad-hoc channel names outside `pkg/events`

Five more channels exist as string literals, defeating the "never hardcode a
channel name" rule the package documents:

* `mitigation.progress_changed`, `mitigation.completed`, `mitigation.auto_completed`,
  `mitigation.reverted` — `internal/infrastructure/redis/mitigation_events.go`
  and `internal/infrastructure/scanmitigation/detector.go`.
* `scanpkg.SSEChannel(tenantID)` / `scanpkg.AgentJobChannel(tenantID)` —
  `internal/scanner/notify.go`, `application/scanner/*`. These are the only
  **per-tenant** channels in the codebase, and the pattern W0-07 generalises.
* The report progress channel — `handler.ReportProgressChannel`.

### 1.3 Transport — three hand-rolled SSE endpoints, zero WebSocket

| Endpoint | Auth | Tenant scoping | Replay | Heartbeat |
| --- | --- | --- | --- | --- |
| `GET /api/v1/mitigations/events` | `?token=` JWT, validated in the handler, mounted **before** the auth gate | filters each message on `evt.TenantID != tenantID` after subscribing to a **global** channel | none | 20 s comment |
| `GET /api/v1/scanner/events` | session (mounted on `protected`) | subscribes to a **per-tenant** channel | none (a `preamble` replays queued *agent jobs*, not events) | 20 s comment |
| `GET /api/v1/reports/:reportId/progress` | session | global channel, filtered per message on tenant **and** report id | replays one initial snapshot | 20 s comment, 10 min ceiling |

Every one of them re-implements: header setup, `SetBodyStreamWriter`, subscribe,
keepalive ticker, flush-error teardown. There is no shared stream primitive.

`grep` for WebSocket in `backend/` returns **nothing**: the server has no
WebSocket route, no upgrade handler, and no dependency on a WebSocket library.

### 1.4 Backpressure — absent

All three stream loops write straight into the `bufio.Writer` and rely on
`Flush()` returning an error to notice a dead client. There is no bounded queue,
no drop policy, no slow-consumer detection, and no connection cap. A client that
stops reading holds a goroutine and a Redis subscription open until the TCP
buffer fills.

### 1.5 Durability — none

Redis pub/sub is fire-and-forget. No outbox table, no event log table, no
`events`-shaped migration in `migrations/` (latest is `0058`). An event published
while a client is reconnecting is gone. The scanner works around this with its
`preamble` replay of *queued jobs* read back from Postgres — the only
reconnect-safety in the codebase, and it is job-specific, not event-generic.

### 1.6 Frontend — three real consumers, two dead clients

Real:

* `features/mitigations/useMitigationEvents.ts` — `EventSource` on
  `/mitigations/events?token=`, invalidates the mitigation queries, toasts.
* `features/infrastructure/useScanner.ts` — polls (comment states EventSource
  cannot send a bearer header).
* `hooks/useSSE.ts` — a generic SSE hook.

Dead (endpoints that do not exist on the server):

* `hooks/useWebSocket.ts` (335 lines) → `wss://<host>/api/v1/ws/dashboard`.
* `hooks/useNotificationWebSocket.ts` (163 lines) → `ws://localhost:8080/ws/notifications`,
  re-exported from `hooks/index.ts`. It also reads `process.env.REACT_APP_WS_URL`
  — a Create-React-App variable in a Vite project, so it is `undefined` at
  runtime.
* `pages/RealTimeAnalyticsDashboard.tsx` consumes `useDashboardWithWebSocket`
  and is not routed anywhere.

Each component that wants live data opens its own connection; there is no shared
transport, no connection state machine, no dedup, no resync.

### 1.7 What is worth reusing

* **Per-tenant monotonic sequence, assigned inside a transaction** — the audit
  trail already does exactly this (`GormAuditChainRepository.Append`: per-tenant
  mutex, `SELECT … FOR UPDATE` on Postgres, unique index on
  `(tenant_id, sequence)` as the final backstop). The realtime log copies this
  pattern rather than inventing another.
* **Exhaustive mutation observation** — `middleware.AuditMutations` already sees
  every successful `POST/PUT/PATCH/DELETE` with actor, tenant, resource type,
  resource id, action, before/after and request id. Canonical domain events can
  be derived from the same observation instead of adding a publish call to
  several hundred use cases.
* **HttpOnly cookie sessions** (`middleware/session_cookie.go`, W0-03) — a
  browser `EventSource` with `withCredentials` authenticates without a token in
  the URL. The existing `?token=` streams predate cookie sessions.
* **Per-tenant Redis channels** — `scanpkg.SSEChannel(tenantID)` proves the
  pattern works in this stack.
* **`promauto` on the default registry** — `pkg/monitoring/activation.go`
  documents why package-level registration beats a collector nobody constructs;
  `GET /metrics` already serves the Prometheus exposition format.

## 2. Gaps W0-07 has to close

1. No event envelope: no id, version, sequence, correlation or causation.
2. No durability, therefore no replay and no honest reconnect story.
3. No dedup: a redelivered payload is processed again.
4. No backpressure anywhere in the three stream loops.
5. Tenant isolation is by post-hoc payload filtering on two of three streams —
   correct today, but one forgotten `if` away from a cross-tenant leak.
6. No stream authorization: any authenticated session may open the scanner and
   report streams; the mitigation stream only needs a valid token.
7. No observability: not one metric, and no structured log line, covers realtime.
8. No shared frontend client, no connection state machine, no resync.
9. Two dead WebSocket clients pointing at endpoints that do not exist.

## 3. Decision taken from this audit

Consolidate on **SSE**, extend the existing `pkg/events` vocabulary into a
versioned envelope, give it a durable per-tenant ordered log modelled on the
audit chain, fan out through per-tenant Redis channels, and derive canonical
domain events from the mutation observation the audit middleware already makes.

Rationale and the full design are in `docs/W0-07_REALTIME_EVENT_HUB.md`.
