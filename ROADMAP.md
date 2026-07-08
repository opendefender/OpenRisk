# 🗺️ OPENRISK — ROADMAP

> **Vision cible (Master Prompt).** L'utilisateur a fourni le 08/07/2026 un document de vision long
> terme complet — voir [`docs/MASTER_PROMPT_V4.md`](docs/MASTER_PROMPT_V4.md). Il couvre 18 modules
> (Score Engine → Plugin Marketplace) + un design system/animations + une stratégie growth. Les
> modules 14.9–14.18 y correspondent déjà 1:1 aux différenciateurs de la Wave 2 ci-dessous (PAM Audit
> Trail, Access Review, Data Discovery, Board Report, Digital Twin, War Room, Attack Path, Champions,
> Offline-First, Marketplace) — ce n'est pas un nouveau plan, c'est la source dont cette roadmap est
> déjà partiellement dérivée. Ce qui en ressort de nouveau et pas encore reflété ici : la section
> Design System (tokens couleur/animation précis) et la stratégie Billing/Growth (17.2 + Partie C),
> toutes deux non commencées. Avant d'implémenter un module cité dans ce document, vérifier son état
> réel dans `ROADMAP.md` (ce fichier) plutôt que de supposer qu'il correspond à son statut narratif.
>
> **Source de vérité d'exécution.** Cette roadmap est fondée sur l'inspection du code réel du dépôt `opendefender/OpenRisk`. Audit initial : branche `master`, clone du 26 juin 2026. **Rafraîchi le 07/07/2026** sur la branche `fix/cti-build-conflict` par relecture directe des migrations/domaine/repos/handlers/frontend + exécution de `go build`, `go vet`, `go test`, `npm run build`. Les statuts ne reflètent PAS le README ni les anciens documents — uniquement ce qui existe dans le code, vérifié à la date ci-dessus.
>
> **Règle d'or.** On n'avance pas en largeur tant qu'une vague n'est pas réellement terminée (Definition of Done, §6). On ne marque jamais ✅ sans preuve (test CI vert + revue humaine). Statuts honnêtes uniquement : `✅ Fait` · `🟡 Partiel` (gaps listés) · `❌ Non commencé`.
>
> **⚠️ Deux blocages critiques découverts le 07/07/2026, absents de l'audit du 26/06 :**
> 1. ~~Build frontend cassé~~ **✅ CORRIGÉ le 07/07/2026** (branche `fix/frontend-build-typescript-errors`, commit `4e5b91f5`). `npm run build` (`tsc -b && vite build`) passait de 79 erreurs TypeScript à 0. Causes : alias `@/` non configuré (imports convertis en relatifs), imports type-only non conformes à `verbatimModuleSyntax`, prop `size` manquante sur `Button`, type `Risk` incomplet dans `useRiskStore` vs `services/riskService`, `react-query` v5 (`keepPreviousData` → `placeholderData`), `sonner` v2 (signature `toast.promise` changée), `DataTableWidget` trop rigide pour l'usage réel. Détail complet dans le message de commit. **Non corrigé dans ce passage** (hors scope, pré-existant, vérifié par `git stash`) : 7 fichiers de tests frontend en échec + ~350 findings lint (surtout `no-explicit-any`) — à traiter séparément, voir §2.4.
> 2. ~~Bug d'isolation cross-tenant~~ **✅ CORRIGÉ le 07/07/2026** (branche `fix/compliance-cross-tenant-update-isolation`, commit `5a0407fc`). `GormComplianceRepository.UpdateControl` utilisait `.Where(...).Save(control)` — GORM ignore la clause `Where` chaînée quand la clé primaire est déjà posée sur le struct, donc un tenant B pouvait écraser un contrôle du tenant A. Remplacé par `.Model(&domain.ComplianceControl{}).Where("id = ? AND tenant_id = ?", ...).Select(...).Updates(control)`, le pattern GORM qui respecte réellement le `Where` sur une update. `gorm_compliance_repository_test.go` (écrit en TDD mais jamais commité) est maintenant commité ; les 15 tests du package repository passent, y compris `TestUpdateControl_CrossTenantFails`. Voir §2.2.

---

## 1. ÉTAT RÉEL DU CODE (au 26/06/2026)

Méthode de vérification : présence de **table en migration** (persistance réelle) + **handler/service/domain** (logique) + **feature frontend** (UI). Un module n'est « réel » que si les trois existent.

> **⚠️ Mise en garde ajoutée le 08/07/2026 :** cette méthode vérifie la *présence* du code, pas son *fonctionnement en production*. Les lignes « Multi-tenant + RBAC ✅ » et « Authentification ✅ » ci-dessous datent d'avant la découverte (08/07/2026, voir §3 M2) que `middleware.SetContext()` — dont dépendent `risk_handler.go`/`asset_handler.go`/`mitigation_handler.go`/`dashboard_handler.go`/`multitenancy_*_handler.go` — n'était jamais appelé en production, et qu'aucun login n'avait jamais réussi de bout en bout dans cet environnement avant cette date (voir §3 M1). Se fier à §3 (M1/M2) pour le statut réel et daté de l'auth/RBAC, pas à cette table.

### 1.1 Ce qui est solide (✅)

