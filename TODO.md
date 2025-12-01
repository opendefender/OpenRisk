# OpenRisk — Roadmap & TODO

Date: 2025-12-01

Ce fichier centralise la todo-list décrite en session. Il regroupe les fonctionnalités par domaines, indique l'état actuel (✅ = complété, ⬜ = à faire) et propose une priorisation initiale.

**Règles rapides**
- Les éléments marqués ✅ sont déjà implémentés ou partiellement implémentés dans cette branche.
- Les éléments marqués ⬜ sont à planifier/implémenter.

---

## Priorités immédiates (Top 5)
- ✅ 7. Add tests for `Risks` pagination (tests unitaires + RTL)
- ✅ 8. Design Risk schema (domain model: Risk, Score, Fields)
  
	 - ✅ Subtasks for `Design Risk schema`:
		 - schema definition (fields, types, relations)
		 - DB migration plan
		 - domain models (backend)
		 - TypeScript interfaces (frontend)
		 - sample fixtures & seeds
- ✅ 9. Implement Risk CRUD API (backend handlers + validation)
  
	 - ⬜ Subtasks for `Implement Risk CRUD API`:
		 - schema & migrations
		 - DB models + associations
		 - REST handlers (list/get/create/update/delete)
		 - validation (request DTOs)
		 - unit tests (handlers + services)
		 - integration/API tests (end-to-end)
		 - OpenAPI contract / docs
- ⬜ 10. Implement Risk CRUD frontend (forms, modals, store)
  
	 - ⬜ Subtasks for `Implement Risk CRUD frontend`:
		 - forms (create / edit) with validation
		 - modals & drawers (UX)
		 - store actions (create/update/delete)
		 - list integration (refresh + optimistic updates)
		 - unit tests + RTL tests
- ⬜ 11. Add score calculation engine (probability × impact × criticité asset)
  
	 - ⬜ Subtasks for `Add score calculation engine`:
		 - define formula & edge cases
		 - backend calculation service (unit tested)
		 - hook into create/update flows
		 - frontend display & recalculation UI
		 - tests & fixtures

---

## 1. Risk Register (coeur du produit)
- ✅ Typeahead keyboard nav
- ✅ Create Risks list page
- ✅ Wire Risks page into router
- ✅ Type-check frontend
- ✅ Add server-side sorting
- ✅ Wire Risks Edit button
- ✅ 7. Add tests for Risks pagination
- ✅ 8. Design Risk schema
- ✅ 9. Implement Risk CRUD API
- ⬜ 10. Implement Risk CRUD frontend
- ⬜ 11. Add score calculation engine
- ⬜ 12. Add frameworks classification (ISO27001, CIS, NIST, OWASP)
- ⬜ 13. Advanced multi-criteria filters
- ⬜ 14. Instant search & typeahead (UX + backend tuning)
- ⬜ 15. Support custom fields
- ⬜ 16. Heatmap dynamic visualization (probability × impact)
- ⬜ 17. Sortable, taggable list UI

## 2. Plans d’atténuation & actions (Mitigations)
- ⬜ 18. Design Mitigation model & API
  
	 - ⬜ Subtasks for `Design Mitigation model & API`:
		 - model schema (actions, sub-actions, checklists)
		 - DB migration plan
		 - API handlers (create/update/assign/complete)
		 - unit & integration tests
		 - OpenAPI contract
- ⬜ 19. Mitigation frontend UI (plans, cards, list)
  
	 - ⬜ Subtasks for `Mitigation frontend UI`:
		 - plan card UI + progress bar
		 - creation / edit forms
		 - assignment UI (users, deadlines)
		 - checklist & sub-actions UI
		 - tests (RTL)
- ⬜ 20. Sub-actions & checklists (sous-actions)
- ⬜ 21. Assign users & deadlines
- ⬜ 22. Mitigation progress bar
- ⬜ 23. Internal notifications system (rappels, alertes)
- ⬜ 24. Gamification states & UI (progress, levels, badges)

