# OpenRisk — Audit total (QA · UX · Cybersécurité · Utilité métier)

**Date :** 2026-08-19
**Branche :** `feat/bank-grade-quality-hardening`
**Mode :** Audit-only (Phases 0→8). **Aucune correction de code, aucune issue GitHub créée.** Les défauts sont observés, reproduits et documentés.
**Testé LIVE contre la vraie pile** : backend Go (`:8080`), frontend Vite (`:5173`), Postgres (`:5434`), Redis. Deux tenants réels créés via le vrai flux d'inscription (`OpenRisk Demo Bank` / `OpenRisk Demo Insurance`), MFA enrôlé par TOTP calculé, sessions cookie + Bearer.

> ⚠️ **Note d'environnement (setup, pas un correctif)** : la base de dev était bloquée `dirty` à la migration 40 → le binaire **refuse de démarrer** (voir BUG‑002). Pour pouvoir tester le produit, la table `schema_migrations` a été passée à `version=55, dirty=false` (AutoMigrate ayant déjà construit le schéma complet). C'est une manipulation d'état de la **DB de dev**, pas une modification de code.

---

## 1. Executive Verdict

**OpenRisk est-il réellement prêt ? → NON (pas encore).**

Le produit est étonnamment abouti pour son ampleur : le cœur GRC fonctionne *réellement* de bout en bout (inscription → onboarding contextuel → registre des risques → score → mitigation → preuve → rapport PDF), l'**isolation multi‑tenant est solide** (tous les accès inter‑tenant testés renvoient `404`, écritures bloquées, zéro fuite de données observée), les entrées sont parametrées (pas de SQLi), et les en‑têtes de sécurité sont corrects. C'est un socle crédible.

**Mais** l'audit a trouvé **une vulnérabilité bloquante d'authentification prouvée en live** : les **codes de secours MFA sont identiques pour tous les utilisateurs** (générateur à graine constante) et **contournent la MFA en direct**. Comme la MFA est **imposée à chaque compte**, cela transforme le second facteur en une serrure à passe‑partout public. Tant que ce point n'est pas corrigé, **aucune mise en production n'est envisageable**.

S'y ajoutent deux défauts d'intégrité de données qui minent la confiance sur l'écran d'atterrissage principal : le **tableau de bord d'accueil affiche « 0 risque / score 0 »** alors que le tenant a 11 risques (les endpoints renvoient pourtant les bons chiffres), et le calcul de sévérité de `/stats` **n'utilise pas les mêmes seuils que le Score Engine** (0 critique affiché vs 3 réels). Un RSSI qui regarde d'abord le dashboard conclurait « rien à signaler » — faux.

---

## 2. Scores globaux (/10)

| Dimension | Note | Justification (preuves live) |
|---|---:|---|
| **Sécurité** | **3** | Isolation multi‑tenant excellente, SQLi safe, headers OK, RBAC/PAT enforce, IA anti‑escape — **mais** MFA bypass S0 prouvé (codes de secours universels). Un seul S0 plombe la note. |
| **Fonctionnalités** | **8** | Cœur GRC complet et réellement fonctionnel (risque→score→mitigation→preuve→PDF), CRQ financier exact, state machine cycle de vie, onboarding contextuel. |
| **UX** | **6** | Onboarding guidé de qualité, feedback d'import clair, i18n FR — mais friction MFA imposée pour néophyte, messages de validation bruts, dashboard trompeur. |
| **Accessibilité** | **N/A (non auditée)** | Non testée cette session (axe-core non exécuté). À faire. |
| **Performance** | **8** | Endpoints < 100 ms en local (health 74 ms, register < 1 s, /stats/exec instantanés). Pas de p99 sous charge mesuré. |
| **Fiabilité** | **5** | Boot bloqué par migration dirty ; ScoreWorker async fiable ; pas de crash observé, mais dashboard incohérent. |
| **Qualité des données** | **4** | Register et Executive cohérents avec le Score Engine ; `/stats` et le dashboard d'accueil incohérents (2 systèmes de seuils). |
| **Valeur métier** | **8** | Résout de vrais problèmes : registre priorisé, ALE FCFA, conformité citée (ISO/ANTIC/COBAC), rapport présentable en comité. |
| **Utilité cyber** | **7** | Priorisation, exposition financière, mapping conformité réels. Attack path/CTI non re-vérifiés cette session. |

