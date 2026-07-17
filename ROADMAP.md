# OpenRisk — ROADMAP (source de vérité unique)

> **Aligné sur le Master Prompt V5.0** (avril 2026). Réécrit et re-cartographié le **2026-07-10**.
> Mission : devenir le standard mondial du GRC, en commençant par la France, la Belgique, le Maghreb
> et l'Afrique subsaharienne (marchés **COBAC / BCEAO / ANSSI / ANTIC**). Concurrents directs :
> Vanta, Drata, OneTrust, ServiceNow GRC, Archer.
> Vision détaillée : `docs/MASTER_PROMPT_V4.md` (le **V5** la supersède — déposer le fichier V5 propre
> dans `docs/MASTER_PROMPT_V5.md`). Ce ROADMAP est la carte **module par module** du V5 avec le statut réel.

---

## Comment lire ce document

**Légende de statut**
- **✅ Fait** — implémenté ET prouvé (test live et/ou suite de tests verte + vérification manuelle documentée).
- **🟡 Partiel** — code présent mais incomplet, non câblé de bout en bout, ou jamais prouvé live.
- **❌ Absent** — aucun code, ou seulement mentionné ailleurs.
- **N/A** — étape ponctuelle (audit) ou méta.

**Règle d'or (répétée trois fois par l'histoire de ce projet, voir §2 « SetContext »)** :
aucun « ✅ » n'est accordé sans **preuve live**. Les affirmations « ça marche » du passé ont été
fausses sur `SetContext`, sur l'architecture réelle des Assets, et sur toute la chaîne
`RequireRole`/dashboard. Ne jamais faire confiance à un statut sans vérification quand l'enjeu est réel.

---

## Verdict global (2026-07-10)

- **Fondations GRC (Modules 0–13)** : le cœur métier est **livré et majoritairement prouvé live** —
  Score Engine, Risk Register, Mitigation, Compliance (ISO 27001 + catalogues africains), Assets,
  Dashboard/Analytics, Notifications, Reporting PDF + Board Report IA. Restent partiels : Auth
  au-delà du login (MFA/OAuth/SAML/refresh non prouvés), CTI (non câblé), Scanner (absent),
  IA Advisor complet (seule la fondation `pkg/ai` existe), SSE (pas de hub dédié).
- **Fonctionnalités avancées (Module 14.1–14.18)** : **1 partielle, 2 faites, 15 ❌.** Faits : 14.12
  Executive Board Report ; **14.1 Incident Management** (base — registre live + War Room sur incident réel,
  fait le 13/07/2026). Partiels : Custom Fields (14.8), PAM Audit Trail (14.9),
  Champions/Gamification (14.16), Plugin Marketplace (14.18). Tout le reste (Vendor, Policy, Trust
  Center, CRQ, BCP, Training, Access Review, Data Discovery, Digital Twin, Attack Path,
  Offline) est **non commencé** — c'est là que se trouve le **moat** vs Vanta/Drata.
- **Transversaux (15–17)** : Sécurité/Observabilité partiels, i18n fait, Billing/Super-Admin/
  Onboarding/Feature-Flags-gérables ❌.
- **Product Growth (Partie C)** : design system partiel, tout le go-to-market (pricing, landing,
  conversion, rétention) ❌.

En une phrase : **OpenRisk est déjà un GRC francophone/africain crédible et fonctionnel (Wave 0/1) ;
il n'est pas encore la plateforme différenciante du Master Prompt V5 (Wave 2/3).**

---

## 1. INVENTAIRE COMPLET DES MODULES V5 (ne rien oublier)

> Statut établi le 2026-07-10 par lecture directe du code (`backend/internal`, `backend/pkg`,
> `frontend/src/features`, `migrations`), pas par mémoire. « Preuves » = fichiers/faits vérifiés.

### 1.1 Fondations — Modules 0 à 13

