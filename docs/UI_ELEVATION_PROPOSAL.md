# OpenRisk — Proposition d'élévation UI (niveau console AWS / app Google)

> **Statut : RATIFIÉE par le fondateur le 2026-07-24.** Décisions §9 arrêtées (a Confort ·
> b master-detail 4K · c confetti/pastille selon contexte · d azure). Implémentation
> incrémentale sur `feat/ia-nav-ui-elevation` — premier lot livré (voir §10 « État »).
> Objectif : hisser l'interface au niveau d'une **console d'infrastructure** (densité,
> lisibilité, tables sérieuses) tout en gardant la **chaleur d'une app grand public**
> (mouvement, états vides accueillants). Aucune refonte de code dans cette session —
> 3 avant/après statiques sont fournis dans [`mockups/`](mockups/).

Le socle actuel (tokens `dc.html`, thèmes clair/sombre × azure/iris, FR/EN) est **bon**.
Cette proposition le **systématise** et comble les manques : hiérarchie, tables denses,
états, responsive 4K.

---

## 1. Jetons de design (design tokens)

### 1.1 Échelle typographique (modulaire, ratio 1,20 — « minor third »)
Base 14 px (densité « confort »), 13 px (densité « compact »).

| Token | px | Usage |
|-------|----|-------|
| `text-2xs` | 10.5 | badges, uppercase labels de section |
| `text-xs` | 12 | méta, aides, cellules secondaires |
| `text-sm` | 13 | **corps par défaut**, cellules de table |
| `text-base` | 14 | corps confort, labels de champ |
| `text-md` | 16 | sous-titres |
| `text-lg` | 19 | titres de carte |
| `text-xl` | 23 | titre d'écran |
| `text-2xl` | 28 | KPI |
| `text-3xl` | 34 | chiffre héro (ALE, cyber-score) |

Deux familles seulement : **UI sans-serif** (Inter/system) et **mono tabulaire**
(chiffres alignés — obligatoire pour montants FCFA, scores, CVSS). `font-variant-numeric: tabular-nums` partout où des nombres s'empilent.

### 1.2 Densité
Trois densités commutables (préférence utilisateur, UX-23 autosave) : **Compact**
(row 32 px), **Confort** (row 40 px, défaut), **Spacieux** (row 48 px). Une console
sérieuse **doit** offrir Compact pour les tables de 50+ lignes.

### 1.3 Espacement (échelle 4 px)
`0/1(4)/2(8)/3(12)/4(16)/5(20)/6(24)/8(32)/10(40)/12(48)/16(64)`. Un seul pas de
gouttière par contexte : **16 px** entre cartes, **8 px** intra-carte, **24 px** entre
sections d'écran.

### 1.4 Rayons
`sm 7px` (champs, boutons), `md 10px` (cartes internes), `lg 14px` (cartes/panneaux),
`xl 18px` (modales), `full` (pastilles/avatars). **Ne pas dépasser 18 px** — les
grands rayons « app mobile » (24–32 px, actuels sur certaines modales de risque)
cassent le registre « console ».

### 1.5 Élévation (ombres) — 5 niveaux, discrets
| Niveau | Usage | Ombre (thème clair) |
|--------|-------|---------------------|
| `e0` | fond de page | aucune |
| `e1` | carte au repos | `0 1px 2px rgba(0,0,0,.06)` |
| `e2` | carte survolée / sticky header | `0 2px 8px rgba(0,0,0,.08)` |
| `e3` | drawer / popover | `0 8px 24px rgba(0,0,0,.12)` |
| `e4` | modale | `0 16px 48px rgba(0,0,0,.20)` |

En thème sombre : l'élévation se lit par la **luminosité de surface** (bg plus clair)
plus qu'à l'ombre. Un seul système, deux expressions.

### 1.6 Couleur (règle dataviz)
- **Couleurs de statut réservées** à la sévérité (critical/high/medium/low) + succès/erreur. **Jamais** décoratives.
- **Accent** (azure/iris) = action et sélection uniquement.
- **Encre** en 3 niveaux (`text-primary/secondary/muted`) — le texte porte la hiérarchie, pas la couleur.
- Contraste **≥ 4.5:1** (corps) / **3:1** (gros texte + éléments UI) — corrige OR-BUG-011/012.

