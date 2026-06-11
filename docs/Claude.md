# OpenRisk — Contexte permanent Claude Code

## Mission du projet
OpenRisk est une plateforme GRC (Governance, Risk, Compliance) enterprise open-source.
Objectif : devenir le standard mondial du GRC, commençant par la France, la Belgique,
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
/internal/audittrail/            → PAM Audit Trail (traçabilité admin actions, append-only)
/internal/accessreview/          → Access Review & Certification (anti privilege creep)
/internal/datadiscovery/         → Sensitive Data Discovery (scanner PII/PCI/secrets)
/internal/boardreport/           → Executive Board Report (rapport DG/Board mensuel IA)
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
11. Transactions DB sur TOUTE opération multi-table
12. Le Score Engine n'est JAMAIS appelé directement depuis un handler → toujours via event Redis
13. Lire TOUS les fichiers existants d'un module avant d'écrire une seule ligne

### Qualité code (niveau obligatoire)
14. Zero `any` TypeScript — tout est typé strictement
15. Zod validation côté client sur tous les formulaires
16. Tests minimum par use case : TestXxx_Success + TestXxx_NotFound + TestXxx_Unauthorized

### UX (niveau non-négociable)
17. Skeleton loaders sur chaque chargement — jamais de spinner pleine page
18. Toujours gérer les 3 états : loading + error + empty
19. Optimistic updates sur toutes les mutations critiques

## Formule Score Engine
Score        = Probability (0.0–1.0) × Impact (0.0–10.0) × AssetCriticality (0.1–3.0)
Criticality  : score ≥ 7.0 = critical · ≥ 4.0 = high · ≥ 2.0 = medium · < 2.0 = low

## Modules et leur statut
| Module | Statut | Fichiers clés | Lacunes principales | Priorité |
|--------|--------|---------------|----------------------|----------|
| **Score Engine** | 🟡 Partiel | `scoring/engine.go`, `workers/score_worker.go`, `handler/score_engine_handler.go` | Handler appelle compute DIRECTEMENT au lieu de publier Redis event | CRITIQUE |
| **JWT RS256 Auth** | ✅ Complet | `pkg/auth/jwt.go`, `middleware/auth.go`, `config.go` | Legacy JWT_SECRET en config (deprecated) mais RS256 utilisé. JTI blacklist Redis OK | Moyenne |
| **Tenant Isolation** | 🟡 Partiel | `middleware/permission_middleware.go`, `repository/*_repository.go` | Filtres tenant_id dans repos OK. Mais handlers certains utilisent `safeGetString()` manuel au lieu de middleware injection | Haute |
| **Notification System** | 🟡 Partiel | `application/notification/usecase.go`, `repository/notification_repository.go`, `handler/notification_handler.go` | Use case minimal. Pas d'événements Redis publiés après création | Haute |
| **Risk Management** | ✅ Complet | `handler/risk_handler.go`, `application/risk/*.go`, `repository/gorm_risk_repository.go` | Clean Architecture OK. Mais pas de triggering Score Engine via Redis event | Haute |
| **RBAC & Permissions** | ✅ Complet | `domain/rbac.go`, `domain/permission.go`, `service/permission_service.go`, `middleware/permission.go` | Structure solide. Tests unitaires présents. Mais pas utilisé partout (handlers legacy) | Moyenne |
| **Audit Trail** | ❌ Manquant | `handler/audit_log_handler.go` | Table `admin_audit_events` référencée mais PAS DE MIGRATION. Append-only constraint non défini. | CRITIQUE |
| **Data Discovery** | ❌ Manquant | Aucun fichier | Annoncé dans Master Prompt. Aucune implémentation. Scanner PII/PCI/secrets absent | Basse (Module ultérieur) |
| **Redis Cache** | ✅ Complet | `infrastructure/redis/client.go`, `pkg/cache/cache.go` | Advanced cache OK. Pool config OK. Utilisé pour JWT blacklist, Score events | Basse |
| **Database Migrations** | 🟡 Partiel | 12 fichiers SQL | Indices `tenant_id` présents. MAIS : pas d'indices COMPOSITES `(tenant_id, status)`, `(tenant_id, severity)`, `(tenant_id, type)` | Haute |
| **Integration TheHive** | 🟡 Partiel | `infrastructure/integrations/thehive/client.go` | SyncEngine initié avec organizationID placeholder hardcodé. Multi-tenant non géré. | Moyenne |
| **OpenCTI/Splunk Integration** | ❌ Manquant | Répertoires vides | Déclarés vides dans structure. Pas d'implémentation | Basse |
| **Error Handling** | ✅ Complet | `domain/errors.go` | Erreurs typées (ErrNotFound, ErrForbidden, ErrConflict, ErrValidation). Respecté partout | Basse |
| **Email Service** | ❌ Manquant | `infrastructure/email/` + `integrations/email/email_provider.go` | Email provider schéma présent mais non intégré aux notifications | Moyenne |
| **Package `crypto/`** | ❌ Manquant | Vide | Credentials chiffrage AES-256-GCM pas documenté. Stockage en DB pas clair. | CRITIQUE |
| **Package `logger/`** | ❌ Manquant | Vide | Logging centralisé zerolog pas configuré. Risk: secrets en logs | CRITIQUE |
| **Package `pagination/`** | ❌ Manquant | Vide | Pagination non standardisée entre handlers | Moyenne |

## Sprint en cours
[Mettre à jour chaque lundi matin]