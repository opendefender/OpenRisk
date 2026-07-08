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
- **Solide (✅)** : Risk Register, Mitigation, Score Engine, Notifications, Audit logging, i18n FR/EN, Dashboard/Analytics (login → dashboard vérifié live le 08/07/2026, plus de déconnexion forcée), **Compliance Frameworks — M1 complet, vérifié live de bout en bout le 08/07/2026** (use cases + handlers + OpenAPI + client généré + frontend + upload de preuve réel + RBAC granulaire, voir `ROADMAP.md` §3 M1), **M2 ISO 27001:2022 — 93 contrôles Annexe A importables en un clic depuis un catalogue générique et extensible, vérifié live le 08/07/2026** (voir `ROADMAP.md` §3 M2), **M3 Assets — inventaire complet, fait le 08/07/2026** (Clean Architecture rétrofitée depuis un handler qui touchait `database.DB` directement sans aucune RBAC ; snapshots historiques ; criticité enfin branchée sur le Score Engine via le flux Redis `asset.criticality_changed` qui existait déjà côté `ScoreWorker` mais n'avait jamais d'émetteur ; voir `ROADMAP.md` §3 M3). Auth (JWT RS256, Argon2id, MFA, OAuth2/SAML2) et RBAC multi-tenant : code présent et maintenant fonctionnel de bout en bout (login réel testé le 08/07/2026 après correction de 11 bugs, voir `ROADMAP.md` §3 M1) mais **jamais vérifié en profondeur au-delà du login/dashboard** — traiter chaque sous-flux (MFA, OAuth2/SAML2, refresh token) comme non prouvé tant qu'il n'a pas été testé live.
- **Partiel (🟡)** : Incident Management (table manquante d'`AutoMigrate`), Custom Fields, Marketplace (structs sans tags `gorm:`, exclues d'`AutoMigrate` le 08/07/2026), PAM Audit Trail, CTI Engine (non câblé, en avance de phase Wave 2). Deux widgets Dashboard (`SecurityScore`, `AssetStatistics`) appellent des endpoints backend qui n'existent pas (`/analytics/security-score`, `/analytics/assets/statistics`) — repli gracieux côté frontend, sans conséquence UX, mais pas de vraies données tant que ces routes ne sont pas implémentées. `CreateRisk` ne renseigne pas `created_by` depuis le contexte réel (reste `uuid.Nil`).
- **Non commencé (❌)** : contenu réglementaire africain COBAC/BCEAO/ANSSI-CM (délibérément reporté — pas de textes source fiables, voir `ROADMAP.md` §3 M2), Offline-first, Billing/Stripe, différenciateurs Wave 2/3.

## ⚠️ Découverte critique du 08/07/2026 — à connaître avant de toucher Risk/Asset/Mitigation/Dashboard/Multitenancy
`middleware.SetContext()` n'était appelé **nulle part en code de production** (seulement dans un harnais de test) — `middleware.GetContext(c)`, lu par 8 handlers (`compliance_handler.go`, `risk_handler.go`, `asset_handler.go`, `mitigation_handler.go`, `mitigation_subaction_handler.go`, `dashboard_handler.go`, `multitenancy_auth_handler.go`, `multitenancy_org_handler.go`), retournait donc toujours `nil`, et ces handlers retombaient silencieusement sur `tenant_id = uuid.Nil`. Corrigé en une ligne dans `AuthMiddlewareRS256` (`internal/middleware/auth.go`). **Toute affirmation antérieure au 08/07/2026 sur le bon fonctionnement multi-tenant de ces modules doit être revérifiée** — voir `ROADMAP.md` §3 M2 pour le détail complet.

