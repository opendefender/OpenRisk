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

| Agent | Model | Why |
|---|---|---|
| backend-go | opus | writes production Go, tenant isolation |
| frontend-react | opus | writes production React |
| qa-automation | opus | writes the tests that decide what is proven |
| performance-engineer | opus | changes queries and bundles |
| devops-sre | opus | CI, Docker, Helm |
| cloud-sysadmin | opus | infrastructure and backups |
| website-dev | opus | writes the marketing site's code |
| motion-designer | opus | implements animation code |
| tech-lead | opus | reviews every diff |
| devsecops | opus | security review |
| product-verifier | opus | blocks false claims |
| compliance-officer | opus | regulatory control text |
| legal-counsel | opus | licences and legal documents |
| ux-designer | opus | the specs frontend code is built from |
| art-director | opus | design system decisions |
| po-openrisk | opus | backlog, specs, owner reporting |
| brand-strategist | opus | positioning |
| copywriter | sonnet | copy, reviewed by product-verifier |
| seo-growth | sonnet | site structure and content plans |
| technical-writer | sonnet | documentation |
| support-engineer | sonnet | guides and FAQ |
| i18n-steward | sonnet | FR/EN key parity |
| data-analyst | sonnet | metric definitions |
| release-manager | sonnet | changelog and release notes |
| issue-triage | haiku | labels and duplicates |

Changing a row means changing the agent's frontmatter and this table in the
same commit.