---

## 2. Grille & largeurs de lecture
- Grille **12 colonnes**, gouttière 16/24 px, marges fluides.
- **Largeur de lecture max 72ch** pour tout bloc de texte long (rapports, descriptions) — jamais du texte pleine largeur en 4K.
- **Content max-width 1440 px** pour les écrans « formulaire/lecture » ; **pleine largeur** pour les tables denses et les dashboards (ils exploitent le 4K, cf. §7).
- Gabarit constant : sidebar (248 px) · entête sticky (56 px) · zone de contenu à padding 24/32 px.

---

## 3. Hiérarchie visuelle — une action dominante par écran (UX-10)
Chaque écran déclare **UN** élément dominant, identifiable en < 3 s :

| Écran | Domine | Secondaire | Tertiaire |
|-------|--------|------------|-----------|
| Piloter (Dashboard) | Cyber-score + ALE (héros chiffrés) | KRI, top-risques | tendances |
| Registre des risques | Bouton **Nouveau risque** + la table | filtres | export/menu |
| Détail de risque | Le **score** + l'action de phase suivante | onglets | méta |
| Conformité | La **jauge de couverture** + « Voir les écarts » | cartes de référentiel | import |
| Vulnérabilités | La **file priorisée** (P1 en tête) | KPI | filtres |

Règle : une seule couleur d'accent pleine par écran (le bouton primaire). Tout le
reste est fantôme/secondaire. Les 3+ boutons pleins concurrents actuels (ex. barres
d'action de conformité) passent à **1 plein + n fantômes**.

---

## 4. Spécification de mouvement
- **Durées** : `fast 120ms` (hover, toggle), `base 180ms` (drawer/menu), `slow 260ms` (modale, transition de page). Rien au-dessus de 300 ms.
- **Courbes** : entrée `cubic-bezier(.2,.8,.2,1)` (decel), sortie `cubic-bezier(.4,0,1,1)` (accel), transform+opacity uniquement (jamais `width/top` animés — coûteux, cf. dette existante).
- **Micro-victoires (UX-32)** : premier risque / premier contrôle / premier rapport → pastille de succès + confetti **sobre** 600 ms, une seule fois.
- **`prefers-reduced-motion`** : respecté globalement (déjà présent, `index.css:222`) — toutes les animations tombent à un fondu 1 opacité, 0 déplacement.
- **Feedback < 100 ms (UX-08)** : tout clic pose immédiatement un état pressé + optimistic update ; le résultat serveur confirme/rollback.

---

## 5. États vides / chargement / erreur (UX-03/04/09)
Trois états canoniques, **un composant réutilisable chacun** :

- **Vide (UX-04)** : illustration légère + phrase de valeur + **1 action primaire** + 1 lien « voir un exemple ». Ex. Risques vide → « Créez votre premier risque » (pas une table vide muette).
- **Chargement (UX-09)** : **skeleton** calqué sur la vraie mise en page (jamais de spinner plein écran — déjà une règle CLAUDE.md #8) ; > 1,5 s → sous-texte de progression informatif (« Analyse de 142 actifs… »).
- **Erreur (UX-03)** : *ce qui s'est passé · pourquoi · quoi faire* + bouton d'action (Réessayer / Contacter). **Zéro** « une erreur est survenue ». Les erreurs réseau 4xx/5xx mappées à un message métier.

---

## 6. Tables denses (le marqueur « console »)
La table est l'objet central d'un outil GRC. Spécification :
- **En-tête sticky** + **1ʳᵉ colonne figée** (nom/titre) au scroll horizontal.
- **Tri** par colonne (indicateur ▲▼), **multi-tri** au `shift-clic`.
- **Sélection** : case par ligne + case d'entête (tout), barre d'action contextuelle flottante à la sélection (actions groupées), avec **radiographie d'impact** avant action destructive (UX-11).
- **Densité** commutable (§1.2), **colonnes** masquables/réordonnables (préférence persistée).
- **Nombres tabulaires** alignés à droite ; sévérité = pastille + libellé (jamais couleur seule, a11y).
- **Pagination** ou **scroll virtuel** au-delà de 100 lignes ; jamais tout charger.
- **Ligne = navigation** (ouvre le drawer) ; les actions par ligne dans un menu `⋯` (pas 5 icônes qui encombrent).

---

## 7. Responsive — 360 px → 4K (UX-27)
- **360–767 px (mobile)** : sidebar en tiroir (déjà fait), tables → **cartes empilées** (pas de scroll horizontal infernal), 1 action primaire en barre basse sticky.
- **768–1279 px (tablette)** : sidebar repliable, table à colonnes essentielles, drawers plein écran.
- **1280–1919 px (desktop)** : gabarit de référence, sidebar 248 px.
- **1920 px+ (2K/4K)** : **on remplit, on ne s'étire pas** — le contenu de lecture reste ≤ 1440 px centré, mais dashboards et tables gagnent des colonnes/panneaux (ex. drawer de détail côte à côte avec la table plutôt qu'en superposition). C'est ce qui distingue une console (exploite l'espace) d'un site (bande centrale perdue dans le vide).
- Vérifié en E2E : le projet **Mobile Chrome** (Pixel 5, 360 px) passe déjà les 42 routes.

