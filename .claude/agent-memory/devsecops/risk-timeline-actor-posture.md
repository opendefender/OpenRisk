---
name: risk-timeline-actor-posture
description: PR #1139 / #793 review 2026-10-28 — risk timeline reads audit trail only, Actor.kind added; PASS with 1 LOW (kind absent on pre-0064 audit rows)
metadata:
  type: project
---

PR #1139 (#793) reviewed 2026-10-28: PASS, no CRITICAL/HIGH.

- RiskHistorySource removed from entity timeline; risk timeline = audit_events only (tenant-scoped via audit.List(tenantID)). Entity path gate unchanged: access() (tenant 401 / perm 403) then Resolver.Summary (OwnColumn risks, 404 cross-tenant).
- Legacy /risks/:id/timeline* still call ownsRisk (fail closed on Nil tenant); ChangedBy blanked via withoutActor.
- Actor.kind: job branch exposes only ActorLabel (job name); service_token with ActorID shows owner email, never token id.
- LOW: pre-migration-0064 audit rows have NULL actor_type, so `kind` is absent on audit entries too, contrary to the OpenAPI description.
- Pre-existing, not from this PR: GormUserRepository.EmailsByIDs/NamesByIDs are not tenant-scoped (they only resolve IDs already in the tenant's trail). Worth re-checking if any caller passes caller-controlled IDs.

Related: [[recurring-defect-areas]], [[router-extraction-posture]]
