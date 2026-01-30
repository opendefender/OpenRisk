 OpenRisk — Roadmap & TODO

Date: --

Ce fichier centralise la todo-list dcrite en session. Il regroupe les fonctionnalits par domaines, indique l'tat actuel ( = complt,  = à faire) et propose une priorisation initiale.


- Les lments marqus  sont djà implments ou partiellement implments dans cette branche.
- Les lments marqus  sont à planifier/implmenter.


 Priorits immdiates (Top )


-  . Implement Risk CRUD API (backend handlers + validation)
	 -  Subtasks for Implement Risk CRUD API:
		 - schema & migrations
		 - DB models + associations
		 - REST handlers (list/get/create/update/delete)
		 - validation (request DTOs)
		 - unit tests (handlers + services)
		 - integration/API tests (end-to-end)
		 - OpenAPI contract / docs





 Strategic Initiatives (long-term)

-  . Strategic: API-First — Full API coverage & CI/CD integration

	- Purpose: Make OpenRisk fully scriptable and automatable via API; enable DevSecOps integration (GitHub Actions, GitLab CI) so the platform can block or annotate risky deployments.

-  . Strategic: Contextualization — correlate threat intel with internal risk

	- Purpose: Reduce false positives by combining external threat indicators (CVE, actors, TTPs) with internal context (asset exposure, data sensitivity, business impact).

-  . Strategic: CTEM Integration — external threat → internal asset mapping

	- Purpose: Implement Continuous Threat Exposure Management: map external threat events to owned assets and compute exposure.

-  . Strategic: Reporting for C-Level — automated PDF/HTML reports

	- Purpose: Executive-ready reports that show risk posture trends and business impact (downloadable PDF/HTML).

-  . Strategic: Templates — Default compliance templates (ISO, SOC, PCI-DSS)

	- Purpose: Provide out-of-the-box templates and mappings to accelerate audits and adoption.

-  . Strategic: False-Positive Reduction — enrichment & context rules

	- Purpose: Enrichment pipelines, heuristics, and ML/IA assists to prioritize true positives and suppress noise.

- Écrire un README exhaustif : incluez screenshots, un quickstart ( min setup), et un contributeur guide.





 Integrations & Ecosystem

-  . Integrations: Ready-made connectors for popular tools (SIEM, SOAR, ticketing)

	- Purpose: Provide out-of-the-box connectors and templates for SIEMs (Splunk, Elastic), SOARs (TheHive, Cortex, Demisto), ticketing (Jira, ServiceNow), and cloud providers (AWS Security Hub, Azure Sentinel).
	- Subtasks:
		- PoC connector for Splunk (events & correlation)
		- PoC connector for Elastic (ingest + query)
		- SOAR playbooks & webhook templates (TheHive/Cortex)
		- Ticketing integration templates (Jira, ServiceNow)





 UI/UX Excellence
-  . UI/UX: World-class modern UI/UX — design system & onboarding flows
	- Purpose: Build the most beautiful and simple UX in risk management: fast onboarding, accessible, performant, and delightfully simple.
	- Subtasks:
		- Create OpenDefender Design System (tokens, Tailwind config, components)
		- Onboarding flows & product tours (first-time user experience)
		- Accessibility & performance audits (ay, Lighthouse)
		- UX research: run tests with real analysts, gather feedback





 Community & Adoption
-  . Community & Adoption: Make OpenRisk a global community success
	- Purpose: Drive adoption, contributions, and make OpenRisk indispensable in cybersecurity.
	- Subtasks:
		- Comprehensive docs & quickstarts (multi-language)
		- Contribution guide + CODE_OF_CONDUCT + maintainers playbook
		- Dedicated community manager / onboarding plan
		- Outreach: webinars, conference talks, partnerships
		- Marketplace for connectors & templates
		- GitHub Actions & CI/CD integration examples (auto-create risk on PR, block deploys)
		- Translation/localization support
		- Enterprise adoption kit (SLA, deployment guide, support options)



 . Risk Register (coeur du produit)
