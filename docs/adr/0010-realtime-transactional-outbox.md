# 0010 — Realtime events: a transactional outbox with a per-tenant counter and a bounded relay

Status: accepted (agent decision under D-089, recorded as D-109 in `docs/DECISIONS.md`)
Proposed: 2026-11-12, on #344
Related: W0-07 (`docs/W0-07_REALTIME_EVENT_HUB.md`), #997 (notice e-mail outbox), D-089

## Context

`realtime_events` is appended after the business commit, by `Publisher.Publish` called from
`MutationBridge` on the response path. A crash between the commit and the append loses the event.
#344 makes the append part of the business transaction for one pilot use case
(`ImportAssetsUseCase`) and moves publication to a relay worker. Four forces decide the shape:

1. **The sequence is assigned in the caller's transaction.** The current append takes a per-tenant
   process mutex, then reads the head `FOR UPDATE`. Both fail inside a business transaction. The
   mutex would be held for the whole business transaction. `FOR UPDATE` on a tenant with no event
   yet locks nothing, so two instances both write sequence 1. The unique index then rolls back a
   business transaction that was valid.
2. **Delivery is ordered per tenant.** The client treats a jump in `sequence` as a gap and asks for
   a resync. If an event backs off and a later one goes out first, that is a phantom resync, and a
   stale event gets applied after a newer one.
3. **There is no dead-letter queue (out of scope).** A row that can never be delivered must not
   block its tenant forever. It must not be leased again forever either: devsecops raised this as
   MEDIUM on the notice outbox.
4. **Schema has two owners.** Compose and self-host boots never run `migrations/*.sql`
   (DB_HOST only). A backfill that exists only in SQL does not happen there. A backfill that is
   replayed late on a live database changes real rows (#757, #1001).

## Decision

1. **The sequence comes from a per-tenant counter row.** The new table `realtime_event_sequences`
   has `tenant_id` as its primary key and holds `last_sequence`. Every append, inline or in a
   transaction, reserves its sequences with one `INSERT … ON CONFLICT (tenant_id) DO UPDATE …
   RETURNING`. The value is `GREATEST(counter, MAX(sequence) of the tenant) + n`. The upsert locks
   the row until the caller commits, so sequences become visible in commit order. The first event of
   a tenant is ordered by the primary key's conflict handling, so there is no race. The `MAX`
   operand seeds the counter for tenants that already have events. It also lets the counter recover
   from a writer that bypassed it, for example an old instance during a rolling deploy. The process
   mutex stays only on the inline `Append`, around its own short transaction. It never covers a
   business transaction.
2. **The relay locks one tenant at a time through that same counter row.** It claims in two
   statements inside one short transaction:
   - It takes the tenants with pending rows, oldest pending first, with
     `FOR UPDATE OF s SKIP LOCKED` on their counter rows.
   - For those tenants only, it leases the longest prefix of pending rows, by `sequence`, that
     contains no row leased or backed off.

   While the relay holds a tenant's counter row, no transaction can hold a reserved but uncommitted
   sequence for that tenant. The pending rows it can see are therefore a complete prefix. If the
   tenant's lowest pending row is backed off, nothing later in that tenant goes out.
3. **Attempts are capped at 20.** Backoff is 1 s and doubles up to 5 min, about one hour in total.
   After the last attempt the row gets `abandoned_at` and its tenant moves on. The event stays in
   the log and can still be replayed. Clients learn about the hole through gap detection and
   resync. The same applies to a row whose lease expired after its last attempt, a process that
   crashed while publishing it.
4. **The backfill is metadata-only and gated on the column being absent.** `published_at` is added
   with `DEFAULT now()` and the default is then dropped. On PostgreSQL 11 and later this is a fast
   default: existing rows read the migration time and the table is not rewritten. New rows default
   to NULL. The SQL migration and `database.PrepareForAutoMigrate` run the same statements, and
   only when `published_at` does not exist, so a late replay changes nothing.
5. **Replay and Bounds serve every committed row, published or not.** A committed row is a fact.
   `published_at` is relay bookkeeping. Retention never purges a row that is still pending.

## Consequences

Easier:
- No event of an enlisted use case is lost on a crash. Delivery is at least once, the dedup key is
  `id`, and order is per tenant by `sequence`.
- Sequences no longer restart at 1 after retention has purged a tenant's whole log.
- Enlisting another use case means giving it the transaction-scoped `RealtimeEventOutbox` port, and
  nothing else.

Harder:
- A business transaction that enlists events holds its tenant's counter row until it commits. Other
  appends for that tenant wait, so enlisting must be the last write in the transaction.
- While Redis is down, enlisted events of a tenant queue behind the head row, for local clients
  too, for up to about an hour. Events not enlisted keep today's behaviour: they are delivered
  locally and Redis is best effort.
- An inline event published while enlisted rows are pending can overtake them. Clients see a gap
  and resync. This holds until every mutation is enlisted (the follow-up issue).
- Claiming uses SQL specific to Postgres, so the relay's correctness tests are `*_pg_test.go`.

## Alternatives rejected

- **`pg_advisory_xact_lock(hashtext(tenant))` then `MAX(sequence)+1`.** It works only on
  Postgres, so the sqlite path would keep the in-process mutex. Hash collisions couple unrelated
  tenants. The advisory key space is shared with other code. The counter still resets after a full
  purge.
- **The current `SELECT … FOR UPDATE` on the head, inside the caller's transaction.** It locks
  nothing for a tenant's first event (force 1).
- **A Postgres SEQUENCE per tenant.** DDL per tenant, and it is not transactional: a rollback
  leaves a gap, which the client reads as a lost event.
- **Retrying forever with no cap.** A single row that can never be delivered would stop its
  tenant's stream for good.
- **Marking an undeliverable row as published.** That is a false record. `abandoned_at` says what
  happened.
- **`UPDATE realtime_events SET published_at = created_at` as the backfill.** It rewrites every row
  in the retention window and holds row locks on a hot table. Replayed late on a live database, it
  would mark pending rows as published and lose them.
- **Queueing inline events behind pending outbox rows.** That would give strict order across both
  paths. But a Redis outage would then stall local delivery of every event of the tenant, which
  breaks W0-07's rule that a Redis outage costs liveness, not correctness. It is deferred to the
  follow-up issue that enlists everything.
