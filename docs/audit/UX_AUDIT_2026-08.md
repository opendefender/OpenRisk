# Audit UX produit — Pass 1 (live, base vierge) · 2026-08-17

> **Méthode.** Audit **live**, pas par lecture de code : stack monté (Postgres **vierge** :5480, backend :8080,
> frontend vite :5173 proxy `/api`, base 100 % vide, admin seedé `admin@opendefender.io`). Chaque constat
> est adossé à une **preuve observée** (statut HTTP, JSON, écran). Ceci est le **Pass 1** : il couvre le
> **BLOC 1 — Acquisition & Activation** et des **sondes de véracité** sur le tenant vide. Les BLOCs 2–5
> (cœur produit, conformité/restitution, admin, états limites) restent à parcourir — voir §7 Couverture.
>
> Référentiel : `PROMPT_AUDIT_UX_PRODUIT.md` (34 parcours, 4 personas, 20 critères).

---

## 1. Verdict (à ce stade du Pass 1)

Sandrine Mbarga (RSSI, sceptique) **ne signerait pas encore** — mais le blocage n'est pas là où on le
croyait. La création de compte **fonctionne** (contrairement à ce que soupçonnait l'audit interne de
juillet). Deux choses la feraient décrocher : (a) elle doit **enrôler une MFA** avant même de voir le
produit, et (b) une fois entrée sur un compte neuf, l'app lui affiche un **score de sécurité 100/100,
grade A « Excellente »** alors qu'elle n'a **aucune** donnée — un chiffre qui ment, le pire défaut pour
un outil de sécurité vendu à des banques. Le socle technique est sain (auth 7 couches réellement
fonctionnelle) ; ce sont des défauts de **véracité** et de **friction d'activation**, pas des façades.

---

## 2. ⚠️ Correction d'un faux positif (intégrité de l'audit)

Une première lecture a conclu que `POST /auth/register` renvoyait **404 → inscription = façade (S1)**.
**C'était FAUX** : un artefact de **process orphelins** (backend/vite d'un démarrage vieux de 3 jours
tenant encore les ports, proxy routant mal). Après nettoyage et redémarrage propre, `register` renvoie
**201** et **crée réellement l'utilisateur + son organisation** (vérifié en base). **Constat retiré.**
*Leçon : sans un stack propre et reproductible, un audit live produit lui-même des faux ✅/❌.*

---

## 3. Métriques mesurées (Pass 1)

| Métrique | Mesuré (live) | Cible | Écart |
|---|---|---|---|
| Inscription possible ? | **Oui** — 201, crée user+org | Oui | ✅ |
| Session après inscription | **Non** — « check your email », re-login requis | Auto-login | ⚠️ friction |
| MFA à la 1re connexion | **Obligatoire** (`mfa_enrollment_required`) | Optionnelle/différable | ⚠️ friction |
| MFA fonctionne (setup→TOTP→session) | **Oui** (cookies HttpOnly émis) | Oui | ✅ |
| Score sécurité sur tenant vide | **100/100** | « non mesuré »/0 | ❌ ment |
| Cyber score exécutif sur tenant vide | **A / « Excellente »** | « non mesuré » | ❌ ment |
| Temps jusqu'au Aha | **non atteint ce pass** (bloqué au parcours d'activation) | < 8 min | — |

---

## 4. Registre des défauts (Pass 1)

