---
name: available-sans-trace-de-validation
description: Les 17 catalogues sont Available:true alors que leurs propres en-têtes disent que la revue conformité n'a pas eu lieu
metadata:
  type: project
---

`backend/pkg/compliance/catalog.go:38` définit le contrat : « Flip [`Available`]
to true once a compliance-competent reviewer has verified the content against the
actual regulatory text ». La skill `compliance-audit` ajoute : « the sign-off
records the date and the source document used ».

Au 2026-10-30, **les 17 catalogues disponibles portent dans leur propre en-tête la
mention que cette revue n'a pas eu lieu** (« À valider par un professionnel avant
usage en audit réel », « has not yet had a dedicated compliance-expert review
pass »). Aucun enregistrement de validation n'existait dans le dépôt.

**Pourquoi c'est la cause racine et pas un détail :** `Available` est le seul
garde-fou entre un catalogue non revu et un rapport de conformité présentable à
un régulateur. Tant qu'il peut être mis à `true` par le même commit qui écrit les
contrôles, il n'atteste rien. Le modèle `Withdrawn` de `cm-loi-2024-017` (D-002)
montre la bonne forme : un état explicite, un motif, un ticket de suivi, des
tests.

Deux conséquences constatées : les décomptes 45/35/25 de `cobac`/`bceao`/`antic-cm`
sont absents de `TestExpectedControlCounts` (donc non verrouillés, alors qu'ils
figurent dans le README et la matrice de claims) ; et `sox-2002` est `Available`
avec 4 contrôles sans source opposable — voir [[sox-itgc-domains-unsourced]].

**Comment l'appliquer :** demander un champ de validation sur `Catalog` (date,
document source, relecteur) plutôt qu'un simple booléen, et refuser de monter
`Available` sans lui. Et noter que `docs/MARKETING_CLAIM_MATRIX.md` (ligne C-005,
`PARTIAL`) et le README sont **honnêtes** sur cette limite : l'écart est interne
au code, pas dans la communication client. Ne pas le présenter comme un surclaim
marketing. Voir [[frameworks-verified-2026-10-30]].
