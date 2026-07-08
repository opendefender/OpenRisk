# 🛡️ OPENRISK — MASTER PROMPT V4.0
## Le guide de référence absolu pour construire le standard mondial du GRC

> **Version 4.0 — Avril 2026**
> *Écosystème OpenDefender · https://github.com/opendefender/OpenRisk*
> *Concurrent direct : Vanta, Drata, OneTrust, ServiceNow GRC, Archer*

> **Note d'intégration (2026-07-08) :** ce document est la vision long terme fournie par l'utilisateur.
> Il précède et a partiellement inspiré la structure Wave 1/2/3 de `ROADMAP.md` — les modules 14.9 à 14.18
> ci-dessous correspondent déjà 1:1 aux différenciateurs listés dans `ROADMAP.md` §4 (Wave 2). Voir
> `ROADMAP.md` §"Vision cible (Master Prompt)" pour la table de correspondance et ce qui est déjà fait
> vs. ce qui reste à construire. Ne pas re-implémenter un module déjà marqué ✅ dans ROADMAP.md sans
> revérifier son état réel dans le code d'abord (règle absolue #1 du projet).

---

## COMMENT UTILISER CE FICHIER

Ce fichier est la **source de vérité unique** du projet OpenRisk.
Il se divise en trois grandes parties :

**PARTIE A — FONDATIONS** : contexte permanent pour Claude Code (à placer dans CLAUDE.md)
**PARTIE B — DÉVELOPPEMENT** : prompts d'implémentation module par module, dans l'ordre exact d'exécution
**PARTIE C — PRODUCT GROWTH** : UX/UI, design system, conversion, rétention, roadmap compétitive

---

# PARTIE A — FONDATIONS

## FICHIER CLAUDE.md — À PLACER À LA RACINE DU PROJET

