---
name: defect-pattern-openapi-middleware-codes
description: OpenAPI ops written "from the handler" miss the status codes the middleware chain adds (IP/tenant 429, CSRF 403, MFA guard 403/503); seen in #1120
metadata:
  type: project
---

When a route is documented by reading only its handler, the spec misses the codes the
middleware chain in main.go returns before the handler runs. On #1120 (2026-10-22) the
/auth ops were faithful to their handlers but omitted:
- 429 from `api.Use(IPRateLimit)` (every /api/v1 route) and `protected.Use(tenantRateLimit)`
- 403 `CSRF_FAILED` from `Protected` on cookie-authenticated mutations
- 403 `MFA_ENROLLMENT_REQUIRED` / 503 `MFA_STATUS_UNAVAILABLE` from `MFAPolicyGuard`
  (exempt only /auth/mfa/{setup,verify,challenge}, /auth/me, /auth/logout)

**Why:** generated frontend types come from the spec since #814; a missing code is a branch
the client never handles.
**How to apply:** for any spec PR, list the middleware between `app.Group("/api/v1")` and the
route's registration line and check each one's refusal codes appear. Related: [[adr-0005-router-matrix]].
Also: removing ghost ops can orphan request schemas (CreateSubActionInput) — lint warns, not errors.
