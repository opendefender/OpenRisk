# Audit UX produit — live, base vierge · 2026-08-17

> **Méthode.** Audit **live**, pas par lecture de code : stack monté (Postgres **vierge** :5480, backend :8080,
> frontend vite :5173 proxy `/api`, base 100 % vide). Chaque constat est adossé à une **preuve observée**
> (statut HTTP, JSON, DOM, écran). Parcours réel d'un **nouveau compte** de l'inscription jusqu'à la
> création du premier risque, en incarnant Persona C (Mme Fotso, conformité COBAC, banque/Cameroun/XAF).
> Référentiel : `PROMPT_AUDIT_UX_PRODUIT.md`. **Pass 1** = acquisition/activation + véracité ;
> **Pass 2** = onboarding authentifié, dashboards, création de risque & moment Aha.

---

## 1. Verdict

Le socle est **plus sain** que ne le laissait craindre l'audit interne de juillet : l'inscription, l'auth
7 couches (MFA TOTP incluse), l'onboarding, l'import COBAC et la création de risque **fonctionnent
réellement**. Mais Sandrine Mbarga (RSSI sceptique) **ne signerait pas encore**, pour deux raisons de
**confiance** et une d'**activation** : (1) « le score de sécurité » affiche **cinq valeurs différentes
selon l'écran** (0, 25, 36, 100) sur le même compte vide ; (2) le **moment Aha promis** — « créez votre
premier risque et voyez immédiatement son exposition financière » — **n'est pas tenu** (le score
qualitatif s'affiche, l'exposition FCFA non) ; (3) la friction d'entrée (MFA obligatoire avant le
produit, tour guidé qui bloque les clics) ralentit l'activation. Ce sont des défauts corrigibles, pas
des trous béants — le produit est **réel**.

---

## 2. ⚠️ Correction d'un faux positif (intégrité de l'audit)

Une première lecture a conclu `POST /auth/register` → **404 → inscription = façade (S1)**. **FAUX** :
artefact de **process orphelins** (backend/vite d'un run vieux de 3 jours tenant les ports, proxy
routant mal). Après stack propre : `register` → **201**, crée réellement user + organisation (vérifié en
base). *Sans un stack propre et reproductible, un audit live produit lui-même de faux ❌.*

---

## 3. Métriques mesurées

| Métrique | Mesuré (live) | Cible | Verdict |
|---|---|---|---|
| Inscription crée un compte | Oui — 201, user + org en base | Oui | ✅ |
| Auth 7 couches (register→login→MFA setup→TOTP→session) | Oui, bout-en-bout | Oui | ✅ |
| Onboarding 5 étapes complété → landing adaptée | Oui → `/compliance` (objectif COBAC) | Oui | ✅ |
| Import référentiel (COBAC, 45 contrôles cités) | Oui, 1 clic, feedback « Importé » | Oui | ✅ |
| Création 1er risque | Oui — score qualitatif 4.8/Élevé immédiat | Oui | ✅ |
| **Aha : exposition financière FCFA immédiate** | **Non** — aucun FCFA affiché après création | Immédiate | ❌ |
| **Cohérence du « score de sécurité »** | **0 / 25 / 36 / 100** selon l'écran | 1 valeur | ❌ |
| Session après inscription | Non (« check your email », re-login) | Auto | ⚠️ |
| MFA à la 1re connexion | Obligatoire (avant tout accès) | Différable | ⚠️ |
| **Clics « rapport COBAC » → PDF** | **2** (Générer → Télécharger) | ≤ 4 | ✅ |
| Violations a11y sérieuses / page (axe) | 1–2 (surtout contraste, clair) | 0 | ⚠️ |
| Responsive 375 px / thème sombre | Sans scroll H · dark OK | Utilisable | ✅ |

---

## 4. Les moments où l'utilisateur décrocherait

1. **Première connexion** — on lui impose de **scanner un QR MFA** avant d'avoir rien vu. Pour une
   évaluation, c'est un mur (**OR26-03**).
