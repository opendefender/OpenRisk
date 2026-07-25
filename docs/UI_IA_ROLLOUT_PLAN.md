# OpenRisk — Plan de revue page par page (UI_ELEVATION × IA_NAVIGATION)

> Document de pilotage vivant. Cartographie **chaque écran** sur les deux
> propositions ratifiées : [`IA_NAVIGATION_PROPOSAL.md`](IA_NAVIGATION_PROPOSAL.md)
> (les 5 intentions) et [`UI_ELEVATION_PROPOSAL.md`](UI_ELEVATION_PROPOSAL.md)
> (tokens, densité, tables denses, états, mouvement, master-detail 4K, micro-victoires).
> But : élever **tous** les écrans au niveau « console AWS / app Google », un lot
> atomique à la fois, sans régresser la surface saine (42 routes, 0 cassé — mesurée
> par `tests/e2e/`).

## 0. Le kit réutilisable déjà livré (`feat/ia-nav-ui-elevation`)
À **adopter** écran par écran — ne rien réinventer :

| Brique | Fichier | Rôle |
|--------|---------|------|
| Tokens motion/échelle/espace/rayon/élévation | `frontend/src/index.css` (`--dur/--ease/--text-*/--space-*/--radius-*/--elev-*`) | vocabulaire de style unique |
| Densité (Confort/Compact/Spacieux) | `uiStore.density` + `[data-density]` + contrôle entête | rythme des tables/listes |
| `DataTable<T>` | `shared/DataTable.tsx` | table dense : tri, colonne figée, sélection, density-aware |
| `EmptyState` | `shared/EmptyState.tsx` | état vide canonique (icône + valeur + action, UX-04) |
| `celebrate()` / confetti | `shared/celebrate.ts` | micro-victoire sobre (UX-32) |
| Master-detail 4K | `index.css` `.or-md-*` | drawer côte à côte ≥ 1920px |
| `.or-den-rows` | `index.css` | rendre density-aware une table existante sans la réécrire |

## 1. Barème de revue (checklist par écran)
Chaque écran est « élevé » quand il coche :
- **IA** — rangé dans la bonne intention ; libellé clair ; landing par rôle cohérent (UX-24).
- **Hiérarchie** — 1 action dominante identifiable en < 3 s (UX-10) ; 1 seul bouton plein.
- **Tables** — `DataTable` (tri, colonne figée, sélection, densité) OU au minimum `.or-den-rows` + en-tête collant.
- **États** — vide (`EmptyState` + action), chargement (skeleton, jamais spinner plein écran), erreur (quoi/pourquoi/action, UX-03) : les 3 présents.
- **Édition** — inline/fantôme pour le simple (UX-06) ; autosave + « Enregistré ✓ » pour les préférences (UX-23) ; modale réservée à l'irréversible (UX-28).
- **Mouvement** — transitions via tokens (≤ 260 ms), `prefers-reduced-motion` respecté.
- **Densité** — la table/liste réagit au contrôle d'entête.
- **4K** — drawer de détail en master-detail (`.or-md-*`) là où il y en a un.
- **A11y** — axe 0 serious/critical ; labels + contraste (garde `a11y.spec.ts`).
- **Responsive** — 360px→4K, table → cartes en mobile (UX-27, garde Mobile Chrome).
- **Micro-victoire** — jalon célébré si pertinent (1ᵉʳ contrôle évalué, 1ᵉʳ rapport…).
- **Testabilité** — `data-testid` stables sur lignes/champs/onglets (convention
  `<entité>-row-<id>`, `<modale>-submit`, `<drawer>-tab-<key>`).

**DoD par écran** : checklist ci-dessus + `tsc -b`/`vite build` verts + smoke E2E de la
route toujours vert + **capture live avant/après** attachée à la PR.

## 1bis. Consignes fondateur — backlog transverse de primitives (2026-07-25)

Consignes UX supplémentaires du fondateur, rattachées à la charte. Chacune = une
**primitive partagée** à livrer puis à adopter par écran (comme `DataTable`/`EmptyState`).

