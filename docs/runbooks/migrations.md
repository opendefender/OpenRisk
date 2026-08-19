# Runbook — Database migrations & recovery

OpenRisk builds its schema in two layers:

1. **GORM AutoMigrate** (the declared schema authority) runs on every boot and creates/updates all tables and columns from the Go models.
2. **The additive SQL layer** (`migrations/NNNN_*.up.sql`, run by golang-migrate at boot) carries only what AutoMigrate cannot express: triggers, CHECK constraints, and one-off **data** backfills.

The boot **fails loudly** if the SQL layer cannot be applied — the app must never serve on a half-applied schema.

## Commands

```bash
make migrate            # apply all pending migrations
make migrate-status     # print current version + dirty flag
make migrate-rollback   # roll back exactly one migration
make migrate-force VERSION=<n>   # recovery: set version, clear the dirty flag
```

These are backed by a real `migrate` subcommand on the server binary:

```bash
cd backend && DATABASE_URL=postgres://user:pass@host:port/db?sslmode=disable \
  go run ./cmd/server migrate <up|down|force <version>|version>
```

## Recovering a DIRTY database

golang-migrate marks the database **dirty** when a migration fails part-way. On the next boot you will see an actionable error naming the version, e.g.:

```
migrations: database is DIRTY at version 40 — a previous migration failed mid-apply ...
```

Recover:

1. **Diagnose.** Open the failing migration (`migrations/00NN_*.up.sql`) and inspect the database. Decide whether it was partly applied.
2. **Reconcile the schema by hand** if needed (finish or undo what the failed migration left behind). Because AutoMigrate also runs on boot, the tables/columns usually already exist — the dirty flag is the only thing blocking startup.
3. **Force the last known-good version** (the one *before* the failed one):
   ```bash
   make migrate-force VERSION=<good_version>
   ```
4. **Re-run** and boot:
   ```bash
   make migrate
   ```

`force` only sets the recorded version and clears the dirty flag — it does **not** run any SQL. Point it at a version whose schema you have verified.

## Notes

- A **fresh** database boots cleanly with no manual steps: AutoMigrate builds the schema and the SQL layer applies from the baseline to head.
- Historical migrations that predate the current baseline live in `migrations/_archive/` and are not applied by the live runner.
