# 🗺️ OPENRISK — ROADMAP

> **Source de vérité d'exécution.** Cette roadmap est fondée sur l'inspection du code réel du dépôt `opendefender/OpenRisk`. Audit initial : branche `master`, clone du 26 juin 2026. **Rafraîchi le 07/07/2026** sur la branche `fix/cti-build-conflict` par relecture directe des migrations/domaine/repos/handlers/frontend + exécution de `go build`, `go vet`, `go test`, `npm run build`. Les statuts ne reflètent PAS le README ni les anciens documents — uniquement ce qui existe dans le code, vérifié à la date ci-dessus.
>
> **Règle d'or.** On n'avance pas en largeur tant qu'une vague n'est pas réellement terminée (Definition of Done, §6). On ne marque jamais ✅ sans preuve (test CI vert + revue humaine). Statuts honnêtes uniquement : `✅ Fait` · `🟡 Partiel` (gaps listés) · `❌ Non commencé`.
>
> **⚠️ Deux blocages critiques découverts le 07/07/2026, absents de l'audit du 26/06 :**
> 1. **Build frontend cassé** (`npm run build` → **79 erreurs TypeScript** : modules manquants `@/hooks/useDashboard`, `@/components/dashboard/*`, violations `verbatimModuleSyntax`, champ `Risk.assigned_to` inexistant). Le Gate 0 (`PROJECT_PLAN.md §3`) n'est **pas** vert malgré les apparences — aucune nouvelle feature frontend ne doit démarrer avant correction.
> 2. **Bug d'isolation cross-tenant** dans le nouveau repository Compliance : `GormComplianceRepository.UpdateControl` utilise `.Where(...).Save(control)` — GORM ignore la clause `Where` chaînée quand la clé primaire est déjà posée sur le struct, donc **un tenant B peut écraser un contrôle du tenant A**. Test `TestUpdateControl_CrossTenantFails` (fichier non commité `gorm_compliance_repository_test.go`) rouge, comme prévu pour l'attraper. Fix : remplacer par `.Model(&domain.ComplianceControl{}).Where("id = ? AND tenant_id = ?", ...).Updates(control)`. Voir §2.2.

---

## 1. ÉTAT RÉEL DU CODE (au 26/06/2026)

Méthode de vérification : présence de **table en migration** (persistance réelle) + **handler/service/domain** (logique) + **feature frontend** (UI). Un module n'est « réel » que si les trois existent.

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

> **Verdict honnête sur le cœur :** c'est un **excellent Risk Register multi-tenant**, sécurisé et testé. C'est un vrai actif. Mais ce n'est pas encore une plateforme **GRC** : le « G » (gouvernance/politiques) et surtout le « C » (conformité/référentiels) manquent — voir §1.3.

### 1.2 Code présent mais incomplet (🟡) — à finir, pas à recommencer

| Domaine | Ce qui existe | Ce qui manque (gap) |
|---|---|---|
| **Compliance Frameworks** | Migration `0028_create_compliance_schema` (`compliance_frameworks`/`compliance_controls`/`control_evidences`) ; domaine `compliance.go` + `compliance_repository.go` ; `GormComplianceRepository` avec tenant_id filtré sur Get/List/Delete | **Bug cross-tenant sur `UpdateControl`** (voir avertissement en tête de fichier) à corriger avant tout ; test repo écrit mais **non commité** ; **aucun** use case / handler / route OpenAPI / feature frontend — c'est encore uniquement la couche donnée |
| Assets | Table dédiée `assets` (domaine `asset.go`, `OrganizationID`, relation many2many `risk_assets`, **dans `AutoMigrate()`**) ; pages frontend `pages/Assets.tsx` + `useAssetStore.ts` | Pas de `src/features/assets` dédié ; snapshots historiques ; criticité pas encore branchée sur le Score Engine |
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

### 2.2 🟡 Fondations du schéma Compliance — EN COURS, bug bloquant à corriger
- [x] Migration `backend/migrations/0028_create_compliance_schema.up/down.sql` : `compliance_frameworks` (global), `compliance_controls` (tenant_id + framework_id), `control_evidences` (tenant_id + control_id), toutes avec soft-delete + index tenant-scopés.
- [x] Entités domaine `backend/internal/domain/compliance.go` + `compliance_repository.go` (interface).
- [x] `GormComplianceRepository` (`backend/internal/infrastructure/repository/gorm_compliance_repository.go`) : tenant_id filtré au repository sur Get/List/Delete.
- [ ] **BUG (voir avertissement en tête de fichier) :** `UpdateControl` utilise `.Where(...).Save(control)` — GORM ignore le `Where` chaîné quand la PK est posée → écriture cross-tenant possible. Test `TestUpdateControl_CrossTenantFails` rouge. **Fix : `.Model(&domain.ComplianceControl{}).Where("id = ? AND tenant_id = ?", control.ID, control.TenantID).Updates(control)`.**
- [ ] `gorm_compliance_repository_test.go` existe mais est **non commité** — à committer une fois le bug corrigé et la suite verte.
- **Acceptation (pas encore atteinte) :** migration up/down réversible testée (✅ fait) ; test cross-tenant vert (❌ actuellement rouge).

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
- [ ] **Nouveau blocage non documenté avant le 07/07 :** `npm run build` échoue avec **79 erreurs TypeScript** (modules manquants `@/hooks/useDashboard`, `@/components/dashboard/*`, violations `verbatimModuleSyntax`, `Risk.assigned_to` inexistant). Gate 0 frontend **rouge**.
- **Acceptation (pas encore atteinte) :** CI bloquante sur gosec/govulncheck/gitleaks, racine propre, `npm run build` + `npm run typecheck` verts.

