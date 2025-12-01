# OpenRisk — Roadmap & TODO

Date: 2025-12-01

Ce fichier centralise la todo-list décrite en session. Il regroupe les fonctionnalités par domaines, indique l'état actuel (✅ = complété, ⬜ = à faire) et propose une priorisation initiale.

**Règles rapides**
- Les éléments marqués ✅ sont déjà implémentés ou partiellement implémentés dans cette branche.
- Les éléments marqués ⬜ sont à planifier/implémenter.

---

## Priorités immédiates (Top 5)
- ⬜ 7. Add tests for `Risks` pagination (tests unitaires + RTL)
- ⬜ 8. Design Risk schema (domain model: Risk, Score, Fields)
  
	 - ⬜ Subtasks for `Design Risk schema`:
		 - schema definition (fields, types, relations)
		 - DB migration plan
		 - domain models (backend)
		 - TypeScript interfaces (frontend)
		 - sample fixtures & seeds
- ⬜ 9. Implement Risk CRUD API (backend handlers + validation)
  
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
- ⬜ 7. Add tests for Risks pagination
- ⬜ 8. Design Risk schema
- ⬜ 9. Implement Risk CRUD API
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

---

## PoC requirement for backend-critical tasks
- Pour toutes les tâches backend critiques (ex: `Integrations`, `sync-engine`, `RBAC & multi-tenant`), ajouter une phase PoC (prototype) avant d'industrialiser. La phase PoC doit produire :
	- un prototype minimal fonctionnel
	- tests de non-régression minimaux
	- métriques/observabilité de base (logs, erreurs)
	- un document court (README) listant les risques et besoins pour production

## Priorité tests
- Prioriser les tests automatisés (unit + API) : chaque feature backend majeure doit être accompagnée de tests unitaires et d'au moins un test d'intégration API. Intégrer ces tests dans CI avant les releases.

---

## Notes & recommandations
- Je recommande de découper chaque gros item (par ex. `Implement Risk CRUD API`) en sous-tâches : schema, migrations, handlers, validation, tests, openapi contract.
- Pour les tâches backend critiques (sync, integrations), ajouter une phase PoC (prototype) avant d'industrialiser.
- Prioriser tests automatisés (unit + API) avant d'ajouter des features majeures.

---

Fichier généré automatiquement par l'assistant. Pour committer :

```bash
git add TODO.md
git commit -m "Add project TODO roadmap"
```

---

Si vous voulez, je peux :
- Prioriser cette liste (réduire à 10 items) ;
- Créer des issues GitHub à partir de ces tâches ;
- Commencer l'implémentation du premier item (`Add tests for Risks pagination`).

Dites-moi la prochaine action souhaitée.