2. **Le score qui change d'écran en écran** — accueil « 0/100 », barre latérale « 25/100 », exécutif
   « F 36/100 » puis « A » selon l'état. *« Lequel est le vrai ? Ce tableau de bord n'est pas fiable. »*
   (**OR26-01**).
3. **Après avoir créé son premier risque** — on lui avait promis « exposition financière calculée
   immédiatement » ; elle voit un score, **pas de FCFA**. La promesse-clé du produit n'est pas tenue à
   l'instant où elle comptait (**OR26-10**).

---

## 5. Registre des défauts

| ID | Écran/API | Attendu | Réel (preuve) | Critère | Sévérité |
|---|---|---|---|---|---|
| **OR26-01** | Score sécurité (4 surfaces) | 1 valeur cohérente, « non mesuré » si vide | `/stats`=**100**, accueil=**0/100**, sidebar=**25/100**, exécutif=**F 36** (et **A/100** avant tout import) — même tenant vide | #8 Chiffres | **S1** |
| **OR26-10** | Création 1er risque → Aha | Exposition FCFA + traitement suggéré **immédiats** (promis par la checklist) | Score qualitatif OK ; **aucun FCFA** sur la page ; CRQ exige saisie SLE/ARO ; traitement = clic « Générer avec l'IA » | Aha / #1 | **S2** |
| **OR26-11** | Visite guidée | Coach marks **non bloquants** | Tour = `role=dialog` qui **intercepte les clics** de l'UI dessous et réapparaît à la navigation | #16 Raccourcis/UX | **S2** |
| **OR26-03** | 1re connexion | MFA différable (après l'Aha) | `mfa_enrollment_required:true` → MFA **imposée** avant tout accès | #3 Charge / activation | **S2** |
| **OR26-04** | Inscription → accès | Auto-login | 201 **sans session** + « check your email » (pas de SMTP) → re-login manuel | #1 Orientation | **S2** |
| **OR26-12** | Modal « Créer un risque » | `role=dialog`/`aria-modal` | Ouvert comme conteneur nu — non annoncé aux lecteurs d'écran | #18/a11y | **S3** |
| **OR26-06/07** | Onboarding Profil/Org | Pré-remplir depuis l'inscription | Nom complet (val `""`) et nom d'org **re-demandés** | #11 Friction | **S3** |
| **OR26-13** | Sidebar (badges nav) | Compteurs réels | « Registre 12 », « Mitigations 3 » sur un tenant à **1 risque / 0 mitigation** | #8 Chiffres | **S4** |
| **OR26-08/09** | Onboarding Objectif/Profil | Choix multiple ; pas d'avatar URL | Objectif **single-select** ; champ **« Avatar (URL) »** en onboarding | #3 Charge | **S4** |

*S1 bloquant · S2 majeur · S3 modéré · S4 mineur.*

---

## 6. Ce qui est bien (à protéger des régressions)

- **Auth 7 couches réellement fonctionnelle** : register → login → **MFA setup → TOTP verify → session**
  (cookies **HttpOnly** `or_access`/`or_refresh`/`or_csrf`, JWT RS256, perms correctes). Preuve d'avance
  pour la phase P3.
- **Contenu réglementaire africain = vrai différenciateur** : l'étape référentiel suggère
  **COBAC/BCEAO/ANTIC** (adaptés secteur+pays), cités article par article, **import 1 clic** avec feedback.
- **Landing adaptée à l'objectif** : objectif COBAC → arrivée sur `/compliance` avec le référentiel déjà
  importé (45 contrôles, 0 % honnête).
- **Dashboard newcomer soigné** : checklist « Prise en main » **pilotée par les vraies données** (2/7),
  **empty states honnêtes et actionnables** partout (matrice/tendance/activité/incidents), cadrage Aha
  explicite. L'accueil affiche « 0/100 » (et non le « 100 » trompeur) — à généraliser aux autres surfaces.
- **Fuseau horaire éditable** (corrige la plainte « timezone non modifiable » de l'audit interne).

---

## 6b. Pass 3 — restitution, accessibilité, responsive, thème

**Rapport COBAC (point fort, [20]).** « Générer un rapport » (**clic 1**) → job `POST /reports/jobs` **201**
→ « Rapport prêt » → « Télécharger » (**clic 2**) → `GET /reports/jobs/:id/download` **200**, PDF 10 Ko.
**2 clics jusqu'au PDF** (cible ≤ 4). Immuabilité annoncée (« figé à la date de génération, même fichier »).

**Accessibilité (axe-core 4.12.1, WCAG 2.1 AA).** Base **saine** : 1–2 violations *sérieuses* par page,
récurrentes et corrigibles.

| Page | Violations sérieuses | Détail |
|---|---|---|
| Dashboard | 2 | `aria-progressbar-name` ×1, `color-contrast` ×1 |
| Registre des risques | 1 | `color-contrast` |
| Dashboard exécutif | 1 | `color-contrast` |

- **OR26-14 (S3)** — `color-contrast` sérieux sur plusieurs pages **en thème clair** (concorde avec
  `--risk-moderate` 3,62:1 de la doc résolution). **Thème sombre : 0 violation de contraste** → défaut
  spécifique au clair.
- **OR26-15 (S3)** — `aria-progressbar-name` : barre de progression sans nom accessible (dashboard).

**Responsive & thème (OK).** Mobile **375 px** : pas de scroll horizontal, menu hamburger présent.
**Thème sombre** : s'applique (`body` fond `rgb(8,9,13)`), passe le contraste AA sur la page testée.

---

## 7. Couverture & suite (honnêteté)

**Couvert (live) :** inscription, auth+MFA complète, onboarding 5 étapes, import COBAC, landing,
dashboard d'accueil, dashboard exécutif, registre des risques, **création du 1er risque + drawer**,
sondes de véracité multi-surfaces, **rapport COBAC PDF [20]**, **axe-core [34]** (dashboard/risques/exécutif),
**mobile 375 px [09]**, **thème sombre [33]** (échantillon).

**Non encore parcouru :** IA Advisor + test d'isolation tenant [21], Administration
(Membres/RBAC/Intégrations/Audit log) [22-27], bug AWS-présélection sur Intégrations [25],
Mitigations [15-16], Heatmap interactive [17], preuves/crosswalks en profondeur [19], états limites &
destructeur [28-30], axe-core **exhaustif** (toutes pages + modals). Le stack reste monté pour enchaîner.

---

## 8. Plan de correction (par cause racine)

- **Lot véracité (S1, OR26-01)** — **une seule source** de « score de sécurité », qui renvoie
  **« non mesuré »** (pas 0/25/100) quand le tenant est vide, et un axe **absent ≠ 100/parfait**.
  Aligner les 4 surfaces (accueil déjà correct → propager). *(= `PROMPT_CLAUDE_RÉSOLUTION §1.2`.)*
- **Lot Aha (S2, OR26-10)** — tenir la promesse : à la création d'un risque, **dériver et afficher
  immédiatement** une exposition financière (même approximative, à partir de la criticité/impact) et
  **proposer un traitement** sans clic supplémentaire. C'est le cœur de l'activation.
- **Lot friction d'activation (S2, OR26-03/04/11)** — MFA **différable** après l'Aha ; **auto-login**
  après inscription ; tour guidé **non bloquant** (overlay qui n'intercepte pas les clics, non
  ré-affiché après passage).
- **Lot pré-remplissage & a11y (S3)** — propager les données d'inscription dans l'onboarding ;
  `role=dialog` sur les modals.

Ces lots alimentent **P2 (Lot 0 sécurité & chiffres)** et **P4 (onboarding & Aha)** du plan de lancement
(`docs/LAUNCH_PLAN_2026.md`).
