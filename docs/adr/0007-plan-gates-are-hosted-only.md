# 0007 — Plan gates are hosted-only, and compliance is capped by count

Status: **accepted**
Proposed: 2026-10-22, on #783
Accepted: 2026-10-22 by the owner (D-075, model A; D-076, option B amended)
Depends on: #612 (self-hosted must not resolve to `PlanFree`)
Related: D-040, D-044, D-006, #415, #613, #1045, #1109

## Context

`docs/PRICING.md` sells ten lines across the hosted plans and says "the backend enforces it".
On origin/master the backend enforces five of them. `backend/cmd/server/main.go:1240-1245`
mounts `RequireLevel` for financial quantification and `RequireFeature` for smart score,
executive dashboard, AI advisor and vendor risk. `FeatAutomation`, `FeatScanner`, `FeatCTI`,
`FeatGovernance` and `FeatSSO` are declared in `backend/pkg/entitlements/entitlements.go` and
read by no route. A hosted Free tenant gets automation, the infra scanner, threat intel and
governance at no cost.

The Compliance row is worse: it shows Basic / Standard / Advanced / Custom, `FeatCompliance`
carries those levels in the matrix, and nothing reads them. No route behaves differently
between Basic and Custom, and nowhere is it written what a tier contains.

Two earlier rulings frame the answer:

- **D-040** — self-hosting is free and complete. The only thing withheld is what calls an API
  the owner pays for (the AI advisor without the operator's own key).
- **#612** (open, `status:ready`) — a self-hosted deployment resolves to a self-hosted
  entitlement set declared at boot, not to `PlanFree`. Today it resolves to `PlanFree`
  (`backend/internal/application/entitlements/service.go`), so any new gate would also hit
  self-hosted installs.

The owner was offered four models: A, limits by hosted service only, code 100% AGPL; A+B, A
plus a short commercial-licensed list; B, a broad open core; C, quotas coded into the AGPL
edition. The Compliance tiers were raised separately, with two options: define each tier's
content, or drop the tiers.

## Decision

1. **Self-hosted is free, complete and fully AGPL.** No licence key, no coded quota, no
   commercial module, no delayed feature (D-075, confirming D-040).
2. **Plan gates differentiate hosted plans only.** `RequireFeature`, `RequireLevel` and
   `RequireCapacity` read the tenant's resolved entitlement set. Once #612 lands, a
   self-hosted deployment resolves to a set that grants every feature and reports every limit
   as unlimited, so the same middleware admits it. No gate checks "am I self-hosted?" itself.
   The deployment property lives in one place, #612's resolver.
3. **Four more features get a hosted gate**, on the plan the matrix already states:

   | Feature | Key | Opens at | Routes |
   |---|---|---|---|
   | Automation (SOAR) | `FeatAutomation` | Pro | every `protected` `/automation/*` route |
   | Infra scanner | `FeatScanner` | Pro | every `protected` `/scanner/*` console route |
   | Threat intel | `FeatCTI` | Business | every `protected` `/cti/*` route |
   | Governance / approvals | `FeatGovernance` | Business | every `protected` `/governance/*` route |

4. **SSO is gated on configuration only, and only once that configuration exists.** The SSO
   sign-in routes (`/auth/saml2/*`, `/auth/oauth2/*`) run before a tenant is known, so a plan
   gate cannot sit on them and never will. On origin/master there is no per-tenant SSO
   configuration route: OAuth providers are set by deployment environment variables, and SAML
   refuses every request since #866 (`backend/internal/handler/saml2_handler.go:25-33`).
   `FeatSSO` stays in the matrix at Business; the gate is mounted by the issue that creates a
   tenant SSO configuration route (#1045 or its follow-up). Until then `docs/PRICING.md` must
   not list SSO / SAML as available on any plan.
5. **Every compliance framework is included on every hosted plan.** The plan caps how many
   frameworks a tenant has active at the same time:

   | Plan | Frameworks active at once |
   |---|---|
   | Free | 1 |
   | Pro | 3 |
   | Business | 10 |
   | Enterprise | unlimited |

   "Active" means a framework row of the tenant that is not soft-deleted. The cap is a new
   count limit (`LimitKey`) enforced by `RequireCapacity` on the one route that creates a
   framework, `POST /compliance/frameworks`. Deleting a framework frees a slot. Self-hosted is
   uncapped through decision 2.
6. **The Compliance levels leave the matrix.** `FeatCompliance` no longer carries
   Basic / Standard / Advanced / Custom. The `Level` constants stay: `LevelBasic` is still read
   by the financial quantification depth gate (#613).
7. **`docs/PRICING.md` is rewritten from `entitlements.go`**, row for row, so that "the backend
   enforces it" becomes true. A row with no gate is either marked included on every plan or
   marked as not available. It never sits in a paid column.

## What this is not

- **Not a quota in the AGPL edition.** If a self-hosted instance ever hits one of these gates,
  that is a #612 defect, not a pricing behaviour.
- **Not a retroactive removal.** A hosted tenant above its framework cap (a downgrade, or data
  from before the cap) keeps every existing framework readable and editable. Only creating one
  more is refused.
- **Not a new hosted service.** The incentives the owner listed for moving to SaaS (sector
  benchmarks, an external attack-surface scan, regulatory-watch updates, managed operations)
  are ideas tracked in #1109. None is built and none may be described as available.
- **Not a Score Engine or SmartScore change.** Neither formula is touched.

## Consequences

- #783 implements decisions 3, 5, 6 and 7 and ships only after #612 merges. Shipping any gate
  before #612 would cap self-hosted installs, which decision 1 forbids.
- Every new gate needs a test that a hosted Free tenant is refused (402, `upgrade_required` or
  `limit_reached`) and a tenant on the opening plan is admitted, plus one test that the
  self-hosted set is admitted.
- Screens that call a gated API from another module must read the 402 as "not on this plan"
  and degrade, not show an error: the executive dashboard lists pending approvals (Pro has the
  dashboard, not governance), the risk register reads `/cti/vulnerabilities/:cve`, and the
  asset register's discover button lists scan configs.
- `FeatureGate` covers the automation, governance, threat intel and infrastructure screens, so
  a locked feature is explained instead of mounted and left to fail.
- Scanner agent routes (`/api/v1/scanner/agent/*`, `/scanner/agents/register`) authenticate
  with agent tokens, not sessions, and stay ungated: a Free tenant cannot create a scan config,
  so it cannot mint a registration token. What happens to the agents of a tenant that
  downgrades is a separate question, not decided here.
- Hosted tenants lose access to the governance audit-trail screens on Free and Pro. The trail
  is still recorded for every tenant; only reading it is gated. If that proves to hurt trust
  more than it sells, it is a new owner decision, not an implementation detail.
- `#415` (what the hosted Free tier contains) inherits the framework cap and the four gates as
  givens.