| Module V5 | Statut | Preuves (code réel) | Ce qui reste |
|---|---|---|---|
| **0. Audit initial** (backend/sécu/frontend) | N/A | Étape ponctuelle d'analyse, réalisée de fait au fil des sessions. | Rejouer avant chaque grosse vague. |
| **1. Score Engine** | ✅ | `pkg/scoring/` pur (P×I×A, seuils 3 décimales), `scorer_test.go`, worker Redis `ScoreWorker`. Vérifié via M3 (flux `asset.criticality_changed`). | Rien de bloquant. |
| **2. Authentification (7 couches)** | 🟡 | JWT RS256 (`internal/auth`, `AuthMiddlewareRS256`), Argon2id, MFA (`domain/mfa.go`, `pkg/otp`), OAuth2 (`oauth2_handler.go`), SAML2 (`saml2_handler.go`), RBAC (`rbac.go`, `permission_service`), PAT (`token_service`), switch-org, audit auth. **Login+dashboard prouvés live 08/07.** | **MFA, OAuth2, SAML2, refresh-token, switch-org : non prouvés live.** `TestSetupMFA_Success`/`TestRiskCRUDFlow` en échec (pré-existants). Traiter chaque sous-flux comme non prouvé. |
| **3. Risk Register** | ✅ | Backend (`application/risk`, `gorm_risk_repository`) + frontend (`features/risks`) prouvés live. Filtres, full-text, bulk, import. **Identification** manuelle + auto (CTI `cti_auto`, scanner `scan_auto`, import). **Éval. qualitative** P×I×AC (Score Engine). **Éval. quantitative** CRQ `pkg/crq` (ALE = SLE×ARO en FCFA+USD) — désormais **exposée live** dans l'onglet « Financier » du drawer du registre. **Cycle de vie ISO 31000** (Identifier→Analyser→Évaluer→Traiter→Surveiller→Clôturer) porté sur l'entité `Risk` réelle : champ `lifecycle_phase`, `TransitionPhaseUseCase` tenant-safe + garde de transition + statut couplé, endpoint `POST /risks/:id/transition`, migration 0031, stepper « Cycle de vie » dans le drawer live. **Prouvé live 16/07** (transitions valides 200 / saut +2 invalide 400 / clôture→statut closed / réouverture→open / CRQ explicite 1 M FCFA). **`created_by` renseigné depuis le contexte réel** (vérifié live 16/07 : nouveau risque → `created_by` = id admin ; l'ancienne note « reste uuid.Nil » était périmée depuis le fix `SetContext` du 08/07 — seul le chemin CTI auto met volontairement `uuid.Nil`, « no human author »). **Legacy `/risk-management/*` SUPPRIMÉ** le 16/07 (branche `fix/remove-legacy-risk-management`) : 4 fichiers backend + câblage main.go + page/hook/service/composants front orphelins retirés. | Client OpenAPI non rétrofit (choix délibéré : le client typé écrit à la main fonctionne, rétrofit = risque sans valeur user). L'entrée d'audit `CreateAuditEntry`→`audit_logs` est un no-op pré-existant (schéma incompatible) ; les transitions sont tracées via `risk_histories` (hook AfterSave). `TestRiskCRUDFlow` rouge (pré-existant, confirmé au commit parent `62191679`). |
| **4. Mitigation Workflow** | ✅ | Backend (`application/mitigation`, sous-actions, progress→review) + Kanban frontend (`features/mitigations`). Routes d'écriture corrigées (bug `RequireRole`). | Auto-mitigation **détectée** par le scanner (diff findings scan N-1→N, onglet dédié du preview, prouvé live 14/07) ; **auto-complétion d'une sous-action** de plan reste à câbler sur ce signal. Vue Gantt à confirmer. |
| **5. CTI Engine** | 🟡 | `pkg/cti/` présent (NVD/CISA/MITRE, matcher CVE→asset). | **Non câblé** : pas de worker de sync émetteur, pas d'auto-création de risque active, endpoints non exposés. « En avance de phase Wave 2 ». |
| **6.5 Vulnerability Management** (intégrations + priorisation) | ✅ (prouvé live 16/07) | **Module dédié** (branche `feat/vulnerability-management`). Registre de vulnérabilités tenant-scoped `domain.Vulnerability` (CVE, CVSS, EPSS, KEV, exploit, criticité métier de l'actif, blast radius, statut de remédiation, priorité P1–P4 ; dans `AutoMigrate`) + repo Gorm (upsert par dedup, filtres/tri, stats). **Priorisation risk-based** `pkg/vulnprio` (pur, 8 tests) sur les 4 axes demandés : **CVSS** (0.40) · **exploitabilité** (0.30 — EPSS/KEV/exploit dispo, KEV plancher P1) · **criticité métier** (0.20 — facteur de l'actif lié) · **actifs concernés** (0.10 — blast radius log). **Intégrations** `internal/vulnscan` : normaliseurs pour **Nessus, OpenVAS, Qualys, Microsoft Defender, AWS Inspector, Azure Defender, CrowdStrike** (mapping du format natif → finding normalisé ; AWS Inspector = live-pull possible, les autres import + seam honnête). Ingest (normalise→priorise→upsert, lie l'actif par hostname/id), list/get/update-status/delete/stats (use cases 1-fichier tenant-safe + tests) → handler + routes `vulnerabilities:*` + `/vulnerability-connectors`. **Frontend** `features/vulnerabilities` (page registre triée par priorité + KPI + filtres, drawer détail avec explication de priorisation + cycle de statut, modal d'import multi-source, panneau connecteurs ; item sidebar « Vulnérabilités » ; tsc+vite verts). **Preuves live** (Postgres:5434) : 7 connecteurs ; ingest Nessus Log4Shell→CVE extraite/sévérité/actif web-01 CRITICAL/priorité ; CrowdStrike exploit→maturité high ; **KEV CVSS 5.0 → floored P1 (80)** ; **blast radius 3 actifs → 66.97** ; tri par priorité ; stats ; update-status ; delete 204/404 ; **capture headless de la page rendue**. | Enrichissement EPSS/KEV automatique depuis le moteur CTI (aujourd'hui les signaux viennent du finding ingéré) ; live-pull réel des 6 connecteurs REST (creds + polling) reste un seam honnête (AWS Inspector câblé) ; pas encore de lien vuln→risque auto. |
| **6. Infrastructure Scanner** (cloud + Agent on-prem) | ✅ (prouvé live de bout en bout) | **Complet 14/07** — backend `internal/scanner/` (interface `Scanner`, pipeline `Validate→Scan→Normalize→Deduplicate→StorePreview→Notify` qui **n'écrit jamais** Asset/Risk, preview Redis 48h, dedup, **auto-mitigation par diff**, isolation tenant, ~30 tests) + **collectors SDK réels** `internal/scanner/collectors` (**AWS** aws-sdk-go-v2 EC2/S3+chiffrement/Security Hub, **Azure** Resource Graph KQL, **GCP** Compute aggregated list ; creds AES-256-GCM déchiffrées au scan) + **binaire Agent** (module `agent/`, **6,5 Mo** stdlib pur : register token 24h → SSE jobs + heartbeat → **nmap `-sV --script vuln`** + osquery locaux → parse XML → **push JWT scopé + HMAC-SHA256** ; stateless ; scope ≤/24 ; `-install` systemd ; auto-update GitHub 24h) + **notif in-app + email** sur fin de scan + **frontend** `features/infrastructure` (console live, ScanConfigDrawer, AgentDeployModal, ScanPreviewPage Actifs/Vulns/Mitigations + import criticité éditable ; tsc+vite verts). **Preuves live** : creds cloud chiffrées + 3 SDK réellement appelés (EC2 401 / AAD auth / GCP parse) ; agent scanne 127.0.0.1/32 → **1 actif + 22 CVE réels** (OpenSSH) → push HMAC vérifié → job completed → notif unread 0→1 → révocation 401. | **Defender (Azure) + Security Command Center (GCP)** findings = SDK/paths supplémentaires (assets cloud OK, alerts cloud à ajouter). Livraison de release binaire Agent (self-replace) reste manuelle. |
| **7. SSE Real-time Engine** | 🟡 | `useSSE` côté frontend ; références SSE dans `notification_service.go` ; `pkg/events`. | **Pas de hub SSE dédié** (`infrastructure/sse/hub.go` absent), endpoint `/api/v1/stream` non confirmé. La route `/risks/events` attendue par le client n'existe pas (repli en log dev). |
| **8. Dashboard & Analytics** | ✅ | `application/analytics`, `analytics_service`, `dashboard_data_service`, `stats_handler`, `features/dashboard`. **Prouvé live 08/07** (crash dashboard corrigé). | 2 widgets (`SecurityScore`, `AssetStatistics`) appellent `/analytics/security-score` & `/analytics/assets/statistics` **inexistants** (repli gracieux). Cache Redis d'invalidation à confirmer. |
| **9. Notifications** | ✅ (cœur) | `pkg/notify`, `notification_service`, `notification_handler`, centre de notifs frontend. | Canaux **Email (Resend/SMTP), Slack, Webhook signé** non prouvés live. Webhooks sortants à vérifier. |
| **10. IA Advisor** | 🟡 | **Fondation `pkg/ai` créée** (10/07, 1er vrai client LLM du repo : interface `Advisor`, `ClaudeAdvisor` sur `anthropics/anthropic-sdk-go` modèle `claude-opus-4-8`, `TemplateAdvisor` fallback). `ai_risk_predictor_service`, `recommendation_service`. | Les use cases V5 (analyze/mitigations/deduplicate/prioritize/narrative/executive-summary), l'`AIAdvisorTab` dans le RiskDrawer, le streaming, le cache/rate-limit IA : **non faits** (stubs sans provider). Chemin `ClaudeAdvisor` non prouvé live (pas de clé). |
| **11. Reporting & Export** | 🟡 | `pkg/report` (PDF `fpdf`, **conformité + Board Report ✅**), export CSV risques (`export_risks.go`), `export_handler`. | Pas de `pkg/export` async (jobs Redis, XLSX `excelize`, MinIO/S3, TTL). **Templates officiels COBAC/BCEAO/ISO/PCI ❌.** `ReportsPage.tsx` = maquette non câblée. |
| **12. Compliance Frameworks** | ✅ (moteur + couverture large + audits + gap + remédiation) | Moteur M1 vérifié live ; **16 référentiels cités et importables** : ISO 27001:2022 (93) + **ISO/IEC 27005:2022 (19, processus de gestion du risque)** + **ISO 31000:2018 (22, principes/cadre/processus)** + **NIST CSF 2.0 (22)** + **NIST SP 800-53 Rev.5 (20 familles)** + **CIS Controls v8 (18)** + **PCI DSS 4.0 (12)** + **HIPAA Security Rule (22, cit. 45 CFR §164)** + **SOC 2 / AICPA TSC (51)** + **RGPD / UE 2016/679 (22 articles, FR)** + **DORA / UE 2022/2554 (19, 5 piliers, FR)** + **NIS2 / UE 2022/2555 (12, art. 21 mesures, FR)** + **SOX 2002 (10, sections + ITGC)** + BCEAO (35) + ANTIC-CM (25) + COBAC (45) — chacun cité article/section par contrôle (`TestNoOrphanControls` + `TestExpectedControlCounts`). Architecture plugin : un fichier `pkg/compliance/catalog_*.go` + `register()` → le catalogue remonte automatiquement dans `GET /compliance/catalogs` et devient importable, **zéro changement handler/frontend**. Frameworks **tenant-scoped** (migration 0030). **Gestion complète** : créer/importer/supprimer référentiel + contrôle (RBAC), **personnalisation** (référentiel vierge + contrôle ad-hoc), preuve en chip cliquable, **progression temps réel**, **seuil de preuve obligatoire (mode strict)**. **Analyse d'écarts (Gap Analysis) ✅ (16/07, branche `feature/compliance-management`)** : `GET /compliance/gap-analysis` (tous référentiels ou 1) → contrôles non satisfaits + roll-ups par référentiel ; page dédiée (jauge, filtres, écarts groupés) branchée sur le bouton « Voir les écarts » (auparavant inerte) ; « Remédier » par écart. **Audits ✅** : `domain.ComplianceAudit` (interne/externe/certification/surveillance ; planifié→en cours→terminé/annulé ; portée référentiel/programme ; auditeur/périmètre/résumé/score/dates), CRUD `/compliance/audits`, page de planification/exécution/historique. **Plans de remédiation ✅** : `domain.RemediationPlan` lié à un contrôle (l'écart) + à l'audit d'origine ; priorité/statut/assignation/échéance ; CRUD `/compliance/remediations`, page de suivi. **Auto-génération depuis un audit terminé ✅ (17/07)** : `POST /compliance/audits/:id/generate-remediations` ouvre un plan pour chaque écart du référentiel de l'audit (priorité déduite : not_implemented→high, in_progress→medium ; idempotent — saute les écarts déjà couverts) ; bouton « Remédier » par audit → toast créés/ignorés → registre. **Cross-mapping entre référentiels ✅ (17/07)** : `domain.ControlMapping` (crosswalk non-dirigé tenant-scoped entre deux contrôles + relation équivalent/partiel/lié), repo Gorm (Exists bidirectionnel, isolation tenant), use cases (Create = double garde cross-tenant + refus self/doublon, List enrichi, Delete) → `GET/POST /compliance/control-mappings` + `DELETE /:mappingId` ; section « Correspondances » dans le drawer de contrôle (lier vers un autre référentiel). **OpenAPI ✅ (17/07)** : gap-analysis/audits/remédiation/mappings dans `docs/openapi.yaml` (schémas + paths + `required`), types régénérés, `types/compliance.ts` re-pointé contract-first (enums dérivés). **Prouvé live 16-17/07** : 17 catalogues (16 dispo + 1 placeholder) bons comptes ; import DORA (19) ; gap-analysis (tous : 326 contrôles/274 écarts ; DORA 19/19/0 %) ; audit create→complete ; auto-remédiation 19 créés (201)→re-run 19 ignorés (idempotent) ; cross-mapping create 201/self-link 400/doublon inverse 409/list symétrique/delete 204 ; audit programme-wide→400. | Catalogues modélisés au niveau catégorie/exigence (sous-contrôles ajoutables en ad-hoc) ; 1 placeholder (`cm-loi-2024-017`, texte non fourni) ; cross-mapping = liens manuels (pas de crosswalk curé pré-rempli ISO↔NIST). |
| **13. Asset Management** | ✅ (inventaire + criticité + **dépendances** + historique, prouvé live 16/07) | M3 : Clean Architecture rétrofitée, snapshots historiques, criticité→Score Engine. `features/assets`. **Gestion centralisée complète (16/07, branche `feat/asset-dependency-mapping`)** : (1) **inventaire** étendu à la taxonomie GRC (Server/Application/Cloud/Database/SaaS/Storage/Network/Laptop/**Data/User/Supplier**) — icônes + chips de filtre ; (2) **classification par criticité** (déjà là, → Score Engine) ; (3) **cartographie des dépendances entre actifs** — nouveau modèle `AssetDependency` (arête dirigée tenant-scoped, 8 types de relation), repo+use cases (Create/List/Delete, gardes self-ref/doublon/cross-tenant, cascade à la suppression d'actif), handler `/asset-dependencies`, OpenAPI, `Asset Universe` **rebranché sur données réelles** (fixtures supprimées) avec éditeur de dépendances (ajout/retrait) dans le panneau ; (4) **historique des modifications** enfin exposé dans l'UI live (bouton « Historique » sur la modale d'édition → `AssetHistoryDrawer`). **Traçabilité « qui » complétée le 17/07 (branche `feature/centralized-asset-management`)** : le snapshot capturait le *quoi*/*quand*/*pourquoi* mais pas l'auteur ; ajout de `AssetSnapshot.ChangedBy` (uuid persisté, migration 0032) renseigné depuis `userID(c)` par les use cases update/delete, résolu en `changed_by_email` en lecture via un port optionnel `UserLookup` (`GormUserRepository.EmailsByIDs`, dégradation gracieuse), affiché « Modifié par … » dans le drawer (i18n FR/EN). **Prouvé live** (Postgres:5434) : login→create→2×PATCH→history = 2 snapshots newest-first, chacun `changed_by`=id admin réel + `changed_by_email`="admin@opendefender.io" ; delete 204→get 404. Preuves live antérieures : graph 7 actifs · 7 liens rendu, inventaire 11 types, endpoints 201/409/404/204, cascade + snapshot vérifiés. | Les 4 autres vues Universe (topology/bubbles/hierarchy) restent « coming soon ». Matching CVE via CPE dépend du CTI/Scanner (déjà câblé, item 21). Criticité = niveau unique (LOW→CRITICAL) ; décomposition en sous-scores DICT/CIA = enrichissement optionnel non demandé par la lettre de la spec. |

### 1.2 Fonctionnalités avancées — Module 14.1 à 14.18 (le **moat** vs Vanta/Drata)

| Module V5 | Statut | Preuves / absence | Note |
|---|---|---|---| 
| **14.1 Incident Management** | ✅ (base) | **Rendu fonctionnel + prouvé live le 13/07/2026** (branche `feat/ui-redesign-dc-html`). Tables ajoutées à `AutoMigrate` ; 3 bugs corrigés (`Preload("Risk")` inexistant, param `:id`/`:incidentId`) ; **registre d'incidents live** (`features/incidents/` : stats KPI, filtres, table, drawer détail/édition, création, statut inline, export CSV) ; **War Room câblée sur un incident réel** (`/incidents/:id/war-room` : en-tête + chronologie réels, durée live/figée, clôture persistée) ; tests handler sqlite (E2E + cross-tenant). | `RiskID *uint` ↔ risques uuid → `LinkRisk`/incidents-par-risque cassés (non exposés) ; roster/tasks/chat War Room = fixtures (pas de backend collaboration) ; service non retrofit Clean Architecture. |
| **14.2 Vendor Risk Management** | ❌ | Aucun package vendor. | Questionnaires publics, auto-scoring, rappels J-7/J-3/J-1. |
| **14.3 Policy Management** | ❌ | Aucun package policies. | Éditeur Markdown, versioning, acknowledgments. |
| **14.4 Trust Center Public** | ❌ | Aucun package trustcenter. | Page publique `trust.openrisk.io/{slug}`. |
| **14.5 Cyber Risk Quantification (FAIR)** | ❌ | Aucun `pkg/crq`. | ⚠️ Le Board Report a un **modèle FCFA simple** (valeur de référence par criticité), **≠ modèle FAIR** (ALE/ARO/SLE) du 14.5. |
| **14.6 Business Continuity (BCP/PCA-PRA)** | ❌ | Aucun package bcp. | RTO/RPO, plans de reprise, tests. |
| **14.7 Security Awareness Training** | ❌ | Aucun package training. | Modules + quiz, génération IA. |
| **14.8 Custom Fields** | 🟡 | `domain/custom_field.go`, `custom_field_service`, `custom_field_handler`, `CustomFields.tsx`. | Rendu dynamique `<DynamicField>` à vérifier. |
| **14.9 PAM Audit Trail (append-only)** | 🟡 | `domain/admin_audit_event.go`, `admin_audit_service.go`. | **Trigger PostgreSQL append-only, anomaly detector, frontend `AdminAuditPage` : non confirmés.** Le cœur du 14.9 (immutabilité garantie) reste à prouver. |
| **14.10 Access Review & Certification** | ❌ | Aucun package accessreview. | Privilege-creep detector, campagnes JML, révocation auto. |
| **14.11 Sensitive Data Discovery** | ❌ | Aucun package datadiscovery. | Scanner PII/PCI/secrets multi-sources + auto-risque. |
| **14.12 Executive Board Report** | ✅ | **Fait 10/07** (branche `feat/m4-compliance-report-pdf`). `pkg/ai` + `application/board` + `domain.BoardReport` + `pkg/report/board_pdf.go` + handler + `features/reports/BoardReportPage`. Flux complet prouvé live (génération→édition→approbation→PDF). | **Chemin `ClaudeAdvisor` non prouvé live** (pas d'`ANTHROPIC_API_KEY`) — repli template déterministe fonctionnel. Voir §2 M4. |
| **14.13 Risk Digital Twin (Simulation)** | ❌ | Aucun package simulation. | Propagation BFS/DFS + suggestions IA. Gamechanger. |
| **14.14 Collaborative War Room** | ❌ | Aucun package warroom. | Chat/kanban/timeline SSE, auto-trigger score≥9. |
| **14.15 Attack Path Graph** | ❌ | Aucun package attackpath. | Chemins d'attaque + blast radius. |
| **14.16 Risk Champions Leaderboard** | 🟡 | `gamification_service.go`, `features/gamification`, `gamification_handler`. | Moteur points/badges/streak V5 complet + notifications de dépassement : partiel/non prouvé. |
| **14.17 Offline-First Mode** | ❌ | Rien (Workbox/Dexie/sync). | Différenciateur Afrique majeur. |
| **14.18 Plugin Marketplace** | 🟡 | `domain/marketplace.go`, `marketplace_service`, `marketplace_handler`, `Marketplace.tsx`. | **Structs sans tags `gorm:` → exclues d'`AutoMigrate`** → non fonctionnel. Dispatcher webhook/sandbox à faire. |

### 1.3 Sécurité, Observabilité & Transversaux — Modules 15, 16, 17.1–17.7

| Module V5 | Statut | Preuves / absence | Note |
|---|---|---|---|
| **15. Sécurité & Hardening** | 🟡 | `middleware/ratelimit.go`, `middleware/security_hardening.go`, helmet dans `main.go`, tenant isolation dans repos. **Découverte critique `SetContext` corrigée (voir §2).** | CSP/HSTS/audit-log global, tests d'isolation systématiques, grep `fmt.Sprintf` SQL : à auditer/compléter. |
| **16. Observabilité** | 🟡 | `pkg/monitoring`, `monitoring_handler`, `MonitoringDashboard.tsx`, zerolog JSON. | Endpoint `/metrics` Prometheus, dashboards Grafana, health `/ready`/`/deep`, Request-ID distribué : à confirmer/compléter. |
| **17.1 Internationalisation (i18n)** | ✅ | `locales/fr.json`, `locales/en.json`, i18n FR/EN « solide ». | Certaines features récentes (Board Report) hardcodent le FR (marché primaire). |
| **17.2 Billing & Plans (Stripe/Mobile Money)** | ❌ | Rien. | Middleware `CheckPlanLimits`, Stripe, Wave/MTN/Orange. |
| **17.3 Feature Flags** | 🟡 | Claim `FeatureFlags` dans le JWT (`internal/auth/jwt.go`). | **Pas de table `feature_flags`, pas de middleware `FeatureFlag()`, pas d'admin.** Scaffolding claim seulement. |
| **17.4 Super Admin Panel** | ❌ | Aucun package superadmin. | Tenants/impersonation/metrics globales/équipe OpenDefender. |
| **17.5 Accessibilité (WCAG 2.1 AA)** | 🟡 | Design system partiel, Framer Motion. | Pas d'audit `axe-core`, focus/ARIA/contraste non garantis. |
| **17.6 Onboarding Flow (5 étapes)** | ❌ | Aucun package onboarding. | Critique pour l'activation (« 1er risque en < 5 min »). |
| **17.7 Sync Engine & Intégrations** | 🟡 | `infrastructure/integrations/thehive` + `SyncEngine` lancé dans `main.go`. | OpenCTI/Splunk/Elastic/AWS Security Hub/Azure Defender/Jira : ❌. |

### 1.4 Product Growth — Partie C (go-to-market)

| Élément V5 | Statut | Note |
|---|---|---|
| **Design System** (tokens, RiskBadge, ScoreMeter, ProgressBar, EmptyState, CommandPalette…) | 🟡 | `shared/components` partiels. `CommandPalette` (Cmd+K) non confirmé. |
| **Dark mode** | 🟡 | Présent mais non audité sur toutes les pages. |
| **Animations & micro-interactions** | 🟡 | Framer Motion sur Dashboard + transitions de route ; pas généralisé. |
| **Raccourcis clavier globaux** (N, M, Esc, Cmd+K…) | 🟡 | Partiels (ex. `N` sur Risks). |
| **Stratégie de conversion** (PlanLimitBanner, FeatureGateModal, UsageDashboard) | ❌ | Dépend du Billing (17.2). |
| **Stratégie de rétention** (Weekly Digest, streak, tips contextuels, re-engagement) | ❌ | Gamification partielle seulement. |
| **Page Pricing publique** | ❌ | — |
| **Landing page marketing** | ❌ | — |

---

## 2. DÉTAIL DES MODULES LIVRÉS & PROUVÉS LIVE

> Historique condensé de ce qui a réellement été construit et vérifié (le détail complet vit dans
> l'historique git et dans `CLAUDE.md`). Toutes ces branches sont **poussées mais non mergées** (voir §3).

### M1 — Compliance engine ✅ (07–08/07/2026, branche `feat/m1-compliance-engine`)
Use cases + handlers + OpenAPI + client généré + frontend + upload de preuve réel + RBAC granulaire.
**Vérifié live de bout en bout le 08/07** après correction de **11 bugs** (l'app n'avait jamais eu de
login fonctionnel dans cet environnement). Voir §2 « SetContext » ci-dessous.

### M2 — Contenu réglementaire africain ✅ (08/07/2026)
Catalogue générique + **93 contrôles ISO 27001:2022** importables en un clic, **vérifié live**. Puis
**BCEAO (35, Règlement 15/2002/CM/UEMOA), ANTIC-CM (25, Loi 2010/012), COBAC (45, R-2016/04)** cités
article par article (branche `fix/dashboard-crash-mitigation-routes-and-ui-polish`). `TestNoOrphanControls`
garantit code de référence + citation source uniques. 1 placeholder subsiste (`cm-loi-2024-017`).

### M3 — Assets ✅ (08/07/2026, branche `feat/m3-assets-inventory`)
Clean Architecture rétrofitée (le handler existant touchait `database.DB` sans use case ni RBAC),
snapshots historiques, criticité enfin branchée sur le Score Engine via le flux Redis
`asset.criticality_changed` (bug de scan varchar→float64 dans `GetRisksByAssetID` corrigé au passage).

### M3+ — Gestion centralisée des actifs : dépendances + taxonomie + historique UI ✅ (16/07/2026, branche `feat/asset-dependency-mapping`)
Réponse à la demande explicite « inventaire complet (serveurs/applications/cloud/données/utilisateurs/fournisseurs) +
classification par criticité + **cartographie des dépendances** + historique des modifications ». État avant : 1 & 2 & 4
existaient côté backend mais (a) la taxonomie ne couvrait pas données/utilisateurs/fournisseurs, (b) l'historique
n'était **atteignable depuis aucun écran live** (le `AssetHistoryDrawer` ne vivait que dans l'orphelin `AssetsPage`),
(c) la **cartographie des dépendances n'existait pas** : l'`Asset Universe` était un canvas **piloté par des fixtures**
(`UNI_NODES`/`UNI_LINKS`) badgé « Aperçu », sans aucun modèle backend d'arêtes.
- **(1) Cartographie des dépendances (le vrai manque).** Nouveau `domain.AssetDependency` — arête **dirigée**
  `SourceAsset → TargetAsset`, tenant-scoped, `DependencyType` (8 relations : depends_on / runs_on / connects_to /
  hosted_by / stores_data_in / authenticates_via / backs_up_to / managed_by), dans `AutoMigrate`. Port
  `AssetDependencyRepository` + impl Gorm (isolation tenant sur chaque query, `Exists` anti-doublon, `DeleteByAsset`
  pour le cascade). Use cases (1 fichier chacun) : `Create` (valide que **les deux** actifs existent dans le tenant —
  double garde cross-tenant —, refuse l'auto-référence et le doublon), `List` (graphe complet du tenant), `Delete`.
  Handler + routes `GET/POST /asset-dependencies`, `DELETE /asset-dependencies/:id` (montées **en sœurs** de `/assets`
  pour que « dependencies » ne soit jamais parsé comme UUID). `DeleteAssetUseCase.WithDependencyRepository(...)` prune
  les arêtes à la suppression d'un actif (option, garde le constructeur 1-arg et ses tests). OpenAPI + types régénérés.
- **(2) Inventaire — taxonomie étendue.** `ASSET_TYPES` (front) + enums OpenAPI (`Asset`/Create/Update) passent à
  **Server, Application, Cloud, Database, SaaS, Storage, Network, Laptop, Data, User, Supplier** — couvre les 6
  catégories demandées. Icônes + chips de filtre par type mis à jour (InventoryPage + panneau Universe).
- **(3) Historique enfin exposé.** Bouton « Historique » ajouté à `EditAssetModal` → ouvre le `AssetHistoryDrawer`
  (endpoint `/assets/:id/history` inchangé, déjà prouvé en M3) depuis l'`InventoryPage` live.
- **(4) `Asset Universe` rebranché sur le réel.** Fixtures supprimées ; nœuds = `/assets`, arêtes =
  `/asset-dependencies` ; physique in-house conservée (répulsion/ressorts/gravité/amortissement, 160 warm-up) ;
  panneau latéral = **éditeur de dépendances** (liste entrantes/sortantes + ajout `cible × relation` + retrait),
  gardé par `assets:update` ; états loading/empty honnêtes.
- **Tests** : use cases (Success/Defaults/SelfRef/TargetNotFound/CrossTenant/Duplicate + List/Delete Success/NotFound/
  CrossTenant) + repo Gorm sqlite (isolation tenant, Exists, GetByID cross-tenant=nil, DeleteByAsset, ListByAsset
  bidirectionnel). `go build`/`vet`/tests verts ; `tsc -b`/`vite build` verts.
- **Preuves live (16/07, Postgres:5434 + Redis, admin@opendefender.io)** : create dep 201, list 200, self-ref 400,
  doublon 409, cible absente 404, type invalide 400, `GET /assets/:id` **non masqué** par la route sœur (200),
  cascade (delete actif → arêtes = `[]`), PATCH actif → snapshot d'historique créé. **Frontend (Chrome headless,
  1600×940)** : `Asset Universe` rend **7 actifs · 7 liens** (graphe force-directed, couleurs par criticité, hub
  web-01) ; `Inventaire` affiche les 11 types avec icônes/badges corrects (User/Cloud/Storage/Supplier…).

### M3++ — Historique des actifs : le « qui » ✅ (17/07/2026, branche `feature/centralized-asset-management`)
Diagnostic « 1. Gestion centralisée des actifs » : inventaire polymorphe (table générique + type flexible, taxonomie
couvrant Matériel/Logiciel-Cloud/**Données/Utilisateurs/Fournisseurs**), classification par criticité (→ Score Engine),
**cartographie des dépendances** (graphe interactif + éditeur) = **déjà complets** (M3+). Seul écart réel vs la spec
(« qui a modifié quoi, et quand ») : l'`AssetSnapshot` n'enregistrait **pas l'auteur**. Comblé :
- **Domaine** : `AssetSnapshot.ChangedBy uuid.UUID` (persisté, `index`, nullable pour lignes legacy/système) +
  `ChangedByEmail string` (`gorm:"-"`, calculé en lecture). Migration `0032_add_asset_snapshot_changed_by`
  (AutoMigrate l'ajoute aussi ; le fichier garde un déploiement migrations-only autosuffisant).
- **Écriture** : `UpdateAssetUseCase.Execute` et `DeleteAssetUseCase.Execute` prennent un `changedBy uuid` et le
  posent sur le snapshot ; le handler passe `userID(c)` (le JWT ne porte que l'id, pas l'email).
- **Lecture** : `ListAssetSnapshotsUseCase` gagne un port **optionnel** `UserLookup` (`WithUserLookup`) qui résout
  chaque `ChangedBy` distinct/non-nil en email via `GormUserRepository.EmailsByIDs` (une requête `WHERE id IN`),
  câblé dans `main.go`. Dégradation gracieuse : lookup absent/en erreur → UUID brut, email vide, jamais d'échec.
- **Frontend** : `AssetHistoryDrawer` affiche « Modifié par {email|id court|Système} » à côté de l'horodatage ;
  i18n `assets.changedBy` / `assets.changedBySystem` (FR + EN). OpenAPI + types régénérés (contract-first).
- **Tests** : use cases update/delete assertent `ChangedBy` ; `ListAssetSnapshots` (résolution email + dédup +
  acteur nil ignoré + dégradation sur erreur) ; repo Gorm — DDL sqlite dérivé corrigé (`changed_by` ajouté) +
  aller-retour `changed_by` en base. `go build`/`vet`/tests verts, `tsc -b`/`vite build` verts.
- **Preuve live (17/07, Postgres:5434 + Redis)** : login→create actif→2×PATCH→`GET /assets/:id/history` = 2 snapshots
  newest-first, chacun `changed_by`=id admin réel **et** `changed_by_email`="admin@opendefender.io" (résolu live) ;
  delete 204→get 404. **Reste optionnel** : décomposition de la criticité en sous-scores DICT/CIA (la lettre de la
  spec — « un niveau de criticité … (ex: Faible/Moyenne/Haute/Critique) » — est déjà satisfaite par l'enum actuel).

### M4 — Reporting officiel + Board Report ✅ (09–10/07/2026, branche `feat/m4-compliance-report-pdf`)
- **Rapport de conformité officiel (PDF, 1 clic)** — vérifié live 09/07. `GET /compliance/frameworks/{id}/report?locale=fr|en`
  → PDF soigné tenant-scoped (garde + synthèse exécutive graduée + tableau paginé). `pkg/report` **pur** ;
  piège `fpdf.SplitText` (panique sur rune > 255 : tiret cadratin, ligature œ, apostrophe typographique)
  contourné par word-wrap maison `wrapText`. Preuve : rapport ISO 27001, 11 pages, accents FR + texte EN corrects.
- **Passe UX + gouvernance** (09–10/07, vérifiée via Chrome CDP) : 5 bugs UI, modals à footer épinglé,
  **suppression de framework** (admin-only), ISO nettoyé (198→93), **frameworks rendus tenant-scoped**
  (migration `0030` + backfill, isolation prouvée live).
- **Board Report mensuel IA/FCFA (14.12) ✅** — fait + prouvé live **10/07**. **1er client LLM du repo.**
  `pkg/ai` (interface `Advisor` + `TemplateAdvisor` déterministe testé + `ClaudeAdvisor` `claude-opus-4-8`
  adaptive thinking, cf. skill `claude-api`) → `application/board` (`GenerateBoardReportUseCase` agrège la
  posture **réelle tenant-scoped** : risques par criticité via `CountRisksByCriticality`, conformité par
  référentiel, **exposition FCFA** via `ExposureModel`, **fallback template si l'appel LLM échoue**) →
  `domain.BoardReport` (snapshot gelé + narration éditable + draft→approved, dans `AutoMigrate`) →
  `pkg/report/board_pdf.go` → handler `reports:board:*` → front React Query. **Preuve live** : login →
  génération (4 référentiels du tenant, 21,1 % global, 1 risque moyen → 3 000 000 FCFA, `created_by`
  renseigné) → édition → approbation → 400 si ré-édition → PDF inspecté en PNG. **Chemin `ClaudeAdvisor`
  non prouvé live faute d'`ANTHROPIC_API_KEY`** (repli template OK) — à revérifier dès qu'une clé existe.

### ⚠️ Découverte critique du 08/07/2026 — `middleware.SetContext()`
`SetContext()` n'était appelé **nulle part en production** (seulement dans un harnais de test). Les 8 handlers
qui lisent `GetContext(c)` retombaient donc silencieusement sur `tenant_id = uuid.Nil`. Corrigé en une ligne
dans `AuthMiddlewareRS256`. **Toute affirmation antérieure au 08/07 sur le bon fonctionnement multi-tenant
de Risk/Asset/Mitigation/Dashboard/Compliance doit être revérifiée.** C'est la source n°1 des « faux ✅ ».

---

## 3. BRANCHES GIT OUVERTES (poussées, **non mergées dans master**) — décision PR/merge requise

- `feat/m3-assets-inventory` — M3 Assets.
- `fix/dashboard-crash-mitigation-routes-and-ui-polish` — crash dashboard + RequireRole + catalogues africains.
- `feat/africa-compliance-catalogs-and-responsive` — base de la branche M4.
- `feat/m4-compliance-report-pdf` — **branche courante**, rapport de conformité + Board Report + tenant-scoping.
  Poussée sur `origin` le 10/07 (commit `412656ff`).

Aucune n'est mergée dans `master`. **Demander avant tout merge/PR.**

---

## 4. PROCHAINES PRIORITÉS (ordonnées par valeur × dépendances)

**Bloc A — Solidifier les fondations (avant d'empiler des features)**
1. **Prouver live l'Auth complète (Module 2)** : MFA, OAuth2, SAML2, refresh-token, switch-org. Corriger
   `TestSetupMFA_Success`/`TestRiskCRUDFlow`. Tant que non prouvé → 🟡.
2. **Câbler le SSE (Module 7)** : hub dédié + `/api/v1/stream` + route `/risks/events` réelle, sinon la
   plupart des « real-time » des autres modules restent des maquettes.
3. **`created_by` réel sur `CreateRisk`** + implémenter `/analytics/security-score` & `/analytics/assets/statistics`
   (widgets en repli gracieux aujourd'hui).
4. ~~**Incident Management (14.1)** : ajouter la table à `AutoMigrate` → premier module avancé *fonctionnel*.~~
   ✅ **fait le 13/07/2026** (branche `feat/ui-redesign-dc-html`) — tables migrées + 3 bugs corrigés + registre live
   (drawer détail/édition, statut inline, export CSV) + War Room câblée sur incident réel (chronologie + clôture).
   **Reste (dette legacy) :** `RiskID *uint` ↔ risques uuid (LinkRisk cassé), collaboration War Room = fixtures,
   pas de retrofit Clean Architecture du service.

**Bloc B — Différenciateurs à fort levier (le moat V5, faible dépendance infra)**
5. **IA Advisor complet (Module 10)** : réutiliser `pkg/ai` (déjà branché sur Claude) pour analyze/mitigations/
   prioritize/narrative + `AIAdvisorTab`. Bloc le plus rentable car la fondation existe déjà.
6. **CRQ FAIR (14.5)** : `pkg/crq` (ALE/ARO/SLE) — nourrit Board Report, Dashboard CISO, Attack Path.
7. **Reporting complet (Module 11)** : templates officiels COBAC/BCEAO/ISO/PCI + export XLSX + jobs async.
8. **PAM Audit Trail (14.9)** : finir l'append-only (trigger PG) + anomaly detector + `AdminAuditPage`.

**Bloc C — Gros chantiers infra (déblocage en cascade)**
9. **Infrastructure Scanner + Agent (Module 6)** : débloque l'auto-mitigation (4.1), le matching CVE des
   Assets (13), la Data Discovery (14.11), l'Attack Path (14.15).
10. **CTI Engine câblé (Module 5)** : worker de sync + auto-création de risque.

**Bloc D — Monétisation & croissance (Partie C + 17)**
11. **Billing & Plans (17.2)** + conversion (Partie C) + **Onboarding (17.6)** + **Super Admin (17.4)**.

**Bloc E — Wave 2/3 (gamechangers restants)**
12. Digital Twin (14.13), War Room (14.14), Attack Path (14.15), Access Review (14.10), Offline (14.17),
    Plugin Marketplace complet (14.18), Vendor (14.2), Policy (14.3), Trust Center (14.4), BCP (14.6),
    Training (14.7), Champions complet (14.16).

---

## 5. RÈGLES D'EXÉCUTION (Master Prompt V5 — Partie A, non négociables)

**Sécurité (critique)**
1. Filtrer par `tenant_id` sur **chaque** query DB — dans le repository, jamais dans le handler.
2. Objet d'un autre tenant → **404** (jamais 403).
3. JWT **RS256** uniquement. Jamais de secrets dans les logs. Credentials **AES-256-GCM** en DB.
4. `admin_audit_events` **APPEND-ONLY** (trigger PG rejetant UPDATE/DELETE) — jamais violé, même en migration.

**Architecture**
5. Erreurs typées uniquement (`ErrNotFound`/`ErrForbidden`/`ErrConflict`/`ErrValidation`).
6. Transactions DB sur toute opération multi-table.
7. Le Score Engine n'est **jamais** appelé depuis un handler — toujours via event Redis.
8. **Lire TOUS les fichiers existants d'un module avant d'écrire une ligne.**

**Qualité & UX**
9. Zéro `any` TypeScript. Zod sur tous les formulaires. Tests min. par use case (Success/NotFound/Unauthorized).
10. Skeleton loaders (jamais de spinner pleine page). Toujours les 3 états (loading/error/empty).
    Optimistic updates sur les mutations critiques.

**Méthode (5 étapes/module)** : LIRE → PLANIFIER → IMPLÉMENTER (backend puis frontend, tests inclus) →
VALIDER (`go test ./...` + `npm test` + live) → COMMITER. **Règle des 2 h** : bloqué > 2 h → nouvelle
session, commencer par « Lis [fichier] et explique-moi le problème avant de proposer une solution ».

**Discipline branche/doc** : une branche par feature ; à chaque fin de module, commit + mise à jour de
`ROADMAP.md` et `CLAUDE.md`. Vérifier chaque page **live** avant de la déclarer faite.
