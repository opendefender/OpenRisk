---
name: sox-itgc-domains-unsourced
description: Les 4 contrôles ITGC de sox-2002 citent une convention de pratique d'audit, pas un texte — seuls UNSOURCED du produit
metadata:
  type: project
---

`catalog_sox_2002.go:29` déclare `soxITGCSource = "SOX ITGC (COSO), Domain "`, et
les contrôles `ITGC-AC`, `ITGC-CM`, `ITGC-PD`, `ITGC-OP` citent
« SOX ITGC (COSO), Domain *Access to Programs and Data* », etc.

**Cette citation ne résout vers aucun article d'aucun texte officiel.** Le
Sarbanes-Oxley Act ne contient pas de « domaines ITGC » ; le référentiel COSO
*Internal Control — Integrated Framework* n'en définit pas non plus. Les quatre
domaines sont une convention de la pratique d'audit — réelle et largement
partagée, mais sans support normatif numéroté. L'en-tête du fichier l'admet :
« the ITGC domains are the widely-recognized **categories** ».

Ce sont les **seuls 4 `UNSOURCED` du produit** au 2026-10-30, et ils bloquent
`sox-2002` (`UNSOURCED` interdit `Available: true`). Les 6 sections statutaires
(302, 404, 409, 802, 806, 906) sont parfaitement sourcées.

**Pourquoi :** le piège est qu'une convention professionnelle *largement
reconnue* ressemble à une source. Le test opérationnel qui tranche :
« le responsable conformité peut-il mettre l'article cité sous les yeux de son
auditeur ? » Ici non — il n'y a pas d'article.

**Comment l'appliquer :** deux correctifs, et c'est une décision propriétaire, pas
un correctif d'agent. (1) Retirer les 4 lignes → 6 contrôles, et passer
`TestExpectedControlCounts` de 10 à 6. (2) Re-citer vers un texte réel : **COSO
2013, Principe 11** (« Selects and Develops General Controls over Technology »)
ou **PCAOB AS 2201**, tous deux opposables. L'option 2 est meilleure sur le fond
— les ITGC sont ce que l'auditeur SOX teste réellement — mais exige la lecture du
texte COSO ou PCAOB. Voir [[available-sans-trace-de-validation]].