### 🎯 GLOBAL SCORE : **5.0 / 10** — « bon produit, bloqué par un défaut de sécurité critique et deux incohérences de données ».

---

## 3. Release readiness

| Sévérité | Nombre |
|---|---:|
| **S0 — BLOCKER** | **1** |
| **S1 — Critique** | **2** |
| **S2 — Majeur** | **1** |
| **S3 — Modéré** | **4** |
| **S4 — Mineur** | **1** |

**Verdict release : BLOQUÉ** par BUG‑001 (S0). Les S1 doivent suivre immédiatement.

---

## 4. Métriques mesurées

| Métrique | Valeur observée |
|---|---|
| Signup → session (API) | < 1 s (register 201) mais **bloquant MFA obligatoire** avant Aha |
| Onboarding → /risks | 5 étapes, complété manuellement, contextuel (banking + Cameroun) ✓ |
| Report → PDF | 1 appel, **PDF valide 5 pages** (`application/pdf`, `%PDF-`) ✓ |
| Health/API latency | `/api/v1/health` 74 ms ; register < 1 s ; /stats, /analytics/executive instantanés |
| Écrans réellement pilotés | Login, MFA, Onboarding (5 étapes), Registre des risques, Dashboard d'accueil |
| Tenant isolation tests | **~30 accès inter‑tenant → tous 404** (0 fuite) |
| Permission (PAT) tests | read `risks:read`→200 ; delete/create hors‑scope→403 ✓ |
| Injection | SQLi (3 payloads)→ table intacte ; XSS stocké → échappé par React ✓ |
| Secrets en réponse | Aucun (creds jamais renvoyées ; JWT typés) |
| MFA bypass | **PROUVÉ** — code de secours public accepté au challenge → session émise |

---

## 5. TOP 10 des problèmes (impact × sécurité × business ÷ effort)

