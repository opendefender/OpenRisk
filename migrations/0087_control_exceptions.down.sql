-- #1428 — undo 0087: remove the control_exceptions table, and nothing else.
-- controls and the rest of the control chain are untouched. Dropping the table
-- loses the exceptions recorded since 0087 ran; this is a rollback of a release,
-- not a routine step, and the audit_events entries that journaled each exception
-- stay in the trail.
DROP TABLE IF EXISTS control_exceptions;
