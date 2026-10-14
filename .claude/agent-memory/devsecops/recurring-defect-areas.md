---
name: recurring-defect-areas
description: Codebase areas that repeatedly produce security findings in OpenRisk, to prioritise in future audits
metadata:
  type: project
---

Areas that keep producing findings, updated 2026-10-01:
- Outbound HTTP to tenant-configured URLs. Each integration builds its own client, and many ignore pkg/netguard. See [[ssrf-outbound-posture]].
- Error strings copied into API responses (err.Error() in `Detail`/`Error` fields). They leak transport details and remote bodies. #573 fixed the probe, livepull and ticketing. Automation channel test still does this.
- In-process scanner collectors with tenant credentials. These are SDK clients that netguard cannot wrap, and some fall back to ambient pod credentials (GCP ADC).
- Risk handler (backend/internal/handler/risk_handler.go): direct `database.DB` queries without tenant_id (UpdateRisk reload `First(&out, "id = ?")`), and use-case errors echoed with `err.Error()` at 400. #792 moved asset linking into a tenant-scoped port (gorm_risk_asset_store.go), which is clean. `GormRiskRepository.Update` is a bare `Save()`, and GORM Save INSERTs when the UPDATE matches 0 rows.
- Return-target sanitisers (backend `sanitiseReturnTo` in oauth2_handler.go, frontend `safeNextPath.ts`) check prefixes only. They allow `/\t/evil` and `/\n/evil`, which browsers strip down to `//evil`. React Router's `push` falls back to `window.location.assign` when pushState throws; `replace` does not. Reviewed in #803 on 2026-10-01.
- SAML2ACS has no signature verification, InResponseTo check, or RelayState handling. This was escalated privately on 2026-10-01.
- Test seams (injected HTTPDoer) that skip validation. So far only tests set them. Check that production wiring never does.

**Why:** these patterns keep coming back with each new connector.
**How to apply:** start every audit by grepping these areas before reading anything else.
