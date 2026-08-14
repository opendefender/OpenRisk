-- Migration: 0029_add_control_source_reference.up.sql
-- Purpose: M2 — cite the regulatory source (article, circular, standard section) a control
-- comes from, so catalog-imported controls (ISO 27001, COBAC, BCEAO, ANSSI-CM, ...) are
-- traceable back to their text. Empty for ad-hoc controls a tenant creates by hand.

ALTER TABLE compliance_controls
    ADD COLUMN IF NOT EXISTS source_reference VARCHAR(255) NOT NULL DEFAULT '';