```markdown
# OpenRisk — Contexte permanent Claude Code

## Mission du projet
OpenRisk est une plateforme GRC (Governance, Risk, Compliance) enterprise open-source.
Objectif : devenir le standard mondial du GRC, commençant par la France, la Belgique,
le Maghreb et l'Afrique subsaharienne (marchés COBAC/BCEAO/ANSSI/ANTIC).
Concurrents directs : Vanta, Drata, OneTrust, ServiceNow GRC.
Dépôt : https://github.com/opendefender/OpenRisk

## Stack technique
Backend  : Go 1.22 · Fiber v2 · GORM · PostgreSQL 16 · Redis 7 · golang-migrate
Frontend : React 19 · TypeScript strict · Zustand · Tailwind CSS 3 · Recharts · React Router v7
Infra    : Docker multi-stage · Kubernetes + Helm · GitHub Actions CI/CD
Obs.     : Prometheus · Grafana · Loki · zerolog JSON · Sentry
Auth     : JWT RS256 · OAuth2 (Google, GitHub) · SAML 2.0 · TOTP MFA

## Architecture Clean Architecture — Backend

/cmd/server/main.go              → DI container, graceful shutdown, signal handling
/internal/domain/                → entités pures — ZÉRO dépendance externe ici
/internal/application/           → use cases (1 fichier = 1 use case)
/internal/infrastructure/        → repositories GORM, Redis, workers, SSE hub
/internal/api/http/handlers/     → handlers Fiber
/internal/api/http/middleware/   → auth, tenant, rate-limit, audit, security headers
/pkg/scoring/                    → Score Engine isolé, zéro dépendance externe
/pkg/cti/                        → CTI Engine (NVD, CISA KEV, MITRE ATT&CK)
/pkg/ai/                         → Anthropic AI Advisor (prompts, cache, rate-limit)
/pkg/notify/                     → Notification multi-canal (InApp, Email, Slack, Webhook)
/pkg/export/                     → PDF (chromedp), CSV, JSON, Excel
/pkg/crq/                        → Cyber Risk Quantification (modèle FAIR)
/pkg/compliance/                 → Chargement JSON des frameworks, calcul score
/internal/audittrail/            → PAM Audit Trail (traçabilité admin actions, append-only)
/internal/accessreview/          → Access Review & Certification (anti privilege creep)
/internal/datadiscovery/         → Sensitive Data Discovery (scanner PII/PCI/secrets)
/internal/boardreport/           → Executive Board Report (rapport DG/Board mensuel IA)
/internal/simulation/            → Risk Digital Twin (simulation What-If BFS/DFS)
/internal/warroom/               → Collaborative War Room (salle de crise SSE temps réel)
/internal/attackpath/            → Attack Path Graph (chemins d'attaque + blast radius)
/internal/champions/             → Risk Champions Leaderboard (gamification + badges)
/internal/marketplace/           → Plugin Marketplace (webhooks + dispatcher + sandbox)
/migrations/                     → fichiers golang-migrate (up + down)

## Architecture Feature-Based — Frontend

/src/features/[module]/
  ├── pages/        → composants de page (route-level)
  ├── components/   → composants locaux au module
  ├── hooks/        → hooks React spécifiques au module
  └── store.ts      → store Zustand du module

/src/shared/
  ├── components/   → design system partagé
  ├── hooks/        → useSSE, useDebounce, useKeyboard, useToast
  └── utils/        → formatters, validators, cn()

/src/services/      → client API typé (généré depuis OpenAPI 3.1)
/src/locales/       → fr.json, en.json (i18n complet)

## RÈGLES ABSOLUES — jamais violer sous aucun prétexte

### Sécurité (niveau critique)
1. Filtrer par tenant_id sur CHAQUE query DB — dans le repository, jamais dans le handler
2. Si un objet appartient à un autre tenant → retourner 404 (jamais 403)
3. JWT en RS256 uniquement (pas HS256 · pas de downgrade)
4. Jamais de secrets dans les logs (tokens, passwords, clés API, PII)
5. Credentials chiffrés AES-256-GCM en DB — jamais en clair, jamais dans les logs
6. SQL injection : toujours des paramètres GORM nommés, grep fmt.Sprintf SQL → corriger immédiatement
7. **PAM Audit Trail** : table `admin_audit_events` est APPEND-ONLY — aucun UPDATE ni DELETE n'est jamais autorisé, même en migration
8. **Data Discovery** : le scanner ne stocke jamais de données sensibles en clair — uniquement les métadonnées et l'emplacement
9. **Access Review** : une révocation de droits déclenchée par une campagne est irréversible via l'API standard — elle nécessite un admin audit trail explicite

### Architecture (niveau haute priorité)
10. Erreurs typées uniquement : ErrNotFound · ErrForbidden · ErrConflict · ErrValidation
12. Transactions DB sur TOUTE opération multi-table
13. Le Score Engine n'est JAMAIS appelé directement depuis un handler — toujours via event Redis
14. Lire TOUS les fichiers existants d'un module avant d'écrire une seule ligne

### Qualité code (niveau obligatoire)
15. Zero `any` TypeScript — tout est typé strictement
16. Zod validation côté client sur tous les formulaires
17. Tests minimum par use case : TestXxx_Success + TestXxx_NotFound + TestXxx_Unauthorized

### UX (niveau non-négociable)
18. Skeleton loaders sur chaque chargement — jamais de spinner pleine page
19. Toujours gérer les 3 états : loading + error + empty
20. Optimistic updates sur toutes les mutations critiques

## Formule Score Engine
Score        = Probability (0.0–1.0) × Impact (0.0–10.0) × AssetCriticality (0.1–3.0)
Criticality  : score ≥ 7.0 = critical · ≥ 4.0 = high · ≥ 2.0 = medium · < 2.0 = low
```

---

# PARTIE B — DÉVELOPPEMENT

## LA MÉTHODE DE TRAVAIL (5 étapes obligatoires par module)

