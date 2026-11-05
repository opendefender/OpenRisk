---
name: frameworks-verified-2026-10-30
description: Trace de validation conformité du 2026-10-30 — quels référentiels sont vérifiés article par article, contre quel document source, et à quel niveau
metadata:
  type: project
---

Premier audit `/compliance-audit` complet du produit, sur `master` `5d4e5b89`,
le **2026-10-30**. C'est la première trace de validation conformité du dépôt :
`docs/audit/` n'en contenait aucune et ce répertoire de mémoire était vide.

| Référentiel | Niveau atteint | Document source utilisé |
|---|---|---|
| `bceao` (35) | **article par article, 35/35** | PDF **officiels BCEAO** : Règlement n°15/2002/CM/UEMOA (248 art.), Instruction n°008-05-2015, Instruction n°009/07/RSP/2010, Instruction n°001-03-2021, Avis n°001-09-2012 (Recueil ch. V, p. V-76) |
| `antic-cm` (25) | **article par article, 25/25** | PDF **officiel ART Cameroun** de la Loi n°2010/012 du 21/12/2010 (97 art.) |
| `cobac` (45) | **article par article, 45/45** mais **source secondaire** | `basereglementaire.com` (reproduction article par article). Le PDF officiel COBAC/BEAC est **inaccessible** : sgcobac.org renvoie du HTML, beac.int renvoie 404. **Reste à confirmer contre le texte officiel.** |
| `iso27001-2022` (93) | **structure exacte** vérifiée ; formulation **non** vérifiée | Aucune copie licenciée d'ISO n'existe dans le projet |
| 13 autres catalogues (359 contrôles) | structure/codes/décomptes uniquement | — |

**Résultat : `COMPLIANCE: FAIL — 4 unsourced, 0 fabricated`.** Zéro article
inventé sur les 465 éléments examinés : les mentions « codes d'article fiables »
des en-têtes de `cobac`/`bceao`/`antic-cm` sont exactes.

Les 4 `UNSOURCED` sont les contrôles ITGC de `sox-2002`. Voir
[[sox-itgc-domains-unsourced]], [[bceao-surveillance-instruction-abrogee]],
[[articles-difficiles-a-resoudre]], [[iso-cis-aicpa-reproduction-verbatim]],
[[available-sans-trace-de-validation]].

**Comment l'appliquer :** avant de re-vérifier un de ces référentiels, lire
d'abord cette trace — ne pas refaire la lecture des 105 articles déjà faite.
Re-vérifier en priorité : COBAC contre le PDF officiel, et la formulation de
`soc2-tsc` (51) / `hipaa-security` (22) / `nist-800-53-r5` (20), jamais vérifiée.
