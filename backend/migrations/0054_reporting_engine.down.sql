-- Copyright (c) 2026 OpenDefender Contributors
-- SPDX-License-Identifier: AGPL-3.0-only

-- Rolling back destroys every generated document. report_jobs is untouched by
-- the up migration, so the earlier compliance reports survive the round trip;
-- anything produced by the new engine does not, which is the honest cost of
-- going back and the reason the up migration does not drop the old table.
DROP TABLE IF EXISTS report_comments;
DROP TABLE IF EXISTS reports;
