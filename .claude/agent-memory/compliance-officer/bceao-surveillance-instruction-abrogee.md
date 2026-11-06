---
name: bceao-surveillance-instruction-abrogee
description: Les textes BCEAO bougent — l'Instruction 127-07-08 est abrogée et les plafonds de monnaie électronique changent le 2026-11-02
metadata:
  type: project
---

Un catalogue réglementaire **se périme**. Deux cas constatés dans `bceao` le
2026-10-30, tous deux invisibles à la lecture du code seul :

1. **`BCEAO-SURV-1` / `BCEAO-SURV-2` citent l'Instruction n°127-07-08 du
   09/07/2008, abrogée.** L'Instruction n°001-03-2021 (art. 16) « abroge et
   remplace […] notamment l'Instruction n°127-07-08 ». Et l'obligation a changé
   de fond : le seuil de déclaration d'incident est passé de **1 heure à 2
   heures** (art. 10 du texte de 2021), et un **rapport circonstancié sous 3
   jours calendaires** a été ajouté. Le contrôle n'a jamais été mis à jour.
2. **`BCEAO-ME-5` (plafonds de monnaie électronique) se périme le
   2026-11-02.** L'Instruction n°020 du 08/10/2026 modifie les art. 31 et 34 de
   l'Instruction n°008-05-2015 : le plafond d'avoirs par client passe de 2 M à
   **4 M FCFA**. Le contrôle est encore exact au 2026-10-30, faux trois jours
   plus tard.

**Pourquoi :** la BCEAO publie des instructions modificatives sans renuméroter
le texte de base, et les instructions abrogatoires ne nomment le texte abrogé
que dans leur dernier article. Rien dans le code ne dit quand une citation a été
vérifiée pour la dernière fois, donc rien ne vieillit visiblement.

**Comment l'appliquer :** à chaque audit, pour chaque instruction ou avis cité
(pas seulement pour le règlement de base), chercher s'il existe un texte
postérieur qui l'abroge ou le modifie — l'article final du texte le plus récent
est l'endroit où le chercher. Et proposer un champ de date de vérification sur
`Catalog` : sans lui, cette dérive est structurellement indétectable.
Voir [[frameworks-verified-2026-10-30]].
