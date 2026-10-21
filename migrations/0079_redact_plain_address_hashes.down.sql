-- #879 — 0079 cannot be undone: the redacted hashes and the deleted reset rows
-- are gone, which is the point (D-074). Rolling back leaves the data as it is.
SELECT 1;