| # | Consigne (UX-xx) | Primitive à livrer | Prio |
|---|---|---|---|
| A | Suppression **mineure** = exécution immédiate + toast « Annulé/Restaurer » ≥ 7 s, sans modale (UX-12/28) | `shared/undoableDelete.ts` (hide optimiste + commit différé + Undo) | P1 |
| B | Suppression **importante** = **radiographie d'impact** (objets liés) + alternative (transfert) avant d'agir (UX-11) | `shared/ImpactDialog.tsx` (« Annuler / Transférer les risques / Supprimer ») | P1 |
| C | Aide contextuelle au 1ᵉʳ survol, non répétée, pas de product tour (UX-14) | `shared/Hint.tsx` (mémorisé localStorage) + compléter le `<Term>` glossaire | P2 |
| D | Attente exploitée : progression + info utile si > 1,5 s (UX-09) | `shared/ProgressState.tsx` (scan / rapport / calcul CRQ) | P2 |
| E | ⏸️ **Reporté (lourd, backend)** — Notifications **catégorisées** (Sécurité/Conformité/Tâches/Collaboration/Produit/Facturation), préférences par catégorie **et** canal (in-app + email) (UX-20/21) | `domain.NotificationCategory` + migration + centre de notif catégorisé + prefs | P1 |
| F | Relance après inactivité + annonce de nouveauté, calées sur fuseau/heures d'activité (UX-29) | backend `last_active_at` + job d'envoi ciblé | P2 |
| G | ✅ Raccourcis clavier découvrables (aide `?`) pour les 5 actions clés (UX-26) | `shared/useHotkeys.ts` + `shared/ShortcutsOverlay.tsx` (voir §1ter) | P2 |
| H | 🟡 Time travel : historique daté & attribué sur chaque entité majeure (UX-25) | `shared/HistoryTimeline.tsx` + **Risk fait** (drawer) ; Asset déjà là ; Control/Mitigation = reste (voir §1ter) | P1 |
| I | Aperçu **flouté** des features payantes + 3 moments de conversion (après Aha / à la limite / après victoire), jamais « limite atteinte » sec (UX-18/19) | `shared/PremiumPeek.tsx` + déclencheurs Fogg | P3 |
| J | Trial court / basé usage (Parkinson), compteur d'usage visible (UX-30) | bandeau d'essai + compteur (dépend billing) | P3 |
| K | Onboarding testé ≥ 1×/semaine (UX-33) | ✅ cron nocturne `e2e.yml` — passer en garde hebdo bloquante | — |

**Déjà appliqué** (ne pas refaire) : onboarding guidé jusqu'au Aha + personnalisation
post-Aha (`features/onboarding/OnboardingChecklist`, UX-01/07/13/17/32/Fogg) ; nav par
intention ≤ 7 (IA) ; autosave Paramètres (UX-23) ; densité + tokens + confetti ;
invitation de membre (OR-BUG-003) ; ⌘K (couverture à étendre).

## 1ter. Journal d'avancement — branche `feat/ux-rollout-p1-p3`

**Primitives partagées livrées (commits atomiques, sans co-auteur) :**
- ✅ A `shared/undoableDelete.ts` (toast + Undo, commit différé) — UX-12/28
- ✅ A' `shared/useUndoableRemove.ts` (hook : `undoableDelete` + set d'ids masqués
  optimiste + restore sur Undo — le chemin d'adoption mécanique pour les listes
  react-query) — UX-12/28
