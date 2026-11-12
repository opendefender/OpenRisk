-- #553 — undo 0085: remove the six control-chain tables, and nothing else.
-- compliance_controls, evidence_control_links and risk_control_mappings were
-- never written by 0085 or by controlchain.Reconcile, so they are untouched and
-- byte-identical to before. What the chain held is derived data in this first
-- slice, and Reconcile rebuilds it on the next boot if the tables return.
DROP TABLE IF EXISTS canonical_edges;
DROP TABLE IF EXISTS control_gap_findings;
DROP TABLE IF EXISTS control_test_results;
DROP TABLE IF EXISTS control_tests;
DROP TABLE IF EXISTS control_implementations;
DROP TABLE IF EXISTS controls;