```
ÉTAPE 1 — LIRE (ne jamais sauter, même si "tu connais déjà")
ÉTAPE 2 — PLANIFIER
ÉTAPE 3 — IMPLÉMENTER (backend d'abord, frontend ensuite)
ÉTAPE 4 — VALIDER (go test ./... && npm test && docker compose up -d)
ÉTAPE 5 — COMMITER (git add . && git commit -m "feat(module): description précise")
```

**RÈGLE DES 2 HEURES** : Si tu bloques depuis plus de 2h → fermer la session → nouvelle session →
"Je bloque sur [problème exact]. Lis [fichier] et explique-moi le problème avant de proposer une solution."

---

## MODULE 0 — AUDIT INITIAL (obligatoire avant tout développement)
Audit Backend (Clean Architecture, sécurité, tenant isolation, index manquants), Audit Sécurité
(table par endpoint : auth/tenant/rate-limit/validation/secrets/isolation), Audit Frontend (any
TypeScript, formulaires sans Zod, composants sans 3 états, appels API hors hooks, strings non-i18n).

## MODULE 1 — SCORE ENGINE
`pkg/scoring/` pur, zéro dépendance externe. `Engine.Calculate/ToCriticality/Breakdown`. Formule
Probability×Impact×AssetCriticality, seuils 7.0/4.0/2.0. Jamais appelé directement d'un handler —
toujours via event Redis `risk.updated` → worker → `risk.score_updated`.
**Statut OpenRisk (2026-07-08) : fait.** Voir `backend/pkg/scoring/`.

## MODULE 2 — AUTHENTIFICATION COMPLÈTE (7 couches)
2.1 JWT RS256 + Middleware (AuthRequired, RequirePermission, TenantMiddleware) · 2.2 MFA TOTP + OAuth2
(Google/GitHub) · 2.3 Multi-tenant + RBAC (organization_members, rôles standards + custom, PAT tokens,
audit log auth).
**Statut : code présent et fonctionnel de bout en bout pour login/dashboard (vérifié live 08/07/2026)** ;
sous-flux MFA/OAuth2/SAML2/refresh non re-vérifiés en profondeur — voir `ROADMAP.md` §3 M1.

## MODULE 3 — RISK REGISTER
Backend : CRUD complet, filtres (search/status/criticality/framework/asset/tags/dates/source), bulk
action, import/export, score via Score Engine uniquement, audit log par modification.
Frontend : RiskListPage (table + filtres + bulk + clavier), RiskDrawer (Détails/Score/Mitigations/
Timeline/CTI/IA/Financier), CreateRiskModal (score live), ImportRisksPage.
**Statut : ✅ solide** (voir `ROADMAP.md` §1.1).

## MODULE 4 — MITIGATION WORKFLOW
MitigationPlan + SubAction, progression auto depuis sous-actions, passage "review" à 100%,
auto-complétion par scanner (comparaison de snapshots), dépendances entre sous-actions.
Frontend : Kanban 4 colonnes (Todo/In Progress/Review/Done), badge "Auto-detected", Gantt, vue Table.
**Statut : ✅ solide** pour le CRUD manuel (voir `ROADMAP.md` §1.1) ; auto-complétion scanner dépend du
MODULE 6 (Infrastructure Scanner), non encore câblé.

## MODULE 5 — CTI ENGINE
`pkg/cti/` : sync NVD (horaire) + CISA KEV (6h) + MITRE ATT&CK statique, matching CVE↔Assets via CPE
overlap, auto-création de risques (`source: cti_auto`) jamais directement — toujours via Risk Service.
**Statut : code présent (`pkg/cti/`) mais NON câblé** dans main.go/routes — scope Wave 2, en avance de
phase. Voir `ROADMAP.md` §1.2.

## MODULE 6 — INFRASTRUCTURE SCANNER
Scanners cloud (AWS/Azure/GCP, exécutés serveur-side) + Scanner Agent on-premise (exécutable téléchargé,
nmap/osquery en local, jamais dans le backend SaaS). Pipeline : Scan → Normalize → Deduplicate →
StorePreview (Redis 24-48h) → import manuel utilisateur. Détection de mitigation par comparaison de
snapshots.
**Statut : non commencé.**

