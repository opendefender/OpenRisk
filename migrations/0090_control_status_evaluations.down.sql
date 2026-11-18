-- #555 — undo 0090: remove the control_status_evaluations table, and nothing else.
-- The table is a cache of what the engine computes from the control chain, the
-- exceptions and the evidence, none of which is touched. Removing it loses no
-- source data; the worker fills it again, cold, when the migration runs once more.
DROP TABLE IF EXISTS control_status_evaluations;