---

## 3. WAVE 1 — LA POINTE : devenir déjà le meilleur GRC francophone africain

> Objectif vérifiable : **un responsable conformité d'une banque/PME CEMAC ou UEMOA peut, seul, monter un programme COBAC ou ISO 27001 et produire un rapport défendable.** Tant que ce parcours n'est pas bout-en-bout, on ne touche pas à la Wave 2.

**Ordre imposé (chaque jalon dépend du précédent) :**

### M1 — Compliance Frameworks (moteur générique) 🔴 #1 — 🟡 fondations posées, use cases pas commencés
Dépend de : 2.2 (bug cross-tenant à corriger avant de construire dessus).
- [x] Migration + domaine + repository tenant-scopé (voir §2.2).
- [ ] Use cases : créer/lier un framework à un tenant, instancier ses contrôles, attacher des preuves, calculer le **score de conformité** et le **% d'avancement**.
- [ ] Handlers + OpenAPI + feature frontend `compliance` (liste des référentiels, vue contrôle, dépôt de preuve, jauge de conformité).
- [ ] Les 4 tests minimum + test cross-tenant (le test repo existe déjà, à étendre au niveau use case/handler).
- **Acceptation :** un tenant peut suivre un référentiel de bout en bout dans l'UI. Pas encore atteint — aucune route ni écran n'existe.

### M2 — Contenu réglementaire africain (le moat) 🔴 #1
Dépend de : M1.
- [ ] Modéliser **ISO 27001:2022** d'abord (référence universelle, validation de la mécanique), puis **COBAC**, **BCEAO**, **ANSSI-CM**, **Loi camerounaise 2024/017**.
- [ ] Chaque contrôle **cite sa source** (article, circulaire). Tests de cohérence : aucun contrôle orphelin.
- **Acceptation :** au moins ISO 27001 + un référentiel COBAC complets et chargés ; revue par une personne compétente conformité.

### M3 — Assets (inventaire autonome) 🟡→✅ — plus avancé que documenté
Finir le partiel §1.2.
- [x] Table `assets` dédiée (`OrganizationID`, dans `AutoMigrate()`), relation risque↔actif (`risk_assets`), pages frontend `Assets.tsx`/`useAssetStore.ts`.
- [ ] Snapshots historiques ; criticité pas encore branchée sur le calcul du Score Engine ; `src/features/assets` dédié (actuellement des pages, pas une feature structurée comme les autres modules).
- **Acceptation :** inventaire utilisable (✅ probable, à confirmer en UI) ; criticité alimentant le Score Engine (❌ pas encore).

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
| Socle existant | Auth/RBAC/multitenant, Risk, Mitigation, Notif, Score, Dashboard, Audit | ✅ solide |
| **Wave 0** | Licence ✅ · schéma Compliance 🟡 (bug cross-tenant à corriger) · AES 🟡 (MFA seulement) · README/hygiène 🟡 · CI sécu 🟡 (gosec non-bloquant, govulncheck/gitleaks absents) · **build frontend 🔴 cassé (79 erreurs TS)** | 🟡 en cours, **pas vert** |
| **Wave 1** | Compliance (fondations posées, M1 use cases à faire) + contenu africain (❌) + Assets (🟡, plus avancé que prévu) + Reporting/Board (❌) + Incident (🟡, table manquante) + Offline (❌) + 3 écrans parfaits (❌) | 🟡 démarré, priorité absolue |
| Wave 2 | 7 différenciateurs (moat) — **CTI Engine déjà entamé en avance de phase**, à ne pas approfondir avant M1/M2 | 🟡 entorse anti-dérive détectée |
| Wave 3 | Plateforme + écosystème + monétisation + portail régulateur | ❌ après Wave 2 |

**La phrase à garder :** aujourd'hui vous avez un excellent *Risk Register*, et les fondations de données du *Compliance engine* sont posées mais invisibles (pas d'API, pas d'UI). La priorité immédiate n'est pas une nouvelle feature : c'est (1) réparer le build frontend, (2) corriger le bug cross-tenant sur `UpdateControl`, (3) puis construire les use cases/handlers/UI de M1 par-dessus des fondations saines.

---

*ROADMAP fondée sur l'état réel du code au 26/06/2026, rafraîchie par inspection directe + exécution (build/vet/test) le 07/07/2026. À mettre à jour à chaque fin de module, avec preuve.*
