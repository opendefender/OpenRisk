# OpenRisk — Rapport de Readiness `release/1.0-rc1`

_Rôle : Release Manager & Principal Architect. Date : 2026-07-22. Branche : `release/1.0-rc1` (coupée de `master` @ `8d69eee3`)._

Ce document n'est **pas** une liste de fonctionnalités. C'est un rapport d'état : ce
qui est prêt, ce qui a été corrigé/nettoyé dans cette RC, ce qui reste bloquant, et un
**verdict argumenté** sur l'aptitude à la démo, à la bêta privée et à la production.

---

## 1. Verdict exécutif

| Objectif | Verdict | Condition |
|---|---|---|
| **Démonstration client** | ✅ **PRÊT** (sur cette branche) | Seeder des données de démo propres ; scénario scripté. |
| **Bêta privée** | 🟡 **PROCHE** | Fusionner cette RC ; SMTP réel ; sweep d'isolation multi-tenant ; store de rate-limit Redis. |
| **Production / vente** | 🔴 **PAS ENCORE** | Fermer les bloquants « Importants » ci-dessous (vérif. E2E réelle, doc API, isolation systématique, versioning, sauvegardes/DR). |

**Note produit globale : ~7,0 / 10.** Produit riche, différenciant et crédible sur le plan
fonctionnel GRC, buildé et testé au vert, mais dont la **maturité opérationnelle** (vérification
réelle de bout en bout, documentation externe, durcissement multi-instance) n'est pas encore au
niveau d'une mise en production commerciale.

---

## 2. Ce qui a été corrigé / nettoyé dans cette RC (7 commits)

| Commit | Axe | Impact |
|---|---|---|
| `ea4c066a` | 🔴 Fiabilité/Sécu | **Fuite cross-tenant corrigée** : `AnalyticsService.GetDashboardSnapshot` et `DashboardDataService.GetCompleteDashboardData` agrégeaient **sans filtre `tenant_id`** (violation de la RÈGLE #2). Désormais scopés par tenant + test de non-régression. |
| `45eb0ba4` | 🔴 Sécurité | **Rate-limiting brute-force câblé** sur `/auth/login`, `/auth/register`, `/auth/refresh` (5 req/15 min/IP). Le middleware existait mais **n'était monté nulle part**. |
| `28e3fcca` | Cohérence | `CORS_ORIGINS` (documenté dans `.env.example`) est enfin **honoré** au lieu d'origines codées en dur. |
| `fc072348` | Qualité | **`TestRiskCRUDFlow` re-synchronisé** : le DDL sqlite du test avait redérivé (colonnes financières/smart manquantes). Suite backend de nouveau **verte**. |
| `44ec63b0` | Sécurité/Cohérence | Suppression de la **route de login legacy HS256** (`/auth/legacy/*`, surface morte mais dangereuse) ; **route `/users/me` dupliquée** retirée ; **refus de démarrer en production** si l'admin initial est seedé sans `INITIAL_ADMIN_PASSWORD` (fini le `admin123` par défaut). |
| `6fccfa88` | Cohérence | Suppression du service **`ai_risk_predictor_service.go`** — 0 référence, jamais câblé (code réellement mort). |
| `3d0ceb80` | Qualité | Test de login re-synchronisé sur le message d'erreur **non-énumérant** réel du produit. |

**État de build/test de la branche (vérifié cette session) :**
- Backend : `go build ./...` ✅ · `go vet ./...` ✅ · `go test ./...` → **36 packages OK, 0 FAIL**.
- Frontend : `tsc -b` ✅ · `vite build` ✅ (bundle produit).

---

## 3. Bloquants restants (priorisés)

### 🔴 Critiques — à traiter avant toute commercialisation
1. **Reporter les correctifs sur `master`.** La fuite cross-tenant, le rate-limiting et le
   durcissement admin **ne sont pas sur `master`** — ils vivent sur cette RC. `master` est
   actuellement vulnérable. _Impact : sécurité multi-tenant. Complexité : Faible (merge)._
2. **Sweep d'isolation multi-tenant systématique.** La RÈGLE #2 est respectée module par module
   mais n'a jamais été **vérifiée de façon exhaustive** (grep + tests d'isolation sur *chaque*
   repository). La fuite analytics prouve que des trous existent. _Impact : conformité RGPD/souveraineté. Complexité : Moyenne._
