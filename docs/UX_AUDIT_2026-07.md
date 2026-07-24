# OpenRisk — Audit UX / parcours (2026-07-24)

Instrument : suite E2E Playwright (`tests/e2e/`) exécutée contre le stack réel
(backend `:8080`, frontend `:5173`, Postgres `:5434`, Redis). **95 tests passent,
0 échec, 19 `fixme`** (flux cassés écrits mais quarantainés). Aucun bug produit
n'a été corrigé dans cette session — seul l'ajout d'attributs `data-testid` a
touché le code produit (commit `test(e2e): add stable test ids`).

Preuves reproductibles : `npm run test:e2e` → rapport HTML + traces + captures
sous `tests/e2e/.artifacts/`, table route×statut via `node scripts/e2e-route-summary.mjs`.

---

## 1. Synthèse (10 lignes)

1. **Le socle tient** : les **42 routes rendent** (0 écran blanc, 0 crash React, 0 réponse 5xx sur la navigation), sur desktop **et** Mobile Chrome.
2. Le **cœur métier fonctionne de bout en bout** : créer un risque → il apparaît → on l'ouvre ; **time-to-value mesuré à 3,7 s** (UX-01, chemin authentifié).
3. **Le trou le plus grave n'est pas un crash, c'est l'entrée** : l'inscription (`RegisterForm`) et la MFA (`MfaForm`) sont des **façades sans backend** — aucun compte n'est créé. Un nouvel utilisateur ne peut pas commencer seul (**UX-01/02/13 en échec**).
4. **Deuxième trou : la collaboration.** Il n'existe **aucun chemin pour ajouter un membre à un tenant** (`POST /users` → 404 puis pas de rattachement d'organisation). RBAC, équipes, dashboards par rôle (UX-24) sont donc non atteignables par l'utilisateur.
5. **Écrans de confiance pollués par des fixtures** : Paramètres affiche des données en dur (`amir@banque-atlantique.ci`, `€24k/an`, `MacBook Pro · Abidjan`, `18/50 membres`) ; la sidebar affiche un score `72` et l'org « Banque Atlantique » codés en dur — **UX-05/UX-23 en échec**, et un risque de perte de confiance chez un auditeur.
6. **Accessibilité sous le seuil** : `axe-core` remonte des violations **serious/critical** sur 5 des 6 écrans clés (contraste partout, plus labels/`button-name` manquants dans Paramètres). `/risks` est propre — preuve que le harnais détecte de vrais défauts.
7. **Navigation en surcharge** : **7 groupes / ~20 entrées** de premier niveau, dont **4 placeholders `soon:true`** (Leaderboard, Infrastructure, Simulations, Universe) mêlés aux vraies features — dette de charge cognitive (UX-16/UX-05/UX-31).
8. **Vocabulaire non traduit pour la cible** : « CVE », « KEV », « EPSS », « P×I×AC », « Smart score » apparaissent sans glose — Awa (conformité microfinance, zéro cyber) décroche (UX-03/UX-07).
9. **Bonnes fondations à capitaliser** : ⌘K existe, le cycle de vie ISO 31000, la gap-analysis, le rapport PDF, la piste d'audit gouvernance et le dashboard exécutif sont réels et rendent — la matière première d'une grande app est là.
10. **Verdict** : produit **démontrable**, pas encore **auto-servi**. Priorité absolue des sessions suivantes : (P0) chemin d'inscription réel + invitation de membre ; (P1) purge des fixtures des écrans de confiance ; (P2) accessibilité + dégraissage de la navigation.

