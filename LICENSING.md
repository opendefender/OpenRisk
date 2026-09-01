# OpenRisk licensing (open-core)

OpenRisk is **open-core**. It ships as two editions in one repository, plus one
permissively-licensed layer that both editions share:

| Edition / layer | License | SPDX in file headers | License file |
|---|---|---|---|
| **Community Edition (CE)** — the core GRC platform | **GNU AGPL v3.0** | `AGPL-3.0-only` | [`LICENSE`](LICENSE) |
| **Enterprise Edition (EE)** — commercial add-ons | **OpenRisk Commercial License** | `LicenseRef-OpenRisk-Commercial` | [`LICENSE.commercial`](LICENSE.commercial) |
| **Design system** — `frontend/design-system/` | **Apache License 2.0** | `Apache-2.0` | [`frontend/design-system/LICENSE`](frontend/design-system/LICENSE) |
| **Design system** — `frontend/src/shared/ds/` | **Apache License 2.0** | `Apache-2.0` | [`frontend/design-system/LICENSE`](frontend/design-system/LICENSE) |

Every source file declares its edition via its `SPDX-License-Identifier` header.
**This file is the authoritative boundary** between them.

The design system is **the exception to the AGPL core**, and it is exactly two
directories — no more. Everything else under `frontend/src/**` is CE (AGPL) or
EE (commercial) per the lists below.

## Why AGPLv3 for the core

The AGPL is a strong copyleft that also covers **use over a network**: anyone who
runs a modified OpenRisk core as a hosted service must release their
modifications under the AGPL. This closes the "SaaS loophole" and keeps a
competitor from taking the community core and offering it as a closed hosted GRC
— while the community remains free to self-host, study, modify and redistribute.

Copyright in the existing code is held by OpenDefender / its contributors
("OpenDefender Contributors"), so OpenDefender retains the right to **dual-license**
(offer the same core under a commercial license to customers who cannot accept
the AGPL). New external contributions are accepted under the project's CLA/DCO so
this right is preserved.

## Why Apache-2.0 for the design system

Two directories are **Apache-2.0**, not AGPL:

| Directory | What it is | Why it is Apache-2.0 |
|---|---|---|
| `frontend/design-system/` | the canonical token contract — `openrisk.tokens.css` and its README | It is the value contract shared with the `opendefender-website` surface and the artifact the project *wants* copied and extended. Its value is protected by trademark, not by copyleft. |
| `frontend/src/shared/ds/` | the primitive component layer — Button, Badge, Field, Modal, Drawer, Tabs, Tooltip, States and their utilities | **This is what Enterprise code imports and what third parties copy.** `frontend/src/features/reports/BoardReportPage.tsx` is `LicenseRef-OpenRisk-Commercial` and imports `Button` and `cn` from `shared/ds` — not the tokens. Relicensing only the token file would have left the copyleft-vs-commercial audit question sitting one directory over, unanswered. |

That second row is the whole reason the boundary is drawn here: **the primitive
layer is relicensed because it is what EE code imports and what third parties
copy** — the token file alone is not the artifact anyone takes. Copyleft on this
layer bought nothing and cost an audit conversation on every enterprise deal.
Decided by the owner on 2026-08-31 (D-014 and D-016, both option A,
`docs/DECISIONS.md`); the boundary is settled and is not reopened per-file.

Attribution lives in [`frontend/design-system/NOTICE`](frontend/design-system/NOTICE),
which covers **both** directories — `frontend/src/shared/ds/` has no `NOTICE` of
its own — and carries the per-component register of anything vendored from
upstream, with its exact upstream commit.

**The boundary is precise. These stay AGPL-3.0-only** even though they live
next door: `frontend/src/shared/ui.tsx`, `frontend/src/shared/EmptyState.tsx`,
and everything else under `frontend/src/shared/`. They are product code that
happens to sit nearby, not the design system.

Apache-2.0 grants copyright and patent rights. It grants **no trademark rights**:
"OpenRisk" and "OpenDefender", their logos and product names are not licensed by
it (Apache-2.0 §6).

## What is Enterprise (commercial), and what is Community (AGPL)

Everything **not** listed below is Community Edition (AGPLv3), except the two
Apache-2.0 design-system directories named above.

### Enterprise Edition — `LicenseRef-OpenRisk-Commercial`

**Advanced SSO** (basic email/password + JWT login, MFA and Personal Access
Tokens stay in CE):
- `backend/internal/handler/oauth2_handler.go`
- `backend/internal/handler/saml2_handler.go`
- `backend/internal/handler/sso_session.go`

**AI copilot** (GRC assistant, board report, treatment-plan/emerging-risk/
evidence generation):
- `backend/pkg/ai/`
- `backend/internal/application/ai/`
- `backend/internal/application/board/`
- `backend/internal/handler/ai_handler.go`
- `backend/internal/handler/board_report_handler.go`
- `frontend/src/features/ai/`
- `frontend/src/features/reports/BoardReportPage.tsx`

**Premium connectors & automation** (SOAR engine, real cloud discovery
collectors, vulnerability live-pull, ITSM ticketing, ChatOps):
- `backend/internal/application/automation/`
- `backend/internal/infrastructure/automation/`
- `backend/internal/handler/automation_handler.go`
- `backend/internal/scanner/collectors/`
- `backend/internal/vulnscan/livepull/`
- `backend/pkg/ticketing/`
- `backend/pkg/notify/chatops.go`
- `frontend/src/features/automation/`
- `frontend/src/features/infrastructure/`

### The multi-tenancy caveat (important)

"Multi-tenant" is listed as an enterprise capability, but it is **not a separable
module**: tenant isolation (`tenant_id` filtering) is a cross-cutting concern
present in *every* Community repository and use case, and it stays **AGPL/CE** —
you cannot meaningfully paywall the plumbing that keeps data safe. What can be
gated commercially is **multi-organisation management** (a super-admin / tenant
provisioning / cross-tenant administration surface). That surface is not built
yet; when it is, it will live under `ee/` and be listed here.

## How the split is enforced (roadmap)

- **Phase 1 — licensing (done):** `LICENSE` (AGPLv3) + `LICENSE.commercial`,
  this manifest, and per-file `SPDX-License-Identifier` headers set to the
  correct edition across the whole repository.
- **Phase 2 — physical isolation (next):** move the EE paths above under an
  `ee/` tree (`backend/ee/…`, `frontend/src/ee/…`) guarded by a Go build tag
  (`//go:build enterprise`) and a matching front-end build flag, so the default
  open-source build compiles and ships **without** the Enterprise Edition. The
  application wiring (`cmd/server/main.go`) is split into a core path and an
  `//go:build enterprise` path; an EE build additionally verifies a license key
  at startup. A CE build simply omits the EE routes.

Until Phase 2 lands, EE files remain in place but are unambiguously marked
commercial by their headers and by this manifest; a build that ships them
requires a Commercial Agreement.

## Third-party licenses

Dependencies retain their own licenses. The AGPL applies to OpenRisk's own
Community source, not to independently-licensed third-party libraries it uses.

For commercial licensing questions: **licensing@opendefender.io**
