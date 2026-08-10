# Contrôles morts — recensement et traitement

> Dans un outil de sécurité, un bouton qui ment est une vulnérabilité de confiance.
> Un opérateur calibre son jugement sur ce que l'interface affiche : un voyant vert
> permanent, un « Synchroniser » sans effet visible ou un « Supprimer
> l'organisation » inerte apprennent à l'utilisateur à ne plus croire l'écran —
> y compris quand l'écran dit vrai.

Date de l'audit : **2026-08-07** · Périmètre : `frontend/src` (hors tests).

## Règle appliquée

Aucun élément interactif ne peut être mergé sans :

1. **(a)** un handler réel ;
2. **(b)** un état *loading / success / error* visible ;
3. **(c)** un test E2E qui vérifie l'**effet observable** (`tests/e2e/datatable.spec.ts`,
   `tests/e2e/dead-controls.spec.ts`).

Cette règle est en partie **rendue structurelle** par `<DataTable>` : les types de
`shared/datatable/types.ts` rendent un en-tête triable, une facette ou une action
de ligne *décorative* littéralement inexprimable — une colonne n'est triable que
si elle déclare **comment** trier (`sortKey` serveur ou `sortValue` client), une
action de ligne n'existe que si elle a un `onSelect`.

## Méthode de recensement

Balayage de tous les `<button>`, `<a>`, `<select>`, `<input>`, `<textarea>` de
`frontend/src` avec analyse de la balise ouvrante complète (multi-lignes),
recherche des éléments sans `onClick` / `href` / `onChange` / `type="submit"` /
props d'interaction *floating-ui*. Puis revue manuelle des contrôles qui **ont**
un handler mais **aucun effet observable** (les plus insidieux : ils passent tous
les greps).

---

## 1. Contrôles morts en production — traités