---

## 8. Avant / Après (mockups statiques)
Trois écrans, HTML autonome, thème clair + sombre :

| Fichier | Écran | Ce que « Après » corrige |
|---------|-------|--------------------------|
| [`mockups/dashboard.html`](mockups/dashboard.html) | Piloter | 1 héros chiffré dominant, KRI tabulaires, fin du score fixture |
| [`mockups/risk-register.html`](mockups/risk-register.html) | Registre des risques | table console (tri, colonne figée, sélection, densité), 1 action primaire |
| [`mockups/settings.html`](mockups/settings.html) | Paramètres | données réelles + autosave « Enregistré ✓ », labels a11y, fin des fixtures |

---

## 9. Décisions ratifiées (fondateur, 2026-07-24)
- **(a) Densité par défaut : Confort** (row 40 px). Compact reste commutable pour les tables 50+ lignes.
- **(b) 4K : drawer côte à côte (master-detail).** Au-delà de 1920 px, le détail (drawer de risque/vuln/contrôle) s'ouvre en panneau adjacent à la liste, pas en superposition.
- **(c) Micro-victoires : confetti sobre OU pastille, selon le contexte.** Confetti (600 ms) réservé aux vrais jalons (1ᵉʳ risque, 1ᵉʳ contrôle évalué, 1ᵉʳ rapport) ; pastille « ✓ » discrète pour le fréquent (autosave, action mineure).
- **(d) Accent par défaut : azure.** Déjà le défaut du `uiStore` (`variant: 'azure'`), confirmé.

## 10. État d'implémentation
- ✅ **Livré (`feat/ia-nav-ui-elevation`)** :
  - **Accent azure** confirmé par défaut ; **IA de navigation à 5 intentions** (voir `IA_NAVIGATION_PROPOSAL.md`) ; mockups de référence polis (light+dark).
  - **Tokens de design** (§1) : motion (`--dur/--ease`), échelle typographique, espacement, rayons (≤18px), élévation — dans `index.css`.
  - **Système de densité** (§1.2) : `--den-*` via `[data-density]`, préférence persistée dans le `uiStore` (Confort défaut), **contrôle dans l'entête** (Confort→Compact→Spacieux) ; le Registre des risques y réagit (vérifié live).
  - **Primitives réutilisables** (§5/§6) : `DataTable` (tri/colonne figée/sélection, density-aware, zéro `any`) et `EmptyState` — prêtes à l'adoption écran par écran.
  - **Micro-victoires** (§4/§9c) : `celebrate()` confetti sobre au 1ᵉʳ risque (UX-32), `prefers-reduced-motion` respecté.
  - **CSS master-detail 4K** (§7b) : `.or-md-*` posé (adoption drawer à venir).
- ⏭️ **À séquencer** (écran par écran, vérif live + garde tsc/E2E verte) : adoption de
  `DataTable`/`EmptyState` sur les autres écrans de liste (Vulnérabilités, Actifs,
  Conformité, Incidents…), branchement du drawer de risque en master-detail 4K,
  composants d'erreur canoniques (§5). Chacun = un lot atomique ; ne pas tout réécrire
  d'un coup pour ne pas régresser la surface saine (42 routes, 0 cassé) mesurée par l'E2E.