## MODULE 7 — SSE REAL-TIME ENGINE
Hub par tenant, `GET /api/v1/stream`, events (`risk.score_updated`, `mitigation.progress`,
`scan.completed`, `notification`, etc.), reconnexion frontend avec backoff exponentiel.
**Statut : à vérifier** — non audité en détail dans cette session.

## MODULE 8 — DASHBOARD & ANALYTICS
Analytics Service avec cache Redis (TTL 5 min, invalidation par event), endpoints dashboard/trends/
heatmap/by-framework/by-asset/score-history/ciso-report. Frontend : KPI Cards animées, RiskHeatmap SVG
natif, TrendChart, FrameworkRadar, TopRiskyAssets, dashboard personnalisable (react-grid-layout).
**Statut : ✅ solide dans l'ensemble**, mais deux bugs réels corrigés le 08/07/2026 (voir
`ROADMAP.md`/`CLAUDE.md`) : `RiskMatrixCell`/`TopVulnerability`/`TrendPoint` scannaient des colonnes
NUMERIC dans des champs Go `int` (500 systématique) ; `TopVulnerabilities.tsx` et `RiskMatrix.tsx`
plantaient/étaient incorrects sur données réelles (champ `severity` absent, échelle probability 1-5 au
lieu de 0.0–1.0). Deux widgets (`SecurityScore`, `AssetStatistics`) appellent des endpoints jamais
implémentés (`/analytics/security-score`, `/analytics/assets/statistics`) — repli gracieux, pas de
vraies données.

## MODULE 9 — NOTIFICATIONS
`pkg/notify/` multi-canal (InApp+SSE, Email Resend/SMTP, Slack, Webhook signé HMAC), triggers
event-driven (risk critical, mitigation due, CTI ≥9.0, etc.), centre de notifications frontend.
**Statut : à vérifier** — non audité en détail dans cette session.

## MODULE 10 — IA ADVISOR
`pkg/ai/` (Claude), rate limit 50/h/tenant, cache Redis 1h, structured output JSON Schema. Analyse
risque, suggestions mitigation, déduplication, priorisation, executive summary. Frontend AIAdvisorTab.
**Statut : non commencé** (`ai_risk_predictor_service`/`recommendation_service` existent mais sans
Advisor/RAG/Control-mapping — voir `ROADMAP.md` §1.2).

## MODULE 11 — REPORTING & EXPORT
`pkg/export/` : PDF (chromedp) + CSV/XLSX/JSON, types de rapport (risk_register, mitigation,
compliance, executive, ciso_monthly, cobac_official, bceao_official...), stockage MinIO/S3, async +
polling. **Bloqué pour COBAC/BCEAO officiels tant que les textes source ne sont pas disponibles** —
même contrainte que MODULE 12.
**Statut : `export_handler` existe, rapports officiels non commencés. C'est M4 dans ROADMAP.md.**

## MODULE 12 — COMPLIANCE FRAMEWORKS
Moteur générique de chargement de catalogues (JSON par framework), scoring covered/partial/uncovered,
cross-mapping entre frameworks, gap analysis. Frontend CompliancePage (grid par framework, onglet
Africain mis en avant), FrameworkDetailPage, GapAnalysisPage.
**Statut : ✅ M1 (moteur générique) + ISO 27001:2022 (93 contrôles) faits et vérifiés live le
08/07/2026. COBAC/BCEAO/ANSSI-CM délibérément différés — pas de textes source fiables, voir
`ROADMAP.md` §3 M2.** Ne pas modéliser ces frameworks depuis la mémoire d'entraînement — le risque de
citer un mauvais numéro d'article dans un vrai produit de conformité est jugé pire que l'absence du
framework.

