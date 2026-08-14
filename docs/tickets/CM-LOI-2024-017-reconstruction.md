# Reconstruction du référentiel « Cameroun — Protection des données personnelles »

**Statut :** ouvert
**Ouvert le :** 2026-08-13
**Cible :** catalogue v1.1
**Clé du catalogue :** `cm-loi-2024-017`
**Fichier :** `backend/pkg/compliance/catalog_placeholders.go`

## Ce qui a changé

Le référentiel est **retiré du catalogue d'import** (`Withdrawn: true`). Il
n'apparaît plus dans le sélecteur « Importer un référentiel » et
`POST /compliance/frameworks/{id}/import-catalog` le refuse avec un message qui
en donne la raison et renvoie vers ce ticket.

Le code n'est **pas supprimé** : la clé doit continuer à résoudre pour les
locataires qui auraient importé ce référentiel avant son retrait, et la
reconstruction doit avoir un endroit où atterrir.

## Pourquoi

Le catalogue ne contenait aucun contrôle. Un référentiel vide n'est pas neutre
dans ce produit : il compte dans le nombre de référentiels du tableau de bord, il
apparaît dans l'analyse d'écarts, et il produit un rapport de conformité que l'on
peut présenter. Un responsable conformité qui l'importe croit avoir un programme
et n'a qu'une coquille.

La règle appliquée ici est celle qui vaut déjà pour les autres catalogues
(voir l'en-tête de `catalog_placeholders.go`) : **on ne modélise pas des citations
d'articles de mémoire**. Une référence légale inventée présentée à un régulateur
est un dommage plus grave que l'absence du référentiel.

## Ce qu'il faut pour le rouvrir

1. **Texte source officiel** — la Loi n° 2024/017 telle que publiée au Journal
   officiel de la République du Cameroun. Le texte doit être fourni ; il ne doit
   pas être reconstitué à partir de commentaires, d'articles de presse ou de la
   mémoire d'un modèle.
2. **Modélisation article par article** — un `CatalogControl` par obligation
   opposable, avec :
   - `ReferenceCode` = le numéro d'article (ex. `Art. 12`) ;
   - `Name` = l'intitulé de l'obligation ;
   - `Description` = un résumé **original** de ce qui est exigé (ne pas recopier
     le texte réglementaire) ;
   - `SourceReference` = la citation exacte (ex. `Loi n° 2024/017, article 12`).
   Les articles qui ne portent pas d'obligation évaluable (définitions, entrée en
   vigueur, dispositions transitoires) sont exclus : une ligne qu'on ne peut ni
   prouver ni clore n'a rien à faire dans un registre.
3. **Relecture par une personne compétente en conformité**, contre le texte
   publié — pas contre cette modélisation.
4. **Tests** — ajouter le compte attendu à `TestExpectedControlCounts`
   (`backend/pkg/compliance/catalog_test.go`) ; `TestNoOrphanControls` impose déjà
   un code unique et une citation par contrôle.
5. **Crosswalks** (facultatif, recommandé) — correspondances vers le RGPD, qui
   partage une large part de sa structure. Chaque lien doit porter sa
   justification (`backend/pkg/compliance/crosswalks.go`) ; les correspondances
   discutables se déclarent `partial`, jamais `full`.
6. **Réactivation** — passer `Withdrawn: false` et `Available: true` dans
   `catalog_placeholders.go`, ou mieux, déplacer le catalogue dans son propre
   fichier `catalog_cm_loi_2024_017.go`, comme les autres référentiels réels.

## Impact sur les locataires existants

Aucun contrôle n'a jamais pu être importé depuis ce catalogue (il n'en contenait
pas), donc aucun registre existant ne perd de contenu. Un locataire ayant créé un
référentiel portant ce nom conserve son référentiel et ses éventuels contrôles
ad hoc : le retrait porte sur le **catalogue d'import**, pas sur les données.

## Référentiels africains déjà modélisés

Pour référence, ces catalogues-là citent bien article par article, à partir de
textes fournis : `cobac` (Règlement COBAC R-2016/04), `bceao`
(Règlement n° 15/2002/CM/UEMOA et instructions), `antic-cm`
(Loi n° 2010/012 du Cameroun). Ce sont eux qui donnent la forme attendue.
