# OpenRisk — Roadmap & TODO

Date: 2025-12-02

Ce fichier centralise la roadmap et la todo-priorisée du projet. J'ai restructuré le document pour faciliter la lecture :
- Section 1 : Priorités immédiates (à livrer)
- Section 2 : Initiatives stratégiques (long-terme)
- Section 3 : Plateforme & intégrations
- Section 4 : UI / Expérience produit
- Section 5 : Communauté & adoption
- Section 6 : Documentation, tests et CI

**Règles rapides**
- ✅ = implémenté / partiellement livré
- ⬜ = à planifier / implémenter

---

## Priorités immédiates
- ✅ Implement Risk CRUD API (backend handlers + validation)
- ✅ Implement Risk CRUD frontend (forms, modals, store)
- ✅ Add score calculation engine (backend + docs)
- ✅ Implement frameworks classification support (backend + frontend)
- ✅ Implement Mitigation frontend UI (edit modal + update endpoint)
- In progress: Add mitigation sub-actions (migration + domain model + handlers + routes)
- ⬜ Run frontend tests (optional)
- ⬜ Tests: Prioritize unit & API tests and CI integration
- ⬜ Docs: INSTALLATION.md, INTEGRATION_GUIDE.md, API_REFERENCE.md

---

## Initiatives Stratégiques (long-terme)
- ⬜ API-First — Full API coverage & CI/CD integration
  - Objectif: rendre OpenRisk pilotable entièrement par API (exemples: GitHub Actions, GitLab CI), enable automations et gating de déploiements risqués.
- ⬜ Contextualization — corréler threat intel avec le risque interne
  - Objectif: réduire les faux-positifs en combinant CVE/TTP/actor avec l'exposition des assets et la sensibilité des données.
- ⬜ CTEM Integration — Continuous Threat Exposure Management
  - Objectif: mapper événements externes → assets internes et calculer l'exposition continue.
- ⬜ Reporting for C-Level — rapports PDF/HTML automatisés
  - Objectif: rapports exécutifs montrant l'évolution du posture sécurité.
- ⬜ Templates — configurations par défaut pour audits (ISO27001, SOC2, PCI-DSS)
  - Objectif: faciliter l'adoption en proposant templates prêts à l'emploi.
- ⬜ False-Positive Reduction — enrichment & rules/IA

---

## Plateforme & Intégrations
- ⬜ Integrations: Ready-made connectors for popular tools (SIEM, SOAR, ticketing, cloud)
  - PoC priorities: Splunk, Elastic, TheHive/Cortex, Jira, ServiceNow, AWS Security Hub, Azure Sentinel.
- ⬜ Implement sync-engine (workers) and robust connectors (idempotency, retries)
- ⬜ EventBus & Webhooks (risk.created, risk.updated, mitigation.progress, etc.)

---

## UI / Expérience produit
- ⬜ UI/UX: World-class modern UI/UX — design system & onboarding flows
  - Créer `OpenDefender Design System` (tokens, Tailwind config, composants réutilisables)
  - Onboarding & product tours, audits d'accessibilité, tests utilisateurs.
- ⬜ Dashboard: widgets, trends, global security score

---

## Communauté & Adoption
- ⬜ Community & Adoption: Make OpenRisk a global community success
  - Docs exhaustifs + quickstarts, contribution guide, CODE_OF_CONDUCT
  - Marketplace connectors & templates, outreach (webinars, talks), traduction/localisation
  - Enterprise adoption kit (SLA, deployment guide, support options)

---

## Documentation, tests & CI
- ⬜ Prioriser tests unitaires & API; intégrer dans CI (GitHub Actions)
- ⬜ Créer `INSTALLATION.md`, `INTEGRATION_GUIDE.md`, `API_REFERENCE.md`
- ⬜ Examples: GitHub Actions + OpenRisk workflows (auto-create risk on PR, block deploys)

---

## Notes & bonnes pratiques
- Pour toute feature backend critique : commencer par un PoC (prototype) puis industrialiser.
- Maintenir la liste priorisée et petite (3–5 priorités actives).

---

## Détails numérotés (toutes les tâches & sous-tâches)