## MODULE 13 — ASSET MANAGEMENT
Modèle Asset (CPE, criticality 0.1–3.0, environment, valeur CRQ), CRUD + bulk-update + import depuis
scanner. Frontend AssetUniversePage (D3 force simulation — "killer feature"), AssetListPage, AssetDrawer
(CRQ/CVEs/Risques/Historique).
**Statut : ✅ M3 fait le 08/07/2026** — Clean Architecture rétrofitée, snapshots historiques, criticité
réellement branchée sur le Score Engine (voir `ROADMAP.md` §3 M3). **AssetUniversePage (vue D3 force-
directed) non implémentée** — l'inventaire actuel est une liste/table classique, pas la killer feature
D3 décrite ici. Bon candidat pour une itération future si le produit veut ce niveau de polish visuel.

## MODULE 14 — FONCTIONNALITÉS AVANCÉES
Sous-modules détaillés dans le corps du prompt original (conservé tel quel plus bas pour référence
complète) :
- 14.1 Incident Management — **🟡 partiel**, table absente d'AutoMigrate (`ROADMAP.md` M5)
- 14.2 Vendor Risk Management — non commencé
- 14.3 Policy Management — non commencé
- 14.4 Trust Center Public — non commencé
- 14.5 Cyber Risk Quantification (CRQ/FAIR) — non commencé
- 14.6 Business Continuity Planning — non commencé
- 14.7 Security Awareness Training — non commencé
- 14.8 Custom Fields — 🟡 partiel (present dans AutoMigrate, UI/tests à vérifier)
- 14.9 PAM Audit Trail — non commencé (différenciateur Wave 2)
- 14.10 Access Review & Certification — non commencé (différenciateur Wave 2)
- 14.11 Sensitive Data Discovery — non commencé (différenciateur Wave 2)
- 14.12 Executive Board Report — non commencé, c'est **M4** dans ROADMAP.md
- 14.13 Risk Digital Twin (simulation) — non commencé (Wave 2)
- 14.14 Collaborative War Room — non commencé (Wave 2)
- 14.15 Attack Path Graph — non commencé (Wave 2)
- 14.16 Risk Champions Leaderboard — non commencé (Wave 2)
- 14.17 Offline-First Mode — non commencé (Wave 2, différenciateur marché africain)
- 14.18 Plugin Marketplace — 🟡 structs présentes sans tags `gorm:`, exclues d'AutoMigrate

## MODULE 15 — SÉCURITÉ & HARDENING
Headers HTTP (HSTS/CSP/X-Frame-Options), rate limiting Redis par route, validation globale, CORS
whitelist stricte, audit log global, protection injection.
**Statut : 🟡 partiel** — gosec en mode non-bloquant, govulncheck/gitleaks absents des workflows CI
(voir `ROADMAP.md` §2.4).

## MODULE 16 — OBSERVABILITÉ
Logs structurés zerolog, métriques Prometheus, health checks (`/health`, `/health/ready`,
`/health/deep`), Request ID distribué, dashboards Grafana.
**Statut : non audité en détail dans cette session.**

## MODULE 17 — ÉLÉMENTS TRANSVERSAUX
17.1 i18n (✅ fr/en complet) · 17.2 Billing & Plans SaaS (non commencé) · 17.3 Feature Flags (non
commencé) · 17.4 Super Admin Panel (non commencé) · 17.5 Accessibilité WCAG 2.1 AA (non audité) ·
17.6 Onboarding Flow gamifié (non commencé) · 17.7 Sync Engine & intégrations externes (TheHive
adapter existe, reste à généraliser).

---

# PARTIE C — PRODUCT GROWTH : UX/UI, CONVERSION & RÉTENTION

