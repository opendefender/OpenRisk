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
| Vendor risk (TPRM) | —      | —         | ✓            | ✓              |
| SSO / SAML       | —        | —         | —            | —              |
| Multi-tenant     | —        | —         | —            | ✓              |
| On-premise       | —        | —         | —            | ✓              |
| SLA              | —        | —         | 99.5 %       | 99.9 %         |
| Support          | Community| Email     | Priority     | Dedicated      |

**SSO / SAML** is not sold on any plan today. SAML sign-in is switched off
(#866) and OAuth providers are configured per deployment, so there is no
tenant SSO for a plan to include. It returns with #1045.

## Self-hosting is not the Free plan

The table above is the **hosted (SaaS)** offer. A self-hosted instance, started
with `OPENRISK_DEPLOYMENT=self_hosted` (set by `deploy/selfhost/docker-compose.yml`),
does not resolve to any of these columns. Every organisation on it gets the
self-hosted set (decision D-040):

- no cap on users, risks, assets or integrations;
- every capability that runs on your own hardware: smart risk score, financial
  quantification, executive dashboard, infra scanner, threat intel (NVD and CISA
  KEV are free public feeds), compliance, automation, governance, vendor risk,
  REST API and multi-tenant;
- the AI advisor, on your own `ANTHROPIC_API_KEY`, or on the built-in template
  answers without one;
- community support only, and no SLA: those are services, not software.

SSO is not on any column, self-hosted included, until #1045. The full generated
table is in [SELF_HOSTING.md](SELF_HOSTING.md#what-a-self-hosted-instance-includes).
Nothing is paid for, and no licence key is involved.

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
- A depth (Basic, Standard, Advanced, Custom) is refused the same way when a
  route requires more than the plan grants. Today that holds for financial
  quantification only: a basic grant reads a risk's closed-form ALE, the
  Monte-Carlo simulation needs ✓. The other depths in the table are not yet
  told apart by the backend; the list of those gaps is kept, and checked
  against the code, in
  [`pricing_enforcement_test.go`](../backend/pkg/entitlements/pricing_enforcement_test.go).
- The **frontend never grants**. It greys the feature and explains the value with
  an upgrade CTA (`FeatureGate` / `UpsellLock`). A wall that explains converts; a
  wall that hides frustrates.
- Payment runs through **Stripe** (cards) or **Notchpay / CinetPay** (MTN MoMo,
  Orange Money, Wave). With no gateway configured, paid plans still start as a
  trial and Enterprise is sales-invoiced — nothing is faked.
