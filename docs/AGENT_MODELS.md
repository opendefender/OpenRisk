# Agent models

Agents live in `.claude/agents/`, one file each. The frontmatter `model:` decides which
model an agent runs on; nothing in `settings.json` overrides it.

Only the aliases `opus`, `sonnet` and `haiku` are used. They follow the latest
model of each tier, so they never go stale the way a dated id like
`claude-opus-5` did (#960).

## Which model, and why

The rule: an agent that writes or reviews code, security-relevant config,
compliance content or a spec that code is built from runs on `opus`. A
mistake there costs a fix PR, a regression or a false claim. An agent that
writes prose a reviewer checks anyway runs on `sonnet`. Mechanical triage
runs on `haiku`.

Mode says what starts an agent:
- **loop** — a step of `/work` on an issue;
- **triggered** — the paths or labels of a change, computed by a script
  (#1005), never by an agent's judgement;
- **scheduled** — a fixed cadence;
- **on demand** — the owner or the orchestrator calls it.

The model column is the frontmatter. `/work` runs the implementers marked
"sonnet on S" on `sonnet` for S issues and moves them to `opus` on their
first BLOCK (#1006); the frontmatter stays `opus`, the safe default.

| Agent | Model | Mode | Trigger | Why this model |
|---|---|---|---|---|
| scout | haiku | loop | first step of M and L issues; writes `brief.md` | reads and locates, decides nothing |
| tech-lead | opus | loop | design gate of M and L issues; arbitration after two blocked rounds | architecture and ADRs |
| qa-automation | opus | loop | failing tests from the criteria before code; live pass when the UI changed | writes the tests that decide what is proven |
| backend-go | opus | loop | `area:backend`; sonnet on S | writes production Go, tenant isolation |
| frontend-react | opus | loop | `area:frontend`; sonnet on S | writes production React |
| devops-sre | opus | loop | `area:infra` | CI, Docker, Helm |
| cloud-sysadmin | opus | loop | `area:infra` | infrastructure and backups |
| ux-designer | opus | loop | `area:frontend` with no spec; `area:design` | the specs frontend code is built from |
| art-director | opus | loop | `area:design` | design system decisions |
| motion-designer | opus | loop | `area:design` | implements animation code |
| brand-strategist | opus | loop | `area:marketing` | positioning |
| copywriter | sonnet | loop | `area:marketing` | copy, reviewed by product-verifier |
| code-reviewer | opus | triggered | every code PR | reviews every diff |
| devsecops | opus | triggered | every backend or infra PR; frontend PRs touching auth, sessions or uploads | security review |
| i18n-steward | sonnet | triggered | every change to visible text | FR/EN key parity |
| product-verifier | opus | triggered | README, ROADMAP, `docs/`, release notes, visible wording that describes a capability | blocks false claims |
| compliance-officer | opus | triggered | framework catalogues, cross-framework mappings, evidence and audit exports | regulatory control text |
| performance-engineer | opus | scheduled | weekly | changes queries and bundles |
| po-openrisk | opus | scheduled | daily `/brief`, weekly `/report`; on demand to refine an issue | backlog, specs, owner reporting |
| issue-triage | haiku | scheduled | `/triage`, weekly or when new issues arrive | labels and duplicates |
| market-na | opus | on demand | Canada and USA regulation or go-to-market question | regulatory research that seeds control work |
| legal-counsel | opus | on demand | new dependency, release, legal document | licences and legal documents |
| release-manager | sonnet | on demand | milestone close, `/ship` | changelog and release notes |
| website-dev | opus | on demand | `/website` | writes the marketing site's code |
| seo-growth | sonnet | on demand | site structure or content plan | site structure and content plans |
| technical-writer | sonnet | on demand | a feature ships or an API changes | documentation |
| support-engineer | sonnet | on demand | a recurring user problem | guides and FAQ |
| data-analyst | sonnet | on demand | a product decision that needs evidence | metric definitions |

Changing a row means changing the agent's frontmatter and this table in the
same commit.
