# Archived migrations — NOT executed

These files are kept for history and review only. **golang-migrate never reads them**: its file
source is non-recursive, so anything under `migrations/_archive/` is invisible to the runner (the
active, executed set is the well-formed `*.up.sql`/`*.down.sql` files directly in `migrations/`).

See `docs/MIGRATIONS.md` for the full story. Short version: **AutoMigrate (GORM) is the schema
authority**; the SQL layer is additive. These archived files predate that decision.

## `root-legacy-preautomigrate/`
The original `0001`–`0024` migrations, written as plain `.sql` (no `.up`/`.down` direction). They
were **already ignored** by golang-migrate (wrong filename format) and were redundant with what
AutoMigrate now builds. They also contained a **duplicate version `0018`** (`organizations_saas`
and `risk_enhancements`) — a latent landmine that would have made golang-migrate fatal the moment
one was renamed to a valid format. Removed from the read path for that reason.

## `backend-0026-0047-neverread/`
The `0026`–`0047` migrations (plus stray duplicate `0052`–`0054`) that lived in `backend/migrations/`
— a directory the runner **never read** (`RunMigrations` reads the repo-root `migrations/`). Their
`ADD COLUMN IF NOT EXISTS` statements are redundant with AutoMigrate; their only non-redundant
content is **data backfills** (e.g. `0044` ownership) that apply to a database with pre-existing
rows. A fresh production database has none, so they are no-ops there. If you ever need to backfill a
legacy database, run the relevant file by hand and record it — do not wire this directory back in.