| Domaine | Preuve dans le code | Statut |
|---|---|---|
| Multi-tenant + RBAC | tables `tenants`, `organizations_saas`, `organization_members`, `organization_roles`, `roles_and_permissions`, `teams`, `user_tenants` ; handlers `rbac_*`, `multitenancy_*` ; services `permission/role/tenant` (+ tests) | ✅ |
| Authentification | JWT **RS256** (migration `add_jwt_rs256_support`), **Argon2id** (`password_hasher.go`, params OWASP), MFA (tables + usecase + tests), OAuth2, SAML2, refresh tokens + blacklist, PAT/API tokens | ✅ |
| Risk Register | tables `risks`, `risk_enhancements`, `risk_management_system` ; handlers `risk_*` (+ tests d'intégration) ; feature frontend `risks` | ✅ |
| Mitigation Workflow | tables `mitigations_comprehensive`, `mitigation_subactions` ; handlers + sous-actions (+ tests) ; feature frontend `mitigations` | ✅ |
| Score Engine | `pkg/scoring`, `score_engine_service` + tests ; feature frontend `scoreEngine` | ✅ |
| Notifications | tables `notifications`, `notification_logs/preferences/templates` ; handler + service (+ tests) | ✅ |
| Audit logging (modifs) | tables `audit_logs`, `auth_audit_logs` ; `audit_service` + tests ; `risk_timeline` | ✅ |
| Dashboard & Analytics | `dashboard_handler`, `enhanced_dashboard_handler`, `analytics_service`, `trend_analysis_service` ; feature frontend `dashboard` | ✅ |
| Perf & cache | `pkg/cache`, `query_optimizer` (+ tests), index composites/perf | ✅ |
| i18n FR/EN | `frontend/src/locales/{fr,en}.json` | ✅ |
| Assets (M3, fait le 08/07/2026) | Clean Architecture complète (`domain.AssetRepository`/`GormAssetRepository`/6 use cases, 30 tests) ; snapshots historiques (`asset_snapshots`) ; criticité réellement branchée sur `pkg/scoring.Engine` via le flux Redis `asset.criticality_changed` → `ScoreWorker` ; feature frontend `src/features/assets` (voir §3 M3) | ✅ |

> **Verdict honnête sur le cœur :** c'est un **excellent Risk Register multi-tenant**, sécurisé et testé. C'est un vrai actif. Mais ce n'est pas encore une plateforme **GRC** : le « G » (gouvernance/politiques) et surtout le « C » (conformité/référentiels) manquent — voir §1.3.

### 1.2 Code présent mais incomplet (🟡) — à finir, pas à recommencer

| Domaine | Ce qui existe | Ce qui manque (gap) |
|---|---|---|
| **Compliance Frameworks** | Migration `0028_create_compliance_schema` (`compliance_frameworks`/`compliance_controls`/`control_evidences`) ; domaine `compliance.go` + `compliance_repository.go` ; `GormComplianceRepository` avec tenant_id filtré et **prouvé par 15 tests verts** (✅ bug cross-tenant corrigé le 07/07) | **Aucun** use case / handler / route OpenAPI / feature frontend — c'est encore uniquement la couche donnée, saine mais invisible pour un utilisateur |
| Reporting / Export | `export_handler` | Rapports **officiels COBAC/BCEAO**, **Board Report**, PDF soigné ; UI dédiée |
| Incident Management | `incident_service`/`incident_handler` font de vrais appels GORM (`Create`/`Where`/`Model`) ; pages frontend `Incidents.tsx` + `useIncidentStore.ts` | **`domain.Incident` absent de `AutoMigrate()` dans `main.go`, aucune migration `CREATE TABLE incidents`** → la table n'existe probablement pas au runtime, le service échoue en pratique malgré du code qui a l'air fini |
| Custom Fields | `domain.CustomField`/`CustomFieldTemplate`, `TableName()`, **présents dans `AutoMigrate()`** | UI à vérifier ; couverture de tests |
| Gamification (Champions) | `gamification_handler/service`, feature frontend `gamification` | `GamificationService` est un **struct vide, zéro DB** → stub pur en mémoire, aucune table, absent d'`AutoMigrate()` |
| Marketplace | `domain.MarketplaceApp`/`Connector`/`ConnectorUpdate`/`MarketplaceLog`, **tous dans `AutoMigrate()`** ; `marketplace_service` (+ test) | Webhooks/sandbox/dispatcher ; UI |
| PAM Audit Trail | `domain.AdminAuditEvent` (table `admin_audit_events`), **dans `AutoMigrate()`**, `admin_audit_service` + repo | Pas de champs de hash-chaînage (PrevHash/Hash) malgré le commentaire « append-only » ; `pkg/auditchain` n'existe pas encore (prévu Wave 2 §2.2) |
| CTI Engine | `pkg/cti/` complet : `client.go`/`cti.go`/`matcher.go`/`model.go`/`repository.go`/`sync.go`, clients NVD + CISA KEV, matcher CPE, `gorm_cti_repository.go` (travail en cours sur la branche `fix/cti-build-conflict`) | **Non câblé** dans `cmd/server/main.go` ni dans les routes ; **aucune migration** pour les tables CTI ; ⚠️ ce module est du scope **Wave 2 §2.5**, en avance sur l'ordre imposé — à ne pas approfondir avant M1/M2 (voir règle anti-dérive §6.5) |
| Intégrations | `integration_handler/service`, `integration_models` | Connecteurs réels (TheHive, OpenCTI, Splunk, Elastic) à valider |
| IA | `ai_risk_predictor_service`, `recommendation_service` | Pas d'Advisor/RAG/Control-mapping ; pas de garde-fou human-in-the-loop formalisé |
| Bulk operations | `bulk_operation_handler/service`, domaine | À couvrir de tests ; UI |

### 1.3 Non commencé (❌) — y compris des éléments CRITIQUES pour le créneau

| Domaine | Pourquoi c'est important | Priorité |
|---|---|---|
| **Contenu réglementaire africain** (COBAC, BCEAO, ANSSI-CM, Loi 2024/017) | Notre seul avantage déloyal. Inexistant dans le code (aucun contrôle/framework chargé — la couche donnée du §1.2 est vide de contenu). | 🔴 **#1** |
| **Use cases / handlers / OpenAPI / frontend Compliance** (M1) | La couche donnée existe (§1.2) mais rien n'est exposé : ni API, ni UI. Sans ça, la table `compliance_controls` est invisible pour un utilisateur. | 🔴 **#1** |
| Offline-first | Différenciateur Afrique (connectivité). Absent (frontend). | 🔴 |
| Billing / Stripe / Subscriptions | Aucune table d'abonnement. Bloque la monétisation. | 🟠 |
| Vendor risk · Policy mgmt · BCP · Training · Trust Center | Modules GRC standards absents. | 🟠 |
| Data Discovery · Access Review · Board Report | Modules avancés absents. | 🟡 |
| Digital Twin · War Room · Attack Path | Modules « waouh » absents. | 🟡 |
| **Différenciateurs V6** : Regulatory Content-as-Code, Audit Defensibility Chain, AI Control-Mapping, CRQ/FAIR en FCFA, RAG réglementaire, Regulator Portal, Benchmarking | Le futur moat. À construire. | Wave 2/3 |

---

## 2. WAVE 0 — BLOCKERS À CORRIGER AVANT TOUTE NOUVELLE FEATURE

> À faire **en premier**, dans l'ordre. Rien d'autre ne démarre tant que la Wave 0 n'est pas ✅.

### 2.1 ✅ Corriger la licence — FAIT
- **Preuve (07/07/2026) :** `LICENSE` = Business Source License 1.1 cohérente ; badge README = `BUSL 1.1` ; section README « License » cite BUSL-1.1 + reversion Apache 2.0 à 4 ans. `grep -ri "boost\|business source" .` ne renvoie plus qu'une seule licence.
- [x] Un seul nom de licence partout (LICENSE + README).
- [ ] Reste à vérifier : `info.license` OpenAPI, `COPYRIGHT.md`, CLA dans `CONTRIBUTING.md` — non re-contrôlés dans cette passe, à confirmer avant de cocher l'item complet.

### 2.2 ✅ Fondations du schéma Compliance — FAIT
- [x] Migration `backend/migrations/0028_create_compliance_schema.up/down.sql` : `compliance_frameworks` (global), `compliance_controls` (tenant_id + framework_id), `control_evidences` (tenant_id + control_id), toutes avec soft-delete + index tenant-scopés.
- [x] Entités domaine `backend/internal/domain/compliance.go` + `compliance_repository.go` (interface).
- [x] `GormComplianceRepository` (`backend/internal/infrastructure/repository/gorm_compliance_repository.go`) : tenant_id filtré au repository sur toutes les opérations (Get/List/Update/Delete).
- [x] **Bug cross-tenant corrigé le 07/07/2026** (commit `5a0407fc`) : `UpdateControl` utilisait `.Where(...).Save(control)`, ignoré par GORM sur update quand la PK est posée. Remplacé par `.Model(...).Where(...).Select(...).Updates(control)`.
- [x] `gorm_compliance_repository_test.go` commité — 15/15 tests du package verts, y compris `TestUpdateControl_CrossTenantFails`.
- **Acceptation : atteinte.** Migration up/down réversible testée ; repository tenant-scopé prouvé par test cross-tenant vert. **Ce qui manque encore pour du Compliance utilisable (M1, §3) : use cases, handlers, OpenAPI, frontend — la couche donnée seule ne suffit pas.**

### 2.3 🟡 Chiffrement au repos — partiellement câblé
- **Constat (07/07/2026) :** `pkg/crypto/aes.go` implémente `EncryptAES256GCM`/`DecryptAES256GCM` correctement (GCM réel). **Seul appelant :** `backend/internal/application/auth/mfa_usecase.go` (secrets MFA chiffrés). Aucun appel trouvé pour les credentials d'intégration ni les autres champs PII.
- [x] Secrets MFA chiffrés au repos.
- [ ] Brancher AES-256-GCM sur les credentials d'intégration (`integration_models`, connecteurs marketplace) et autres PII stockées.
- [ ] `DeriveKey` est actuellement un **stub SHA256** (commentaire : « in production use HKDF ») — à remplacer par une vraie dérivation de clé via KMS/HKDF avant tout déploiement de champs sensibles supplémentaires.
- **Acceptation :** test prouvant qu'aucun secret (MFA + credentials d'intégration) n'est lisible en clair en base.