3. **Vérification réelle de bout en bout.** Une grande partie des features est « prouvée par build
   + endpoint », **pas par un parcours utilisateur réel** (le pilotage navigateur est bloqué par le
   sandbox de dev). Avant vente : campagne E2E (Playwright déjà configuré) sur les parcours clés.
   _Impact : confiance produit. Complexité : Moyenne._

### 🟠 Importantes — avant la 1.0 finale
4. **Store de rate-limit en mémoire** → à backer sur Redis (sinon inefficace en scale horizontal).
5. **Documentation API : 51 chemins OpenAPI pour ~300 routes.** Gros écart. Contract-first à compléter.
6. **Versioning à assainir.** GitHub porte déjà `1.0.0 → 1.0.8` (nommage incohérent `1.0.x` vs `v1.0.7`)
   qui **ne correspondent pas** au produit actuel. Décider un schéma (proposé : repartir en `1.1.0-rc.1`
   ou `2.0.0-rc.1`), retagger proprement, documenter le changelog.
7. **Transport email = mock par défaut.** Invitations/notifications e-mail non fonctionnelles sans SMTP réel.
8. **Dette de tests frontend** : 6 tests + 2 fichiers de test non chargeables (rot d'outillage `vi.mocked`,
   pas des bugs produit). À réparer pour un CI vert.
9. **Chemins non prouvés live** : legs IdP réels OAuth2/SAML2, `ClaudeAdvisor`/`ClaudeAssistant` sans clé,
   collecteurs cloud/connecteurs réels (prouvés hors-ligne/httptest, pas contre une vraie infra).

### 🟡 Recommandées — forte valeur
10. **Code-splitting frontend** : bundle unique **1,56 Mo (454 Ko gzip)**. Découpage par route → temps de
    premier rendu. _Complexité : Faible._
11. **Unifier le vocabulaire `RiskStatus`** (lowercase `open/…` vs uppercase `DRAFT/…`). Rendu non-fatal
    côté front mais source de confusion durable. _Complexité : Moyenne (choisir un seul vocabulaire backend)._
12. **~300 erreurs ESLint** et **~85 `any`** résiduels (dont hooks/tests) contre la règle « zéro any ».
13. **Consolider le scoring** : `service.ScoreEngineService` (config in-memory legacy, toujours câblé via
    `/score-engine/*`) fait doublon conceptuel avec `pkg/scoring/smart.go`. À fusionner/retirer.
14. **Nettoyage Git** : 83 branches locales, 30 non fusionnées. Élaguer les branches mergées, archiver le reste.
15. **Nettoyer `shared/fixtures.ts`** (données de démo mortes, 0 consommateur) et les 12 `console.log` résiduels.