1. **BUG‑001 (S0) — Codes de secours MFA identiques pour tous → contournement MFA prouvé.** Effort de correction faible, impact maximal. **À corriger avant tout.**
2. **BUG‑002 (S1) — Démarrage impossible (migration `dirty` à 40).** Bloque tout déploiement/CI ; effort faible.
3. **BUG‑003 (S1) — Dashboard d'accueil affiche 0 risque / score 0** alors que 11 risques existent et que `/stats` renvoie 11/83. L'écran principal ment sur la posture.
4. **BUG‑004 (S2) — Sévérité `/stats` calculée sur des seuils (≥20/≥15/≥10) qui ne correspondent PAS au Score Engine (≥7/≥4/≥2)** → 0 critique affiché vs 3 réels.
5. **BUG‑006 (S3) — Message de validation brut** (`Key: 'CreateRiskInput.Title' Error:Field validation…`) exposé à l'utilisateur.
6. **BUG‑008 (S3) — MFA imposée dès l'inscription** : un néophyte/juriste sans appli d'authentification est bloqué hors du produit (contre l'objectif « Aha < 8 min »).
7. **BUG‑005 (S3) — Champ `level` figé à `medium`** quel que soit le score ; `RiskCard.tsx` colore la sévérité dessus.
8. **BUG‑007 (S3) — Criticité renvoyée `low` pendant ~2 s** après création (recalcul async) : la réponse de création affiche une mauvaise bande.
9. **BUG‑009 (S4) — Listes sous parent étranger renvoient `200 []` au lieu de `404`** (`/risks/:id/mitigations`, `/frameworks/:id/controls`) — incohérence de convention, sans fuite.
10. *(Observation à vérifier)* — Le flux OAuth/SAML et les intégrations n'ont pas été exercés live (pas de credentials) ; à traiter comme non prouvés.

---

## 6. Moments d'abandon probables

1. **Écran MFA à l'inscription** : « je voulais juste essayer, on me demande une appli d'authentification » → abandon néophyte.
2. **Dashboard d'accueil à 0** : « j'ai créé mes risques, le tableau de bord dit 0 — l'outil ne marche pas » → perte de confiance immédiate.
3. **Message de validation cryptique** en cas d'oubli d'un champ.
4. **Impossible de démarrer** (opérateur/dev) : migration dirty → binaire quitte en erreur.

---

## 7. Fonctionnalités exceptionnelles (à conserver)

- **Isolation multi‑tenant** : rigoureuse, convention `404` (pas `403`) respectée, écritures inter‑tenant bloquées. C'est le point le plus fort du produit.
- **Onboarding contextuel** : référentiels suggérés selon secteur + pays (banking + Cameroun → ISO 31000/27005, ANTIC, COBAC), import inline avec feedback « Importé ».
- **Score Engine + registre** : bandes exactes (10→Critique, 6.9→Élevé, 3.9→Moyen, 1.9→Faible), chip « Critique·3 » cohérent, XSS échappé.
- **Quantification financière (CRQ)** : ALE = SLE×ARO exact, breakdown complet (pire‑cas, résiduel, ROSI), double devise XAF/USD.
- **Rapport PDF** conforme et paginé, généré en un appel.
- **IA anti‑escape** : RAG strictement tenant‑scoped, injection de prompt (« ignore permissions ») sans effet.

---

## 8. Fonctionnalités faibles / trompeuses

- **Dashboard d'accueil** : donne une fausse lecture de la posture (0 partout) → *anti‑valeur* tant que BUG‑003 n'est pas corrigé.
- **Deux systèmes de sévérité** cohabitent (Score Engine ≥7/≥4/≥2 vs `/stats` ≥20/≥15/≥10) : source d'incohérence structurelle.
- **Champ `level`** : legacy figé, à supprimer ou recalculer.

---

## 9. Manques produit (constatés cette session)

- Chemin de premier accès **sans MFA obligatoire** (ou MFA différable) pour l'Aha néophyte.
- **Messages d'erreur** orientés utilisateur (QUOI / POURQUOI / QUE FAIRE).
- **Accessibilité** (axe-core), **responsive** et **perf sous charge** : non audités → à planifier.

---

## 10. Matrice Néophyte → Expert (modules pilotés)

| Module | Néophyte | Analyste | RSSI | Auditeur | Valeur |
|---|---|---|---|---|---|
| Inscription + MFA | ❌ (MFA impose une appli) | ✅ | ✅ | ⚠️ | Indispensable |
| Onboarding | ✅ (guidé, contextuel) | ✅ | ✅ | ✅ | Utile |
| Registre des risques | ✅ | ✅ | ✅ | ✅ | **Indispensable** |
| Score Engine | ⚠️ (jargon) | ✅ | ✅ | ✅ | **Indispensable** |
| Dashboard d'accueil | ❌ (chiffres faux) | ❌ | ❌ | ⚠️ | Trompeur (à réparer) |
| Executive dashboard | ✅ | ✅ | ✅ | ✅ | Utile |
| Conformité + PDF | ✅ | ✅ | ✅ | ✅ | Indispensable |
| Quantification financière | ⚠️ | ✅ | ✅ | ✅ | Différenciant |
| IA Advisor (template) | ✅ | ✅ | ✅ | ⚠️ | Utile |

---

## 11. Registre complet des défauts

### BUG-001 — [S0 · security] Codes de secours MFA identiques pour tous les utilisateurs → contournement MFA prouvé en live
- **Module / fichier** : Auth / `backend/pkg/otp/totp.go:106` `GenerateBackupCodes()`
- **Type** : security / authentication-bypass
- **Reproductibilité** : 100 %
- **Étapes** :
  1. Inscrire deux utilisateurs distincts (tenants différents) → chacun enrôle la MFA (`POST /auth/mfa/setup`).
  2. Comparer les `backup_codes` renvoyés → **identiques** :
     `['4F2LIBGHU5SD','AZ67MVK3YRWX','ENCTQJOP4F2L','IBGHU5SDAZ67','MVK3YRWXENCT','QJOP4F2LIBGH','U5SDAZ67MVK3','YRWXENCTQJOP']`
  3. Se connecter avec **mot de passe seul** → `mfa_required` + `mfa_token`.
  4. `POST /auth/mfa/challenge` avec `{"code":"4F2LIBGHU5SD"}` (code public, jamais fourni par l'app à cet attaquant) → **HTTP 200, `token_pair.access_token` émis**.
- **Résultat attendu** : codes uniques par utilisateur, imprévisibles (`crypto/rand`).
- **Résultat réel** : graine **constante** `seed := int64(0x1234567890abcdef)` + LCG déterministe (`seed = (seed*1103515245 + 12345) & 0x7fffffff`), commentaire du code : *« not cryptographically secure »*. Les 8 codes sont donc les **mêmes pour tout le monde** et **dérivables depuis les sources AGPL publiques**.
- **Impact** : la MFA — **imposée à chaque compte** — n'offre aucune protection réelle. Tout attaquant disposant d'un mot de passe (phishing, réutilisation, fuite) obtient une session via un code de secours public. Faux sentiment de sécurité pour un produit qui vend la conformité.
- **Correctif recommandé** : générer chaque code avec `crypto/rand` (≥ 128 bits d'entropie), un par utilisateur ; invalider les codes de secours déjà émis en base ; ré‑enrôlement forcé. Ajouter un test qui asserte l'unicité entre deux appels.
- **Critères d'acceptation** : deux appels successifs → 16 codes tous distincts ; un code d'un utilisateur refusé pour un autre ; test de non‑régression.

### BUG-002 — [S1 · reliability] Le backend refuse de démarrer (migration `dirty` version 40)
- **Module** : boot / `golang-migrate` + `cmd/server/main.go`
- **Étapes** : lancer le binaire sur la DB de dev en l'état → AutoMigrate réussit, puis `SQL migrations failed: Dirty database version 40. Fix and force version.` → **`exit code 1`**.
- **Résultat réel** : l'application ne démarre pas du tout. Contourné pour l'audit via `UPDATE schema_migrations SET version=55, dirty=false`.
- **Impact** : bloque déploiement, CI, tout onboarding d'un nouveau dev. Pré‑existant (documenté de longue date) mais toujours non résolu.
- **Correctif** : corriger/rejouer la migration 0040 fautive, `migrate force`, et faire échouer proprement (ou avertir) plutôt que quitter en erreur ; réconcilier `migrations/` (0048–0055) avec l'historique `_archive`.

> **⚠️ CORRECTION (post-remédiation, issue #242).** Après investigation approfondie, les « 0 » observés sont **majoritairement un artefact du harnais de test headless**, PAS un bug produit réel : les chiffres sont rendus via une animation `useCountUp` basée sur `requestAnimationFrame`, or **rAF est gelé quand le panneau navigateur n'est pas composité** (prouvé live : aucun callback rAF ne se déclenche). Le composant détient la bonne donnée (`kpis.total=12`, `ScoreGauge` lit `score.value=57.4`) → dans un vrai navigateur, il affiche 12 / 57. **Ce constat était surévalué (pas S1 data-integrity).** Deux faiblesses *réelles* trouvées en le confirmant ont été corrigées (`d85c3d0`) : absence de refresh de token (session morte après 15 min) + `useCountUp` non-robuste (désormais respecte `prefers-reduced-motion`). Le seul chiffre *réellement* faux sur le dashboard (« Critiques : 0 » vs 3) relève de **BUG-004** (seuils `/stats`), pas de celui-ci.

### BUG-003 — [~~S1~~ → corrigé/reclassé · voir note] Le dashboard d'accueil affiche 0 risque / score 0 alors que 11 risques existent
- **Module / fichier** : `frontend/src/features/dashboard/DashboardPage.tsx` (PostureDashboard) + `useStats.ts`
- **Étapes** :
  1. Tenant A a 11 risques (registre le confirme, chip « Critique·3 »).
  2. Dans la **même session navigateur (cookie)** : `fetch('/api/v1/stats')` → `total_risks:11, global_risk_score:83` ; `fetch('/api/v1/risks?size=1')` → `total:11`.
  3. Le dashboard `/` rend : **« Risques totaux : 0 », « Critiques : 0 », « Score de sécurité : 0/100 »** — persistant après rechargement complet (donc pas une course de premier rendu), sans erreur console.
- **Résultat réel** : les cartes KPI et la jauge de score affichent 0 alors que la matrice et l'activité récente (mêmes données) sont peuplées. Défaut **localisé au frontend** (le payload correct n'atteint pas `KpiGrid`/`ScoreGauge`). La cause exacte de ligne nécessite un débogueur (hors périmètre observe‑only).
- **Impact** : l'écran d'atterrissage principal ment sur la posture ; perte de confiance immédiate d'un RSSI.
- **Correctif** : tracer `useDashboardStats` → `KpiGrid`/`ScoreGauge` ; ajouter un test qui monte le dashboard avec un `/stats` non‑vide et asserte les valeurs rendues.

### BUG-004 — [S2 · data-integrity] `/stats` calcule la sévérité sur des seuils incompatibles avec le Score Engine
- **Module / fichier** : `backend/internal/handler/dashboard_handler.go:86-89`
- **Détail** : `/stats` bande sur le **score brut** : CRITICAL ≥ 20, HIGH 15–20, MEDIUM 10–15, LOW < 10. Or le Score Engine et le registre bandent à **≥7 / ≥4 / ≥2** (P×I×AC, max ~30). Résultat pour 11 risques identiques : `/stats` = `{CRITICAL:0, HIGH:0, LOW:9, MEDIUM:2}` alors que le **registre et l'Executive dashboard** = `{critical:3, high:2, medium:3, low:3}`.
- **Preuve croisée** : `GET /analytics/executive` (`risk_distribution` + KRI `critical_risks:3`) **concorde avec le registre**, ce qui isole `/stats` comme fautif.
- **Impact** : compteurs de sévérité faux sur le dashboard (0 critique affiché vs 3 réels), `high_risks:0`.
- **Correctif** : bander `/stats` avec les seuils du Score Engine (≥7/≥4/≥2), ou mieux, réutiliser la colonne `criticality` déjà calculée par le ScoreWorker (source unique).

### BUG-005 — [S3 · bug] Champ `level` de risque figé à `medium`
- **Fichier** : DTO risque backend + `frontend/.../components/RiskCard.tsx:33` (`risk.level || 'MEDIUM'`).
- **Détail** : `level` renvoie toujours `medium` (score 0.1 comme 10). Le registre principal lit `criticality` (correct via `riskMap.ts`), donc rayon de blast limité, mais `RiskCard` colore la sévérité sur `level` → toujours « medium ».
- **Correctif** : supprimer `level` ou le dériver de `criticality` ; pointer `RiskCard` sur `criticality`.

### BUG-006 — [S3 · ux] Messages de validation bruts exposés à l'utilisateur
- **Étapes** : `POST /risks` sans titre → `400` avec `Key: 'CreateRiskInput.Title' Error:Field validation for 'Title' failed on the 'required' tag`.
- **Impact** : jargon développeur, viole QUOI/POURQUOI/QUE FAIRE.
- **Correctif** : mapper les erreurs de validation en messages humains i18n (« Le titre est obligatoire »).

### BUG-007 — [S3 · ux] Criticité `low` renvoyée pendant ~2 s après création (recalcul async)
- **Détail** : la réponse de `POST /risks` renvoie `criticality:"low"` jusqu'à ce que le ScoreWorker (Redis) recalcule (~2 s → `critical`). Tout client lisant la réponse de création immédiatement affiche une mauvaise bande.
- **Correctif** : calculer la bande de façon synchrone dans la réponse de création (le score `P×I×AC` est déjà connu) ; garder le worker pour les recomputes.

### BUG-008 — [S3 · ux/onboarding] MFA obligatoire dès l'inscription bloque l'Aha néophyte
- **Détail** : après register, `login` renvoie `mfa_enrollment_required` — l'utilisateur DOIT enrôler un TOTP (appli d'authentification) avant de voir le produit. Sécurité‑positif mais contre l'objectif « premier insight < 8 min » pour un profil non technique.
- **Correctif** : permettre l'exploration avant MFA, ou proposer email‑OTP, ou différer l'enrôlement avec une bannière — sans supprimer BUG‑001 d'abord.

### BUG-009 — [S4 · security-hygiene] Listes sous parent étranger renvoient `200 []` au lieu de `404`
- **Étapes** : en tant que Tenant B, `GET /risks/{risqueDuTenantA}/mitigations` → `200 []` ; idem `GET /compliance/frameworks/{étranger}/controls` → `200 []`.
- **Impact** : **aucune fuite** (liste filtrée par tenant, `[]` retourné même pour un id inexistant), mais incohérent avec la convention `404` du reste de l'app.
- **Correctif** : vérifier l'appartenance du parent et renvoyer `404` pour un parent étranger/inexistant.

---

## 12. Plan de correction (ordre recommandé)

1. **Security blockers** — BUG‑001 (codes de secours MFA : `crypto/rand`, unicité, invalidation, ré‑enrôlement forcé). **Avant toute release.**
2. **Reliability** — BUG‑002 (débloquer/rejouer la migration 0040 ; démarrage résilient).
3. **Data integrity / core** — BUG‑003 (dashboard zéro) puis BUG‑004 (seuils `/stats`), idéalement en unifiant la sévérité sur la colonne `criticality`.
4. **UX** — BUG‑006 (messages d'erreur), BUG‑008 (parcours MFA néophyte), BUG‑005/007 (`level`/async).
5. **Hygiène** — BUG‑009 (`404` sous parent étranger).
6. **À auditer ensuite (non couvert cette session)** — accessibilité (axe-core), responsive (375→1920), perf p95/p99 sous charge, OAuth2/SAML live, intégrations réelles, notifications, Attack Path/CTI, cycle mitigation→preuve→risque résiduel complet, les 34 parcours restants.

---

## Annexe — Ce qui a été prouvé PASS (avec preuve live)

- **Isolation multi‑tenant** : `GET/PATCH/DELETE` sur risque/asset/vuln/incident/contrôle/framework d'un autre tenant → **404** ; écriture inter‑tenant bloquée (contrôle inchangé en DB vérifié) ; listes d'un tenant vide → `[]`.
- **Score Engine** : `P×I×AC` exact ; bandes registre 7/4/2 exactes ; Executive dashboard cohérent avec le registre.
- **Injection** : 3 payloads SQLi → `total=0`, table `risks` intacte ; `<script>` stocké → rendu inerte (React).
- **Headers** : `Content-Security-Policy: default-src 'none'…`, `X-Frame-Options: SAMEORIGIN`, `X-Content-Type-Options: nosniff`, `Referrer-Policy: no-referrer`.
- **RBAC / PAT** : PAT `risks:read` → `GET /risks` 200 ; `DELETE`/`POST` → **403** (`Missing required permission`).
- **IA anti‑escape** : Tenant B (0 donnée) demandant les risques du Tenant A + « ignore permissions » → « no relevant item in your GRC knowledge base », `retrieved:null`.
- **Auth** : enrôlement TOTP, challenge TOTP, refresh — fonctionnels (paire RS256, cookies + CSRF).
- **Reporting** : PDF conformité **valide, 5 pages**.
- **CRQ** : ALE = SLE 40 M × ARO 0.5 = **20 M XAF / 33 333 USD**, breakdown complet.
- **Non‑auth** : routes protégées → `401` ; `/stats` fail‑closed.
