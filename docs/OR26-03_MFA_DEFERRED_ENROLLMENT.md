# OR26-03 — MFA différable

> **Issue** : [#290 — \[OR26-03\] MFA différable — ne pas bloquer l'accès avant l'Aha moment](https://github.com/opendefender/OpenRisk/issues/290)
> **Branche** : `feat/or26-03-defer-mfa-enrollment`
> **Sévérité d'origine** : S2 — majeur (audit UX live du 2026-08-17)

## Context

L'audit UX live a relevé qu'à la **première connexion**, OpenRisk répondait
`mfa_enrollment_required: true` **sans session**. L'utilisateur devait scanner un
QR code avant d'avoir vu un seul écran du produit.

Pour une évaluation, une démo ou un nouvel utilisateur, c'est un mur devant la
valeur — pas une barrière devant un risque. La règle cible devient :

> Le MFA doit être **fortement recommandé** au premier accès, mais ne doit pas
> bloquer le parcours de découverte et d'onboarding des utilisateurs non
> privilégiés. Pour les rôles privilégiés (Admin / RSSI), il reste une exigence
> de sécurité, applicable après un **délai configurable**.

## Current Behavior (avant OR26-03)

```text
Login (mot de passe vérifié)
  ↓
member.Role ∈ {admin, root} ?                     ← contrôle de RÔLE, immédiat
  ↓ oui
mfa_enrollment_required: true + mfa_token
  ↓
AUCUNE session. Écran d'enrôlement MFA.
```

* La décision vivait dans `LoginUseCase.mfaRequiredFor(role)` — un simple test
  d'appartenance à `MFA_REQUIRED_ROLES` (défaut `admin,root`).
* Elle ne portait **que** sur le rôle d'organisation. Le **RSSI** — qui porte le
  rôle d'org `user` et tire ses droits du préset métier `rssi` — n'était donc
  **pas** couvert, alors que c'est exactement le compte visé par l'exigence.
* Il n'existait **aucun** enrôlement volontaire dans l'application : le seul
  écran d'enrôlement vivait à l'intérieur du flux de login. « Activez le MFA »
  était un conseil sans bouton.
* L'application n'était enforcée **qu'au login**. Une session ouverte restait
  valable indéfiniment, et le refresh la renouvelait.
* Aucune configuration par tenant. `MFA_REQUIRED_ROLES` est une variable de
  déploiement, pas un réglage client.

Réponses aux questions du modèle de sécurité, telles qu'établies par lecture du
code avant modification :

| Question | Réponse (avant) |
| --- | --- |
| Qui était obligé ? | Rôle d'org `admin` ou `root` (env `MFA_REQUIRED_ROLES`). |
| À quel moment ? | Immédiatement, à la première connexion. |
| Quel composant décidait ? | `LoginUseCase.Execute` — **le serveur**, pas le frontend. |
| Où vit l'état MFA ? | Table `mfa_secrets` (`is_verified`), tenant-scopée. |
| Le rôle influençait-il déjà ? | Oui, mais uniquement le rôle d'org. |
| Un délai existait-il ? | **Non.** |
| Le dashboard savait-il ? | **Non** — il n'était jamais atteint. |
| Onboarding accessible sans MFA ? | **Non** pour un compte privilégié. |
| Création de risque sans MFA ? | **Non** pour un compte privilégié. |
| Révocation après dépassement ? | Sans objet — il n'y avait pas de délai. |

## Problem

Deux défauts distincts, et le second est un défaut de sécurité :

1. **UX** — un mur d'activation avant l'Aha moment. Taux d'abandon attendu élevé.
2. **Couverture** — le contrôle ne visait que le rôle d'org, donc le RSSI, nommé
   dans l'exigence, y échappait.

## Security / UX Rationale

Différer n'est pas contourner. Le compromis retenu :

* **Utilisateur standard** : le MFA n'est **jamais** exigé. Il est recommandé, de
  façon visible et non bloquante. Imposer un authentificateur à tout le monde
  dans un produit vendu à des organisations qui n'en ont pas toutes, c'est
  fabriquer des comptes partagés.
* **Compte privilégié** : le MFA reste **obligatoire**, mais « obligatoire » ne
  veut plus dire « immédiatement ». Il veut dire : dans la fenêtre du tenant,
  après quoi le serveur refuse l'accès.
* **La friction retirée est une friction d'UX, pas une autorisation.** Aucun
  droit supplémentaire n'est accordé pendant la période de grâce : le RBAC, les
  gardes de permission et l'isolation tenant sont inchangés.

## New MFA Policy

Une fonction pure, `domain.DecideMFA`, combine les trois entrées qui décident :

```text
état MFA (enrôlé ?) + privilège (le rôle impose-t-il le MFA ?) + temps écoulé
        =
   l'enrôlement est-il obligatoire maintenant ?
```

`internal/domain/mfa_policy.go`. Les règles, dans l'ordre où on se les pose :

1. **Enrôlé** → terminé. Rien à demander, rien à imposer.
2. **Non privilégié** → `recommended`, jamais `required`. *C'est le changement
   OR26-03.*
3. **Privilégié, fenêtre à 0 jour** → `required` immédiatement.
4. **Privilégié, ancre inconnue** → `required`. Une garde non vérifiable
   **échoue fermée** : un horodatage manquant ne doit pas se lire « grâce
   infinie », puisque c'est précisément l'état qu'un attaquant chercherait à
   produire.
5. **Privilégié, dans la fenêtre** → accès autorisé, échéance affichée. Passe à
   `grace_expiring` dans les 48 dernières heures.
6. **Privilégié, hors fenêtre** → `required`.

Le vocabulaire d'état est fermé (`MFARequirementState`) — un booléen ne suffisait
pas : « non configuré » couvre à la fois un membre simplement encouragé et un
compte privilégié à trois jours du blocage, et l'UI doit pouvoir les distinguer
sans redériver la règle.

| État | Accès | Bandeau | Dismissible |
| --- | --- | --- | --- |
| `configured` | oui | aucun | — |
| `recommended` | oui | invitation douce | oui (session seulement) |
| `grace_active` | oui | compte à rebours | non |
| `grace_expiring` | oui | avertissement | non |
| `required` | **non** (403) | alerte | non |

## Role Policy

Le privilège couvre **les deux vocabulaires de rôle** du produit
(`domain.MFAPrivilegeSet`), parce qu'ils confèrent tous deux un pouvoir réel :

| Vocabulaire | Valeurs par défaut | Variable de déploiement |
| --- | --- | --- |
| Rôle d'organisation | `root`, `admin` | `MFA_REQUIRED_ROLES` |
| Préset métier | `rssi` | `MFA_REQUIRED_BUSINESS_ROLES` |

Une liste vide (`MFA_REQUIRED_ROLES=`) désactive entièrement l'exigence pour ce
déploiement. **Qui** est privilégié est une décision de déploiement ; **combien
de temps** il peut différer est une décision du tenant.

## Grace Period

* **Défaut : 7 jours.** Assez long pour couvrir un week-end prolongé, assez court
  pour qu'un compte privilégié ne vive pas discrètement sans second facteur.
* **Bornes : 0 à 90 jours**, imposées à trois niveaux — `MFAPolicy.Validate()`,
  le use case, et une contrainte `CHECK` en base. Une valeur non bornée
  transformerait « configurer la politique » en « désactiver la politique ».
  `MFAPolicy.EffectiveGraceDays()` reclampe à la lecture, pour qu'une édition SQL
  directe ne puisse pas exprimer « jamais ».
* **`0` est une valeur légitime** : « MFA exigé dès la première connexion »,
  c'est-à-dire le comportement d'avant OR26-03, resté disponible.

### Point d'ancrage du compte à rebours

`organization_members.mfa_grace_started_at` (migration `0060`).

```text
mfa_grace_started_at + grace_days = échéance
```

* **Création de l'adhésion** : posé à l'entrée dans l'organisation.
* **Backfill de migration** : `COALESCE(joined_at, created_at)` — **pas**
  `now()`. Un administrateur en production depuis six mois ne doit pas recevoir
  une semaine neuve ; une organisation créée aujourd'hui — le cas d'évaluation
  visé par l'issue — obtient sa fenêtre complète.
* **Promotion** : `ChangeRole` **réancre à maintenant** quand un membre passe de
  non privilégié à privilégié. Compter depuis `joined_at` verrouillerait un
  collègue promu à l'instant même où son nouveau rôle prend effet.
* **Rétrogradation** : l'ancre est laissée en place. L'exigence ne s'applique
  plus, et une promotion ultérieure réancre de toute façon.
* **Membre privilégié des deux côtés du changement** (ex. `admin` → `user`+`rssi`) :
  l'ancre **ne bouge pas**, pour qu'une modification latérale n'achète pas une
  semaine de plus.
* **Lecture** : `OrganizationMember.MFAGraceAnchor()` retombe sur
  `JoinedAt`/`CreatedAt` pour les lignes écrites avant la colonne. Un zéro
  signifie *réellement inconnu*, ce que `DecideMFA` traite en échec fermé.

## Admin Configuration

`Paramètres › Sécurité › Politique MFA` — « **Forcer le MFA après N jours** ».

| Endpoint | Garde | Rôle |
| --- | --- | --- |
| `GET /api/v1/security/mfa-policy` | membre authentifié | lecture |
| `PUT /api/v1/security/mfa-policy` | `RequireRole("admin","root")` | écriture |

La lecture est ouverte à tout membre : quiconque est soumis à une échéance a le
droit de savoir laquelle. La réponse embarque `min_days` / `max_days` /
`default_days`, pour que le formulaire valide contre **les bornes du serveur**
plutôt que contre une copie qui peut dériver.

Le corps de la requête utilise `grace_days *int` : `0` est une valeur porteuse de
sens, donc un `int` nu transformerait silencieusement un enregistrement partiel
en réglage le plus strict.

Enregistrer purge le cache du résolveur **pour ce tenant uniquement**, de sorte
qu'une fenêtre raccourcie s'applique à la requête suivante. Un administrateur qui
règle « 0 jour » et voit ses collègues continuer à travailler en conclut que le
réglage est décoratif.

**Audit** : `domain.MFAPolicy` implémente `Auditable` (`governance_auditable.go`),
donc le plugin GORM journalise acteur / tenant / avant → après sans qu'aucun
point d'appel ait à y penser. Le middleware `AuditMutations` couvre par ailleurs
tout `PUT` réussi sous `protected`.

## Authentication Middleware

Deux points d'application, parce qu'un seul ne suffirait pas.

### 1. Login — `internal/application/auth/login.go`

Interroge la politique au lieu de tester un rôle. Un membre ordinaire entre avec
un bandeau ; un compte privilégié entre jusqu'à expiration de sa fenêtre ; passé
l'échéance, le login s'arrête exactement comme avant — pas de session, un token
`MFA_ENROLLMENT`, rien d'autre.

Une lecture de politique **en erreur** retombe sur la fenêtre par défaut (7 jours),
jamais sur « aucune exigence » : un incident base de données ne doit pas être ce
qui laisse passer un compte privilégié.

### 2. Requête — `internal/middleware/mfa_policy_guard.go`

Monté sur le groupe `protected`, juste après la porte d'authentification.

N'appliquer qu'au login ne serait pas appliquer : un compte privilégié connecté
le jour 1 qui ne ferme jamais son onglet garde une session vivante au-delà de
l'échéance, et le refresh la renouvelle. La fenêtre bornerait alors *combien de
temps on peut attendre avant de se connecter*, pas *combien de temps on peut
rester sans second facteur*.

* `403 MFA_ENROLLMENT_REQUIRED` + le bloc `mfa` (échéance incluse) une fois
  l'enrôlement obligatoire.
* `503 MFA_STATUS_UNAVAILABLE` si l'état ne peut pas être résolu — **échec
  fermé**, mais avec un code honnête : « impossible de vérifier » et « vous devez
  vous enrôler » sont deux faits différents, et envoyer quelqu'un scanner un QR
  code qui ne l'aidera pas est pire qu'une erreur franche.
* S'applique **aussi aux jetons d'accès personnels (PAT)**. Un PAT frappé avant
  l'échéance et laissé tourner serait une exemption permanente pour exactement le
  compte que l'exigence vise. L'intégration qui casse est la conséquence
  recherchée ; le remède de son propriétaire est de s'enrôler.
* **Exemptions** (`mfaGuardExemptSuffixes`) : `/auth/mfa/setup`,
  `/auth/mfa/verify`, `/auth/mfa/challenge`, `/auth/me`, `/auth/logout`. Une
  exigence qu'on ne peut pas satisfaire est un verrouillage, pas un contrôle.
  Tout le reste — lectures comme écritures — est refusé.

### Résolution et performance

`application/auth.MFAStatusResolver` répond pour `/auth/me` **et** pour la garde,
ce qui est ce qui empêche le bandeau et l'application de diverger.

Le résultat est mis en cache par `(user, tenant)` pendant **60 s** : une garde sur
chaque requête authentifiée coûte alors environ une résolution par utilisateur et
par minute, au lieu de trois requêtes SQL par appel. Le tenant fait partie de la
clé — une entrée partagée laisserait la politique d'un tenant décider de l'accès
d'un autre. Les trois événements qui changent la décision purgent explicitement :
enrôlement vérifié, MFA désactivé, politique modifiée, changement de rôle.

L'indice `orgRoleHint` (le rôle porté par le jeton signé) **n'élargit jamais que**
l'ensemble privilégié : un jeton ne peut pas se sortir de l'exigence par la
parole, mais une ligne d'adhésion disparue ne peut pas non plus exempter
silencieusement un administrateur.

## MFA Enrollment Banner

`frontend/src/features/auth/MFAEnrollmentBanner.tsx`.

Le bandeau rend l'état que **le serveur** a résolu. Toutes ses branches partent de
`status.state`, jamais d'un test de rôle ou d'une comparaison de date faite
localement : deux implémentations de « est-ce exigé ? », c'est ainsi que le
bandeau et la garde finissent par se contredire, et celle que l'utilisateur croit
est la mauvaise.

Il est monté sur les six dashboards de persona (`DashboardShell` pour cinq
d'entre eux, `PostureDashboard` explicitement) : l'échéance s'applique au compte,
pas à la mise en page sur laquelle on atterrit.

`MFAEnrollmentDialog` est l'enrôlement volontaire in-app qui n'existait pas —
`setup` puis `verify`, sur la session en cours. Accessible depuis le bandeau et
depuis `Paramètres › Sécurité`.

## Banner States

| État | Rendu |
| --- | --- |
| **Loading** | Ligne discrète « Vérification de la sécurité du compte… ». On ne fait pas clignoter un avertissement de sécurité qui pourrait ne pas s'appliquer. |
| **Grace active** | Bandeau info + jours restants + CTA. Non dismissible. |
| **Deadline approaching** | Bandeau `--high`, copie renforcée. Non dismissible. |
| **Required** | Bandeau `--critical`, `role="alert"`, `aria-live="assertive"`. |
| **Recommended** | Bandeau info, dismissible **pour la session**. |
| **Configured** | Rien. |
| **Error / bloc absent** | Ligne honnête + « Réessayer ». **Jamais** un « vous êtes protégé » vert : affirmer qu'un contrôle de sécurité est actif quand on l'ignore est le seul mode de défaillance qu'un produit de sécurité ne peut pas se permettre. |

Le rejet du rappel doux est écrit en **`sessionStorage`**, pas en `localStorage` :
une recommandation qu'on peut faire taire pour toujours est une recommandation
sur laquelle personne n'agit — et un compte à rebours privilégié n'est pas
rejetable du tout. Rien de durable n'est écrit qu'on pourrait plus tard prendre
pour une décision de sécurité.

## Aha Moment Integration

L'Aha moment est déjà un fait serveur (`docs`, `domain/activation.go`) :

> « premier score cyber calculé sur les données **propres** de l'utilisateur
> **AVEC** au moins un écart de conformité identifié »

Il est enregistré une seule fois par tenant par `GetExecutiveDashboardUseCase` et
exposé en `activation.aha_reached_at`.

## Post-Aha Prompt

`frontend/src/features/auth/MFAPostAhaPrompt.tsx`.

« Sécurisez votre compte » tombe mal avant qu'on sache à quoi sert le compte. Une
fois l'Aha atteint, la même demande est raisonnable — c'est là qu'elle est posée.

**Le déclencheur est celui du serveur.** Le composant lit
`activation.aha_reached_at` ; il ne pose ni ne simule ce drapeau. Un prompt qui se
déclencherait parce que le client a décidé que le moment était venu se
déclencherait au mauvais moment pour tout utilisateur ayant pris un autre chemin.

Anti-répétition :

* **Cooldown de 24 h** (`localStorage`), assez long pour respecter un refus le
  reste de la journée, assez court pour rappeler demain. Cosmétique : il régit un
  prompt, jamais une obligation.
* **Silencieux si `state === 'required'`** : le bandeau est déjà une alerte et le
  serveur refuse déjà les requêtes. Une modale par-dessus, c'est du bruit empilé
  sur un blocage.
* **Silencieux si `configured`**, et avant l'Aha.

## Session Contract

Le bloc `mfa` accompagne `/auth/login` et `/auth/me` — la réponse qui existe
déjà, plutôt qu'un appel supplémentaire par page.

```jsonc
{
  "mfa": {
    "state": "grace_active",          // configured | recommended | grace_active | grace_expiring | required
    "configured": false,
    "required": false,                // le bit d'application
    "privileged": true,
    "grace_period_active": true,
    "deadline": "2026-09-01T09:00:00Z",
    "grace_days": 7
  }
}
```

**Aucun secret** : ni graine TOTP, ni charge utile de QR code, ni code de secours.

`/auth/me` **omet** le bloc plutôt que de le deviner quand il ne peut pas
résoudre. Un `MFADecision` à zéro se sérialiserait en « non configuré, non
requis », indiscernable d'une vraie réponse, et laisserait un incident base de
données annoncer à l'UI qu'un compte privilégié va bien. L'application ne dépend
pas de ce champ — c'est la garde qui bloque — donc reporter « inconnu » est sûr.

## API Contract

Contract-first : `docs/openapi.yaml` porte les schémas `MFAStatus`, `MFAPolicy`,
`UpdateMFAPolicyInput`, les chemins `/security/mfa-policy`, et `mfa` sur
`AuthResponse`. `frontend/src/types/openapi.generated.ts` en est régénéré ;
`mfaPolicyService.ts` alias les types générés (zéro `any`).

Rétro-compatibilité : les champs sont **additifs**. `mfa_enrollment_required`
garde sa forme et sa sémantique — il se déclenche simplement plus tard.

## Tenant Isolation

* `mfa_policies` : une ligne par tenant, index unique sur `tenant_id`. Chaque
  requête filtre sur le tenant (RÈGLE #2).
* L'écriture est scopée sur `tenant_id`, **pas** sur la clé primaire : un client
  qui devinerait l'id de politique d'un autre tenant ne peut pas orienter
  l'écriture. L'id fourni est ignoré — ici, l'identité, c'est le tenant.
* La clé de cache du résolveur inclut le tenant ; `InvalidateTenant` ne purge que
  celui-là.
* Prouvé par : `TestMFAPolicyRepo_IsTenantScoped`,
  `TestMFAPolicyRepo_ForgedIDCannotSteerAnotherTenantsRow`,
  `TestMFAPolicy_IsTenantScoped`, `TestDeferredMFA_PolicyIsTenantScoped`,
  `TestResolver_InvalidateTenantDropsEveryMemberOfThatTenantOnly`.

## Security Boundaries

Le MFA différable est une politique de **friction UX**, pas un contournement
d'autorisation. Pendant la période de grâce :

* aucune permission supplémentaire n'est accordée — RBAC, gardes de route et
  prédicat tenant sont inchangés ;
* aucun secret n'est exposé ;
* modifier la politique reste `admin`/`root` ;
* passé l'échéance, **toute** route protégée est refusée, y compris frapper un
  PAT et rouvrir la fenêtre.

### Invariants

| # | Invariant | Où il est tenu |
| --- | --- | --- |
| 1 | Un utilisateur standard en grâce accède au produit sans MFA | `DecideMFA` règle 2 ; `TestDeferredMFA_OrdinaryMemberIsOnlyEverInvited` |
| 2 | Différer n'accorde aucun droit supplémentaire | Aucun changement au RBAC ; `TestDeferredMFA_ClientCannotTalkItsWayOut` |
| 3 | Un compte privilégié ne peut pas rester indéfiniment sans MFA | Garde requête + échec fermé ; `TestDeferredMFA_ALiveSessionIsRefusedOnceTheWindowCloses` |
| 4 | Le frontend ne peut pas désactiver la politique serveur | Rien de la requête n'entre dans la décision ; `TestDeferredMFA_ClientCannotTalkItsWayOut` |
| 5 | La politique est toujours tenant-scopée | Index unique + filtre + clé de cache ; `TestDeferredMFA_PolicyIsTenantScoped` |
| 6 | L'échéance n'est pas falsifiable via le navigateur | Calculée serveur depuis `mfa_grace_started_at` ; jamais lue d'une entrée client |
| 7 | Les changements de politique sont audités | `Auditable` + `AuditMutations` |
| 8 | Aucun secret MFA n'est exposé | `TestDeferredMFA_SessionContractCarriesNoSecret` |

## Test Matrix

| Portée | Fichier | Cas |
| --- | --- | --- |
| Moteur de décision (pur) | `internal/domain/mfa_policy_test.go` | 16 |
| Login | `internal/application/auth/login_mfa_test.go` | 19 |
| Résolveur + cache | `internal/application/auth/mfa_status_test.go` | 13 |
| Use cases de politique | `internal/application/auth/mfa_policy_usecases_test.go` | 8 |
| Garde (HTTP) | `internal/middleware/mfa_policy_guard_test.go` | 7 |
| Repository (sqlite) | `internal/infrastructure/repository/gorm_mfa_policy_repository_test.go` | 6 |
| Transitions de rôle | `internal/application/membership/service_test.go` | 4 |
| **Parcours (pile HTTP réelle)** | `internal/handler/auth/mfa_deferred_e2e_test.go` | **15** |
| Bandeau + helpers | `frontend/src/features/auth/__tests__/mfaEnrollmentBanner.test.tsx` | 13 |
| Prompt post-Aha | `frontend/src/features/auth/__tests__/mfaPostAhaPrompt.test.tsx` | 8 |
| Formulaire de politique | `frontend/src/features/settings/__tests__/mfaPolicyPanel.test.tsx` | 8 |
| E2E navigateur + API | `tests/e2e/mfa-deferred.spec.ts` | 7 (×2 projets) |

## Live Proof

Voir `docs/OR26-03_MFA_DEFERRED_LIVE_PROOF.md`.

## Known Limitations

1. **La migration SQL `0060` n'a pas été exécutée contre une base peuplée.** La
   pile live a tourné sur un cluster Postgres jetable dont le schéma vient
   d'`AutoMigrate`, donc le **backfill**
   (`mfa_grace_started_at = COALESCE(joined_at, created_at)`) n'avait rien à
   rétro-remplir. **À appliquer et vérifier avant tout déploiement.**
2. **Deux cas restent prouvés par la suite HTTP uniquement** : RSSI après la
   période de grâce (affecter un préset métier exige une invitation) et
   « l'enrôlement restaure l'accès » (il faudrait calculer un TOTP valide à la
   main). Les deux passent dans `mfa_deferred_e2e_test.go`.
3. **Les 7 scénarios E2E ne peuvent pas tourner en parallèle** : chacun
   enregistre un compte, et le limiteur d'auth (15 req / 5 min par IP) les fait
   échouer en 429. Ils passent en série avec purge du compteur — un artefact du
   harnais, pas du produit.
4. **La réancre à la promotion est réinitialisable par un administrateur.**
   Rétrograder puis re-promouvoir un collègue lui redonne une fenêtre neuve. Cela
   exige des droits d'administrateur — que seul un compte lui-même soumis à
   l'exigence détient — et chaque changement est audité. Documenté plutôt que
   verrouillé : verrouiller signifierait qu'une promotion légitime verrouille son
   bénéficiaire dehors.
5. **Le blocage d'un PAT est volontairement brutal.** Une intégration dont le
   propriétaire dépasse son échéance cesse de fonctionner sans préavis
   proportionné. Une alerte anticipée par e-mail au propriétaire est l'itération
   suivante.
6. **La fenêtre est par tenant, pas par membre.** Un cas « ce collègue précis a
   30 jours » n'est pas exprimable.
7. **Le TTL de 60 s du résolveur est un plancher de fraîcheur** pour les
   changements qui ne passent pas par les chemins qui purgent (édition SQL
   directe, par exemple).
8. **`/auth/refresh` n'est pas gardé** : il peut encore émettre un jeton pour un
   compte bloqué. Le jeton est inutilisable — la garde requête refuse tout — mais
   refuser plus tôt serait plus net.