### 🔵 Futures évolutions (v1.1+)
- Scope `own`/`assigned` (Asset/Risk Owner) appliqué **au niveau requête** (aujourd'hui accès tenant-large).
- Alertes cloud Defender/SCC ; SNMP ; base vectorielle pour le RAG IA ; streaming IA.
- Offline-first, Billing/Stripe, Super-Admin, cross-mapping curé entre référentiels.
- Observabilité prod : SLO/alerting, sauvegardes/DR documentées, tests de charge à l'échelle.

---

## 4. Definition of Done — v1.0

| # | Critère | État |
|---|---|---|
| 1 | `go build` + `go vet` + `go test ./...` verts | ✅ (sur RC) |
| 2 | `tsc -b` + `vite build` verts | ✅ |
| 3 | Suite de tests frontend verte en CI | ❌ (dette d'outillage) |
| 4 | Zéro fuite cross-tenant (audit exhaustif + tests par repo) | 🟡 (1 corrigée ; audit global à faire) |
| 5 | Auth durcie : RS256 unique, MFA, PAT, rate-limit, pas de secret par défaut | 🟡 (fait ; store RL Redis + SSO réel à finir) |
| 6 | Aucune route/handler mort ; un seul chemin d'auth | 🟡 (legacy login retiré ; ScoreEngineService à consolider) |
| 7 | OpenAPI couvre toutes les routes exposées | ❌ (51/~300) |
| 8 | Parcours E2E clés verts (Playwright) | ❌ (à exécuter) |
| 9 | SMTP réel + notifications e-mail fonctionnelles | ❌ |
| 10 | Chaque écran : états loading/error/empty + copie finalisée | 🟡 (bon ; War Room encore « Aperçu ») |
| 11 | Versioning & changelog cohérents, tag RC propre | 🟡 (branche RC prête ; retag à décider) |
| 12 | Sauvegardes/DR, runbook de déploiement, SLO | ❌ |

**Score DoD : 2 ✅ / 6 🟡 / 4 ❌ sur 12.** → RC crédible, **pas** encore « done » pour la prod.

---

## 5. Grille de notation (justifiée)

| Domaine | Note /10 | Justification courte |
|---|---:|---|
| Architecture | 8.0 | Clean Architecture globalement respectée ; quelques services legacy parallèles. |
| Backend | 7.5 | Build/tests verts, large surface ; code legacy + store RL in-memory. |
| Frontend | 7.0 | Build OK, typé ; bundle monolithique, dette ESLint/tests. |
| UX | 7.0 | Parcours réfléchis, états vides, i18n FR/EN à parité (300/300 clés). |
| UI | 7.5 | Design system dc.html, thèmes clair/sombre, soigné. |
| Sécurité | 7.0 | RS256 unifié, MFA, PAT, audit ; mais fuite existait sur master, RL in-memory, SSO non prouvé. |
| Performance | 6.0 | Pas de code-splitting, stores in-memory, aucune preuve de charge. |
| Qualité du code | 6.5 | `vet` propre ; ~300 ESLint, double vocabulaire, restes de code mort. |
| Fonctions GRC | 8.5 | Couverture très large et crédible vs marché (voir §6). |
| RBAC | 7.5 | Rôles métiers + catalogue de permissions ; scope own/assigned non appliqué en requête. |
| Documentation | 5.5 | Doc interne riche (ROADMAP/CLAUDE) ; OpenAPI partiel, pas de doc utilisateur. |
| Maintenabilité | 6.5 | Clean arch mais sprawl de branches + double vocabulaire. |
| Maturité produit | 6.5 | Sur les rails RC ; non vérifié E2E à l'échelle. |
| Potentiel commercial | 8.0 | Différenciation forte : Afrique/FCFA, CRQ FAIR, smart scoring, scanner, SOAR, IA. |

---

## 6. OpenRisk est-il un produit GRC crédible, différenciant et vendable ?

**Crédible : oui.** La couverture fonctionnelle est réelle et large — registre de risques + cycle
de vie ISO 31000, 17 catalogues de conformité (ISO/NIST/PCI/HIPAA/SOC2/RGPD/DORA/NIS2/SOX + BCEAO/
COBAC/ANTIC), gap analysis, audits, remédiation, gestion d'actifs + cartographie des dépendances,
vulnérabilités priorisées (KEV→P1), scanner d'infrastructure, quantification financière FAIR,
scoring intelligent multifactoriel, SOAR, gouvernance (audit immuable + maker-checker), dashboard
exécutif, IA GRC. C'est un périmètre comparable, sur le papier, aux suites du marché.

**Différenciant : oui.** L'ancrage **Afrique francophone / FCFA** (référentiels BCEAO/COBAC/ANTIC,
montants en XAF, quantification locale) est un positionnement que ni Archer, ServiceNow GRC,
MetricStream, AuditBoard ni OneTrust ne couvrent nativement. Couplé à l'open-core et à la
quantification FAIR, c'est une vraie proposition de valeur.

**Vendable aujourd'hui : pas encore — mais très proche pour une bêta.** Le produit est
**démontrable immédiatement** et **à un merge + quelques durcissements d'une bêta privée**. Ce qui
le sépare d'une **vente en production** n'est pas fonctionnel mais **opérationnel** : preuve E2E
réelle, isolation multi-tenant auditée de bout en bout, documentation API/utilisateur, versioning
assaini, e-mail réel, sauvegardes/DR. Ce sont des chantiers cadrés et de complexité maîtrisée, pas
des trous fonctionnels.

**En une phrase :** OpenRisk est un **RC1 crédible et différenciant**, prêt pour la démo et quasi
prêt pour la bêta privée ; la production commerciale exige de fermer une liste **finie et connue**
de bloquants de fiabilité et de documentation — pas de nouvelles fonctionnalités.
