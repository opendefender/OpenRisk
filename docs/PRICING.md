# OpenRisk Plans & Pricing (open-core)

OpenRisk is **open-core**: the Community Edition (this repository, AGPL-3.0) is
free forever and self-hostable, with every capability below and none of the
caps (D-040, D-075). The plans below are the **hosted service** plans: they
differ in what the hosted service includes, never in what the code can do
([ADR 0007](adr/0007-plan-gates-are-hosted-only.md)).

The matrix is the single source of truth
[`backend/pkg/entitlements/entitlements.go`](../backend/pkg/entitlements/entitlements.go);
edit it there to change what a plan grants. The backend enforces it on every
paid row, the frontend greys and explains from it, and
[`pricing_doc_test.go`](../backend/pkg/entitlements/pricing_doc_test.go) fails
the build when this page and the code disagree.

## Feature matrix

|                  | **Free** | **Pro ⭐** | **Business** | **Enterprise** |
|------------------|:--------:|:---------:|:------------:|:--------------:|
| Users            | 2        | 10        | 50           | ∞              |
| Risks            | 50       | 500       | ∞            | ∞              |
| Assets           | 50       | ∞         | ∞            | ∞              |
| Integrations     | 1        | 10        | ∞            | Custom         |
| Active frameworks | 1       | 3         | 10           | ∞              |
| API              | Limited  | ✓         | ✓            | ✓              |
| Automation (SOAR)| —        | ✓         | Advanced     | Advanced       |
| AI Advisor       | —        | ✓         | Advanced     | Advanced       |
| Financial quant. (Monte-Carlo) | — | ✓ | ✓          | ✓              |
| Smart risk score | —        | ✓         | ✓            | ✓              |
| Executive dashboard | —     | ✓         | ✓            | ✓              |
| Infra scanner    | —        | ✓         | ✓            | ✓              |
| Compliance       | ✓        | ✓         | ✓            | ✓              |
| Threat intel (CTI) | —      | —         | ✓            | Advanced       |
| Governance / approvals | —  | —         | ✓            | Advanced       |
| Vendor risk (TPRM) | —      | —         | ✓            | ✓              |
| SSO / SAML       | —        | —         | —            | —              |
| Multi-tenant     | —        | —         | —            | ✓              |
| On-premise       | —        | —         | —            | ✓              |
| SLA              | —        | —         | 99.5 %       | 99.9 %         |
| Support          | Community| Email     | Priority     | Dedicated      |

**Compliance.** Every framework in the catalogue is available on every plan.
The plan sets how many frameworks a tenant keeps active at once (D-076); a
deleted framework frees its slot.

**SSO / SAML** is not sold on any plan today. SAML sign-in is switched off
(#866) and OAuth providers are configured per deployment, so there is no
tenant SSO for a plan to include. It returns with #1045.

## Pricing (PPP by zone)

Prices are purchasing-power adjusted by region. A trial of any paid plan runs
**14 days with no credit card**.

### Europe (EUR)

| Plan       | Monthly   | For |
|------------|----------:|-----|
| Free       | 0 €       | Discover OpenRisk |
| Pro ⭐     | 49 €      | IT / Security teams |
| Business   | 149 €     | SMEs / organizations |
| Enterprise | On quote  | Large / regulated |

### Africa (XAF/XOF — CFA franc)

| Plan       | Monthly       | For |
|------------|--------------:|-----|
| Free       | 0 XAF         | Discover OpenRisk |
| Pro ⭐     | 12 500 XAF    | IT / Security teams |
| Business   | 39 000 XAF    | SMEs / organizations |
| Enterprise | On quote      | Large / regulated |

## How enforcement works

- The **backend refuses** a premium route the plan does not include, returning
  `402 Payment Required` with an explaining body (feature, current plan, required
  plan). Creation routes are capped by plan limits the same way.
- Every ✓ in a paid column has a gate on its routes: automation (except the
  notification channels, which incident alerts also use), the infrastructure
  scanner, threat intel, governance, financial quantification, the smart
  score, the executive dashboard, the AI advisor and vendor risk.
- A depth (Basic, Advanced) is refused the same way when a route requires more
  than the plan grants. Today that holds for financial quantification only: a
  basic grant reads a risk's closed-form ALE, the Monte-Carlo simulation needs
  ✓. "Advanced" on automation, AI, threat intel and governance, and "Limited"
  on the Free API, are not yet told apart from ✓ by the backend; the list of
  those gaps is kept, and checked against the code, in
  [`pricing_enforcement_test.go`](../backend/pkg/entitlements/pricing_enforcement_test.go).
- The **frontend never grants**. It greys the feature and explains the value with
  an upgrade CTA (`FeatureGate` / `UpsellLock`). A wall that explains converts; a
  wall that hides frustrates.
- Payment runs through **Stripe** (cards) or **Notchpay / CinetPay** (MTN MoMo,
  Orange Money, Wave). With no gateway configured, paid plans still start as a
  trial and Enterprise is sales-invoiced — nothing is faked.
