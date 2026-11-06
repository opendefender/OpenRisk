---
name: defect-pattern-sql-backfill-not-run
description: Data backfills placed only in migrations/*.sql never run on compose/self-host boots (DB_HOST only); they belong in prepare.go — found on #757
metadata:
  type: project
---

Boot order in `cmd/server/main.go` (~208-222): `database.PrepareForAutoMigrate`, then
`AutoMigrate(schemaModels()...)`, then `migrations.RunMigrations()`. That last step
is a NO-OP unless `DATABASE_URL` is set. The compose files configure `DB_HOST` only,
so on a compose/self-host deploy `migrations/*.sql` never runs. A new column arrives
through AutoMigrate, and a backfill written only in the SQL file never happens.

Seen on #757 (2026-10-15): migration 0076 backfilled `reminder_overdue_sent_at` to
stop a mass "deadline passed" e-mail at upgrade. On DB_HOST deploys the due worker's
boot sweep would have mailed every late plan. Fixed by
`prepareMitigationOverdueReminder` in prepare.go: postgres only, runs only while the
column is absent, done in a transaction.

**Why:** e-mails and other side effects cannot be undone. The SQL file reads as
correct in review, and it passes tests that run the migration chain explicitly.

**How to apply:** for any migration that pairs a new column with a data backfill, ask
"what does a DB_HOST-only boot do?". Data repairs that must happen before AutoMigrate
or before a worker starts belong in `internal/infrastructure/database/prepare.go`.
They must be idempotent and gated on the column being absent, and the SQL file
should mirror them. Related: [[defect-pattern-tenant-isolation]].

**The other side, found on #1001 / PR #1018 (2026-10-18):** turning the SQL layer on
for compose (setting DATABASE_URL) replays the active chain, 0048–0076 (0001–0047
are in `_archive/` and never run), once, on a live AutoMigrate-built DB. The
unconditional statements written for "the day it shipped" then hit months of real
rows. 0056 `DELETE FROM mfa_backup_codes` wipes every post-fix (CSPRNG) code. 0076
stamps currently-overdue plans, which suppresses their reminder. 0049 writes a
false "disabled before reasons were recorded" text. A live replay
(boot → seed → boot → boot with DATABASE_URL, table md5 diff) proved this in
minutes, and it is the check to repeat. Every unique index in the chain already
exists on an AutoMigrate DB after two boots, so the risk is silent data change,
not a dirty boot.
**How to apply:** a backfill/DELETE in migrations/ must be gated on "this file just
created the column", or bounded by a date, so a late replay does nothing. Docker
VM disk is full here: run throwaway pg with `--tmpfs /var/lib/postgresql/data`.
