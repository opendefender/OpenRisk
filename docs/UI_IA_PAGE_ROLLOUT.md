# OpenRisk — Revue page par page : déploiement UI_ELEVATION + IA_NAVIGATION

> Plan d'exécution qui applique, **écran par écran**, `UI_ELEVATION_PROPOSAL.md`,
> `IA_NAVIGATION_PROPOSAL.md` et les consignes UX du fondateur (rattachées à la
> charte `UX_CHARTER.md`). Objectif : une app aussi évidente qu'une console AWS /
> une app Google, où un utilisateur sans background cyber réussit seul.
>
> Statut au 2026-07-25. Légende : ✅ fait · 🟡 partiel · ⏳ à faire. Chaque ligne =
> un lot atomique (vérif tsc/build + live quand le sandbox le permet).

---

## 0. Ce qui est déjà appliqué (socle transverse)

| Consigne (UX-xx) | État | Où / preuve |
|---|---|---|
| Inscription = 1ᵉʳ engagement minimal (email+mdp), onboarding **après** (UX-02/13) | ✅ | `RegisterForm` 3 champs → compte → onboarding dashboard |
| Onboarding qui **fait accomplir** jusqu'au Aha, guide vers l'action suivante, pas de product tour (UX-01/07/14) | ✅ | `features/onboarding/OnboardingChecklist` (4 étapes, auto-cochées, étape courante mise en avant) |
| Micro-victoires célébrées (UX-32) + confetti reduced-motion-safe | ✅ | `shared/celebrate.ts` (1ᵉʳ risque + fin d'onboarding) |
| Personnalisation **après le Aha** (thème/couleur) (UX-17) | ✅ | étape « Personnaliser » (ghost edit inline, thème + accent) |
| Autosave, plus de bouton Enregistrer inutile + « Enregistré ✓ » (UX-23) | 🟡 | fait sur Paramètres (`settingsPrefs`) ; **à généraliser** aux préférences des autres écrans |
| Navigation par **intention**, ≤ 7 espaces (UX-16) | ✅ | `navModel` 5 intentions (Piloter/Identifier/Évaluer/Traiter/Prouver) + utilitaire |
| Messages d'erreur explicites (quoi/pourquoi/quoi faire) (UX-03) | 🟡 | login + invite faits ; **à auditer** sur chaque mutation |
| États vides avec action primaire (UX-04) | 🟡 | `shared/EmptyState` livré + adopté (Risques, RBAC) ; **à adopter partout** |
| Recherche universelle ⌘K (UX-22) | 🟡 | palette existe ; **étendre** la couverture (CVE, contrôle, audit, rapport, membre) |
| Dashboard différencié par rôle (UX-24) | 🟡 | landing par rôle + Piloter par rôle (onboarding) ; **vues dédiées** RSSI/Analyste/Auditeur/Direction à finir |
| Tokens de design + densité commutable + mouvement (UI §1/§4) | ✅ | `index.css` tokens + `data-density` + contrôle entête |
| Table dense réutilisable (tri/colonne figée/sélection) (UI §6) | 🟡 | `shared/DataTable` livré ; **adopter** écran par écran (Registre reste sur sa table + densité) |
| Confirmation d'action / retour visuel < 100 ms (UX-08) | 🟡 | toasts présents ; **normaliser** optimistic + toast succès/échec partout |
| Invitation de membre / collaboration (UX-24) | ✅ | `POST /rbac/members` + modale (OR-BUG-003) |
| Responsive 360→4K (UX-27) | 🟡 | sidebar off-canvas + tables scroll ; **master-detail 4K** (`.or-md-*`) à brancher |

## 1. Consignes transverses restantes (à implémenter en primitives partagées)

Chacune = une primitive réutilisable, puis adoption par écran.

| # | Consigne | Primitive à livrer | Priorité |
|---|---|---|---|
| A | **Suppression mineure** = exécution immédiate + toast « Annulé/Restaurer » ≥ 7 s, **pas de modale** (UX-12/28) | `shared/undoableDelete.ts` (hide optimiste + commit différé + Undo) | P1 |
| B | **Suppression importante** = **radiographie d'impact** (objets liés) + alternative (transfert) avant d'agir (UX-11) | `shared/ImpactDialog.tsx` (liste conséquences + actions « Annuler / Transférer / Supprimer ») | P1 |
| C | **Aide contextuelle** au 1ᵉʳ survol, non répétée (UX-14) | `shared/Hint.tsx` (tooltip 1ʳᵉ rencontre, mémorisé localStorage) ; compléter le `<Term>` glossaire existant | P2 |
| D | **Attente exploitée** : progression + info utile si > 1,5 s (UX-09) | `shared/ProgressState.tsx` (étapes + stat pendant un scan / génération de rapport / calcul CRQ) | P2 |
| E | **Notifications catégorisées** (Sécurité/Conformité/Tâches/Collaboration/Produit/Facturation), préférences par catégorie **et** canal (in-app + email) (UX-20/21) | `domain.NotificationCategory` + préférences par catégorie/canal + centre de notif catégorisé | P1 |
| F | **Relance après inactivité** + **annonce de nouveauté**, calées sur le fuseau/heures d'activité (UX-29) | backend : suivi `last_active_at` + job d'envoi (email + in-app) au bon créneau | P2 |
| G | **Raccourcis clavier** découvrables (aide `?`) pour les 5 actions clés (UX-26) | `shared/useHotkeys.ts` + overlay d'aide `?` | P2 |
| H | **Time travel** : historique daté & attribué sur chaque entité majeure (UX-25) | générraliser le drawer d'historique (déjà sur Assets) à Risk/Control/Mitigation | P1 |
| I | **Aperçu flouté** des features payantes + 3 moments de conversion (après Aha / à la limite / après victoire), **jamais** « limite atteinte » sec (UX-18/19/30) | `shared/PremiumPeek.tsx` + déclencheurs Fogg aux 3 moments | P3 |
| J | **Trial court / basé usage** (loi de Parkinson), compteur d'usage visible (UX-30) | bandeau d'essai + compteur d'usage (dépend du billing) | P3 |
| K | **Onboarding testé ≥ 1×/semaine** (UX-33) | job CI hebdo `journey.newcomer.spec.ts` | ✅ (cron nocturne `e2e.yml`) — passer en garde bloquante hebdo |

---

## 2. Revue écran par écran

Pour chaque écran : (a) **intention** (IA), (b) **action dominante** (UI §3), (c) **états** vide/chargement/erreur (UX-03/04/09), (d) **spécifiques**. Priorité globale P1 (parcours cœur) → P3.

### Piloter
| Écran | Action dominante | À appliquer | Prio |
|---|---|---|---|
| **Dashboard `/`** | Onboarding (si vierge) → sinon Cyber-score + ALE | ✅ onboarding ; 🟡 **dashboard par rôle** (RSSI top-risques/expo · Analyste vulns · Auditeur contrôles · Direction coûts/KPI) ; retirer widgets fixtures restants (`SecurityScore`/`AssetStatistics`) | P1 |
| **Exécutif `/analytics`** | Note A–F + ALE | 🟡 filtres temporels ; adoption tokens/densité ; radar contrôles | P2 |
| **Financier `/analytics/financial`** | ROSI / ALE portefeuille | 🟡 simulateur en ghost-edit ; ProgressState pendant le calcul CRQ (D) | P2 |

### Identifier
| Écran | Action dominante | À appliquer | Prio |
|---|---|---|---|
| **Actifs `/assets`** | + Nouvel actif | 🟡 `DataTable` (tri/colonne figée/sélection) ; EmptyState actionnable ; time-travel déjà présent (H ref) ; suppression → radiographie B (actif support de risques ?) | P1 |
| **Asset Universe `/assets/universe`** | Graphe | 🟡 densité/tokens ; légende ; master-detail 4K | P3 |
| **Vulnérabilités `/vulnerabilities`** | File priorisée P1 | 🟡 `DataTable` ; `<Term>` CVE/KEV/EPSS ✅ ; a11y contraste ✅ ; suppression mineure → undo A | P1 |
| **Intel Threat `/threat-map`** | Flux CVE | 🟡 densité ; tooltips MITRE (C) | P2 |
| **Infrastructure `/infrastructure`** | Lancer un scan | 🟡 ProgressState pendant le scan (D) ; EmptyState « connectez un cloud » | P2 |

### Évaluer
| Écran | Action dominante | À appliquer | Prio |
|---|---|---|---|
| **Registre `/risks`** | + Nouveau risque | ✅ densité ; 🟡 migrer la table vers `DataTable` (tri/sélection/radiographie de suppression B) ; ⌘K couvre les risques | P1 |
| **Drawer de risque** | Action de phase suivante | 🟡 master-detail 4K ; time-travel (H) ; édition ghost des champs simples (UX-06) | P1 |
| **Pondération `/risks/weighting`** | Sliders | 🟡 autosave + « Enregistré ✓ » (UX-23) | P2 |

### Traiter
| Écran | Action dominante | À appliquer | Prio |
|---|---|---|---|
| **Mitigations `/mitigations`** | Kanban / + Mitigation | 🟡 confirmations : complétion = toast, suppression mineure = undo A ; retour visuel < 100 ms | P1 |
| **Incidents `/incidents`** | Déclarer / War Room | 🟡 EmptyState ; **victoire significative** (incident résolu) → moment de conversion I | P2 |
| **Automatisation `/automation`** | Nouvelle règle | 🟡 constructeur en ghost-edit ; ProgressState au test d'une règle | P2 |

### Prouver
| Écran | Action dominante | À appliquer | Prio |
|---|---|---|---|
| **Conformité `/compliance`** | Importer / Voir les écarts | 🟡 jauge dominante ; EmptyState « importez votre 1ᵉʳ référentiel » (déjà relié à l'onboarding) ; suppression référentiel = radiographie B (contrôles liés) | P1 |
| **Détail référentiel `/compliance/:id`** | Progression | 🟡 preuve = 1ᵉʳ moment de fierté → micro-victoire (1ᵉʳ contrôle implémenté) | P1 |
| **Gap / Audits / Remédiations** | Remédier | 🟡 ProgressState à la génération auto de remédiations (D) | P2 |
| **Rapports `/reports` · Board** | Générer | 🟡 ProgressState pendant la génération PDF/IA (D) ; **victoire** → conversion I | P1 |
| **IA `/recommendations` · `/ai/emerging-risks`** | Poser une question | 🟡 « analyse de votre base… » (déjà) ; sources ✅ | P2 |
| **Gouvernance `/governance`** | Boîte d'approbation | 🟡 time-travel = cœur ✅ ; radiographie sur révocation de délégation B | P2 |

### Utilitaire
| Écran | Action dominante | À appliquer | Prio |
|---|---|---|---|
| **Paramètres `/settings`** | — | ✅ données réelles + autosave (OR-BUG-004) ; 🟡 notifications par **catégorie**×canal (E) | P1 |
| **Rôles & accès `/settings/roles`** | Inviter | ✅ invitation (OR-BUG-003) ; 🟡 **radiographie** à la suppression/rétrogradation d'un membre (groupes, risques possédés, + « transférer ses risques ») (B) | P1 |
| **Auth `/login` `/register`** | Se connecter / S'inscrire | ✅ inscription réelle, MFA façade retirée ; 🟡 vrai challenge MFA interactif | P1 |

---

## 3. Séquencement proposé (lots atomiques, une branche/lot)

1. **P1 — Frictions & confiance** : primitives A (undo-delete) + B (radiographie d'impact) → adoption Risques/Membres/Conformité ; H (time-travel) sur Risk/Mitigation ; retour visuel < 100 ms normalisé.
2. **P1 — Dashboards par rôle** (UX-24) : 4 vues (RSSI/Analyste/Auditeur/Direction) ; retirer les derniers widgets fixtures.
3. **P1 — Notifications catégorisées** (E) : catégories + préférences par canal + centre in-app ; base pour F (relance) et les 3 moments de conversion (I).
4. **P2 — Adoption `DataTable`/`EmptyState`/`ProgressState`** écran par écran (Actifs, Vulns, Registre) ; ⌘K couverture complète (C).
5. **P2 — Aide contextuelle** (C `Hint`) + raccourcis clavier (G) + master-detail 4K.
6. **P3 — Conversion** : aperçu flouté premium (I) + trial/usage (J) — dépend du billing.

**Garde permanente** : chaque lot passe `tsc -b` + `vite build` + E2E vert ; l'onboarding est testé chaque nuit (UX-33). Ne jamais régresser la surface saine (42 routes, 0 cassé) mesurée par la suite E2E.
