# OpenRisk Plans & Pricing (open-core)

OpenRisk is **open-core**: the Community Edition (this repository, AGPL-3.0) is
free forever and self-hostable. Paid plans add commercial features and managed
hosting. The matrix below is the single source of truth
[`backend/pkg/entitlements/entitlements.go`](../backend/pkg/entitlements/entitlements.go);
edit it there to change what a plan grants — the backend enforces it and the
frontend greys/explains from it.

## Feature matrix

|                  | **Free** | **Pro ⭐** | **Business** | **Enterprise** |
|------------------|:--------:|:---------:|:------------:|:--------------:|
| Users            | 2        | 10        | 50           | ∞              |
| Risks            | 50       | 500       | ∞            | ∞              |
| Assets           | 50       | ∞         | ∞            | ∞              |
| Integrations     | 1        | 10        | ∞            | Custom         |
| API              | Limited  | ✓         | ✓            | ✓              |
| Automation (SOAR)| —        | ✓         | Advanced     | Advanced       |
| AI Advisor       | —        | ✓         | Advanced     | Advanced       |
| Financial quant. (Monte-Carlo) | — | ✓ | ✓          | ✓              |
| Smart risk score | —        | ✓         | ✓            | ✓              |
| Executive dashboard | —     | ✓         | ✓            | ✓              |
| Infra scanner    | —        | ✓         | ✓            | ✓              |
| Compliance       | Basic    | Standard  | Advanced     | Custom         |
| Threat intel (CTI) | —      | —         | ✓            | Advanced       |
| Governance / approvals | —  | —         | ✓            | Advanced       |
| SSO / SAML       | —        | —         | ✓            | Advanced       |
| Multi-tenant     | —        | —         | —            | ✓              |
| On-premise       | —        | —         | —            | ✓              |
| SLA              | —        | —         | 99.5 %       | 99.9 %         |
| Support          | Community| Email     | Priority     | Dedicated      |

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
- The **frontend never grants**. It greys the feature and explains the value with
  an upgrade CTA (`FeatureGate` / `UpsellLock`). A wall that explains converts; a
  wall that hides frustrates.
- Payment runs through **Stripe** (cards) or **Notchpay / CinetPay** (MTN MoMo,
  Orange Money, Wave). With no gateway configured, paid plans still start as a
  trial and Enterprise is sales-invoiced — nothing is faked.