1.0 Priorités immédiates — Livraison MVP Risques & Mitigations
  1.1 Implement Risk CRUD API
    1.1.1 DTOs & Validation (Create/Update)
    1.1.2 DB Models & Migrations (Risk, Asset, Mitigation)
    1.1.3 REST Handlers (list/get/create/update/delete)
    1.1.4 OpenAPI contract & docs
  1.2 Implement Risk CRUD frontend
    1.2.1 Create/Edit forms + validation (Zod + react-hook-form)
    1.2.2 Risk list, details, optimistic store updates
    1.2.3 Edit/update flow + error handling
  1.3 Score Calculation Engine
    1.3.1 Formula definition & unit tests
    1.3.2 Hook into create/update flows
    1.3.3 Frontend display & recalculation UI
  1.4 Frameworks classification support
    1.4.1 Model + migrations
    1.4.2 API surface
    1.4.3 Frontend selectors & filters
  1.5 Mitigation model & API
    1.5.1 Mitigation CRUD + Update endpoint
    1.5.2 Mitigation sub-actions (checklist) - migration + model
    1.5.3 Sub-actions handlers (create/toggle/delete)
  1.6 Mitigation frontend UI
    1.6.1 MitigationEditModal: edit fields + checklist management
    1.6.2 Mitigation list & prioritized view
  1.7 Tests & CI integration (MVP scope)
    1.7.1 Backend unit tests (score, handlers)
    1.7.2 Frontend unit/RTL tests (stores, components)

2.0 Initiatives Stratégiques — roadmap long-terme
  2.1 API-First Strategy
    2.1.1 Complete API coverage (all features)
    2.1.2 API examples & CI/CD playbooks (GitHub Actions)
    2.1.3 Webhooks & event-driven endpoints
  2.2 Contextualization & Enrichment
    2.2.1 Threat intel ingestion (OpenCTI, feeds)
    2.2.2 Asset exposure mapping & enrichment pipelines
    2.2.3 Context rules engine (reduce false positives)
    2.2.4 ML/IA assistant (suggestions + deduplication)
  2.3 CTEM Integration (Continuous Threat Exposure Management)
    2.3.1 Map external events → internal assets (PoC)
    2.3.2 Exposure scoring & alerting
    2.3.3 Automated mitigations suggestions
  2.4 Reporting & Executive Summaries
    2.4.1 PDF/HTML report generator (Executive Summary)
    2.4.2 Historical trends & KPI dashboards
  2.5 Templates & Compliance
    2.5.1 Default templates & mappings (ISO27001, SOC2, PCI-DSS)
    2.5.2 Template UI & import/export

3.0 Platform integrations & connectors
  3.1 SIEM & Log sources
    3.1.1 Splunk connector PoC (ingest events)
    3.1.2 Elastic connector PoC (ingest & query)
    3.1.3 Azure Sentinel connector PoC
  3.2 SOAR & Case management
    3.2.1 TheHive adapter & playbooks
    3.2.2 Cortex playbooks integration
    3.2.3 Cortex/C3 response orchestration
  3.3 Ticketing & ITSM
    3.3.1 Jira integration templates (issues & transitions)
    3.3.2 ServiceNow integration
  3.4 Cloud security feeds
    3.4.1 AWS Security Hub / GuardDuty PoC
    3.4.2 Azure Security Center PoC

4.0 UX & Product Excellence
  4.1 OpenDefender Design System
    4.1.1 Design tokens, Tailwind config, color system
    4.1.2 Base components (Button, Input, Modal, Table)
    4.1.3 Documentation & Storybook
  4.2 Onboarding & Product Tours
    4.2.1 First-time user flows, product tours
    4.2.2 In-app tips & contextual help
  4.3 Accessibility & Performance
    4.3.1 A11y audits, fixes
    4.3.2 Lighthouse performance improvements

5.0 Community & Adoption
  5.1 Documentation & Quickstarts
    5.1.1 INSTALLATION.md (dev & prod)
    5.1.2 INTEGRATION_GUIDE.md (connectors)
    5.1.3 API_REFERENCE.md (OpenAPI-generated)
  5.2 Contribution & Governance
    5.2.1 Contribution guide, CODE_OF_CONDUCT, MAINTAINERS
    5.2.2 Contributor onboarding & reviews
  5.3 Outreach & Growth
    5.3.1 Webinars, conference talks, blog posts
    5.3.2 Partnerships with security vendors
  5.4 Marketplace & Ecosystem
    5.4.1 Host connectors, templates, playbooks
    5.4.2 Create publisher onboarding for 3rd party modules

6.0 Infra, CI/CD & Reliability
  6.1 Dockerfile optimizations & healthchecks
  6.2 Helm chart & Kubernetes manifests
  6.3 CI: GitHub Actions (lint → test → build → release)

7.0 Observability & Security
  7.1 Monitoring & metrics (Prometheus / Grafana)
  7.2 Security hardening (dependency scanning, SCA)

8.0 Playbooks & Automations (no-code flows)
  8.1 Example automation: CVE>9 => create critical risk
  8.2 Playbook editor & triggers

9.0 Roadmap governance & milestone planning
  9.1 Quarterly milestones & release cadence
  9.2 PoC requirements for backend-critical tasks
