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
