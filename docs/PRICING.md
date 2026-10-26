# OpenRisk Plans & Pricing (open-core)

OpenRisk is **open-core**: the Community Edition (this repository, AGPL-3.0) is
free forever and self-hostable, with every capability below and none of the
caps (D-040, D-075). The plans below are the **hosted service** plans: they
differ in what the hosted service includes, never in what the code can do
([ADR 0007](adr/0007-plan-gates-are-hosted-only.md)).

**Free does almost everything, in small quantities** (D-078). A free tenant
uses automation, the scanner, governance, the smart score, the executive
dashboard and financial quantification for real, and meets the cap where it
would want more. The paid plans raise the caps.

The matrix is the single source of truth
[`backend/pkg/entitlements/entitlements.go`](../backend/pkg/entitlements/entitlements.go);
edit it there to change what a plan grants. The backend enforces it, the
frontend shows the counters and explains the caps, and
[`pricing_doc_test.go`](../backend/pkg/entitlements/pricing_doc_test.go) fails
the build when this page and the code disagree.

## Feature matrix

|                  | **Free** | **Pro ⭐** | **Business** | **Enterprise** |
|------------------|:--------:|:---------:|:------------:|:--------------:|
| Users            | 2        | 10        | 50           | ∞              |
| Risks            | 30       | 500       | ∞            | ∞              |
| Assets           | 30       | ∞         | ∞            | ∞              |
| Integrations     | 1        | 10        | ∞            | Custom         |
| Active frameworks | 1       | 3         | 10           | ∞              |
| Active automation rules | 2 | 20        | ∞            | ∞              |
| Scan sources     | 1        | 10        | ∞            | ∞              |
| Scans per month  | 4        | ∞         | ∞            | ∞              |
| Approval workflows | 1      | 3         | ∞            | ∞              |
| API              | Limited  | ✓         | ✓            | ✓              |
| Automation (SOAR)| ✓        | ✓         | ✓            | ✓              |
| AI Advisor       | —        | ✓         | Advanced     | Advanced       |
| Financial quant. | Basic    | ✓         | ✓            | ✓              |
| Smart risk score | ✓        | ✓         | ✓            | ✓              |
| Executive dashboard | ✓     | ✓         | ✓            | ✓              |
| Infra scanner    | ✓        | ✓         | ✓            | ✓              |
| Compliance       | ✓        | ✓         | ✓            | ✓              |
| Threat intel (CTI) | —      | —         | ✓            | Advanced       |
| Governance / approvals | ✓  | ✓         | ✓            | ✓              |
| Vendor risk (TPRM) | —      | —         | ✓            | ✓              |
| SSO / SAML       | —        | —         | —            | —              |
| Multi-tenant     | —        | —         | —            | ✓              |
| On-premise       | —        | —         | —            | ✓              |
| SLA              | —        | —         | 99.5 %       | 99.9 %         |
| Support          | Community| Email     | Priority     | Dedicated      |

**Financial quantification.** Free computes each risk's deterministic
annualised loss expectancy; the Monte-Carlo simulation, the portfolio loss
curve and the appetite drawn on it start at Pro (#613).

**Compliance.** Every framework in the catalogue is available on every plan.
The plan sets how many frameworks a tenant keeps active at once (D-076); a
deleted framework frees its slot.

**Automation.** A suspended rule does not count: a tenant at its cap can still
draft and dry-run. Notification channels are on every plan.

**Scans per month** count every scan started since the first of the month
(UTC), by hand, on a schedule or by an automation rule.

**Threat intel, AI Advisor and vendor risk** keep a plan gate for now. The
owner's grid (D-078) gives Free the 10 most relevant threats, 10 AI requests a
month and 3 vendors; each ships with its own issue, and this page changes when
the code applies it.

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
- Every cap is applied where the thing it counts is created: users on
  invitation and user creation, risks, assets and frameworks on create and
  import, scan sources and approval workflows on create. Active automation
  rules and scans per month are applied in their use cases, so a rule turned
  on by any route and a scan started by the scheduler or an automation rule
  count the same. **Integrations is counted but not yet enforced**: its route
  is an upsert, and capping it would also block editing an existing one.
- A depth (Basic, Advanced) is refused the same way when a route requires more
  than the plan grants. Today that holds for financial quantification only: a
  basic grant reads a risk's closed-form ALE, the Monte-Carlo simulation needs
  ✓. "Advanced" on AI and threat intel, and "Limited" on the Free API, are not yet told apart from ✓ by the backend; the list of
  those gaps is kept, and checked against the code, in
  [`pricing_enforcement_test.go`](../backend/pkg/entitlements/pricing_enforcement_test.go).
- The **frontend never grants**. It shows the counter, greys a feature the plan
  lacks and explains the value with an upgrade CTA (`FeatureGate` /
  `UpsellLock`, `PlanCapNotice`). A wall that explains converts; a
  wall that hides frustrates.
- Payment runs through **Stripe** (cards) or **Notchpay / CinetPay** (MTN MoMo,
  Orange Money, Wave). With no gateway configured, paid plans still start as a
  trial and Enterprise is sales-invoiced — nothing is faked.
