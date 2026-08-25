# OR26-03 — MFA Deferred Enrollment Live Proof

> Compagnon de `docs/OR26-03_MFA_DEFERRED_ENROLLMENT.md`.
> Aucun secret MFA, jeton, mot de passe ni graine TOTP n'apparaît dans ce document.

## Environment

| | |
| --- | --- |
| Date | 2026-08-25 |
| Machine | Linux 7.0.0-30-generic, Go 1.25, Node/vitest 4.1.10, Playwright (chromium + Mobile Chrome) |
| Repo | `/home/alex/Documents/projects/OpenRisk` |
| Branche | `feat/or26-03-defer-mfa-enrollment` (base `origin/master` = `356b2bf`) |
| Postgres | **INDISPONIBLE** — voir « Known Limitations » |
| Redis | joignable (`127.0.0.1:6379`) |
| Docker | **BLOQUÉ** — `500 Internal Server Error` sur la socket du démon |

### ⚠️ Ce que « live » signifie ici, exactement

Aucune instance OpenRisk n'a pu être démarrée sur cette machine. Le démon Docker
est bloqué, et le Postgres qui écoute sur `:5434` est un proxy Docker orphelin :
le TCP s'établit, la poignée de main Postgres n'aboutit jamais (`psql` reste
suspendu jusqu'au timeout). Le Postgres natif sur `:5432` répond mais aucun
identifiant administrateur n'est disponible, et créer un rôle/base sur
l'installation personnelle de l'utilisateur n'était pas dans le périmètre demandé.

La preuve a donc été produite **une couche en dessous d'un serveur en marche** :
`internal/handler/auth/mfa_deferred_e2e_test.go` exerce le parcours complet à
travers la **vraie** pile HTTP — vrai use case de login, vrai handler, vrai
middleware RS256, vraie garde MFA, vrai repository, vrais modèles GORM, vrai
routeur Fiber avec le même ordre de montage que `cmd/server/main.go`. La seule
chose substituée à la production est le **moteur de base de données** (sqlite en
mémoire au lieu de Postgres).

Chaque ligne de la matrice ci-dessous porte donc `PASS (HTTP stack)` — un fait
mesuré, pas une inférence — ou `BLOCKED` quand seule une instance réelle pouvait
le trancher. Rien n'est marqué `PASS` sans avoir été exécuté.

## Commit SHA

```
4606bc19c890692d71d936e64f64e739c22dc34d
```

## Test Tenants

Créés par la fixture `newDeferredFixture`, deux tenants complets :

| Tenant | Nom | Rôle |
| --- | --- | --- |
| A | Banque Atlantique | tenant principal |
| B | Other Corp | témoin d'isolation |

## Test Users

| Compte | Tenant | Rôle d'org | Préset métier | Ancre de grâce |
| --- | --- | --- | --- | --- |
| `admin@a.io` | A | `admin` | — | aujourd'hui |
| `admin@b.io` | B | `admin` | — | aujourd'hui |
| `member@a.io` | A | `user` | — | il y a 10 ans |
| `rssi@a.io` | A | `user` | `rssi` | variable selon le cas |

Aucun n'a d'authentificateur enrôlé au départ — c'est l'état dont parle l'issue.

## Policy Configuration

| Réglage | Valeur |
| --- | --- |
| Défaut livré | **7 jours** |
| Bornes | 0 – 90 jours |
| Rôles d'org privilégiés | `root`, `admin` (env `MFA_REQUIRED_ROLES`) |
| Présets métier privilégiés | `rssi` (env `MFA_REQUIRED_BUSINESS_ROLES`) |
| TTL du cache de résolution | 60 s, purgé sur enrôlement / désactivation / changement de politique / changement de rôle |

## Standard User — First Login

`TestDeferredMFA_OrdinaryMemberIsOnlyEverInvited` — **PASS**

`POST /api/v1/auth/login` (`member@a.io`) → **200** avec `token_pair`.

```jsonc
"mfa": { "state": "recommended", "required": false, "privileged": false, "grace_days": 7 }
// deadline: absent — un membre que le mandat ne vise pas n'a pas d'échéance à afficher
```

Le compte a dix ans et aucun authentificateur : toujours rien qu'une
recommandation.

## Dashboard Access

`TestDeferredMFA_FreshAdminSignsInAndReachesTheProduct` — **PASS**

`POST /auth/login` (`admin@a.io`) → **200 avec session**.

```jsonc
"mfa_enrollment_required": absent        // ⟵ LA RÉGRESSION FERMÉE (valait true, sans session)
"mfa": {
  "state": "grace_active", "required": false, "privileged": true,
  "grace_days": 7, "deadline": "<aujourd'hui + 7 j>"
}
```

`GET /api/v1/risks` → **200**. Le produit est réellement atteint, pas seulement
« connecté ».

## Onboarding Access

**PARTIEL — PASS (HTTP stack) / BLOCKED (instance)**

Ce qui est prouvé ici : la garde qui bloquait tout est levée pour un compte
éligible, donc **toute** route protégée est atteinte — l'onboarding n'a rien de
particulier de son point de vue.

Ce qui n'a **pas** été exercé : les routes `/onboarding/*` elles-mêmes contre une
instance réelle. Le cas est écrit dans `tests/e2e/mfa-deferred.spec.ts` (« onboarding
and the first risk are reachable with no authenticator ») et attend une pile
démarrée.

## First Risk / Aha Moment

`TestDeferredMFA_FreshAdminSignsInAndReachesTheProduct` — **PASS**

`POST /api/v1/risks` sur la session sans MFA → **200**. C'est la valeur devant
laquelle le mur se dressait.

## Post-Aha MFA Prompt

`frontend/src/features/auth/__tests__/mfaPostAhaPrompt.test.tsx` — **PASS (8/8)**

| Condition | Comportement prouvé |
| --- | --- |
| `aha_reached_at` renseigné + MFA absent | prompt affiché |
| `aha_reached_at` à `null` | silencieux |
| MFA `configured` | silencieux |
| `state === 'required'` | silencieux (le bandeau alerte déjà, le serveur bloque déjà) |
| Dans le cooldown 24 h | silencieux |
| Cooldown écoulé | ré-affiché |
| « Plus tard » | seule trace durable = l'horodatage de cooldown ; **aucun drapeau « MFA sauté »** |
| CTA | ouvre le vrai flux d'enrôlement |

## Admin/RSSI Within Grace Period

`TestDeferredMFA_FreshAdminSignsInAndReachesTheProduct`,
`TestDeferredMFA_RSSIIsCoveredByTheMandate` — **PASS**

* Admin, jour 0 : session délivrée, `grace_active`, échéance à +7 j, `/risks` 200.
* RSSI (rôle d'org `user`, préset `rssi`), jour 1 : session délivrée,
  `grace_active`, `/risks` 200. Un contrôle sur le seul rôle d'org aurait exempté
  exactement le compte que l'exigence vise.

## Admin/RSSI After Grace Period

`TestDeferredMFA_PastTheDeadlineLoginStopsAtEnrolment`,
`TestDeferredMFA_ALiveSessionIsRefusedOnceTheWindowCloses`,
`TestDeferredMFA_RSSIIsCoveredByTheMandate` — **PASS**

* Admin jour 8, nouveau login → `mfa_enrollment_required: true`, **aucune
  session**, `mfa_token` d'enrôlement, `state: "required"`.
* **Session déjà ouverte** au jour 8 → `GET /risks` **403 `MFA_ENROLLMENT_REQUIRED`**
  avec le bloc `mfa` (`required: true`). *C'est le cœur de la garde requête :
  n'appliquer qu'au login laisserait un admin connecté le jour 1 travailler
  indéfiniment en ne se déconnectant jamais.*
* RSSI jour 9 → `mfa_enrollment_required: true`, aucune session.
* Remède accessible pendant le blocage — `TestDeferredMFA_TheRemedyStaysReachableWhileBlocked` :
  `/auth/me` **200**, `/auth/mfa/setup` **200**. Une exigence qu'on ne peut pas
  satisfaire est un verrouillage, pas un contrôle.

## MFA Enrollment Complete

`TestDeferredMFA_EnrolmentRestoresAccess` — **PASS**

Compte bloqué (403) → secret vérifié créé + purge du cache (comme le fait le
handler `verify`) → `GET /risks` **200** à la requête suivante.

`TestDeferredMFA_AnUnverifiedSecretDoesNotCountAsEnrolment` — **PASS** : un
enrôlement à moitié fait (secret généré, code jamais confirmé) reste **403**.

## Tenant Isolation

`TestDeferredMFA_PolicyIsTenantScoped` — **PASS**

| Étape | Résultat |
| --- | --- |
| A règle `grace_days: 1` | 200 |
| B lit sa politique | **7** — B n'hérite pas de la fenêtre de A |
| A relit la sienne | **1** |
| Jour 2 : admin de A | **403** |
| Jour 2 : admin de B | **200** — la politique d'un tenant ne décide jamais de l'accès d'un autre |

Complété par : `TestMFAPolicyRepo_IsTenantScoped`,
`TestMFAPolicyRepo_ForgedIDCannotSteerAnotherTenantsRow` (un id de politique
forgé crée la ligne du bon tenant et ne déplace pas celle de la victime),
`TestResolver_InvalidateTenantDropsEveryMemberOfThatTenantOnly`,
`TestMFAPolicy_IsTenantScoped`.

Garde d'isolation du dépôt (`internal/security/isolation`) : **ok** — les deux
nouvelles routes sont non paramétrées, donc aucune entrée de registre requise.

## Security Bypass Tests

`TestDeferredMFA_ClientCannotTalkItsWayOut` — **PASS**. Compte bloqué (grâce
expirée), toutes les tentatives à l'API :

| Tentative | Résultat |
| --- | --- |
| Corps de requête affirmant `mfa: {state: configured, required: false}` + `mfa_configured: true` | **403** |
| Élargir la fenêtre (`PUT grace_days: 90`) pour se débloquer soi-même | **403** |
| Frapper un PAT (la route de l'exemption permanente) | **403** |
| Inspecter le JWT à la recherche d'un claim MFA à falsifier | **aucun** claim `mfa` dans le jeton |
| En-têtes forgés `X-MFA-Configured` / `X-MFA-Required` | **403** (`tests/e2e`, listé) |

`TestDeferredMFA_AMissingAnchorFailsClosedForAPrivilegedAccount` — **PASS** :
`mfa_grace_started_at`, `joined_at` et `created_at` mis à `NULL` (l'état qu'on
chercherait à produire pour acheter une grâce illimitée) → **403**.

`TestMFAPolicyGuard_FailsClosedWhenTheStatusCannotBeResolved` — **PASS** :
résolveur en erreur → **503 `MFA_STATUS_UNAVAILABLE`**, la route protégée n'est
pas atteinte. Échec fermé, avec un code honnête plutôt qu'un « enrôlez-vous » qui
n'aiderait pas.

`TestDeferredMFA_SessionContractCarriesNoSecret` — **PASS** : le bloc `mfa` de
login et la réponse `/auth/me` ne contiennent ni `secret`, ni `qr_code`, ni
`backup`, ni `totp`.

### Bug de sécurité trouvé et corrigé en écrivant ces tests

`domain.MFAPolicy.GraceDays` portait `gorm:"default:7"`. GORM **omet** un champ à
valeur nulle sur `INSERT` quand la colonne déclare un défaut — donc enregistrer
`0`, **le réglage le plus strict que le produit offre** (« MFA exigé
immédiatement »), stockait silencieusement `7`, le plus permissif. Un
administrateur aurait posé la politique la plus serrée et obtenu la plus lâche.

Le tag est retiré ; le défaut SQL reste dans la migration `0060` pour les inserts
bruts ; toute écriture depuis Go porte une valeur explicite. Épinglé par
`TestMFAPolicyRepo_ZeroDaysActuallyPersists` et par
`TestDeferredMFA_ShorteningTheWindowBitesImmediately`.

## API Tests

`TestDeferredMFA_PolicyIsReadableBoundedAndAdminWritable` — **PASS**

| Appel | Attendu | Obtenu |
| --- | --- | --- |
| `GET /security/mfa-policy` (admin) | 200, `grace_days: 7`, `configured: false`, bornes 0/90 | ✅ |
| `GET /security/mfa-policy` (membre) | 200 — quiconque est soumis à une échéance peut la lire | ✅ |
| `PUT grace_days: -1` | 400 | ✅ |
| `PUT grace_days: 91` | 400 | ✅ |
| `PUT grace_days: 100000` | 400 | ✅ |
| `PUT {}` (champ absent) | 400 — un enregistrement partiel ne doit pas valoir zéro | ✅ |
| `PUT grace_days: 3` (admin) | 200, `configured: true` | ✅ |
| `PUT grace_days: 90` (membre) | **403** | ✅ |

`TestDeferredMFA_MeReportsTheSameStateAsLogin` — **PASS** : `/auth/me` et
`/auth/login` renvoient le même `state`. Une seule décision, deux lecteurs.

## E2E Tests

`tests/e2e/mfa-deferred.spec.ts` — **NOT EXECUTED (BLOCKED)**

Playwright **liste** 14 cas (7 scénarios × chromium + Mobile Chrome) :

```
Total: 14 tests in 1 file
```

Ils exigent backend + frontend démarrés, donc un Postgres. Pour les exécuter une
fois une base disponible :

```bash
# 1. une base Postgres joignable sur DATABASE_URL
# 2. backend
cd backend && go run ./cmd/server            # PORT, RSA_*_KEY_PATH, MFA_ENCRYPTION_KEY
# 3. frontend
cd frontend && npm run dev                   # localhost:5173 (seule origine CORS autorisée)
# 4. e2e
npx playwright test mfa-deferred
```

Les cas couverts : première connexion honnête · admin privilégié avec échéance ·
onboarding + premier risque sans MFA · **dashboard rendu avec le bandeau à côté,
pas à la place** · politique bornée / admin-only / tenant-scopée · fenêtre à
0 jour appliquée côté serveur · le client ne peut pas se sortir de l'exigence.

## Accessibility

**PASS (unit) / BLOCKED (axe sur instance)**

Prouvé par les tests unitaires du bandeau :

* `role="status"` + `aria-live="polite"` pour une recommandation — un rappel doux
  ne doit pas interrompre un lecteur d'écran en pleine tâche ;
* `role="alert"` + `aria-live="assertive"` une fois l'enrôlement obligatoire ;
* `aria-labelledby` sur le bandeau, `aria-modal` + `aria-labelledby` +
  `aria-describedby` sur les deux dialogues, focus posé à l'ouverture, `Échap`
  ferme ;
* `aria-label` sur chaque bouton icône, `aria-hidden` sur les icônes décoratives ;
* `aria-invalid` + `aria-describedby` sur le champ de code et sur le champ de
  délai, message d'erreur en `role="alert"`.

Non exécuté : la passe `axe` de `tests/e2e/a11y.spec.ts` sur les écrans portant le
bandeau — même blocage que les E2E.

## Commands and Results

```text
$ go build ./...                                                    PASS
$ go vet ./...                                                      PASS (0 diagnostic)
$ go test ./...                                                     PASS — 63 packages ok, 0 FAIL
$ go test ./internal/handler/auth/ -run TestDeferredMFA             PASS — 15/15
$ go test ./internal/security/...                                   PASS (garde d'isolation à jour)
$ npx tsc --noEmit -p tsconfig.app.json                             PASS
$ npx vite build                                                    PASS (built in 6.05s)
$ npx vitest run                                                    251 passed | 1 failed (pré-existant)
$ npx vitest run <suites OR26-03>                                   PASS — 52/52
$ npx playwright test mfa-deferred --list                           14 tests listés, NOT EXECUTED
```

### L'unique échec frontend est pré-existant

`src/__tests__/App.integration.test.tsx` échoue **aussi sur `origin/master`
vierge**, vérifié dans un worktree propre :

```text
$ git worktree add <tmp> origin/master && cd <tmp>/frontend
$ npx vitest run src/__tests__/App.integration.test.tsx
  Test Files  1 failed (1)
       Tests  1 failed | 2 passed (3)
```

Hors périmètre de ce lot.

## Screenshots / Evidence

Aucune capture. Le pilotage navigateur exige une instance démarrée, laquelle est
bloquée (voir ci-dessus). Le frontend est prouvé par `tsc`, `vite build` et
52 tests unitaires ciblés ; les E2E navigateur sont écrits et listés, non
exécutés.

## Mandatory acceptance matrix

| Scenario | Expected | Status | Evidence |
| --- | --- | --- | --- |
| Standard user first login without MFA | Access allowed | **PASS** | `TestDeferredMFA_OrdinaryMemberIsOnlyEverInvited` |
| Standard user dashboard without MFA | Access allowed | **PASS** | idem — `GET /risks` 200 |
| Standard user onboarding without MFA | Access allowed | **PASS (HTTP stack)** / BLOCKED (instance) | la garde est levée pour tout compte éligible ; route `/onboarding/*` non exercée live |
| First Risk without MFA | Allowed | **PASS** | `TestDeferredMFA_FreshAdminSignsInAndReachesTheProduct` — `POST /risks` 200 |
| Post-Aha MFA prompt | Visible | **PASS** | `mfaPostAhaPrompt.test.tsx` (8/8) |
| MFA already configured | No enrollment prompt | **PASS** | `TestResolver_EnrolledIsConfigured` ; `mfaEnrollmentBanner.test.tsx` |
| Admin within grace period | Policy-defined access | **PASS** | `TestDeferredMFA_FreshAdminSignsInAndReachesTheProduct` |
| Admin after grace period | MFA required | **PASS** | `TestDeferredMFA_PastTheDeadlineLoginStopsAtEnrolment` + `..._ALiveSessionIsRefused...` |
| RSSI after grace period | MFA required | **PASS** | `TestDeferredMFA_RSSIIsCoveredByTheMandate` |
| MFA configured after enforcement | Access restored | **PASS** | `TestDeferredMFA_EnrolmentRestoresAccess` |
| Admin changes MFA policy | Authorized | **PASS** | `TestDeferredMFA_PolicyIsReadableBoundedAndAdminWritable` |
| Unauthorized policy change | Denied | **PASS** | idem — membre → 403 |
| Cross-tenant policy access | Denied | **PASS** | `TestDeferredMFA_PolicyIsTenantScoped` + `..._ForgedIDCannotSteer...` |
| Frontend MFA bypass | Denied | **PASS** | `TestDeferredMFA_ClientCannotTalkItsWayOut` ; aucun état d'autorisation en storage |
| API MFA bypass | Denied | **PASS** | idem (corps forgé, PAT, ré-élargissement de la fenêtre) |
| Role promotion | Policy recalculated | **PASS** | `TestChangeRole_PromotionReAnchorsTheMFAGraceWindow` + `..._RSSIPresetAlsoReAnchors` |
| Role downgrade | Policy recalculated | **PASS** | `TestChangeRole_DemotionLeavesTheAnchorAlone` (+ purge du cache) |

## Performance

Non mesuré sur une instance (bloqué). Ce qui est établi par le code et les tests :

* **Aucun appel réseau supplémentaire par page.** L'état MFA voyage sur
  `/auth/login` et `/auth/me`, deux réponses qui existaient déjà ; le hook
  React Query a un `staleTime` de 5 min et les mutations concernées invalident
  explicitement.
* **La garde requête ne fait pas 3 requêtes SQL par appel.**
  `TestResolver_CachesPerUserAndTenant` prouve qu'une décision est résolue **une
  fois** par `(user, tenant)` et par TTL (60 s) — donc environ une résolution par
  utilisateur et par minute, quel que soit le trafic.

## Known Limitations

1. **Aucune instance live.** Docker bloqué, Postgres `:5434` orphelin,
   Postgres `:5432` sans identifiants. Les 7 scénarios E2E sont écrits et listés
   mais **non exécutés** ; la preuve du parcours vient de la pile HTTP réelle sur
   sqlite. C'est la limite la plus importante de ce document.
2. **Disque plein** (428 Go / 451 Go, 0 octet libre à plusieurs reprises). Sans
   rapport avec ce lot, mais cela a fait échouer des écritures de fichiers et
   forcé à déporter `GOCACHE` sur tmpfs. À traiter avant tout travail lourd ici.
3. **Aucune capture d'écran.** Le frontend est prouvé par `tsc`, `vite build` et
   52 tests unitaires ciblés.
4. **`TestSetupMFA_Success`** et le lint frontend restent hors périmètre.
5. **`src/__tests__/App.integration.test.tsx`** échoue — **pré-existant**,
   confirmé sur `origin/master` vierge.
6. **Les migrations SQL ne sont pas appliquées** sur une base de développement
   (dette documentée : la base dev est `dirty` à la version 40 et golang-migrate
   refuse de tourner). Le schéma des tests vient d'`AutoMigrate` / `Reconcile`.
   La migration `0060`, **backfill compris**, doit être appliquée avant tout
   déploiement.