### 2.4 🟡 README / hygiène dépôt / CI sécurité — partiellement fait
- [ ] README : toujours à vérifier contre la liste §1.1 (non re-audité dans cette passe).
- [ ] **Racine toujours polluée** : `FILES_CREATED.sh` et `SECURITY_AUDIT_HANDLERS.csv` présents ; `claude_status_update.txt` marqué supprimé en working tree mais **suppression non commitée** ; nouveau script `add_headers.py` (untracked, ajoute l'en-tête BUSL-1.1 aux fichiers qui n'en ont pas) à committer ou supprimer selon décision.
- [ ] CI sécurité **plus faible que documenté** : `gosec` tourne avec `-no-fail` (SARIF only, **ne bloque jamais**) dans `security-scanning.yml` et `security.yml` ; **`govulncheck` absent de tous les workflows** ; **`gitleaks` absent de tous les workflows** ; `npm audit --production` est bloquant (bon point) ; `go test -race` est bloquant dans `ci.yml` (bon point).
- [x] **Build frontend** — ✅ corrigé le 07/07/2026 (commit `4e5b91f5`, branche `fix/frontend-build-typescript-errors`). `npm run build` (`tsc -b && vite build`) : 0 erreur. Note : il n'existe pas de script `npm run typecheck` séparé — `build` fait déjà tout le type-checking via `tsc -b`, donc `PROJECT_PLAN.md §3` doit être lu comme `npm run build` seul pour le frontend.
- [ ] **Nouveau gap découvert en corrigeant le build (07/07/2026), non traité — hors scope de ce fix :** `npm run lint` remonte **~350 problèmes** (330 erreurs, 20 warnings), majoritairement `@typescript-eslint/no-explicit-any` (viole la règle §1 « zéro any ») et variables non utilisées, répartis sur des dizaines de fichiers non touchés par le fix du build (`RiskTimeline.tsx`, `RoleManagement.tsx`, `TenantManagement.tsx`, `TokenManagement.tsx`, `Users.tsx`, `types/risk.ts`, `types/mitigation.ts`, `utils/*.ts`, etc.). Confirmé pré-existant (`git stash` avant fix). `npm run lint` n'est pas dans la CI bloquante actuelle.
- [ ] **Nouveau gap découvert le 07/07/2026 — hors scope de ce fix :** 7 fichiers de tests frontend (`vitest`) échouent déjà sur `master` (confirmé par `git stash`) : `notifications.test.tsx`, `PermissionGates.test.tsx`, `App.integration.test.tsx` (3 tests), `useRiskStore.test.ts` (1 test), `Login.test.tsx` (1 test), `CreateRiskModal.test.tsx`, `EditRiskModal.test.tsx`. 47 tests passent, 7 échouent sur 11 fichiers.
- **Acceptation (pas encore atteinte) :** CI bloquante sur gosec/govulncheck/gitleaks, racine propre, `npm run build` vert (✅ fait) ; lint et suite de tests frontend encore rouges.
- [ ] **Backlog ajouté 07/07/2026 :** rétrofit du client TypeScript **généré depuis OpenAPI** (`openapi-typescript`, mis en place pour Compliance dans M1) sur les types **Risk et Mitigation**, actuellement écrits à la main (`src/types/risk.ts`, `src/types/mitigation.ts`). Décision explicite : pas fait en même temps que M1 pour ne prendre aucun risque de régression sur des modules qui fonctionnent déjà — à faire en tâche dédiée séparée.
- [ ] **Nouveaux gaps backend découverts le 07/07/2026 (branche `feat/m1-compliance-engine`), confirmés pré-existants via `git stash`, hors scope compliance :**
  - `TestRiskCRUDFlow` (`backend/internal/handler/risk_handler_test.go`) échoue seul (`expected 201 got 400`) : le payload de test utilise `probability: 4`, hors du domaine `[0,1]` que `CreateRiskUseCase` valide désormais — données de test obsolètes.
  - `TestSetupMFA_Success` (`backend/internal/application/auth/mfa_usecase_test.go`) échoue dans cet environnement : `failed to encrypt secret: key must be exactly 32 bytes, got 31` — dépend d'une variable d'env de longueur incorrecte, pas un bug de code.
  - `TestStartAndStop` (`backend/internal/infrastructure/workers/sync_engine_test.go`) — data race déjà documentée plus haut dans cette ROADMAP.