## 3. Dashboard moderne & dynamique
- ⬜ 25. Dashboard widgets framework (drag & drop)
- ⬜ 26. Charts & animated cards (Recharts, Framer Motion)
- ⬜ 27. Global security score widget
- ⬜ 28. Trends (30/60/90 days)
- ⬜ UI Kit + composant Atom/Molecule/Organism
- ⬜ Standardisation animations & theme
- ⬜ Composants réutilisables dans toute la suite

## 4. Backend & API
- ⬜ 29. Unified API endpoints (risks, mitigations, assets, stats)
  
	 - ⬜ Subtasks for `Unified API endpoints`:
		 - API design & OpenAPI spec
		 - versioning strategy
		 - auth & RBAC checks
		 - unit & API tests
- ⬜ 30. Integrations: TheHive / OpenCTI / Cortex / OpenRMF
  
	 - ⬜ Subtasks for `Integrations`:
		 - PoC connector for each integration (prototype)
		 - mapping design (fields/events)
		 - reliable sync logic (idempotency)
		 - integration tests / mocks
		 - production hardening (retries, backoff, metrics)
- ⬜ 31. Implement sync-engine (workers)
  
	 - ⬜ Subtasks for `Implement sync-engine`:
		 - PoC worker that pulls from one integration
		 - queue design (in-memory / Redis)
		 - error handling & retries
		 - monitoring & metrics
		 - tests (unit + integration)
- ⬜ 32. Orchestration & cron jobs
- ⬜ 36. Unit & API tests (backend coverage)
  
	 - ⬜ Subtasks for `Unit & API tests`:
		 - testing strategy (tools + coverage targets)
		 - unit test suites for core services
		 - API/integration tests (docker-compose + test DB)
		 - CI integration (run tests in GitHub Actions)

## 5. Infrastructure & CI/CD
- ⬜ 33. Dockerfile optimisés & healthchecks
  
	 - ⬜ Subtasks for `Dockerfile optimisés & healthchecks`:
		 - multi-stage Dockerfiles (backend/frontend)
		 - healthcheck endpoints
		 - security best practices (non-root, minimal image)
		 - local dev compose with env examples
- ⬜ 34. Helm chart & k8s manifests
  
	 - ⬜ Subtasks for `Helm chart & k8s manifests`:
		 - helm chart scaffold
		 - values schema & secrets handling
		 - manifests for deployments, svc, ingress
		 - health/readiness probes
		 - docs for deployment
- ⬜ 35. CI/CD GitHub Actions (build/test/release)
  
	 - ⬜ Subtasks for `CI/CD GitHub Actions`:
		 - workflow: lint -> build -> test -> release
		 - caching & artifact strategy
		 - auto-release to GHCR/Docker Hub
		 - security scanning (dependabot / Snyk)

## 6. Documentation & install
- ⬜ 37. `INSTALLATION.md`
- ⬜ 38. `INTEGRATION_GUIDE.md`
- ⬜ 39. `API_REFERENCE.md`
- ⬜ 40. Create `deploy.sh` installer
  
	 - ⬜ Subtasks for docs & installer:
		 - draft installation steps (dev & prod)
		 - integration guide for external systems
		 - generate API reference from OpenAPI
		 - create `deploy.sh` with checks and rollback hints

## 7. Scalabilité, sécurité et product features avancées
- ⬜ 41. RBAC & multi-tenant support
  
	 - ⬜ Subtasks for `RBAC & multi-tenant support`:
		 - tenant model & data isolation plan
		 - RBAC roles & policies
		 - API enforcement & middleware
		 - tests for tenant isolation
- ⬜ 42. IA Risk Advisor PoC (génération + recommandations)
- ⬜ 43. IA deduplication PoC
- ⬜ 44. IA prioritization PoC
- ⬜ 45. Risk timeline UI (zoomable / events)
- ⬜ 46. Playbooks & automations (no-code flows)
- ⬜ 47. OpenDefender native integrations (OpenAsset / OpenSec...)
- ⬜ 48. Reports: PDF / HTML / JSON export

## 8. Qualité & UX
- ⬜ 49. Accessibility & a11y polish
- ⬜ 50. UX polish & theme/dark mode


