-- #670 — TPRM v1: at most one ACTIVE public link per vendor assessment
-- (ADR 0004 D4).
--
-- The five TPRM tables (vendor_questionnaire_templates,
-- vendor_questionnaire_questions, vendor_assessments, vendor_assessment_items,
-- vendor_assessment_tokens) are purely additive and arrive through AutoMigrate,
-- the declared schema authority (internal/migrations/migrator.go). This file
-- carries the one thing AutoMigrate cannot express: a PARTIAL unique index.
--
-- WHY IT MATTERS. Every reminder and every resend issues a new token and
-- supersedes the previous one, so a vendor always holds exactly one working
-- link. The repository supersedes before it inserts, inside one transaction.
-- This index is the backstop that makes two racing issuers fail loudly instead
-- of leaving two live tokens for one assessment — which would quietly double
-- the credentials that can write a vendor's answers.

BEGIN;

CREATE UNIQUE INDEX IF NOT EXISTS idx_vendor_assessment_tokens_one_active
    ON vendor_assessment_tokens (assessment_id)
    WHERE superseded_at IS NULL;

COMMIT;