- [ ] **Nouveau gap infra découvert le 07/07/2026 :** sur une base Postgres fraîche, le serveur ne démarre pas — `database.DB.AutoMigrate(&domain.User{}, ...)` dans `cmd/server/main.go` échoue avec `relation "users" does not exist` car GORM tente de créer la table `organizations` (association implicite du modèle `User`) avant `users`. Bloque toute vérification manuelle live tant que ce n'est pas corrigé (réordonner l'`AutoMigrate` ou migrer `Organization` séparément).

---

## 3. WAVE 1 — LA POINTE : devenir déjà le meilleur GRC francophone africain

> Objectif vérifiable : **un responsable conformité d'une banque/PME CEMAC ou UEMOA peut, seul, monter un programme COBAC ou ISO 27001 et produire un rapport défendable.** Tant que ce parcours n'est pas bout-en-bout, on ne touche pas à la Wave 2.

**Ordre imposé (chaque jalon dépend du précédent) :**

### M1 — Compliance Frameworks (moteur générique) ✅ fait le 07/07/2026 (branche `feat/m1-compliance-engine`)
Dépend de : 2.2 (✅).
- [x] Migration + domaine + repository tenant-scopé (voir §2.2).
- [x] Use cases (13, `backend/internal/application/compliance/`) : frameworks (create/get/list), contrôles (create/get/list/update/delete), preuves (upload/download/list/delete — **vrai fichier**, pas juste une URL, via `backend/pkg/storage` : interface `Storage` + `LocalStorage` disque, prêt pour un driver S3 futur sans changer les call sites), calcul de progression/% de conformité.
- [x] Handlers HTTP (`compliance_handler.go`, 13 endpoints) + routes + RBAC **granulaire** dans `main.go`/`permission.go` : 3 ressources distinctes (`ComplianceFramework`/`ComplianceControl`/`ComplianceEvidence`) au lieu d'une seule, pour qu'un Analyst avec droit "créer un contrôle" n'hérite pas du droit "créer un référentiel global" — Framework Create est de facto admin-only (Analyst/Viewer n'ont que Read), Delete sur contrôle/preuve est admin-only (intégrité de piste d'audit).
- [x] OpenAPI (`docs/openapi.yaml`, tag `Compliance`, 13 paths + schémas) + client TypeScript **généré** (`openapi-typescript`, `npm run generate:api-types` → `src/types/openapi.generated.ts`) — satisfait réellement la règle contract-first pour ce module (Risk/Mitigation restent à rétrofiter, voir §2.4).
- [x] Feature frontend `compliance` complète : liste des référentiels, table de contrôles avec changement de statut optimiste, jauge de conformité animée, drawer détails/preuves avec vrai widget d'upload, création de contrôle/référentiel (react-hook-form + zod), 3 états UI partout, i18n FR/EN complet, RBAC miroir côté client (bouton "créer un référentiel" caché hors admin).
- [x] Tests : 14 tests repository + 25 tests use case (Success/NotFound/CrossTenant/Conflict/validation) + tests handler incluant `TestComplianceE2EFlow` (parcours complet admin+analyst via de vraies requêtes HTTP contre un vrai handler/repo/stockage local) et des preuves cross-tenant dédiées (contrôle et téléchargement de preuve). `go build/vet` propres, `npm run build` propre.
- [x] Bug corrigé au passage : le doublon `reference_code`/`name+version` était vérifié uniquement en pré-check applicatif (fenêtre de concurrence) — le repository détecte maintenant `gorm.ErrDuplicatedKey` (nécessite `TranslateError: true`, activé dans `database.go`) et le traduit en conflit typé, garantie DB en plus du pré-check.
- **Acceptation : atteinte.** Un tenant peut créer un référentiel (admin), instancier des contrôles, changer leur statut, déposer/télécharger une preuve, voir la jauge de conformité — bout en bout dans l'UI, prouvé par `TestComplianceE2EFlow`.
- **✅ Vérification manuelle live complète le 08/07/2026** (même branche, DB fraîche, Postgres+Redis+backend+frontend réels, navigateur headless réel) : login → `/compliance` → création d'un référentiel via l'UI → jauge "0% compliant" → état vide "No controls" — bout en bout, zéro erreur console, zéro requête réseau en échec. La tentative précédente (07/07/2026) avait révélé un `AutoMigrate` cassé sur base fraîche ; en creusant pour terminer cette vérification, il s'est avéré que l'app n'avait **jamais réussi un login de bout en bout** dans cet environnement — 8 bugs pré-existants corrigés dans la foulée (aucun lié à la logique métier Compliance elle-même) :
  1. `AutoMigrate` : cycle implicite `User.DefaultOrg` ↔ `Organization.Owner` (GORM tentait de créer `organizations` avant `users`) → `constraint:-` des deux côtés + `DisableForeignKeyConstraintWhenMigrating: true` (16 modèles interdépendants, le tri topologique de GORM n'est pas fiable à cette échelle).
  2. `AutoMigrate` plantait fatalement sur `domain.Connector`/`MarketplaceApp`/`ConnectorUpdate`/`MarketplaceLog` (aucun tag `gorm:`, jamais migrables) — exclus de l'appel (cohérent avec leur statut 🟡 Marketplace ci-dessous).
  3. `migrations.RunMigrations()` (SQL versionné) s'exécutait **avant** `AutoMigrate` alors que ces migrations référencent des tables que seul `AutoMigrate` crée — réordonné.
  4. Login/register câblés sur `SimplePasswordHasher` (`Hash()`/`Verify()` déprécié, toujours en échec par construction) → `Argon2idPasswordHasher` (déjà utilisé par `SeedAdminUser`, maintenant cohérent).
  5. `SeedAdminUser` ne créait ni Organization ni OrganizationMember, or `LoginUseCase` exige une organisation par défaut → admin de bootstrap désormais rattaché à une org "OpenDefender" avec rôle `root`, comme le fait déjà `register.go` pour un signup normal.
  6. `TokenManager.GenerateTokenPair` chargeait des clés RSA en dur sur un chemin factice (`/path/to/private.pem`) → panic systématique après authentification réussie. Clés désormais injectées au constructeur depuis la config réelle.
  7. `useAuthStore.ts` (frontend) attendait `{token, user, expires_in}` alors que le backend renvoie `{user, token_pair, organization}` avec `role` imbriqué → store aligné sur le contrat réel, `role` aplati en chaîne.
  8. Toutes les routes `/compliance/*` utilisaient `middleware.RequirePermissions` (legacy, lit `*domain.UserClaims`) alors que le middleware RS256 réellement branché sur `protected` stocke un type différent → 401 "user context not found" sur **toute** requête compliance, peu importe l'utilisateur. Remplacé par `middleware.RequirePermission` (le bon, déjà existant) avec des permissions `compliance:{frameworks,controls,evidences}:{read,create,...}` ; `domain.PermissionSet` n'accorde `"*"` qu'à root/admin aujourd'hui (pas encore de règles Profile par ressource pour la conformité), donc c'est admin/root-only en pratique jusqu'à extension du modèle de permissions — pas de régression par rapport à avant (avant, c'était 100% cassé pour tout le monde).
  - Suite de tests complète relancée après coup : aucune régression, `TestComplianceE2EFlow` toujours au vert, les deux seuls échecs restants (`TestSetupMFA_Success`, `TestRiskCRUDFlow`) sont les échecs pré-existants déjà documentés plus bas.
- **✅ Dashboard 401/déconnexion forcée corrigé le 08/07/2026** (même branche, signalé en direct par l'utilisateur après un test manuel : "login → dashboard → déconnexion vers /login"). Trois causes distinctes, toutes indépendantes de Compliance :
  9. Clé de contexte Fiber incohérente : `AuthMiddlewareRS256` ne posait que `c.Locals("user_id"/"tenant_id")` (snake_case), mais `analytics_handler.go`, `enhanced_dashboard_handler.go`, `rbac_*_handler.go` et `token_handler.go` lisent `c.Locals("userID"/"tenantID")` (camelCase) — `/analytics/*` répondait 401 "unauthorized" pour **tout** utilisateur authentifié. Corrigé en posant les deux variantes de clé dans le middleware (`auth.go`, les deux fonctions RS256) plutôt que de retoucher chaque handler.
  10. `/risks` (lecture ET écriture) utilisait le même bug que le point 8 ci-dessus (`middleware.RequirePermissions` legacy) — même correctif appliqué (`middleware.RequirePermission("risks:{read,create,update,delete}")`).
  11. **Cause racine de la déconnexion forcée elle-même :** l'intercepteur axios (`frontend/src/lib/api.ts`) redirigeait vers `/login` sur **n'importe quel** 401, y compris ceux de widgets qui ont déjà leur propre repli gracieux (`SecurityScore.tsx`/`AssetStatistics.tsx` affichent une donnée de démo par défaut en cas d'échec) — la redirection globale écrasait ce repli. Seul le middleware d'auth RS256 pose un champ `code` (`TOKEN_EXPIRED`/`TOKEN_REVOKED`/`TOKEN_INVALID`/`UNAUTHORIZED`) sur ses 401 ; aucun des autres chemins 401 cassés (legacy, bug de clé, route absente) ne le fait. L'intercepteur ne déclenche plus la déconnexion que si ce `code` est présent — un 401 isolé sur un widget non critique n'éjecte plus l'utilisateur.
  - Vérifié en direct (formulaire de login réel, pas d'injection) : login → reste sur `/` pendant 6s d'observation, zéro redirection forcée, dashboard affiché avec données réelles (Risk Distribution, Top Vulnerabilities, etc. en état vide correct).
  - **Gap restant, non corrigé (nouvelle feature, pas un bug) :** `/api/v1/analytics/security-score` et `/api/v1/analytics/assets/statistics` n'existent tout simplement pas côté backend (aucune route enregistrée) — `SecurityScore.tsx`/`AssetStatistics.tsx` appellent des endpoints qui n'ont jamais été implémentés. Sans conséquence sur l'UX maintenant (repli gracieux + plus de déconnexion forcée), mais à combler si ces deux widgets doivent un jour afficher de vraies données.

### M2 — Contenu réglementaire africain (le moat) ✅ ISO 27001 fait le 08/07/2026 (branche `feat/m1-compliance-engine`) + BCEAO/ANTIC-CM/COBAC faits le 08/07/2026 (branche `fix/dashboard-crash-mitigation-routes-and-ui-polish`)
Dépend de : M1.
- [x] **ISO/IEC 27001:2022** : moteur de catalogue générique (`backend/pkg/compliance`, extensible — ajouter un référentiel = un nouveau fichier + `register()`, rien d'autre à toucher) + les 93 contrôles Annexe A (37 Organizational + 8 People + 14 Physical + 34 Technological), chacun avec `reference_code`/`name`/`description` (texte original, pas le texte protégé de la norme) et `source_reference` citant `"ISO/IEC 27001:2022, Annexe A, A.X.Y"`. Nouveau champ `source_reference` sur `ComplianceControl` (migration `0029`). Nouveaux endpoints `GET /compliance/catalogs` (liste, y compris les référentiels pas encore disponibles) et `POST /compliance/frameworks/{id}/import-catalog` (idempotent — relancer après extension d'un catalogue ne recrée pas l'existant), mêmes permissions que la création de référentiel (admin/root). Frontend : modale "Importer un catalogue" dans `/compliance`, citation de la source visible dans le tiroir de détail d'un contrôle, i18n FR/EN complet.
- [x] **BCEAO/UEMOA, ANTIC-CM, COBAC** : **modélisés le 08/07/2026 dès que l'utilisateur a fourni les textes source** (les 3 PDF réglementaires), levant le blocage « textes source manquants ». Trois catalogues cités article par article, même moteur qu'ISO 27001 (un fichier + `register()`), `Available: true` : `bceao` (35 contrôles — Règlement n°15/2002/CM/UEMOA relatif aux systèmes de paiement + Instructions 127-07-08 surveillance / 008-05-2015 monnaie électronique / 009/07/RSP/2010 CIP + Avis 001-09-2012 e-relevés ; `catalog_bceao_2002.go`), `antic-cm` (25 contrôles — Loi n°2010/012 du 21 décembre 2010 cybersécurité/cybercriminalité Cameroun, régulateur ANTIC ; `catalog_antic_cm_2010.go` ; remplace l'ancien placeholder mal attribué `anssi-cm`), `cobac` (45 contrôles — Règlement COBAC R-2016/04 contrôle interne CEMAC ; `catalog_cobac_2016.go`). Descriptions = résumés originaux (pas le texte réglementaire verbatim), `source_reference` citant l'article exact (ex. `Règlement COBAC R-2016/04, art. 48`). **Décision d'honnêteté conservée** : un seul placeholder subsiste (`cm-loi-2024-017`, protection des données personnelles Cameroun), texte non fourni → `Available: false`, pour garder l'affichage "Bientôt disponible" et le garde-fou d'import indisponible.
- [x] Test de cohérence "aucun contrôle orphelin" : `pkg/compliance/catalog_test.go` (`TestNoOrphanControls`) vérifie que chaque contrôle d'un catalogue disponible a un `reference_code`/`name`/`source_reference` non vide et qu'il n'y a pas de code dupliqué ; vérifie aussi qu'un catalogue marqué indisponible n'a bien aucun contrôle.
- **Acceptation atteinte** : ISO 27001 (93 contrôles, vérifié live) + BCEAO (35) + ANTIC-CM (25) + COBAC (45) = 198 contrôles cités, tous importables via le même flux. `TestNoOrphanControls` passe sur les 4 catalogues. Reste à faire : une passe de revue par un juriste/spécialiste conformité sur la formulation des 3 nouveaux catalogues (codes d'article fiables, relevés dans les textes fournis ; wording à confirmer) — noté en tête de chaque fichier catalogue.
- **✅ Vérification manuelle live le 08/07/2026** : création d'un référentiel "ISO 27001"/"2022" via l'UI → clic "Importer un catalogue" → les 3 référentiels africains apparaissent bien grisés "Bientôt disponible", ISO/IEC 27001 2022 apparaît disponible avec "93 controls" → import → toast "93 controls imported (0 already present)" → les 93 contrôles apparaissent réellement dans le tableau → ouverture d'un contrôle (A.5.1) confirme la citation `ISO/IEC 27001:2022, Annexe A, A.5.1` affichée. Zéro erreur console, zéro requête en échec.
- **🔴 Bug fondamental découvert et corrigé pendant cette vérification, sans lien avec le contenu réglementaire lui-même :** `middleware.SetContext()` (qui alimente `middleware.GetContext(c)`, lu par **8 fichiers handler** — `compliance_handler.go`, `risk_handler.go`, `asset_handler.go`, `mitigation_handler.go`, `mitigation_subaction_handler.go`, `dashboard_handler.go`, `multitenancy_auth_handler.go`, `multitenancy_org_handler.go`) **n'était appelé nulle part dans le code de production** — uniquement dans le harnais de test (`compliance_handler_test.go`). Résultat : `tenantID(c)`/`userID(c)` retournaient silencieusement `uuid.Nil` pour **toute requête en production**, sur ces 8 modules, depuis toujours. Le garde-fou `if control.TenantID == uuid.Nil { return err }` dans `GormComplianceRepository.CreateControl` (déjà présent, pas ajouté par moi) est ce qui a fini par transformer ce bug silencieux en 500 visible, au tout premier `CreateControl` de l'import. Corrigé en une ligne dans `AuthMiddlewareRS256` (`internal/middleware/auth.go`) : `SetContext(c, &RequestContext{UserID: claims.Sub, OrganizationID: claims.TenantID})`, juste à côté des autres `c.Locals(...)`. Revérifié en direct après coup : `POST /api/v1/risks` renvoie maintenant un `tenant_id` réel (celui de l'org de l'admin) au lieu de `00000000-0000-0000-0000-000000000000`. **Implication documentaire :** toute affirmation passée comme quoi Risk/Asset/Mitigation/Dashboard "fonctionnent" doit être re-vérifiée — ils tournaient tous avec un tenant Nil jusqu'à cette correction, ce qui, avec plusieurs tenants réels en base, aurait mélangé les données de tout le monde dans un même compartiment fictif.
  - Gap plus mineur repéré au passage, non corrigé (hors scope) : `CreateRisk` ne renseigne pas `created_by` depuis le contexte réel (reste `uuid.Nil`) — le handler ne lit `mwCtx` que pour `organization_id`, jamais pour `UserID`. À vérifier/corriger séparément.
  - Suite de tests complète relancée après cette correction : aucune régression (`TestRateLimit_DifferentIPs` a échoué une fois puis repassé au vert en isolation — flaky, pas lié à ce changement).

### M3 — Assets (inventaire autonome) ✅ fait le 08/07/2026 (branche `feat/m3-assets-inventory`)
Finir le partiel §1.2.
- [x] Table `assets` dédiée (`OrganizationID`/`TenantID`, dans `AutoMigrate()`), relation risque↔actif (`risk_assets`).
- [x] **Clean Architecture rétrofitée** : `asset_handler.go` ne touchait jamais `database.DB` — direct SQL dans le handler, aucun use case, `POST /assets` accessible à n'importe quel rôle authentifié (aucun `RequirePermission`). Remplacé par le même pattern que Compliance (M1) : `domain.AssetRepository` (interface) + `GormAssetRepository` (tenant-scoped, 10 tests repository — Success/NotFound/CrossTenant sur Get/Update/Delete) + 6 use cases (`Create/Get/List/Update/Delete/ListSnapshots`, 20 tests use case) + handler fin + routes `protected` avec `assets:{read,create,update,delete}` (`main.go`).
- [x] **Snapshots historiques** : nouvelle entité `domain.AssetSnapshot` (table `asset_snapshots`, dans `AutoMigrate()`) — `UpdateAssetUseCase`/`DeleteAssetUseCase` capturent l'état complet de l'actif (nom/type/criticité/propriétaire) juste avant chaque modification/suppression. Endpoint `GET /assets/{id}/history` + tiroir `AssetHistoryDrawer` côté frontend (timeline, motif update/delete).
- [x] **Criticité branchée sur le Score Engine** — le vrai gap, plus profond que documenté :
  - `domain.AssetCriticality.ScoreFactor()` (nouveau, `asset.go`) : mapping canonique unique LOW=0.5/MEDIUM=1.5/HIGH=2.5/CRITICAL=3.0 (plage `[0.1, 3.0]` de la formule officielle, CLAUDE.md). Avant ce fix, **trois** mappings différents coexistaient (`get_score_breakdown.go` inline, `score_service.go` 0.8/1.0/1.25/1.5, `score_engine_service.go` idem) — et les deux use cases canoniques `CreateRiskUseCase`/`UpdateRiskUseCase` n'appliquaient **aucune** pondération de criticité (`Score = Impact × Probability`, commentaire "score engine can override later" jamais honoré).
  - **Bug critique découvert et corrigé** : `GormRiskRepository.GetRisksByAssetID` (utilisé par le flux Redis `asset.criticality_changed` → `ScoreWorker`, jamais atteint en pratique car rien ne publiait cet événement) scannait `assets.criticality` (varchar `"HIGH"`) directement dans un champ `float64` — aurait paniqué/erroré au premier appel réel. Corrigé + ne considère plus un seul asset lié à un risque mais fait la moyenne de `ScoreFactor()` sur tous les assets liés (4 tests de régression).
  - `AssetHandler.UpdateAsset` publie désormais l'event Redis `asset.criticality_changed` quand la criticité change → `ScoreWorker` (déjà existant, jamais câblé côté émission) recalcule chaque risque lié via le vrai `pkg/scoring.Engine`.
  - `risk_handler.go` : `CreateRisk`/`UpdateRisk` publient `risk.updated` avec la vraie criticité moyenne des assets liés (plus un `1.0` codé en dur) ; `UpdateRisk` ne fait plus de calcul de score synchrone via `service.ComputeRiskScore` (violait la Règle #12 "jamais de calcul de score dans le handler") — ce chemin est supprimé, `score_service.go`/`score_service_test.go` supprimés (devenus totalement morts). `get_score_breakdown.go` fait maintenant la moyenne sur tous les assets liés au lieu du premier seul (4 tests, dont 1 test qui n'existait pas avant ce fix — ce use case n'avait aucune couverture).
- [x] Feature frontend dédiée `src/features/assets/` (store Zustand UI-only, hooks React Query `useAssets`/`useAssetHistory`, `AssetsPage`/`CreateAssetModal`/`EditAssetModal`/`AssetHistoryDrawer`, react-hook-form+zod, 3 états UI, optimistic updates sur update/delete, i18n FR/EN complet) — remplace les pages `Assets.tsx`/`useAssetStore.ts` (celui-ci reste utilisé tel quel par `CreateRiskModal`/`EditRiskModal`/`DashboardGrid` pour le sélecteur d'assets d'un risque, volontairement non touché, même décision que le retrofit OpenAPI Risk/Mitigation différé en M1).
- [x] OpenAPI (`docs/openapi.yaml`) : `GET/PATCH/DELETE /assets/{id}`, `GET /assets/{id}/history`, schémas `UpdateAssetInput`/`AssetSnapshot` + client TS régénéré.
- **Acceptation : atteinte.** Inventaire CRUD complet avec historique, tenant-scoped, testé (38 tests neufs/modifiés : 10 repository + 20 use case + 4 régression `GetRisksByAssetID` + 4 `GetScoreBreakdown`) ; criticité d'un actif alimente réellement le Score Engine via le flux événementiel Redis existant (jusque-là mort faute d'émetteur). `go build/vet/test` propres (mêmes 2 échecs pré-existants et déjà documentés : `TestSetupMFA_Success`, `TestRiskCRUDFlow`) ; `npm run build` propre ; suite vitest inchangée (47 passent/7 échouent, mêmes 7 fichiers pré-existants).
- **Non fait, hors scope de ce fix :** vérification manuelle live en navigateur (fait pour M1/M2, pas encore ici) ; `src/hooks/useAssetStore.ts` reste un chemin de données non typé/non contract-first pour les sélecteurs de risque.

### M4 — Reporting officiel + Board Report
- [ ] Export PDF soigné ; **rapport COBAC/BCEAO en 1 clic** ; **Board Report mensuel** (IA, human-in-the-loop, ton non-technique, montants en **FCFA**).
- **Acceptation :** PDF généré, relisible et validable avant diffusion.

### M5 — Incident Management (finir le partiel)
- **Constat (07/07/2026) :** le service (`incident_service.go`) et le handler font déjà de vrais appels GORM, et l'UI existe (`Incidents.tsx`, `useIncidentStore.ts`) — mais **`domain.Incident` est absent d'`AutoMigrate()` dans `main.go` et aucune migration ne crée la table `incidents`**. Le module a l'air fini en lisant le code mais échoue probablement au runtime.
- [ ] Ajouter `domain.Incident` (+ `timeline_entries`) à `AutoMigrate()` ou créer une migration dédiée dans `backend/migrations/`.
- [ ] Vérifier en conditions réelles (DB locale) que le cycle de vie incident fonctionne de bout en bout ; lien incident↔risque.
- **Acceptation :** cycle de vie d'incident complet et persisté — probablement une correction rapide (migration manquante), pas une réécriture.

### M6 — Offline-first (différenciateur Afrique)
- [ ] Service Worker (Workbox) + IndexedDB + file de synchronisation + résolution de conflits, sur les 3 écrans signature (dashboard, risques, compliance).
- **Acceptation :** l'app reste utilisable sans réseau et resynchronise proprement.

### M7 — Polir les 3 écrans signature (qualité « parfaite »)
- [ ] Dashboard, Risk Register, Board Report : < 100 ms perçu, palette ⌘K, états vides soignés, zéro saut de layout, dark mode, WCAG 2.1 AA.
- [ ] **Cohérence de la navigation entre tous les modules livrés** (ajouté 07/07/2026) : chaque module (Risks, Mitigations, Compliance, Assets, …) a été construit indépendamment au fil de l'eau, avec son entrée de nav ajoutée au fil des sessions sans plan d'ensemble — ex. `Mitigations` n'a même pas d'entrée dans `Sidebar.tsx` aujourd'hui. Faire une passe dédiée sur l'ordre/l'organisation de la nav une fois tous les modules du Wave 1 livrés, pas module par module.
- **Acceptation :** revue design dédiée ; aucune aspérité sur ces 3 écrans.

> **Fin de Wave 1 = jalon majeur :** OpenRisk est un vrai GRC, déjà numéro un sur le créneau africain. C'est ici qu'on peut commencer à chercher 3 institutions pilotes (pas avant).

---

## 4. WAVE 2 — LES DIFFÉRENCIATEURS (le moat)

> Ne démarrer qu'après Wave 1 ✅. Ordre par ratio valeur/effort.

- [ ] **2.1 Regulatory Content-as-Code + Diff Engine** — référentiels versionnés ; un changement de version génère diff + tâches + alertes ciblées aux tenants concernés.
- [ ] **2.2 Audit Defensibility Layer** — chaîne de preuves hash-chainée **append-only** (`pkg/auditchain`) ; export vérifiable par un tiers ; **portail auditeur** en lecture seule traçable. *(Construire la vraie table append-only `admin_audit_events` ici, en finissant le 🟡 PAM.)*
- [ ] **2.3 AI Control-Mapping & Gap Analysis** — mapping des politiques existantes vers contrôles, **avec justification + citation source**, human-in-the-loop obligatoire.
- [ ] **2.4 Cross-Framework Harmonization** — « implémente une fois, satisfais N référentiels » ; inclure les référentiels africains dans le graphe.
- [ ] **2.5 Continuous Compliance from Scans** — CTI Engine (NVD/CISA KEV/MITRE) + auto-création/clôture de risques.
- [ ] **2.6 CRQ / FAIR en FCFA** — quantification financière du risque, multidevise.
- [ ] **2.7 AI Regulatory Assistant (RAG)** — réponses réglementaires sourcées, jamais inventées.

---

## 5. WAVE 3 — LA PLATEFORME (effets de réseau)

> Tout le reste de l'ambition, à sa juste place.

- [ ] Vendor Risk · Policy Management (cycle + attestation) · BCP/PCA-PRA · Security Awareness Training · Trust Center public
- [ ] Data Discovery · Access Review & Certification
- [ ] Risk Digital Twin · Collaborative War Room (SSE) · Attack Path Graph
- [ ] Risk Champions (finir la gamification : tables + badges + leaderboard) · Plugin & Template Marketplace (finir)
- [ ] **Billing/Stripe + plans** (Community/Pro/Business/Enterprise) — prérequis monétisation
- [ ] Super Admin · Feature Flags
- [ ] ⭐ **Supervisor/Regulator Portal** (attestations standardisées vers COBAC) — la pièce « infrastructure de marché »
- [ ] ⭐ **Anonymized Benchmarking** (k-anonymat, conforme Loi 2024/017)

---

## 6. RÈGLES D'EXÉCUTION (à respecter à la lettre)

1. **Ordre non négociable :** Wave 0 → 1 → 2 → 3. À l'intérieur de Wave 1, M1 → M7 dans l'ordre.
2. **Definition of Done** d'un module : tests CI verts ; isolation multi-tenant prouvée par test (404 cross-tenant) ; aucun secret en clair ; i18n FR/EN ; états loading/error/empty ; OpenAPI à jour ; si conformité, validation human-in-the-loop tracée. Sinon → 🟡.
3. **Jamais ✅ sans preuve.** Pas de fichier d'auto-félicitation. `ROADMAP.md` + `CHANGELOG.md` mis à jour à chaque fin de module.
4. **Discipline IA :** toute ligne générée (surtout sécurité + contenu réglementaire) est comprise, testée et défendable avant d'être « faite ».
5. **Anti-dérive :** ne pas démarrer un module « waouh » (Digital Twin, War Room, Attack Path) tant que Compliance Frameworks + contenu africain ne sont pas ✅. Le créneau d'abord.
   **Constat 07/07/2026 :** du code CTI Engine (Wave 2 §2.5) existe déjà sur la branche `fix/cti-build-conflict` (`pkg/cti/` complet). C'est une entorse à cette règle — le CTI est du Wave 2, M1/M2 ne sont pas ✅. Ne pas approfondir le CTI davantage avant que M1+M2 soient faits ; se limiter à faire compiler proprement ce qui existe déjà si c'est l'objet de la branche en cours, sans ajouter de nouvelle portée.

---

## 7. TABLEAU DE BORD CONSOLIDÉ

| Vague | Contenu | Statut global |
|---|---|---|
| Socle existant | Risk, Mitigation, Notif, Score, Dashboard, Audit ✅ · Auth/RBAC multitenant : code présent, login+dashboard **vérifiés live 08/07/2026** après correction de 11 bugs (voir §3 M1), sous-flux (MFA/OAuth2/SAML2/refresh) encore non prouvés | 🟡 vérifié en partie |
| **Wave 0** | Licence ✅ · schéma Compliance ✅ **corrigé 07/07** (bug cross-tenant) · AES 🟡 (MFA seulement) · README/hygiène 🟡 · CI sécu 🟡 (gosec non-bloquant, govulncheck/gitleaks absents) · build frontend ✅ **corrigé 07/07** · lint frontend 🔴 (~350 findings pré-existants) · tests frontend 🟡 (7/11 fichiers en échec, pré-existants) | 🟡 en cours, **pas vert** |
| **Wave 1** | **M1 Compliance engine ✅ fait + vérifié live 08/07/2026** (voir §3) + **M2 ✅ fait 08/07/2026** : ISO 27001 (93 contrôles, vérifié live) **+ BCEAO/UEMOA (35) + ANTIC-CM (25) + COBAC (45)** cités article par article (voir §3) + **M3 Assets ✅ fait le 08/07/2026** (Clean Architecture + snapshots + criticité→Score Engine, voir §3) + Reporting/Board (❌, M4 suivant) + Incident (🟡, table manquante) + Offline (❌) + 3 écrans parfaits (❌, + nav coherence ajoutée) | 🟡 M1+M2+M3 faits, priorité = M4 (Reporting/Board, peut désormais s'appuyer sur les catalogues COBAC/BCEAO) |
| Wave 2 | 7 différenciateurs (moat) — **CTI Engine déjà entamé en avance de phase**, à ne pas approfondir avant M1/M2 | 🟡 entorse anti-dérive détectée |
| Wave 3 | Plateforme + écosystème + monétisation + portail régulateur | ❌ après Wave 2 |

**La phrase à garder :** OpenRisk a maintenant un vrai *Compliance engine* bout-en-bout (M1) **et** son contenu réglementaire moat chargé (M2 : ISO 27001:2022 vérifié live, **plus les référentiels africains BCEAO/UEMOA, ANTIC-CM et COBAC** modélisés le 08/07/2026 dès réception des textes source — 198 contrôles cités au total) — pas seulement un excellent *Risk Register*. Ne reste en attente de texte source que le cadre camerounais de protection des données personnelles (placeholder assumé). Un bug fondamental de multi-tenancy (`middleware.SetContext` jamais appelé en production, tenant_id silencieusement `Nil` sur 8 modules) a été découvert et corrigé le 08/07/2026 pendant cette vérification — voir §3 M2 pour le détail ; toute affirmation de statut antérieure à cette date sur Risk/Asset/Mitigation/Dashboard doit être considérée comme non fiable sur l'isolation multi-tenant tant qu'elle n'a pas été revérifiée.

---

*ROADMAP fondée sur l'état réel du code au 26/06/2026, rafraîchie par inspection directe + exécution (build/vet/test) le 07/07/2026, puis par vérification live (navigateur réel) le 08/07/2026. À mettre à jour à chaque fin de module, avec preuve.*