-  Typeahead keyboard nav
-  Create Risks list page
-  Wire Risks page into router
-  Type-check frontend
-  Add server-side sorting
-  Wire Risks Edit button
-  . Add tests for Risks pagination
-  . Design Risk schema
-  . Implement Risk CRUD API
-  . Implement Risk CRUD frontend
-  . Add score calculation engine
 -  . Add frameworks classification (ISO, CIS, NIST, OWASP)
	 -  Subtasks for Add frameworks classification:
		 -  model schema (fields, types, relations)
		 -  DB migration plan
		 -  API handlers (create/update/assign/complete)
		 -  unit & integration tests
		 -  OpenAPI contract
 -  . Design Mitigation model & API
	 -  Subtasks for Design Mitigation model & API:
		 -  model schema (actions, sub-actions, checklists)
		 -  DB migration plan
		 -  API handlers (create/update/assign/complete)
		 -  unit & integration tests
		 -  OpenAPI contract
-  . Mitigation frontend UI (plans, cards, list)
	 -  Subtasks for Mitigation frontend UI:
		 - plan card UI + progress bar
		 - creation / edit forms
		 - assignment UI (users, deadlines)
		 - checklist & sub-actions UI
		 - tests (RTL)
-  . Sub-actions & checklists (sous-actions)
-  . Assign users & deadlines
-  . Mitigation progress bar
-  . Internal notifications system (rappels, alertes)
-  . Gamification states & UI (progress, levels, badges)





 . Dashboard moderne & dynamique