## Priorités du sprint en cours
1. ~~Corriger le build frontend~~ ✅ fait le 07/07/2026 (branche `fix/frontend-build-typescript-errors`, commit `4e5b91f5`).
2. ~~Corriger le bug cross-tenant `GormComplianceRepository.UpdateControl`~~ ✅ fait le 07/07/2026 (branche `fix/compliance-cross-tenant-update-isolation`, commit `5a0407fc`).
3. ~~M1 Compliance — use cases + handlers + OpenAPI + frontend~~ ✅ fait le 07/07/2026 (branche `feat/m1-compliance-engine`, voir `ROADMAP.md` §3 M1 pour le détail des 6 commits et des preuves).
4. ~~Vérification manuelle live de M1 + bug d'amorçage `AutoMigrate`~~ ✅ fait le 08/07/2026 (même branche) — 8 bugs corrigés en chaîne, l'app n'avait jamais eu de login fonctionnel dans cet environnement ; détail complet dans `ROADMAP.md` §3 M1.
5. ~~Dashboard : déconnexion forcée après login~~ ✅ fait le 08/07/2026 (même branche, signalé en direct par l'utilisateur) — 3 bugs de plus (clé de contexte `userID`/`user_id` incohérente, `/risks` sur le même bug `RequirePermissions` legacy que Compliance, intercepteur axios trop agressif sur tout 401) ; détail complet dans `ROADMAP.md` §3 M1.
6. ~~M2 — ISO 27001:2022~~ ✅ fait + vérifié live le 08/07/2026 (même branche) — catalogue générique + 93 contrôles + import en un clic + bug fondamental de multi-tenancy découvert et corrigé (`SetContext` jamais appelé en prod, voir avertissement ci-dessus) ; détail complet dans `ROADMAP.md` §3 M2.
7. ~~M3 — Assets~~ ✅ fait le 08/07/2026 (branche `feat/m3-assets-inventory`) — Clean Architecture rétrofitée (le handler existant touchait `database.DB` directement, zéro use case, zéro RBAC sur `POST /assets`), snapshots historiques, criticité enfin branchée sur le Score Engine (bug de scan varchar→float64 dans `GetRisksByAssetID` corrigé au passage, jamais atteint en pratique faute d'émetteur Redis) ; détail complet dans `ROADMAP.md` §3 M3.
8. ~~Bugs live découverts en vérifiant "Compliance a disparu de la sidebar"~~ ✅ fait le 08/07/2026 (branche `fix/dashboard-crash-mitigation-routes-and-ui-polish`) — Compliance n'avait pas disparu, c'est le **dashboard entier qui crashait en page blanche** après login : `stats_handler.go` scannait des colonnes NUMERIC Postgres dans des champs Go `int` (500 systématique sur risk-matrix/trends/top-vulnerabilities, jamais atteint avant faute de vraies données) ; corriger ça a exposé un champ `severity` manquant et une échelle de probabilité fausse (1-5 au lieu de 0.0–1.0) qui crashaient `TopVulnerabilities`/`RiskMatrix` ; `StatusDot.tsx` n'avait pas de `default` et crashait sur tout statut hors de son union stricte (domain.RiskStatus a DEUX vocabulaires incompatibles en usage — `open/in_progress/...` et `DRAFT/ACTIVE/...`). En creusant sur Mitigations (401 partout), trouvé que **`middleware.RequireRole` lit `c.Locals("role")`, une clé jamais posée par `AuthMiddlewareRS256`** (qui pose `org_roles`, une map) — toutes les routes d'écriture Mitigations/Incidents/Risk-Management renvoyaient 401 pour tout le monde, tout le temps ; corrigé + ajouté la route `GET /mitigations` qui n'existait tout simplement pas. Polish demandé au passage : overflow sidebar (20 items sans scroll), transition de route globale (`AnimatePresence` dans `App.tsx`), skeleton+stagger sur les widgets dashboard. Détail complet dans le message de commit.
9. ~~Master Prompt V4 (vision long terme fournie par l'utilisateur)~~ ✅ intégré le 08/07/2026 — sauvegardé dans `docs/MASTER_PROMPT_V4.md` (condensé + croisé avec l'état réel de chaque module), référencé en tête de `ROADMAP.md`.
10. Prochaine étape : **M4 — Reporting officiel + Board Report** (voir `ROADMAP.md` §3 M4 ; la partie « rapport COBAC/BCEAO en 1 clic » reste bloquée par l'absence de textes source, comme M2), ou basculer sur **COBAC/BCEAO/ANSSI-CM** dès que de vrais textes source sont disponibles. Deux branches ouvertes en attente de PR/merge : `feat/m3-assets-inventory` et `fix/dashboard-crash-mitigation-routes-and-ui-polish` (toutes deux poussées sur GitHub, aucune des deux mergée dans master).
11. Hors scope immédiat mais à planifier : passe d'animation complète sur les pages restantes (Reports, Marketplace, CustomFields, Users, Roles, Tenants, AuditLogs, TokenManagement — non touchées cette session, seul le Dashboard a reçu un vrai polish) ; les deux vocabulaires `RiskStatus` (lowercase vs uppercase) ne sont pas unifiés, seulement rendus non-fatals côté frontend — un vrai nettoyage nécessiterait de choisir un seul vocabulaire côté backend ; `CreateRisk` ne renseigne pas `created_by` depuis le contexte réel ; implémenter `/analytics/security-score` et `/analytics/assets/statistics` (routes jamais créées, widgets en repli gracieux) ; ~350 findings lint frontend ; 7 fichiers de tests frontend en échec (pré-existants, build) ; rétrofit du client OpenAPI généré sur Risk/Mitigation (pré-existant, M1) ; `TestRiskCRUDFlow`/`TestSetupMFA_Success` en échec (pré-existants, découverts pendant M1 — `TestStartAndStop`/`TestRateLimit_DifferentIPs` flaky, repassent au vert en isolation) ; `src/hooks/useAssetStore.ts` reste un chemin de données non contract-first (utilisé par les sélecteurs d'assets de `CreateRiskModal`/`EditRiskModal`/`DashboardGrid`, volontairement non touché pendant M3).

