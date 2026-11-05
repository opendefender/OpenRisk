---
name: openapi-auth-docs-posture
description: #1120 review 2026-10-22 — /auth spec claims vs code; PAT reaches /auth/switch-org and mints a full session (HIGH, code); false 409 enumeration claim
metadata:
  type: project
---

#1120 (worktree OpenRisk-wt-1120) documents the 22 /auth routes in docs/openapi.yaml. Read-only review 2026-10-22.

- Token types in the spec match the code: challenge = MFA_REQUIRED via the Authorization header only (MFATokenMiddleware, auth.go:298); setup/verify = access or MFA_ENROLLMENT, CSRF only on cookie (mfa_enrollment.go); the rest sit behind Protected. Per-IP numbers match auth_limits.go.
- False prose: register says "the two 409 codes do not reveal whether an address is registered". EMAIL_ALREADY_REGISTERED reveals it (D-073 accepted). Username is checked first now (register.go:145, before GetByEmail at :150), which reverses the order noted in [[duplicate-signup-posture]].
- Code finding, outside the doc diff: PATMiddleware authenticates /auth/switch-org and switch_handler.go has no is_pat refusal, so any PAT, whatever its scope, can mint a full JWT session pair through IssueSessionForOrg. That is a credential-upgrade sink (see [[sign-out-others-posture]]). /auth/pat GET/DELETE also accept a PAT.
- Spec schema drift: reset `sessions_revoked` is a bool in the code but an integer in the spec.

r2 2026-10-22: MEDIUM and LOW fixed. The guard responses are documented accurately: 429 comes from the API-wide IPRateLimit (main.go:520), CSRF is mutations only, MFA_ENROLLMENT_REQUIRED is absent from the exempt routes, and the 503 fails closed. PASS. The HIGH is tracked as #1123.

#1123 fix reviewed 2026-10-22 (branch fix/1123-pat-switch-org): is_pat 403 in switch_handler + change_password_handler, e2e test with real PATMiddleware. PASS. Only session-mint sinks: login, refresh, oauth2 callback (public), mfa challenge/setup/verify (JWT-only guards, before PATMiddleware), switch-org, password/change, sessions/others (all now refuse PAT). SAML ACS is a stub. Residual LOW: PAT can revoke other PATs/sessions and disable MFA (password still required).

**How to apply:** for any spec PR, check each claimed property against the handler. For any route that mints tokens, grep for `is_pat`.
