-- #813 (D-056) — recreate control_evidences, EMPTY, in its last shape
-- (migration 0028 plus the ownership columns of 0044). The rows the up
-- migration dropped are not restored: they live in the evidence library, which
-- this down leaves untouched. The owner accepted that loss in D-056.
--
-- Nothing in the code reads or writes this table any more; the down exists so
-- the version chain can step back, not to bring the old register back to life.
-- The foreign key to compliance_controls is added only when that table exists,
-- since the SQL layer may run before the first AutoMigrate.

CREATE TABLE IF NOT EXISTS control_evidences (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL,
    control_id  UUID NOT NULL,
    filename    VARCHAR(255) NOT NULL DEFAULT '',
    url         TEXT NOT NULL DEFAULT '',
    description TEXT,
    uploaded_by UUID,
    owner_id    UUID,
    assignee_id UUID,
    reviewer_id UUID,
    created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at  TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_control_evidences_tenant_id      ON control_evidences (tenant_id);
CREATE INDEX IF NOT EXISTS idx_control_evidences_control_id     ON control_evidences (control_id);
CREATE INDEX IF NOT EXISTS idx_control_evidences_deleted_at     ON control_evidences (deleted_at);
CREATE INDEX IF NOT EXISTS idx_control_evidences_tenant_control ON control_evidences (tenant_id, control_id);
CREATE INDEX IF NOT EXISTS idx_control_evidences_owner_id       ON control_evidences (owner_id);
CREATE INDEX IF NOT EXISTS idx_control_evidences_assignee_id    ON control_evidences (assignee_id);
CREATE INDEX IF NOT EXISTS idx_control_evidences_reviewer_id    ON control_evidences (reviewer_id);

DO $$
BEGIN
    IF to_regclass('compliance_controls') IS NOT NULL
       AND NOT EXISTS (SELECT 1 FROM pg_constraint
                       WHERE conname = 'fk_evidences_control'
                         AND conrelid = 'control_evidences'::regclass) THEN
        ALTER TABLE control_evidences
            ADD CONSTRAINT fk_evidences_control FOREIGN KEY (control_id)
            REFERENCES compliance_controls (id) ON DELETE CASCADE;
    END IF;
END $$;