- ✅ B `shared/ImpactDialog.tsx` (radiographie d'impact + alternatives) — UX-11
- ✅ C `shared/Hint.tsx` (tooltip 1ᵉʳ survol, non répété) — UX-14
- ✅ D `shared/ProgressState.tsx` (attente informative, étapes + stat) — UX-09
- ✅ I `shared/PremiumPeek.tsx` (aperçu flouté premium + bénéfice) — UX-18/19

**Adoptions par écran faites :**
- ✅ Conformité (`ComplianceScreen`) : suppression de référentiel → `ImpactDialog`
  (N contrôles + preuves + alternative « exporter le rapport d'abord »).
- ✅ `undoableDelete` sur les suppressions **mineures** (via `useUndoableRemove`,
  masquage optimiste + toast Annuler ≥ 7 s, plus de `window.confirm`) : correspondance
  de contrôle (`ControlMappingsSection`), preuve (`FrameworkDetail`), plan de
  remédiation (`RemediationPage`), audit (`AuditsPage`), dépendance d'actif
  (`AssetUniverse` — l'arête disparaît du graphe **et** du panneau pendant le délai).
- ✅ `ImpactDialog` sur les suppressions **importantes** (radiographie d'impact +
  alternative non destructive, plus de `window.confirm`) :
  - **Risque** (`RiskRegisterPage`) : conséquences = plans de mitigation liés (compte)
    + historique/scores perdus ; alternative « Exporter le risque (CSV) avant de
    supprimer ».
  - **Actif** (`EditAssetModal`) : bouton Supprimer **gardé `assets:delete`** ajouté à
    la modale live (l'inventaire n'exposait aucune suppression) → conséquences =
    N risques qui s'appuient sur l'actif + arêtes de dépendance retirées ; alternative
    « Consulter l'historique avant de supprimer ».
  - **Membre** (`SettingsScreen` → onglet Membres) : conséquences = perte d'accès +
    actifs/risques possédés laissés sans responsable ; alternative **réelle**
    « Désactiver le compte au lieu de révoquer » (réversible, via `setStatus`). NB :
    `owner` est un texte libre (pas une FK user) → pas de recompte fiable des risques
    possédés, et **« transférer ses risques » nécessite un backend** (reporté) — la
    désactivation est l'échappatoire honnête disponible.

- ✅ `ProgressState` (attente informative UX-09) sur les vraies attentes LLM/agrégation :
  plan de traitement **IA** du drawer de risque (`DrawerAI` → étapes analyse/stratégie/
  priorisation + nom du risque) et **génération du Board Report** (`BoardReportPage`
  `EmptyList` → étapes agrégation/rédaction/finalisation). Remplace le spinner nu.
- ✅ `Hint` (aide 1ᵉʳ survol, non répété, UX-14) sur les **actions non évidentes** (le
  jargon a déjà `<Term>`) : contrôle de **densité** de l'en-tête (`AppHeader`) et bouton
  **« Remédier »** des audits (`AuditsPage` — explique l'auto-génération des plans).
- ✅ `PremiumPeek` (aperçu flouté premium UX-18/19) : **un** teaser honnête sur l'écran
  **Rapports** — « Rapports programmés » (envoi auto par e-mail, **non encore développé**)
  en aperçu flouté + CTA **« Bientôt disponible »** (jamais de mur « limite atteinte »).
  Moment de conversion « après une victoire » (l'utilisateur vient de générer des
  rapports). **Vérifié live** (capture headless authentifiée, page `/reports`).
- ✅ **G — Raccourcis clavier découvrables + overlay `?`** (UX-26) : nouvelle primitive
  `shared/useHotkeys.ts` (couche de raccourcis mono-touche globale, **ignore la frappe
  en champ** + les combos ⌘/Ctrl/Alt) + `shared/ShortcutsOverlay.tsx` (panneau d'aide
  thème-aware). 5 actions clés câblées dans le shell (`App` `DashboardLayout`) :
  `N` nouveau risque · `/` recherche & commandes (⌘K) · `G` tableau de bord · `T` thème ·
  `?` afficher/masquer l'aide. Affordance **découvrable** : bouton clavier dans l'en-tête
  (`AppHeader`, avec `Hint` 1ᵉʳ survol) qui ouvre l'overlay. **Vérifié live** (Playwright :
  `?` → l'overlay « Raccourcis clavier » s'ouvre avec les 5 actions + ⌘K/Esc).

**Reste à adopter (prochaine session) :**
- Étendre `ProgressState` au **scan** (Infrastructure) et au **calcul CRQ/smart-score**
  quand ces attentes deviennent > 1,5 s de façon observable.
- `PremiumPeek` aux **2 autres moments de conversion** (après le Aha / à la limite) —
  dépend d'un vrai **billing** pour ne pas mentir (CTA « bientôt » en attendant).
- ✅ **H (partiel) — Time travel généralisé** (UX-25) : nouvelle primitive
  `shared/HistoryTimeline.tsx` (timeline daté + attribué qui/quoi/quand, thème-aware,
  alimentée par un `HistoryEntry[]` normalisé depuis n'importe quelle source). **Risque
  câblé** : l'onglet « Timeline » du drawer (`RiskRegisterPage` `DrawerTimeline`) affiche
  désormais le **vrai** historique via `GET /risks/:id/timeline` (`riskTimelineService` +
  `useRiskTimeline`, client `api` typé) — remplace le « bientôt ». **Vérifié live**
  (Playwright : drawer → onglet Timeline → entrée « Mise à jour » Score 2.5 · Statut
  DRAFT · acteur · date relative ; 0 placeholder « bientôt »).
  - **Restes honnêtes** : **Asset** a déjà son drawer d'historique (fonctionnel, pourrait
    adopter la primitive plus tard) ; **Control** n'a d'historique que via
    `audit_events` **admin-only** (`/governance/audit-events?entity_type=compliance_control`
    — il faudrait un accès lecture non-admin ou un endpoint dédié) ; **Mitigation** n'est
    **pas `Auditable`** → aucune source d'historique (nécessite un `mitigation_histories`
    ou opt-in Auditable côté backend). La page orpheline `/risks/:riskId/timeline`
    (`pages/RiskTimeline.tsx`, non liée, bugs de token/fetch) est **supplantée** par
    l'onglet du drawer.

**Adoption du kit (Vague 1, §2/§3)** : ✅ **Vulnérabilités** = 1ʳᵉ adoption réelle de
  `DataTable` (le primitive du kit était livré mais **jamais adopté**) — tri par colonne,
  colonne Priorité **figée**, densité, drawer préservé. **Vérifié live** (Playwright : tri
  CVSS desc → 1ʳᵉ ligne 9.8 ; 14 lignes ; clic ligne → drawer). Reste : **Registre des
  risques** (le pilote « officiel », plus lourd : multi-sélection + barre groupée + vue
  Matrice + menu de ligne à préserver) puis **Inventaire des actifs**.

**Reporté / non commencées** (plus lourdes, backend) : ⏸️ **E notifications catégorisées**
  (reporté à la demande du fondateur — lourd), F relance inactivité, Control/Mitigation
  time-travel (endpoints d'historique), dashboards par rôle. Voir §1bis + §2.

## 2. Revue par intention

### 0 · Piloter
| Écran | Route | Priorité UI | Points de revue clés | Statut |
|-------|-------|-------------|----------------------|--------|
| Dashboard | `/` | **Haute** | héros chiffré par rôle (UX-24) ; retirer le reste de mock ; KRI tabulaires | 🟡 **onboarding guidé jusqu'au Aha + personnalisation** livré (`OnboardingChecklist`) ; **dashboard par rôle** + hero à élever |
| Tableau exécutif | `/analytics` | Moyenne | déjà data-réel ; appliquer tokens/hiérarchie ; filtres temporels | 🟢 données OK, polish |
| Quantification financière | `/analytics/financial` | Moyenne | tokens ; table top-expositions → `DataTable` ; simulateur en master-detail | ⬜ |

### 1 · Identifier
| Écran | Route | Priorité UI | Points de revue clés | Statut |
|-------|-------|-------------|----------------------|--------|
| Inventaire des actifs | `/assets` | **Haute** | table → `DataTable` (tri/sélection/figée) ; `EmptyState` ; drawer master-detail 4K | ⬜ |
| Asset Universe | `/assets/universe` | Basse | graphe : tokens + états ; panneau de dépendances en master-detail | ⬜ |
| Vulnérabilités | `/vulnerabilities` | **Haute** | table priorisée → `DataTable` ; glossaire déjà là (OR-BUG-010) ; drawer master-detail | 🟢 **table migrée vers `DataTable`** (1ʳᵉ adoption du kit : tri par colonne, colonne Priorité figée, densité) + glossaire ; drawer master-detail 4K = reste |
| Intel Threat (CTI) | `/threat-map` | Moyenne | flux CVE → `DataTable` ; badges KEV/MITRE ; états | ⬜ |
| Infrastructure (scanner) | `/infrastructure` | Moyenne | cartes providers : tokens ; historique scans → `DataTable` ; preview en master-detail | ⬜ |

### 2 · Évaluer
| Écran | Route | Priorité UI | Points de revue clés | Statut |
|-------|-------|-------------|----------------------|--------|
| Registre des risques | `/risks` | **Haute (pilote)** | migrer la table bespoke vers `DataTable` (tri/sélection/figée) ; barre d'actions groupées + radiographie d'impact (UX-11) ; drawer master-detail 4K ; `data-testid` par ligne | 🟡 density-aware fait ; migration DataTable = prochain lot |
| Import de risques | `/risks/import` | Basse | tokens ; retour clair ; états | ⬜ |
| Pondération smart-risk | `/risks/weighting` | Basse | tokens ; autosave « Enregistré ✓ » | ⬜ |

### 3 · Traiter
| Écran | Route | Priorité UI | Points de revue clés | Statut |
|-------|-------|-------------|----------------------|--------|
| Mitigations | `/mitigations` | **Haute** | Kanban/Table/Gantt : tokens ; vue Table → `DataTable` ; `EmptyState` ; micro-victoire 1ʳᵉ mitigation | ⬜ |
| Incidents | `/incidents` | Moyenne | registre → `DataTable` ; drawer master-detail ; états | ⬜ |
| Automatisation (SOAR) | `/automation` | Moyenne | constructeur de règles : tokens/hiérarchie ; table SLA → `DataTable` | ⬜ |

### 4 · Prouver
| Écran | Route | Priorité UI | Points de revue clés | Statut |
|-------|-------|-------------|----------------------|--------|
| Conformité | `/compliance` | **Haute** | grille de référentiels : tokens ; `EmptyState` ; jauges | ⬜ |
| Détail référentiel | `/compliance/:id` | **Haute** | table de contrôles → `DataTable` ; drawer preuve master-detail ; micro-victoire 1ᵉʳ contrôle | ⬜ |
| Analyse des écarts | `/compliance/gap-analysis` | Moyenne | écarts → `DataTable` ; action « Remédier » dominante | ⬜ |
| Audits | `/compliance/audits` | Moyenne | table → `DataTable` ; états | ⬜ |
| Remédiations | `/compliance/remediations` | Moyenne | table → `DataTable` ; overdue coloré | ⬜ |
| Rapports | `/reports` | Moyenne | cartes : tokens ; micro-victoire 1ᵉʳ rapport | ⬜ |
| Board Report | `/reports/board` | Basse | tokens ; états | ⬜ |
| Assistant IA | `/recommendations` | Basse | tokens ; états chargement informatifs | ⬜ |
| Risques émergents (IA) | `/ai/emerging-risks` | Basse | tokens ; états | ⬜ |
| Gouvernance | `/governance` | Moyenne | journal d'audit → `DataTable` (diff avant/après) ; états | ⬜ |

### Utilitaire
| Écran | Route | Priorité UI | Points de revue clés | Statut |
|-------|-------|-------------|----------------------|--------|
| Paramètres | `/settings` | — | fixtures purgées + autosave (OR-BUG-004) ; a11y (OR-BUG-012) ; reste = tokens | 🟢 données réelles + autosave faits |
| Rôles & accès | `/settings/roles` | — | invitation de membre livrée (OR-BUG-003) ; table membres → `DataTable` | 🟡 invite fait, table à élever |

### Écrans de détail / paramétrés (transverses)
War Room `/incidents/:id/war-room` · Aperçu de scan `/infrastructure/scans/:jobId` ·
Chronologie de risque `/risks/:id/timeline` : tokens + états ; adopter master-detail
là où un détail s'ouvre.

## 3. Séquencement en vagues (chaque vague = quelques PR atomiques, E2E vert entre chaque)
- **Vague 1 — le pilote & la preuve du kit** : migrer **Registre des risques** vers `DataTable` (tri/sélection/figée/master-detail 4K) + `data-testid` par ligne ; c'est le patron de référence pour toutes les autres tables. Puis **Vulnérabilités** et **Inventaire des actifs** (mêmes gestes).
- **Vague 2 — Prouver** : **Détail référentiel** (contrôles → `DataTable`, preuve en master-detail, micro-victoire 1ᵉʳ contrôle), Gap analysis, Audits, Remédiations.
- **Vague 3 — Traiter** : Mitigations, Incidents (drawer master-detail), Automatisation.
- **Vague 4 — Piloter & finitions** : hero Dashboard par rôle, Financier, Exécutif (filtres), puis les écrans « Basse » priorité (Import, Pondération, Board, IA) + Asset Universe.
- **Transverse (continu)** : audit de mouvement (tokens partout), adoption `EmptyState` sur **tous** les écrans de liste, re-passe a11y (vider `A11Y_KNOWN`), convention `data-testid`, garde Mobile Chrome verte.

## 4. Garde-fous
- **Une table à la fois** : la migration vers `DataTable` doit préserver clic-ligne→drawer, sélection, menu par ligne, export — re-jouer `smoke.routes` + `workflows` après chaque écran.
- **Ne pas mentir** : un écran sans backend (billing managé, historique de sessions, roster War Room) reste en état honnête « bientôt », jamais en fixture.
- **Chaque PR** : atomique, `tsc`/`vite build` verts, E2E de la route verte, capture avant/après, sans co-auteur.

## 5. Suivi
Cochez le **Statut** ci-dessus (⬜ à faire · 🟡 partiel · 🟢 fait) à chaque PR, et
référez la PR à la règle UX correspondante (UX_CHARTER) + à ce plan.