## VISION DESIGN OPENRISK
Trois impressions simultanées : **Confiance** (épuré, sérieux) · **Puissance** (données complexes
rendues lisibles) · **Modernité** (animations fluides, jamais tape-à-l'œil). Différenciation vs
Vanta (trop plat) / Drata (trop complexe) / ServiceNow (trop lourd) : **clarté + vitesse + élégance**.

## DESIGN SYSTEM — TOKENS CLÉS
```
Couleurs criticité (identiques en dark mode, saturation ajustée) :
  critical : #DC2626 (red-600) · high : #EA580C (orange-600)
  medium   : #D97706 (amber-600) · low : #16A34A (green-600)

Surfaces dark : base #0F172A (slate-900) · elevated #1E293B (slate-800) · overlay #334155 (slate-700)

Typographie : "DM Sans" (UI) · "JetBrains Mono" (scores, CVEs, codes)

Animations (durées de référence) :
  fast   : 150ms ease-out  (hover, toggles)
  normal : 250ms ease-out  (drawers, modals)
  slow   : 400ms ease-out  (page transitions, confettis)
  → toujours respecter prefers-reduced-motion ; aucune animation > 500ms ;
    jamais d'animation qui bloque l'interaction (pointer-events: auto)
```

Composants partagés attendus : `RiskBadge`, `ScoreMeter` (arc SVG + breakdown), `ProgressBar`,
`StatusDot`, `FrameworkTag`, `UserAvatar`, `EmptyState`, `ConfirmModal`, `SkeletonTable`, `ErrorState`,
`ToastNotification`, `FloatingBulkBar`, `CommandPalette` (⌘K).

**Règle animation route-level (appliquée le 08/07/2026) :** chaque changement de route fade ~180-200ms
via `AnimatePresence` keyed par pathname — voir `frontend/src/App.tsx` (`AnimatedOutlet`). C'est la
règle la plus rentable de cette section : un seul wrapper au niveau du layout rend toute la navigation
fluide sans toucher chaque page individuellement.

## RACCOURCIS CLAVIER GLOBAUX CIBLES
`⌘K` palette · `N` nouveau risque · `M` nouvelle mitigation · `I` nouvel incident · `Esc` fermer ·
`↑↓` naviguer table · `Enter` ouvrir · `?` aide raccourcis · `⌘E` exporter vue · `⌘S` sauvegarder form.
**Statut : non implémenté** (au-delà de la recherche ⌘K visuelle dans la sidebar) — bon candidat pour
M7 (polish des 3 écrans signature).

## STRATÉGIE CONVERSION & RÉTENTION (non commencé)
`PlanLimitBanner`, `FeatureGateModal`, `UsageDashboard`, Weekly Security Digest, re-engagement emails,
page Pricing publique, landing page marketing. Tout ce périmètre correspond à **Billing & Plans
(17.2)**, non commencé — dépend d'un modèle de plans/quotas qui n'existe pas encore dans le code.

## ROADMAP COMPÉTITIVE — TABLE DE COMPARAISON
Voir le tableau complet en fin de section Partie C originale (ServiceNow/Vanta/Drata/OpenRisk) pour le
pitch concurrentiel complet ; conservé tel quel plus bas pour référence.

### Phases de croissance (rappel)
Phase 1 (MVP, Modules 0–10) → Phase 2 (Différenciation, Modules 11–14.8, dont M2/M4 ROADMAP) →
Phase 2.5 (Gouvernance avancée, Modules 14.9–14.12) → Phase 3 (Platform, Modules 15–17 + Marketplace).
OpenRisk est actuellement en Phase 1, avec M1/M2(ISO)/M3 de `ROADMAP.md` faits — la suite naturelle
(voir `ROADMAP.md`) est **M4 (Reporting/Board Report, = Module 14.12 ici)**, puis M5 (Incidents),
avant d'attaquer Phase 2.

---

## RÉFÉRENCE COMPLÈTE

Le texte intégral du Master Prompt V4.0 (tous les prompts détaillés module par module, modules 14.9 à
14.18 in extenso, Partie C complète avec pricing/landing/animations détaillées) a été fourni par
l'utilisateur le 2026-07-08 et doit être consulté directement dans l'historique de conversation ou
redemandé à l'utilisateur si le détail d'un module précis est nécessaire pour l'implémenter — ce
fichier n'en est qu'un résumé de navigation croisé avec l'état réel du code, pas une copie exhaustive.
