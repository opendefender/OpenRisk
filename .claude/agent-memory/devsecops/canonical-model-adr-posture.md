---
name: canonical-model-adr-posture
description: ADR 0008 canonical model (#541) security read 2026-11-03 — edge target tenant check under-specified for shape-deferred targets (HIGH), envelope mass assignment, provenance free text
metadata:
  type: project
---

ADR 0008 draft (worktree OpenRisk-wt-541, issue #541) read 2026-11-03: FAIL, 1 HIGH.

- HIGH: D5 edge shape `{verb,target_type,target_id,...}` carries no target tenant; "shape deferred" (Asset, System, Service, Threat, Action, Mitigation) did not say tenant ownership is still checked at write. Named test TestCanonical_EdgeRefusesAnotherTenant is unprovable in a pure package that cannot resolve targets. risk_assets gated through risks proves only the source end.
- MEDIUM: envelope fields (tenant_id, source, source_id, confidence, provenance) client-writable → in-tenant spoofing of scanner records via the identity upsert; connector_import confidence has no ceiling.
- MEDIUM: provenance via/rule/raw_ref/ingest_id and Evidence.external_url are free strings (presigned URLs, userinfo); ADR 0004 keys conflict with "unknown keys refused".
- LOW: derived_from may name global CVE (not a tenant record).

**Why:** canonical model becomes the contract for #542/#543/#553; isolation gaps here propagate to every ingest.
**How to apply:** when the implementation issue lands, check the edge write path does `WHERE id=? AND tenant_id=?` per target type, including deferred ones. See [[recurring-defect-areas]].

**#1240 implementation review 2026-11-03: PASS (0 CRITICAL/HIGH).** Edge.Validate now refuses To.TenantID != TenantID (nil included); envelope relationships must match tenant and source end. Open MEDIUMs: credentialShaped bypassed by wrapped URLs `(https://u:p@h)`, scheme-less `user:pw@host`, unparseable `https://u:p@h/%zz`; secretParams misses pwd/jwt/private_token; Target.TenantID has a json tag (caller-decodable); Evidence 0.90 uplift trusts body reviewed_by. LOWs: attrText uncapped, error echoes Verb/key names, qualifier/resolved_by tenant left to persistence.
