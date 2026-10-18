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