**Le risque le plus grave** : un prospect qui teste seul (le mode d'évaluation par
défaut d'un SaaS) **ne peut littéralement pas créer de compte** — l'inscription ne
fait rien. Tout le reste est secondaire tant que la porte d'entrée est murée.

---

## 2. Table de couverture des routes

42 routes parcourues par `smoke.routes.spec.ts` (chromium + Mobile Chrome).
Statut : **OK** rend + données · **dégradé** rend mais erreurs console/5xx · **placeholder**
`soon:true` (écran « bientôt ») · **mock** rend des données factices · **cassé** écran blanc/crash.
Preuve = résultat E2E `passed` + capture archivée (`tests/e2e/.artifacts/`), sauf mention.

| Route | Écran | Statut | Preuve | Sév. | Session |
|-------|-------|--------|--------|------|---------|
| `/` | Dashboard | 🟡 mock (widgets) | passed ; score sidebar en dur `Sidebar.tsx:94` ; a11y contraste | P2 | S4 |
| `/analytics` | Dashboard exécutif | ✅ OK | passed ; ALE/cyber-score réels | — | — |
| `/analytics/financial` | Quantification financière | ✅ OK | passed | — | — |
| `/leaderboard` | Leaderboard | 🔵 placeholder | passed ; `navModel.ts:45 soon:true` | P2 | S4 |
| `/risks` | Registre des risques | ✅ OK | passed ; a11y **propre** | — | — |
| `/risks/import` | Import de risques | ✅ OK | passed | — | — |
| `/risks/weighting` | Pondération smart-risk | ✅ OK | passed | — | — |
| `/risks/:id/timeline` | Chronologie du risque | ✅ OK | passed (id seedé) | — | — |
| `/vulnerabilities` | Vulnérabilités | ✅ OK | passed ; a11y contraste | P2 | S4 |
| `/mitigations` | Tableau des mitigations | ✅ OK | passed | — | — |
| `/incidents` | Registre des incidents | ✅ OK | passed | — | — |
| `/incidents/:id/war-room` | War Room | ✅ OK (partiel) | passed ; roster/chat = « Aperçu » | P3 | S4 |
| `/automation` | Automatisation / SOAR | ✅ OK | passed | — | — |
| `/infrastructure` | Scanner d'infra | 🔵 placeholder | passed ; `navModel.ts:56 soon:true` | P2 | S4 |
| `/infrastructure/scans/:jobId` | Aperçu de scan | 🟡 dégradé | passed ; erreurs console sur id inconnu | P3 | S4 |
| `/compliance` | Conformité | ✅ OK | passed ; a11y contraste | P2 | S4 |
| `/compliance/:id` | Détail du référentiel | ✅ OK | passed (id seedé) | — | — |
| `/compliance/gap-analysis` | Analyse des écarts | ✅ OK | passed | — | — |
| `/compliance/audits` | Audits | ✅ OK | passed | — | — |
| `/compliance/remediations` | Plans de remédiation | ✅ OK | passed | — | — |
| `/threat-map` | Intel Threat (CTI) | ✅ OK | passed | — | — |
| `/simulations` | Simulations | 🔵 placeholder | passed ; `navModel.ts:64 soon:true` | P2 | S4 |
| `/assets` | Inventaire des actifs | ✅ OK | passed | — | — |
| `/assets/universe` | Asset Universe | 🔵 placeholder | passed ; `navModel.ts:71 soon:true` | P2 | S4 |
| `/reports` | Rapports | ✅ OK | passed | — | — |
| `/reports/board` | Board Report | ✅ OK | passed | — | — |
| `/recommendations` | Assistant IA | ✅ OK | passed | — | — |
| `/ai/emerging-risks` | Risques émergents (IA) | ✅ OK | passed | — | — |
| `/governance` | Gouvernance | ✅ OK | passed | — | — |
| `/settings` | Paramètres | 🟡 mock | passed ; fixtures `SettingsScreen.tsx:420+` ; a11y label/button-name | P1 | S3 |
| `/settings/roles` | Rôles & accès | ✅ OK | passed | — | — |
| `/users` `/roles` `/tenants` `/audit-logs` `/tokens` `/marketplace` `/custom-fields` `/analytics/permissions` | → Paramètres | ↪️ redirect | passed (8 redirections) | — | — |
| `/risk-management` `/bulk-operations` | → Risques | ↪️ redirect | passed | — | — |
| `*` (inconnue) | → Dashboard | ↪️ redirect | passed | — | — |

**Bilan : 42 routes · 0 cassé · 1 dégradé · 4 placeholders · 2 écrans mock ·
11 redirections.** Aucune régression bloquante — la surface est saine ; les défauts
sont de **maturité** (entrée, données réelles, a11y), pas de **stabilité**.

---

## 3. Registre de bugs

> Chaque entrée : ID stable · reproduction ≤ 3 étapes · attendu · cause racine (si
> identifiée) · correction proposée. Assignée à une session ultérieure (le gel du
> code produit de cette session interdit toute correction ici).

### OR-BUG-001 — L'inscription ne crée aucun compte · **P0** · Session 2
- **Repro** : (1) aller à `/register` ; (2) remplir les champs ; (3) cliquer « Créer un compte ».
- **Attendu** : un compte + un tenant sont créés, l'utilisateur entre dans l'app (UX-01/13).
- **Constaté** : le formulaire bascule simplement vers l'écran MFA, puis « valider » navigue vers `/` sans jeton → `ProtectedRoute` renvoie à `/login`. Aucun appel réseau.
- **Cause racine** : `AuthScreen.tsx:177` `RegisterForm onSubmit={() => onMfa()}` et `:203` `MfaForm onSubmit={() => navigate('/')}` — deux façades sans backend.
- **Correction proposée** : câbler `POST /auth/register` (ou le flux d'inscription existant) → jeton → landing ; réduire à ≤ 3 champs (UX-02). Test de sortie : `journey.newcomer` « UX-02/UX-13 ».

### OR-BUG-002 — Aucun onboarding ni guidage premier lancement · **P1** · Session 2
- **Repro** : (1) première connexion ; (2) observer l'écran d'accueil.
- **Attendu** : l'app conduit vers la première action de valeur (UX-01/07/32).
- **Constaté** : dépôt direct sur le dashboard, sans première tâche ni micro-victoire.
- **Correction proposée** : premier lancement = « créez votre premier risque / importez un référentiel » ; célébrer le premier risque (UX-32).

### OR-BUG-003 — Impossible d'ajouter un membre à un tenant · **P0** · Session 3
- **Repro** : (1) admin ; (2) tenter de créer un utilisateur analyste dans son org via l'API/UI.
- **Attendu** : un membre est créé, peut se connecter, reçoit un rôle métier (UX-24).
- **Constaté** : `POST /users` → **404** dans cet environnement ; même quand il répond, il crée un `domain.User` **sans `OrganizationMember`** → login sans tenant résolu (cf. `seed-e2e.mjs` personas `usable:false`, raison enregistrée).
- **Cause racine** : pas de use case d'invitation/rattachement d'org exposé.
- **Correction proposée** : endpoint d'invitation créant `User` + `OrganizationMember(tenant, business_role)` ; débloque `journey.rbac` et les dashboards par rôle.

### OR-BUG-004 — Paramètres affiche des fixtures, toggles inertes, pas d'autosave · **P1** · Session 3
- **Repro** : (1) `/settings` ; (2) onglet Notifications/Sécurité/Facturation ; (3) basculer un toggle puis recharger.
- **Attendu** : données réelles du tenant ; changement persistant avec « Enregistré ✓ » (UX-23/UX-25) ; aucun élément inerte (UX-05).
- **Constaté** : e-mails/plans/sessions **en dur** (`SettingsScreen.tsx` fonctions `NotifTab`/`SecurityTab`/`BillingTab`), toggles = `useState` local non persisté (cf. `journey.settings` « UX-23/UX-25 » en `fixme`).
- **Correction proposée** : brancher chaque onglet sur son endpoint réel ; autosave par champ.

### OR-BUG-005 — Score de sécurité et nom d'organisation en dur dans la sidebar · **P2** · Session 3
- **Repro** : (1) n'importe quel écran ; (2) regarder le pied de la sidebar et l'entête.
- **Attendu** : score réel du tenant ; nom réel de l'organisation.
- **Constaté** : `Sidebar.tsx:94` `const score = 72` ; `:198` `Banque Atlantique` / `:190` badge `BA` codés en dur.
- **Correction proposée** : lire le score depuis le dashboard exécutif (déjà calculé) ; l'org depuis la session.

### OR-BUG-006 — Aperçu de scan bruyant sur job inconnu · **P3** · Session 4
- **Repro** : (1) ouvrir `/infrastructure/scans/<uuid-inconnu>`.
- **Attendu** : état vide propre (UX-04), sans erreur console.
- **Constaté** : rend l'état vide mais journalise des erreurs (statut « dégradé »).
- **Correction proposée** : court-circuiter le fetch sur job absent → état vide guidé.

### OR-BUG-007 — 4 placeholders `soon:true` mêlés à la vraie navigation · **P2** · Session 4
- **Repro** : (1) parcourir la sidebar ; (2) cliquer Leaderboard / Simulations / Infrastructure / Universe.
- **Attendu** : ne pas exposer d'entrées sans valeur, ou les marquer clairement (UX-05/UX-31).
- **Constaté** : `navModel.ts` `soon:true` sur 4 items ; écrans « bientôt ».
- **Correction proposée** : les regrouper en « À venir » (divulgation progressive) ou les retirer — voir `IA_NAVIGATION_PROPOSAL.md`.

### OR-BUG-008 — Rate limiter d'auth trop agressif · **P2** · Session 4
- **Repro** : enchaîner ~6–8 logins (tests, seed) → `429 Rate limit exceeded` durablement.
- **Attendu** : protéger le brute-force sans bloquer un usage légitime (CI, MFA, multi-onglets).
- **Constaté** : le seed E2E doit implémenter un backoff pour ne pas se coincer (`seed-e2e.mjs login()`).
- **Correction proposée** : fenêtre/He plus tolérante par IP+compte, exempter les réussites, message UX-03 explicite avec délai.

### OR-BUG-009 — MFA « validation » sans vérification · **P1 (sécurité)** · Session 2
- **Repro** : (1) `/register` → MFA ; (2) cliquer valider.
- **Attendu** : un code TOTP est réellement vérifié.
- **Constaté** : `AuthScreen.tsx:203` navigue vers `/` sans vérifier ni poser de jeton — façade (inoffensive aujourd'hui car pas de jeton, mais trompeuse).
- **Correction proposée** : brancher le vrai challenge MFA (`POST /auth/mfa/challenge`, déjà existant côté backend).

### OR-BUG-010 — Vocabulaire cyber non traduit pour la cible non-technique · **P1** · Session 3
- **Repro** : (1) profil conformité ; (2) lire Dashboard / Vulnérabilités / Risques.
- **Attendu** : glose ou libellé métier pour CVE, KEV, EPSS, P×I×AC (UX-03/UX-07).
- **Constaté** : acronymes bruts, sans infobulle de première rencontre (UX-14).
- **Correction proposée** : infobulles contextuelles + libellés « en clair » (ex. « KEV = faille activement exploitée »).

### OR-BUG-011 — Contraste WCAG AA insuffisant (5 écrans) · **P2** · Session 4
- **Repro** : `a11y.spec.ts` sur `/`, `/compliance`, `/vulnerabilities`, `/analytics`, `/settings`.
- **Attendu** : 0 violation `serious/critical` (WCAG 2.1 AA).
- **Constaté** : `color-contrast` (serious) sur les 5 ; rapports axe joints par écran dans les artefacts.
- **Correction proposée** : relever les ratios texte secondaire / états ; re-tester avec `A11Y_KNOWN` vidé.

### OR-BUG-012 — Paramètres : champs sans label, boutons sans nom accessible · **P2** · Session 4
- **Repro** : `a11y.spec.ts` sur `/settings`.
- **Attendu** : chaque `input` a un label, chaque bouton un nom accessible (WCAG 4.1.2 / 1.3.1).
- **Constaté** : `input value="GMT · Abidjan"` sans label, toggles sans `aria-label` (`SettingsScreen.tsx`).
- **Correction proposée** : `<label>`/`aria-label` sur les champs et toggles.

---

## 4. Écrans mock / données factices

| Emplacement | Donnée factice | Preuve |
|-------------|----------------|--------|
| Sidebar (pied) | Score de sécurité **72/100** | `Sidebar.tsx:94` |
| Sidebar (entête) | Organisation **« Banque Atlantique »**, badge **BA** | `Sidebar.tsx:190,198` |
| Paramètres › Notifications | e-mail **amir@banque-atlantique.ci**, canal **#soc-alerts** | `SettingsScreen.tsx` `NotifTab` |
| Paramètres › Sécurité | Sessions **MacBook Pro · Abidjan / iPhone 15 / Windows · Dakar** | `SettingsScreen.tsx` `SecurityTab` |
| Paramètres › Facturation | Plan **Enterprise €24k/an**, usage **18/50 · 142/500 · 23/100** | `SettingsScreen.tsx` `BillingTab` |
| Paramètres › Général | champ **« GMT · Abidjan »** (sans label) | `SettingsScreen.tsx` |
| Dashboard | Widgets `SecurityScore` / `AssetStatistics` sur endpoints inexistants (repli gracieux) | cf. `CLAUDE.md` §Partiel |
| War Room | Roster / chat / tâches = fixtures « Aperçu » | `/incidents/:id/war-room` |

Aucune de ces valeurs n'existe en base pour le tenant connecté — elles minent la
confiance sur les écrans que **l'auditeur** et **la direction** regardent en premier.

---

## 5. Testabilité (sélecteurs stables)

Avant cette session, **aucun sélecteur stable** n'existait ; les écrans s'appuient
sur des classes utilitaires Tailwind volatiles et du texte i18n. Ajoutés (commit
`add stable test ids`, attribut seul) : `login-email/password/submit`,
`nav-<key>`, `app-main`, `settings-tab-<key>`. **Restent sans sélecteur stable**
(les tests s'appuient sur le texte, donc fragiles) :

- **Lignes de liste** : risques, vulnérabilités, incidents, cartes de référentiel, membres — pas de `data-testid` par ligne/entité.
- **Modales** : Création/Édition de risque, de mitigation, d'actif, dialogs de conformité — pas de `data-testid` sur les champs ni le bouton de soumission (le harnais cible `input[name="title"]` / `button[type=submit]`, ce qui casse dès qu'une 2ᵉ modale coexiste).
- **Drawers** : détail de risque (onglets Cycle de vie / Financier / Score intelligent / IA), détail de vulnérabilité — onglets non ciblables.
- **Quick actions** : « Nouveau risque » n'est atteignable que via l'événement `window` `openrisk:new-risk` (pas de bouton ciblable de façon stable).
- **KPI / cartes** : bandeaux de stats (dashboard, vulnérabilités, incidents) sans hook de valeur.

Recommandation : convention `data-testid="<entité>-row-<id>"`, `<modale>-submit`,
`<drawer>-tab-<key>`, ajoutée au fil des sessions de correction (attribut seul).

---

## 6. Anti-bloat documentaire

`docs/` contient **188 fichiers `.md`** (mesuré : `find docs -name '*.md' | wc -l`),
en tension avec la règle « deux documents de pilotage vivants » de PROJECT_PLAN.md.
Classement complet (vivant / archive / à supprimer) — **proposition seule, aucune
suppression** — dans [`DOCS_INVENTORY.md`](DOCS_INVENTORY.md).

---

## 7. Parcours des personas

### 7.1 Awa — responsable conformité microfinance, zéro background cyber
Objectif : produire un **état COBAC** sans savoir ce qu'est un CVE.
- ✅ `/compliance` : peut importer un référentiel (COBAC/BCEAO présents), voir la progression, exporter un PDF, lancer une gap-analysis — **le chemin existe et rend**.
- ⛔ **Bloquée à l'entrée** si elle arrive sans compte (OR-BUG-001).
- ⚠️ **Vocabulaire** : le Dashboard et les Vulnérabilités lui parlent « CVE / KEV / EPSS / P×I×AC » sans glose (OR-BUG-010) — elle ne sait pas si ça la concerne.
- ⚠️ **Guidage** : rien ne lui dit « pour votre état COBAC, commencez ici » — pas de piste depuis l'accueil (OR-BUG-002, UX-07).
- ⚠️ **Confiance** : le score « 72 » et « Banque Atlantique » en dur la troublent (est-ce mes données ?) (OR-BUG-005).
- **Verdict** : la matière est là, la **conduite** manque. Awa réussit si on lui met un rail « Conformité » et qu'on traduit le jargon.

### 7.2 Un RSSI — « où suis-je exposé, en 30 secondes ? »
- ✅ `/analytics` (Dashboard exécutif) : cyber-score A–F, ALE en FCFA, top-10 risques, KRI, tendance incidents — **la réponse tient en un écran, données réelles**.
- ⚠️ **Landing** : il atterrit sur `/` (dashboard générique), pas sur sa vue exécutive (UX-24 partiel — le landing par rôle existe pour certains rôles métier mais pas pour l'admin).
- ⚠️ `/vulnerabilities` priorise bien (P1/KEV/flamme) mais **contraste faible** (OR-BUG-011).
- **Verdict** : **le meilleur parcours de l'app.** À un clic près (landing direct), le RSSI est servi en 30 s.

### 7.3 Un auditeur externe — « la preuve, pas le dashboard »
- ✅ `/governance` : piste d'audit immuable, diff avant/après, export CSV — exactement ce qu'il veut.
- ✅ `/compliance/:id` : preuves par contrôle, citations de source, rapport PDF ; `/compliance/audits` + remédiations.
- ⛔ **Confiance entamée** dès qu'il ouvre `/settings` (plans/sessions/e-mails fictifs) : un auditeur qui voit une donnée inventée doute de **toutes** les autres (OR-BUG-004).
- **Verdict** : les artefacts de preuve sont **solides** ; il faut **purger les fixtures** des écrans périphériques pour ne pas saboter la crédibilité de l'ensemble.

---

## 8. Verdict d'ensemble et priorisation des sessions

| Session | Thème | Bugs |
|---------|-------|------|
| **S2** | Activation : inscription + onboarding + MFA réels | OR-BUG-001, 002, 009 |
| **S3** | Multi-utilisateur & données réelles | OR-BUG-003, 004, 005, 010 |
| **S4** | Accessibilité & dégraissage | OR-BUG-006, 007, 008, 011, 012 |

Les deux propositions structurantes ([`IA_NAVIGATION_PROPOSAL.md`](IA_NAVIGATION_PROPOSAL.md),
[`UI_ELEVATION_PROPOSAL.md`](UI_ELEVATION_PROPOSAL.md)) doivent être **validées par le
fondateur avant toute implémentation** (session 5).
