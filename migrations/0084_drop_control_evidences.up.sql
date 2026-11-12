-- #813 (D-056) — retire the legacy control_evidences table. The evidence
-- library (evidences + evidence_control_links) is the only evidence register:
-- the product has read only the library since it shipped, and until this
-- release every boot copied control_evidences into it. That boot backfill is
-- removed in the same release, so this migration does its work one last time
-- and then proves it before it drops anything.
--
-- In order, in one transaction:
--   1. No table: nothing to do. No migration or AutoMigrate model creates
--      control_evidences any more, so a database installed since then has none.
--   2. An empty table is dropped.
--   3. Rows, but no library to hold them (the SQL layer run before the first
--      boot of a database that predates the library): refuse. Dropping would
--      lose them.
--   4. The last backfill, the same two statements the boot ran: each row into
--      evidences with its id kept, so stored file keys and bookmarked URLs
--      still resolve, and each live row's control link. Both idempotent by
--      NOT EXISTS, so a row the boot already moved is left as it is.
--   5. The proof of #813 criterion 1: every row has a library row with the same
--      id AND the same tenant (a soft-deleted library row counts: the user
--      deleted that proof in the library, it was not lost), and every live row
--      has its control link in the same tenant. Any miss refuses the drop and
--      names the counts.
--   6. The drop.
--
-- A refusal raises, so golang-migrate marks the version dirty and the boot
-- stops (RunMigrations is fatal). Nothing has been dropped at that point: the
-- transaction rolls back, the table and its rows are intact.
--
-- Only the columns of the original schema (migration 0028) are read; the
-- ownership columns of 0044 are absent from databases where migrations were
-- blocked. owner_id comes from uploaded_by, which is what 0044 filled it from.

DO $$
DECLARE
    missing_document bigint;
    missing_link     bigint;
BEGIN
    IF to_regclass('control_evidences') IS NULL THEN
        RETURN;
    END IF;

    IF NOT EXISTS (SELECT 1 FROM control_evidences) THEN
        DROP TABLE control_evidences;
        RETURN;
    END IF;

    IF to_regclass('evidences') IS NULL OR to_regclass('evidence_control_links') IS NULL THEN
        RAISE EXCEPTION 'migration 0084 (#813): control_evidences has rows and the evidence library does not exist yet; boot the server once so AutoMigrate creates it, then run the migrations again. Nothing was dropped.';
    END IF;

    INSERT INTO evidences (
        id, tenant_id, title, type, description, file_ref, filename,
        collected_at, collected_by, review, source, owner_id,
        created_at, updated_at, deleted_at
    )
    SELECT ce.id, ce.tenant_id,
           COALESCE(NULLIF(ce.filename, ''), 'Preuve'),
           'document', ce.description, COALESCE(ce.url, ''), COALESCE(ce.filename, ''),
           ce.created_at, ce.uploaded_by, 'accepted', 'manual', ce.uploaded_by,
           ce.created_at, ce.updated_at, ce.deleted_at
    FROM control_evidences ce
    WHERE NOT EXISTS (SELECT 1 FROM evidences e WHERE e.id = ce.id);

    INSERT INTO evidence_control_links (id, tenant_id, evidence_id, control_id, linked_by, created_at)
    SELECT gen_random_uuid(), ce.tenant_id, ce.id, ce.control_id, ce.uploaded_by, ce.created_at
    FROM control_evidences ce
    WHERE ce.deleted_at IS NULL
      AND NOT EXISTS (
        SELECT 1 FROM evidence_control_links l
        WHERE l.evidence_id = ce.id AND l.control_id = ce.control_id);

    SELECT count(*) INTO missing_document
    FROM control_evidences ce
    WHERE NOT EXISTS (
        SELECT 1 FROM evidences e
        WHERE e.id = ce.id AND e.tenant_id = ce.tenant_id);

    SELECT count(*) INTO missing_link
    FROM control_evidences ce
    WHERE ce.deleted_at IS NULL
      AND NOT EXISTS (
        SELECT 1 FROM evidence_control_links l
        WHERE l.evidence_id = ce.id AND l.control_id = ce.control_id
          AND l.tenant_id = ce.tenant_id);

    IF missing_document > 0 OR missing_link > 0 THEN
        RAISE EXCEPTION 'migration 0084 (#813): % control_evidences row(s) have no evidence of the same id and tenant in the library, and % live row(s) have no control link in the same tenant. Nothing was dropped.',
            missing_document, missing_link;
    END IF;

    DROP TABLE control_evidences;
END $$;
