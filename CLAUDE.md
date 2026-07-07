# OpenRisk — Contexte permanent Claude Code
 
## Projet
OpenRisk est une plateforme GRC enterprise open-source (https://github.com/opendefender/OpenRisk).
Partie de l'écosystème OpenDefender. Marché cible : France, Belgique, Maghreb, Afrique subsaharienne.
 
## Stack technique
- Backend  : Go 1.25 · Fiber v2 · GORM · PostgreSQL 16 · Redis · golang-migrate
- Frontend : React 19 · TypeScript strict · Zustand · Tailwind CSS 3 · Recharts · React Router v7
- Infra    : Docker multi-stage · Kubernetes + Helm · GitHub Actions CI/CD
- Obs.     : Prometheus · Grafana · Loki · zerolog JSON · Sentry
## Architecture obligatoire
Backend — Clean Architecture stricte :
  /cmd/server/main.go         → DI container, graceful shutdown
  /internal/domain/           → entités pures (pas de Fiber, pas de GORM ici)
  /internal/application/      → use cases (1 use case = 1 fichier)
  /internal/infrastructure/   → repositories, messaging, integrations
  /internal/api/http/         → handlers Fiber + middleware
  /pkg/                       → packages partagés (scoring, cti, notify, export, ai, crq)
 
Frontend — Feature-based :
  /src/features/[module]/     → pages, components, hooks, stores par feature
  /src/shared/                → design system, hooks globaux, utils
  /src/services/              → client API typé (généré depuis OpenAPI)
  /src/locales/               → fr.json, en.json
 
## Règles ABSOLUES — jamais violer
1. Lire TOUS les fichiers existants d'un module avant d'écrire une seule ligne
2. Filtrer par tenant_id sur CHAQUE query DB — aucune exception
3. Erreurs typées uniquement : ErrNotFound, ErrForbidden, ErrConflict, ErrValidation
4. Tests minimum par use case : TestXxx_Success + TestXxx_NotFound + TestXxx_Unauthorized
5. Zero `any` TypeScript — tout est typé strictement
6. Jamais de secrets dans les logs (tokens, passwords, clés)
7. Transactions DB sur toute opération multi-table
8. Skeleton loaders côté frontend — jamais de spinner pleine page
9. Toujours gérer les 3 états UI : loading + error + empty
10. Optimistic updates sur toutes les mutations critiques (UX perçue)
11. Zod validation côté client sur tous les formulaires
## Formule Score Engine
Score = Probability (0.0–1.0) × Impact (0.0–10.0) × AssetCriticality (0.1–3.0)
Criticality : score ≥ 7.0 = critical · ≥ 4.0 = high · ≥ 2.0 = medium · < 2.0 = low
 
## Ce qui est déjà implémenté
Détail module par module + preuves : voir `ROADMAP.md` (source de vérité unique, mise à jour à chaque fin de module). Résumé :
- **Solide (✅)** : Auth (JWT RS256, Argon2id, MFA, OAuth2/SAML2), RBAC multi-tenant, Risk Register, Mitigation, Score Engine, Notifications, Audit logging, Dashboard/Analytics, i18n FR/EN.
- **Partiel (🟡)** : Compliance Frameworks (migration + domaine + repo faits et sains, aucun use case/handler/UI), Assets, Incident Management (table manquante d'`AutoMigrate`), Custom Fields, Marketplace, PAM Audit Trail, CTI Engine (non câblé, en avance de phase Wave 2).
- **Non commencé (❌)** : contenu réglementaire africain (COBAC/BCEAO/ANSSI), Offline-first, Billing/Stripe, différenciateurs Wave 2/3.

## Priorités du sprint en cours
1. ~~Corriger le build frontend~~ ✅ fait le 07/07/2026 (branche `fix/frontend-build-typescript-errors`, commit `4e5b91f5`).
2. ~~Corriger le bug cross-tenant `GormComplianceRepository.UpdateControl`~~ ✅ fait le 07/07/2026 (branche `fix/compliance-cross-tenant-update-isolation`, commit `5a0407fc`).
3. Prochaine étape : M1 Compliance — use cases + handlers + OpenAPI + frontend par-dessus les fondations existantes (voir `ROADMAP.md` §3, M1).
4. Hors scope immédiat mais à planifier : ~350 findings lint frontend (`no-explicit-any` surtout) et 7 fichiers de tests frontend en échec, tous deux pré-existants et découverts en corrigeant le build.