| # | Contrôle | Emplacement | Diagnostic | Décision |
|---|----------|-------------|-----------|----------|
| 1 | **Point vert « Realtime »** | `components/layout/AppHeader.tsx` | `<span>` stylé qui pulsait en vert **quoi qu'il arrive** : backend éteint, PC hors ligne, tenant sans canal temps réel. | **Implémenté pour de vrai.** Nouveau `lib/connection.ts` alimenté par les événements `online`/`offline` du navigateur **et** par l'issue de chaque appel axios (`lib/api.ts`). Vert = l'API a répondu · ambre = requête sans réponse (refus/timeout/DNS/CORS) · gris = navigateur hors ligne. Une réponse HTTP 4xx **n'est pas** une panne de connexion (le backend est vivant) et ne fait pas passer le voyant à l'ambre. `role="status"` + `aria-live` pour les lecteurs d'écran. |
| 2 | **Bouton micro « Voice assistant »** | `components/layout/AppHeader.tsx:122` | Aucun `onClick`. Aucune fonctionnalité vocale n'existe nulle part dans le produit. | **Supprimé** (arbitrage utilisateur). |
| 3 | **« Voir toutes les notifications »** | `AppHeader.tsx` (pied du panneau) | Le bouton **fermait le panneau**. Il n'existe aucune vue « toutes les notifications ». | **Supprimé**, ainsi que la clé i18n `notifViewAll`. Le panneau liste déjà le flux réel avec défilement ; la cloche, le badge non-lu, « Tout marquer comme lu », le filtre par catégorie et le clic → marquer-comme-lu étaient, eux, réels et sont conservés. |
| 4 | **« Supprimer l'organisation »** | `features/settings/SettingsScreen.tsx` (onglet Danger) | `<button>` **sans `onClick`** — le contrôle le plus destructeur du produit ne faisait rien. | **Implémenté.** `DELETE /rbac/tenants/:id` réel, derrière une radiographie d'impact `DangerConfirm` (membres déconnectés, registres supprimés, piste d'audit supprimée, jetons invalidés) + alternative « exporter d'abord », puis `logout()` et retour au login. Le backend restreint au propriétaire : un admin non-propriétaire reçoit 403 → message explicite. Désactivé si le tenant est inconnu. |
| 5 | **« Actualiser » (tableau exécutif)** | `features/analytics/ExecutiveDashboard.tsx` | Handler réel (`refetch()`) **mais aucun état visible** : sur un dashboard qui se rafraîchit déjà toutes les 60 s, la charge utile est souvent identique → le bouton *paraît* mort. | **État rendu visible** : désactivé + icône en rotation + libellé « Actualisation… » pendant le vol, toast de succès ou d'échec à l'atterrissage. |
| 6 | **« Synchroniser » (Threat Intel)** | `features/cti/ThreatIntel.tsx` | Idem : `useCTISync` invalidait bien le flux, mais la synchro NVD + CISA KEV dure plusieurs secondes sans le moindre retour → lu comme mort. | **État rendu visible** : désactivé, spinner, libellé « Synchronisation… », toast avec le nombre réel de CVE. |
| 7 | **« Matcher les actifs » (Threat Intel)** | `features/cti/ThreatIntel.tsx` | Idem. De plus, `0 risque créé` est un **résultat**, pas un silence. | **État rendu visible** + message explicite « Aucune nouvelle exposition détectée ». |
| 8 | **Bouton « Filtres » (Asset Universe)** | `features/universe/AssetUniverse.tsx:307` | `<button>` **sans `onClick`**, portant en plus l'infobulle de l'*autre* bouton (« Réinitialiser la vue »). | **Implémenté** : filtre réel par criticité. Décocher un niveau retire les actifs **et toutes les arêtes qui y aboutissent** (une arête pendante vers un nœud masqué affirmerait une dépendance vers rien). Le compteur de la barre d'état affiche `n / total` quand un filtre est actif. |
| 9 | **Bouton « Actif » / recentrage (Asset Universe)** | `features/universe/AssetUniverse.tsx:308` | Vérifié : `onClick={resetView}` **réel** (recentre + remet le zoom à 1). Seule son infobulle prêtait à confusion avec le bouton mort voisin. | **Conservé.** Le voisin mort étant réparé, l'ambiguïté disparaît. |
| 10 | **Bouton « Filtres » (registre des risques)** | `features/risks/RiskRegisterPage.tsx` | Le bouton libellé « Filtres » ouvrait **un champ de recherche**. Recherche et filtrage sont deux intentions différentes. | **Séparé en deux affordances** : la recherche instantanée est toujours visible dans la barre d'outils ; « Filtres » ouvre un panneau de **facettes** réelles (criticité, statut, phase ISO 31000, origine), combinables, reflétées dans l'URL, avec compteur de résultats, réinitialisation et **filtres sauvegardés nommés**. |
| 11 | **Menu de ligne « ⋯ » tronqué** | toutes les tables | `position: absolute` dans le `<td>` → le menu était **coupé** par l'`overflow: hidden` de la carte sur les dernières lignes ; il fallait scroller la page pour atteindre ses entrées. | **Corrigé structurellement** : `shared/datatable/RowMenu.tsx` rend le menu dans un **portail** (aucun ancêtre ne peut le couper) positionné par `@floating-ui/react` avec `flip` + `shift` + `size`. Test E2E obligatoire : dernière ligne d'une table de 200 items, menu entièrement dans le viewport. |
| 12 | **Icône « copier » du préfixe de jeton API** | `SettingsScreen.tsx` (onglet Jetons) | Un glyphe `Copy` affiché à côté du préfixe **ne copiait rien** (simple décoration dans une cellule non cliquable). | **Implémenté** : vrai bouton `copyPrefix` avec toast de confirmation ; aucun bouton rendu quand il n'y a pas de préfixe. |
| 13 | **`InlineVulnStatus` non branché** | `features/vulnerabilities/VulnerabilitiesPage.tsx` | Un éditeur de statut inline **complet** (optimiste, avec toast) était défini… et jamais utilisé : la colonne « Statut » rendait un simple `StatusChip`. Le composant était du code mort, et la promesse d'édition inline présente ailleurs manquait ici. | **Branché** dans la colonne Statut, sous garde de permission `vulnerabilities:update` (chip en lecture seule sinon). |

## 2. Contrôles morts dans du code **inatteignable** — supprimés

Ces fichiers ne sont importés par aucune route ni aucun composant (vérifié par
recherche de chemin d'import exact). Leurs contrôles morts n'étaient donc pas
visibles par un utilisateur, mais ils empoisonnent la recherche de code et
finissent par être recopiés.

| Fichier | Contrôle mort | Décision |
|---------|---------------|----------|
| `pages/TenantManagement.tsx` | 2 `<input>` `disabled` sans handler, page non routée | **Supprimé** |
| `components/rbac/RoleTemplateBuilder.tsx` | bouton d'action sans `onClick`, non importé | **Supprimé** |
| `features/risks/RiskListPage.tsx` | ancien registre orphelin (remplacé par `RiskRegisterPage`) | **Supprimé** |
| `features/risks/RiskDrawer.tsx` | bouton « Voir » sans `onClick` ; importé **uniquement** par `RiskListPage` | **Supprimé** |

Les entrées correspondantes de la liste d'exemption `eslint.config.js` ont été
retirées (« cette liste ne peut que rétrécir »).

## 3. Contrôles morts restants — **non traités**, avec leur raison

Honnêteté avant exhaustivité : ces éléments sont recensés mais **pas corrigés**
dans ce lot.

| Contrôle | Emplacement | Pourquoi c'est laissé |
|----------|-------------|------------------------|
| Cloche + badge non-lu | `components/layout/PageHeader.tsx:199` | Fichier **non routé** (le header live est `components/layout/AppHeader.tsx`). Non supprimé parce que d'autres écrans legacy l'importent encore ; à traiter dans la passe de nettoyage des pages legacy. |
| Bouton de changement d'avatar | `features/settings/GeneralTab.tsx:78` | Atteignable **uniquement** via `pages/Settings.tsx`, non routé (l'écran live est `features/settings/SettingsScreen.tsx`). Implémenter un vrai upload d'avatar demande un endpoint qui n'existe pas. |
| Boutons d'action de `features/assets/AssetsPage.tsx` | idem | Écran d'actifs legacy, remplacé par `features/assets/InventoryPage.tsx` (routé). Même passe de nettoyage. |
| Chat / roster de la War Room | `features/incidents/WarRoom.tsx` | Assumé et **badgé « Aperçu »** dans l'UI : il n'y a pas de backend de chat ni de participants. Un badge explicite n'est pas un mensonge ; c'est le contrat honnête en attendant `incident_participants`. |
| Verrou d'upsell du Classement | `shared/UpsellLock.tsx` | Cosmétique et **documenté comme tel** : il ne garde rien de sécurité. Le vrai gating d'abonnement relève du module Billing, absent. |

## 4. Serveur vs client : ce que « tri et pagination SERVEUR » veut dire ici

`<DataTable>` est écrit **serveur d'abord**. Le mode client existe uniquement
pour les endpoints qui renvoient la collection entière — le prétendre paginé
côté serveur serait exactement le genre de mensonge que ce document combat.

| Table | Mode | Raison |
|-------|------|--------|
| Registre des risques | **serveur** | `GET /risks` : `page`, `limit`, `sort_by`, `sort_dir`, `q`, `status`, `criticality`, `phase`, `source` (facettes ajoutées dans ce lot). |
| Vulnérabilités | **serveur** | `GET /vulnerabilities` : `page`, `limit`, `sort_by`, `sort_dir`, `q`, `tier`, `severity`, `status`, `kev`. |
| Piste d'audit | **serveur** | `GET /governance/audit-events` : `limit`, `offset`, `search`, `entity_type`, `action`. |
| Actifs | client | `GET /assets` renvoie l'inventaire complet **par nécessité** : l'Asset Universe a besoin de tous les nœuds. |
| Mitigations | client | `GET /mitigations` sans pagination ; le Kanban et le Gantt ont besoin de l'ensemble. |
| Incidents | client | `GET /incidents` a des filtres mais pas de pagination (le filtre `status` est tout de même poussé à l'API). |
| Jetons API | client | `GET /tokens` renvoie une liste courte non paginée. |

Ajouts backend de ce lot, tous **au-dessus** du prédicat tenant (RÈGLE #2) :
facettes `criticality` / `phase` / `source` (+ `status` multi-valeurs) sur
`GET /risks`, tri autorisé sur `name`, `criticality` et `source`, et surtout la
recherche `q` qui associe désormais le `tsvector` **OR** un `ILIKE` — le
`plainto_tsquery` seul est exact au lexème près : taper « log » ne remontait
rien alors que « Log4Shell » était dans le registre, ce qui faisait passer la
boîte de recherche pour cassée.

## 5. Ce qui n'a **pas** pu être vérifié dans cette session

- **`go build` / `go vet` / tests Go non exécutés** : la chaîne d'outils Go n'est
  pas installée sur cette machine (`/usr/local/go/bin` vide, `go` absent du
  `PATH`). Les trois fichiers backend touchés
  (`internal/handler/risk_handler.go`, `internal/handler/query_values.go`,
  `internal/domain/risk_repository.go`,
  `internal/infrastructure/repository/gorm_risk_repository.go`) ont été relus
  ligne à ligne (imports présents, littéraux de struct tous nommés, paramètres
  liés — jamais interpolés), mais **ils n'ont pas été compilés**. À faire tourner
  avant merge.
- Les tests E2E sont écrits mais nécessitent un backend + un frontend démarrés
  (`make dev`) ; ils n'ont pas été exécutés ici pour la même raison (pas de
  backend compilable dans cet environnement).