| ID | Écran/API | Persona | Attendu | Réel (preuve) | Critère | Sévérité |
|---|---|---|---|---|---|---|
| **OR26-01** | `GET /stats` (dashboard) | A | Sur 0 risque : « non mesuré » ou 0 | `global_risk_score: 100` sur registre vide | #8 Chiffres | **S1** |
| **OR26-02** | `GET /analytics/executive` | A | Pas de note sur 0 donnée | `cyber_score {score:100, grade:'A', label:'Excellente'}` (1 seul axe) | #8 Chiffres | **S1** |
| **OR26-03** | Login 1re fois | B, C | Voir le produit vite ; MFA différable | `mfa_enrollment_required:true` → MFA **imposée** avant tout accès (nécessite une app TOTP) | #3 Charge, activation | **S2** |
| **OR26-04** | Inscription → accès | A, B | Auto-login après signup | 201 **sans session** + « check your email » (aucun serveur mail) → re-login manuel | #1 Orientation | **S2** |
| **OR26-05** | Formulaire d'inscription (`/password/check`) | B | Retour de robustesse serveur | Indicateur « Vérification… » alimenté par un check serveur ; robustesse zxcvbn OK côté client (à re-vérifier hors artefact) | #5 Feedback | **S3** |

*Sévérité : S1 bloquant · S2 majeur · S3 modéré · S4 mineur.*

---

## 5. Les moments où l'utilisateur décrocherait (observés)

1. **Juste après « Créer un compte »** — le compte est créé mais rien ne connecte l'utilisateur ; le
   message renvoie vers un e-mail qui n'arrivera pas (pas de SMTP en démo). *« J'ai créé mon compte…
   et maintenant ? »* → **OR26-04**.
2. **Première connexion** — on lui demande de **scanner un QR MFA** avant d'avoir rien vu du produit.
   Pour un essai/évaluation, c'est un mur. *« Je voulais juste regarder. »* → **OR26-03**.
3. **Premier dashboard** (compte neuf) — l'app annonce **A / Excellente, 100/100** sans qu'elle ait
   saisi le moindre risque. *« Un score parfait sur un compte vide ? Ce produit invente. »* → **OR26-01/02**.

---

## 6. Ce qui est bien (à protéger des régressions)

- **Inscription réelle** : crée user + organisation nommée d'après l'entreprise, en base, en une requête.
- **Auth 7 couches fonctionnelle** : register → login → MFA setup → **TOTP verify → session** (cookies
  **HttpOnly** `or_access`/`or_refresh`/`or_csrf`, JWT RS256 avec `permissions`/`org_roles` corrects).
  C'est un socle solide (et une preuve d'avance pour la phase P3 « Auth prouvée live »).
- **Écran de login soigné** : bascule thème, FR/EN, citations, OAuth Google/GitHub/Microsoft présents.

---

## 7. Couverture & suite (honnêteté)

**Couvert (live, Pass 1) :** parcours [01] inscription, [06] amorce d'activation, chaîne d'auth complète,
sondes de véracité dashboard/exécutif sur tenant vide.

**Non encore parcouru (Pass 2+), à faire dans le même stack :** onboarding wizard [02-05] & Aha [06-07],
dashboard authentifié (visuel) & mobile [08-09], Risk Register liste/carte/création/détail [10-14],
Mitigations [15-16], Heatmap [17], Score Engine [18], Conformité + rapport COBAC PDF [19-21],
Administration [22-27], états limites/destructeur/thème sombre/axe-core [28-34]. Ces parcours exigent
de piloter l'app **authentifiée** (login+MFA en navigateur) — le stack reste monté pour enchaîner.

---

## 8. Plan de correction proposé (cause racine)

- **Lot véracité (S1, OR26-01/02)** — un score n'existe pas tant qu'il n'y a rien à scorer. Renvoyer un
  état **« non mesuré »** (et non 100) quand le tenant a 0 risque ; le dashboard doit inviter à créer le
  1er risque, pas afficher un A trompeur. *(= `PROMPT_CLAUDE_RÉSOLUTION §1.2`.)*
- **Lot activation (S2, OR26-03/04)** — rendre la **MFA différable** (proposée après l'Aha, pas avant),
  et **auto-connecter** après inscription (ou dire clairement quoi faire sans dépendre d'un e-mail).
  Objectif : inconnu → 1er risque + exposition financière **< 8 min**.

Ces deux lots alimentent directement **P2 (Lot 0 sécurité & chiffres)** et **P4 (onboarding)** du plan de
lancement (`docs/LAUNCH_PLAN_2026.md`).
