-- Copyright (c) 2026 OpenDefender Contributors
-- SPDX-License-Identifier: AGPL-3.0-only

-- control_evidences was never dropped by the up migration, so rolling back
-- returns the register to exactly the rows it had. Artifacts created *after* the
-- up migration only exist in the library and are lost on rollback — that is the
-- honest cost of going back, and it is why the up leaves the old table alone.
DROP TABLE IF EXISTS evidence_control_links;
DROP TABLE IF EXISTS evidences;
