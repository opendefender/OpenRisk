---
name: articles-difficiles-a-resoudre
description: Où trouver les textes réglementaires COBAC / BCEAO / Cameroun, et lesquels résistent — recette de récupération et pièges
metadata:
  type: reference
---

Recette de récupération des textes sources, établie le 2026-10-30. `pdftotext
-layout` puis découpage sur `^\s*Article\s+(\d+)\b` fonctionne sur tous les PDF
ci-dessous.

**Accessibles directement (sources primaires) :**
- Règlement n°15/2002/CM/UEMOA — `bceao.int/sites/default/files/2017-11/8-%20Reglement_n_15_2002_CM_UEMOA_...pdf` (248 articles)
- Instruction n°008-05-2015 (monnaie électronique) — `bceao.int/sites/default/files/2017-11/instruction_no008_05_2015_intranet.pdf` (42 articles)
- Instruction n°009/07/RSP/2010 (CIP-UEMOA) — `bceao.int/sites/default/files/2017-11/4-%20Instruction_No009-07-RSP-2010_du_26_juillet_2010-CIP-UEMOA.pdf`
- Instruction n°001-03-2021 (surveillance) — `bceao.int/sites/default/files/2021-03/...`
- **Recueil des textes légaux BCEAO, chapitre V** — `bceao.int/sites/default/files/2017-11/ch5chapitre_5_-_reglementation_relative_aux_systemes_et_moyens_de_paiement.pdf`. C'est **le** document à prendre pour les textes sans numérotation d'articles : il contient l'Avis n°001-09-2012 (e-relevés) intégral p. V-76, introuvable autrement.
- Loi camerounaise n°2010/012 — `art.cm/sites/default/files/documents/loi_2010-012_cybersecurite_cybercriminalite.pdf` (97 articles). L'URL avec le titre long en `%20` renvoie 404 ; celle-ci, en minuscules avec tirets bas, fonctionne.

**Résistent :**
- **Règlement COBAC R-2016/04 : aucune source primaire atteignable.** sgcobac.org
  renvoie du HTML, beac.int renvoie 404, scribd/studocu sont des dépôts
  utilisateurs à OCR mélangé. Repli utilisé : `basereglementaire.com/article_reglementair/article-<N>-cobac-r-2016-04/`
  — une page HTML par article, URL prédictible, donc récupérable en boucle `curl`.
  Cohérence interne vérifiée (renvois corrects à COBAC R-2003/01, R-2008/01,
  R-2014/01 et au texte n°01/15/CEMAC/UMAC/COBAC). **Mais le site porte
  lui-même « seule la version officielle fait foi »** : ce n'est pas une
  validation définitive.
- Les textes ISO (27001, 22301, 31000, 27005) sont payants. Le projet n'en a
  aucune copie : toute vérification de formulation ISO est impossible en l'état.

**Pièges relevés :**
- Un article peut porter **deux obligations sans rapport** : l'art. 43 du
  Règlement 15/2002 traite de l'identité à l'ouverture de compte **et**, en
  dernier alinéa, du relevé mensuel. Deux contrôles citant le même article ne
  sont pas forcément un doublon — toujours lire l'article complet avant de
  conclure.
- Tronquer la lecture d'un article fait rater l'essentiel : l'obligation de
  localisation des données dans la CEMAC est au **(iii)** de l'art. 65 COBAC,
  après 1 200 caractères de clauses de prestataires.

Voir [[frameworks-verified-2026-10-30]].
