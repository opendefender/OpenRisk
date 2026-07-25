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

## 2. Revue par intention

### 0 · Piloter
| Écran | Route | Priorité UI | Points de revue clés | Statut |
|-------|-------|-------------|----------------------|--------|
| Dashboard | `/` | **Haute** | héros chiffré par rôle (UX-24) ; retirer le reste de mock ; KRI tabulaires ; micro-victoire onboarding | 🟡 onboarding+greeting faits (OR-BUG-002), reste du hero à élever |
| Tableau exécutif | `/analytics` | Moyenne | déjà data-réel ; appliquer tokens/hiérarchie ; filtres temporels | 🟢 données OK, polish |
| Quantification financière | `/analytics/financial` | Moyenne | tokens ; table top-expositions → `DataTable` ; simulateur en master-detail | ⬜ |

### 1 · Identifier
| Écran | Route | Priorité UI | Points de revue clés | Statut |
|-------|-------|-------------|----------------------|--------|
| Inventaire des actifs | `/assets` | **Haute** | table → `DataTable` (tri/sélection/figée) ; `EmptyState` ; drawer master-detail 4K | ⬜ |
| Asset Universe | `/assets/universe` | Basse | graphe : tokens + états ; panneau de dépendances en master-detail | ⬜ |
| Vulnérabilités | `/vulnerabilities` | **Haute** | table priorisée → `DataTable` ; glossaire déjà là (OR-BUG-010) ; drawer master-detail | 🟡 glossaire fait, table à élever |
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