-  . Dashboard widgets framework (drag & drop)
-  . Charts & animated cards (Recharts, Framer Motion)
-  . Global security score widget
-  . Trends (// days)
-  UI Kit + composant Atom/Molecule/Organism
-  Standardisation animations & theme
-  Composants rutilisables dans toute la suite





 . Backend & API

-  . Unified API endpoints (risks, mitigations, assets, stats)
	 -  Subtasks for Unified API endpoints:
		 - API design & OpenAPI spec
		 - versioning strategy
		 - auth & RBAC checks
		 - unit & API tests


-  . Integrations: TheHive / OpenCTI / Cortex / OpenRMF
	 -  Subtasks for Integrations:
		 - PoC connector for each integration (prototype)
		 - mapping design (fields/events)
		 - reliable sync logic (idempotency)
		 - integration tests / mocks
		 - production hardening (retries, backoff, metrics)
-  . Implement sync-engine (workers)
	 -  Subtasks for Implement sync-engine:
		 - PoC worker that pulls from one integration
		 - queue design (in-memory / Redis)
		 - error handling & retries
		 - monitoring & metrics
		 - tests (unit + integration)
-  . Orchestration & cron jobs
-  . Unit & API tests (backend coverage)
	 -  Subtasks for Unit & API tests:
		 - testing strategy (tools + coverage targets)
		 - unit test suites for core services
		 - API/integration tests (docker-compose + test DB)
		 - CI integration (run tests in GitHub Actions)


 . Infrastructure & CI/CD

-  . Dockerfile optimiss & healthchecks
	 -  Subtasks for Dockerfile optimiss & healthchecks:
		 - multi-stage Dockerfiles (backend/frontend)
		 - healthcheck endpoints
		 - security best practices (non-root, minimal image)
		 - local dev compose with env examples


-  . Helm chart & ks manifests
	 -  Subtasks for Helm chart & ks manifests:
		 - helm chart scaffold
		 - values schema & secrets handling
		 - manifests for deployments, svc, ingress
		 - health/readiness probes
		 - docs for deployment


-  . CI/CD GitHub Actions (build/test/release)
	 -  Subtasks for CI/CD GitHub Actions:
		 - workflow: lint -> build -> test -> release
		 - caching & artifact strategy
		 - auto-release to GHCR/Docker Hub
		 - security scanning (dependabot / Snyk)



 . Documentation & install

-  . INSTALLATION.md
-  . INTEGRATION_GUIDE.md
-  . API_REFERENCE.md
-  . Create deploy.sh installer
	 -  Subtasks for docs & installer:
		 - draft installation steps (dev & prod)
		 - integration guide for external systems
		 - generate API reference from OpenAPI
		 - create deploy.sh with checks and rollback hints



 . Scalabilit, scurit et product features avances

-  . RBAC & multi-tenant support
	 -  Subtasks for RBAC & multi-tenant support:
  	 - tenant model & data isolation plan
		 - RBAC roles & policies
		 - API enforcement & middleware
		 - tests for tenant isolation
-  . IA Risk Advisor PoC (gnration + recommandations)
-  . IA deduplication PoC
-  . IA prioritization PoC
-  . Risk timeline UI (zoomable / events)
-  . Playbooks & automations (no-code flows)
-  . OpenDefender native integrations (OpenAsset / OpenSec...)
-  . Reports: PDF / HTML / JSON export





 . Qualit & UX


-  . Accessibility & ay polish
-  . UX polish & theme/dark mode





 . Ajouter un "OpenDefender Design System"

> couleurs
> spacing
> composants tailwind rutilisables
> typography scale
> badges
> alerts
> cards
> states (loading/error/empty/success)
> animations standardises

-  OpenDefender UI Kit (frontend library)
-  Standardisation des composants (atoms/molecules/organisms)



 . Ajouter les vnements (Webhooks + EventBus)

 OpenRisk doit envoyer :

> risk.created
> risk.updated
> risk.mitigated
> risk.deleted
> asset.linked
> mitigation.progress

-  EventBus interne (Redis / NATS / Kafka)
-  Webhooks configuration UI
-  Retry logic
-  Signature HMAC des webhooks



 Ajouter un module Notifications (email + Slack + webhook)

> Trs important pour :
> deadlines
> risques critiques
> nouvelles vulnrabilits
> actions assignes

-  Notification service (backend)
-  Notification rules engine
-  Templates email
-  Slack & Teams support
-  UI de configuration



 Ajouter l'Export Pro (PDF / HTML / JSON)

> rapport des risques
> rapport mitigation
> tableau complet heatmap


-  Service de gnration PDF
-  Modle “Executive Summary”
-  Export HTML interactif
-  Export JSON via API


 Ajouter un vrai systme de tags & taxonomies

-  Taxonomie centrale OpenDefender : ISO, CIS Controls, NIST -, MITRE ATT&CK, OWASP Top 
-  Mapping automatique (IA suggre plus tard)
  

 Ajouter un module “Risk Templates”

> Rutilisables lors de la cration d’un risque.

> Exemples : “Risque intrusion externe”, “Risque donnes sensibles exposes”, “Risque CVE critique non patche”, “Risque configuration cloud non conforme”


À ajouter :

-  templates backend
-  mapping automatique metadata
-  UI de gestion des templates


 Ajouter un SLA / SLO pour la mitigation

> Trs utile pour les quipes :
> Critique → SLA  jours
> High → SLA  jours
> Medium →  jours
> Low →  jours


À ajouter :

-  SLA module
-  badges SLA respects / dpasss
-  graphes SLA

---


 Risk Timeline avance

Djà dans ta roadmap, mais il faut la dtailler :

À ajouter :


-  Zoom / Pan
-  Évnements cls (changement probabilit/impact)
-  État avant/aprs mitigation
-  Snapshots historiques





 Risk Matrix Designer

> Donner à l’utilisateur la possibilit de :
> dfinir sa propre matrice
> changer le nombre de niveaux
> personnaliser la couleur
> adapter aux ralits locales


 Risk Comments / Discussion Thread


> Comme GitHub issues mais pour les risques :
> commentaires
> mentions @user
> pices jointes
> historique complet





 Gestion des Assets enrichie (mini-CMDB)


> OpenRisk doit afficher :

> asset
> criticit
> propritaire
> type
> statut
> localisation
Cela renforce les calculs de risques.


 Playbooks Automations (inspir de Zapier)

> Exemples :

> “Si CVE >  → crer un risque critique”
> “Si action en retard → envoyer email responsable”



 Mode auditor (lecture seule avance)

> Pour les audits externes (ISO, SOC, RGPD).



 Marketplace (futur)

 Place pour modules externes


 PoC requirement for backend-critical tasks


- Pour toutes les tâches backend critiques (ex: Integrations, sync-engine, RBAC & multi-tenant), ajouter une phase PoC (prototype) avant d'industrialiser. La phase PoC doit produire :
	- un prototype minimal fonctionnel
	- tests de non-rgression minimaux
	- mtriques/observabilit de base (logs, erreurs)
	- un document court (README) listant les risques et besoins pour production
  


 Priorit tests

- Prioriser les tests automatiss (unit + API) : chaque feature backend majeure doit être accompagne de tests unitaires et d'au moins un test d'intgration API. Intgrer ces tests dans CI avant les releases.