## 9. Ajouter un "OpenDefender Design System"
> couleurs
> spacing
> composants tailwind réutilisables
> typography scale
> badges
> alerts
> cards
> states (loading/error/empty/success)
> animations standardisées

- ⬜ OpenDefender UI Kit (frontend library)
- ⬜ Standardisation des composants (atoms/molecules/organisms)

## 10. Ajouter les événements (Webhooks + EventBus)
> # OpenRisk doit envoyer :
> risk.created
> risk.updated
> risk.mitigated
> risk.deleted
> asset.linked
> mitigation.progress

- ⬜ EventBus interne (Redis / NATS / Kafka)
- ⬜ Webhooks configuration UI
- ⬜ Retry logic
- ⬜ Signature HMAC des webhooks

## Ajouter un module Notifications (email + Slack + webhook)
> Très important pour :
> deadlines
> risques critiques
> nouvelles vulnérabilités
> actions assignées

- ⬜ Notification service (backend)
- ⬜ Notification rules engine
- ⬜ Templates email
- ⬜ Slack & Teams support
- ⬜ UI de configuration

## Ajouter l'Export Pro (PDF / HTML / JSON)
> rapport des risques
> rapport mitigation
> tableau complet heatmap

- ⬜ Service de génération PDF
- ⬜ Modèle “Executive Summary”
- ⬜ Export HTML interactif
- ⬜ Export JSON via API

## Ajouter un vrai système de tags & taxonomies

- ⬜ Taxonomie centrale OpenDefender : ISO27001, CIS Controls, NIST 800-53, MITRE ATT&CK, OWASP Top 10
- ⬜ Mapping automatique (IA suggérée plus tard)

## Ajouter un module “Risk Templates”
> Réutilisables lors de la création d’un risque.
> Exemples : “Risque intrusion externe”, “Risque données sensibles exposées”, “Risque CVE critique non patchée”, “Risque configuration cloud non conforme”

À ajouter :

- ⬜ templates backend
- ⬜ mapping automatique metadata
- ⬜ UI de gestion des templates


## Ajouter un SLA / SLO pour la mitigation
> Très utile pour les équipes :
> Critique → SLA 7 jours
> High → SLA 14 jours
> Medium → 30 jours
> Low → 90 jours
À ajouter :

- ⬜ SLA module
- ⬜ badges SLA respectés / dépassés
- ⬜ graphes SLA
---

## Risk Timeline avancée
Déjà dans ta roadmap, mais il faut la détailler :
À ajouter :

- ⬜ Zoom / Pan
- ⬜ Événements clés (changement probabilité/impact)
- ⬜ État avant/après mitigation
- ⬜ Snapshots historiques

## Risk Matrix Designer
> Donner à l’utilisateur la possibilité de :
> définir sa propre matrice
> changer le nombre de niveaux
> personnaliser la couleur
> adapter aux réalités locales

## Risk Comments / Discussion Thread
> Comme GitHub issues mais pour les risques :
> commentaires
> mentions @user
> pièces jointes
> historique complet

## Gestion des Assets enrichie (mini-CMDB)
> OpenRisk doit afficher :
> asset
> criticité
> propriétaire
> type
> statut
> localisation
> Cela renforce les calculs de risques.

## Playbooks Automations (inspiré de Zapier)
> Exemples :
> “Si CVE > 9 → créer un risque critique”
> “Si action en retard → envoyer email responsable”

## Mode auditor (lecture seule avancée)
> Pour les audits externes (ISO, SOC2, RGPD).



## Marketplace (futur)

⬜ Place pour modules externes

## PoC requirement for backend-critical tasks
- Pour toutes les tâches backend critiques (ex: `Integrations`, `sync-engine`, `RBAC & multi-tenant`), ajouter une phase PoC (prototype) avant d'industrialiser. La phase PoC doit produire :
	- un prototype minimal fonctionnel
	- tests de non-régression minimaux
	- métriques/observabilité de base (logs, erreurs)
	- un document court (README) listant les risques et besoins pour production

## Priorité tests
- Prioriser les tests automatisés (unit + API) : chaque feature backend majeure doit être accompagnée de tests unitaires et d'au moins un test d'intégration API. Intégrer ces tests dans CI avant les releases.

---

