# OpenRisk — Inventaire documentaire (`docs/`)

> **Proposition de classement uniquement. AUCUNE suppression n'est effectuée dans
> cette session** (gel du code produit + STOP « suppression de fichier »).
> Mesuré le 2026-07-24 : **192 fichiers `.md`** dans `docs/`, contre la cible « deux
> documents de pilotage vivants » de `PROJECT_PLAN.md`. C'est un **graveyard**
> d'artefacts de session (résumés « COMPLETE », phases, sprints) qui noie les rares
> documents de référence.

## Règle de classement (déterministe, reproductible)

Le tri ci-dessous est produit par motif de nom de fichier (script joint dans le corps
de la session) :

- **Vivant** = référence unique et courante d'un sujet (API, RBAC, run local, runbook prod, formules, chartes UX).
- **À supprimer** = doublon exact, index redondant, fichier vide/one-off, ou artefact « TODO/START_HERE/INDEX » supplanté.
- **Archive** = valeur historique réelle mais **datée** (rapports de phase/sprint/session, « _COMPLETION_ », « _IMPLEMENTATION_ », audits ponctuels) → à déplacer dans `docs/archive/` (pas à détruire).

Répartition : **Vivant 19 · À supprimer 18 · Archive ~110 · Divers à consolider ~45.**

---

## 1. Vivant — à garder à la racine `docs/` (19)

`ENDPOINTS.md` · `RBAC_BUSINESS_ROLES.md` · `VERSIONING.md` · `MASTER_PROMPT_V4.md` ·
`SCORE_ENGINE_FORMULA.md` · `STRATEGIC_ROADMAP_2026.md` · `MULTI_TENANCY_GUIDE.md` ·
`LOCAL_DEVELOPMENT.md` · `TESTING_GUIDE.md` · `CI_CD.md` · `KUBERNETES_DEPLOYMENT.md` ·
`MONITORING_SETUP_GUIDE.md` · `PRODUCTION_RUNBOOK.md` · `DISASTER_RECOVERY_PLAN.md` ·
`AUTHORS.md` · **`UX_CHARTER.md`** · **`UX_AUDIT_2026-07.md`** ·
**`IA_NAVIGATION_PROPOSAL.md`** · **`UI_ELEVATION_PROPOSAL.md`** (+ ce fichier).

> Les vraies sources de vérité restent **hors `docs/`** : `ROADMAP.md`, `CLAUDE.md`,
> `PROJECT_PLAN.md`, `CHANGELOG.md` à la racine du dépôt.

## 2. À supprimer — doublons / index / cruft (18, actionnable)

`Claude.md` (doublon minuscule de `CLAUDE.md`) · `TODO.md` + `TODO-copy.md` ·
`CACHING_INTEGRATION_GUIDE.md` (doublon de `CACHE_INTEGRATION_GUIDE.md`) ·
`ANALYSIS_INDEX.md` · `DOCUMENTATION_INDEX.md` · `DOCUMENTATION_INDEX_PHASE6.md` ·
`DESIGN_SYSTEM_MASTER_INDEX.md` · `PHASE_5_INDEX.md` · `START_HERE.md` ·
`README_PHASE6_START_HERE.md` · `IMMEDIATE_ACTION_FIX_BUILD.md` · `FIX_SUMMARY.md` ·
`COMPLETION_SUMMARY.md` · `FINAL_COMPLETION_SUMMARY.md` · `TASKS_COMPLETION_SUMMARY.md` ·
`DELIVERABLES_SUMMARY.md` · `DELIVERY_SUMMARY.md`.

**Doublons FR/EN et variantes à fusionner** (garder 1) : `USE_CASES.md`/`USE_CASES_EN.md` ·
`QUICK_ONBOARDING.md`/`QUICK_ONBOARDING_EN.md` · `KEYBOARD_SHORTCUTS.md`/`KEYBOARD_SHORTCUTS_QUICK_REF.md` ·
`STAGING_DEPLOYMENT.md`/`STAGING_DEPLOYMENT_GUIDE.md` · `PERFORMANCE_*` (8 fichiers → 1 runbook).

## 3. Archive — valeur historique, à déplacer dans `docs/archive/` (~110)

Buckets par motif (comptés) :
- **Phases** : `PHASE_*`, `PHASE6*` — rapports de complétion de phase.
- **Sprints** : `SPRINT*`, `RBAC_SPRINT*` — complétions de sprint.
- **Sessions / statuts** : `SESSION_*`, `PROJECT_STATUS_*`, `*_HANDOFF`.
- **« Terminé »** : `*_COMPLETION_*`, `*_COMPLETE`, `*_SUCCESS`.
- **Implémentations datées** : `*_IMPLEMENTATION_*`, `DASHBOARD_*` (10+ variantes), `RBAC_*_SUMMARY`.
- **Audits/rapports ponctuels** : `*_AUDIT*`, `*_REPORT`, `RC_HARDENING_REPORT`, `PENETRATION_TESTING_REPORT`, `SECURITY_*`, `RELEASE_READINESS_*`, `DEPLOYMENT_READINESS_*`.

Ces fichiers documentent *ce qui a été fait* à un instant T ; ils gardent une valeur
d'historique (comme les entrées de `ROADMAP.md`) mais **ne doivent pas concurrencer**
les documents vivants. Un simple `git mv` vers `docs/archive/` suffit.

## 4. Divers à consolider (~45)
Guides topiques légitimes mais **redondants** (plusieurs sur le même sujet : API ×5,
RBAC ×4, dashboard ×3, performance ×8). Proposition : **un guide vivant par sujet**,
le reste en archive. À arbitrer sujet par sujet avec le fondateur.

---

## Effet attendu
De **192** fichiers à **~20 vivants** + `docs/archive/` (historique) + `docs/mockups/`.
Un nouveau contributeur trouve la référence en 1 coup d'œil ; l'historique reste
traçable sans polluer. **Rien n'est supprimé tant que le fondateur n'a pas validé la
liste §2.**
