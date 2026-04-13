# OpenRisk — Contexte permanent Claude Code

## Projet
OpenRisk est une plateforme GRC (Governance, Risk, Compliance) enterprise open-source.
Objectif : devenir le standard mondial du GRC, commençant par la France, la Belgique, Europe, 
le Maghreb et l'Afrique subsaharienne (marchés COBAC/BCEAO/ANSSI).
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
  ├── utils/        → formatters, validators, cn()
  └── types/        → types TypeScript globaux

/src/services/      → client API typé (généré depuis OpenAPI 3.1)
/src/locales/       → fr.json, en.json (i18n complet)


## RÈGLES ABSOLUES — jamais violer sous aucun prétexte

### Sécurité (niveau critique)
0. Lire TOUS les fichiers existants d'un module avant d'écrire une seule ligne
1. Filtrer par tenant_id sur CHAQUE query DB — dans le repository, jamais dans le handler
2. Si un objet appartient à un autre tenant → retourner 404 (jamais 403)
3. JWT en RS256 uniquement (pas HS256 · pas de downgrade)
4. Jamais de secrets dans les logs (tokens, passwords, clés API, PII)
5. Credentials chiffrés AES-256-GCM en DB — jamais en clair, jamais dans les logs
6. SQL injection : toujours des paramètres GORM nommés, grep fmt.Sprintf SQL → corriger immédiatement

### Architecture (niveau haute priorité)
7. Erreurs typées uniquement : ErrNotFound · ErrForbidden · ErrConflict · ErrValidation
8. Transactions DB sur TOUTE opération multi-table
9. Le Score Engine n'est JAMAIS appelé directement depuis un handler → toujours via event Redis
10. Lire TOUS les fichiers existants d'un module avant d'écrire une seule ligne

### Qualité code (niveau obligatoire)
11. Zero `any` TypeScript — tout est typé strictement
12. Zod validation côté client sur tous les formulaires
13. Tests minimum par use case : TestXxx_Success + TestXxx_NotFound + TestXxx_Unauthorized

### UX (niveau non-négociable)
14. Skeleton loaders sur chaque chargement — jamais de spinner pleine page
15. Toujours gérer les 3 états : loading + error + empty
16. Optimistic updates sur toutes les mutations critiques


## Formule Score Engine
Score = Probability (0.0–1.0) × Impact (0.0–10.0) × AssetCriticality (0.1–3.0)
Criticality : score ≥ 7.0 = critical · ≥ 4.0 = high · ≥ 2.0 = medium · < 2.0 = low

## Ce qui est déjà implémenté
[À remplir après l'audit semaine 0 — lister module par module]

## Priorités du sprint en cours
[À mettre à jour chaque début de sprint]


## Formule Score Engine
Score        = Probability (0.0–1.0) × Impact (0.0–10.0) × AssetCriticality (0.1–3.0)
Criticality  : score ≥ 7.0 = critical · ≥ 4.0 = high · ≥ 2.0 = medium · < 2.0 = low

## Modules et leur statut
[À remplir après l'audit — module | statut ✅/🟡/❌ | fichiers | gaps]

## Sprint en cours
[Mettre à jour chaque lundi matin]