---
name: iso-cis-aicpa-reproduction-verbatim
description: Cinq catalogues reproduisent le texte normatif de leur source tout en affirmant le contraire en en-tête — CIS v8 est le cas le plus net
metadata:
  type: project
---

Constat du 2026-10-30, à router vers `legal-counsel` : cinq catalogues portent en
en-tête l'affirmation que leurs descriptions sont « original summaries […] not
the standard's own text », alors qu'une part substantielle est l'énoncé de la
source **dé-modalisé** (le passif « X *shall be* Y-ed » devient l'impératif
« Y X »), voire recopié sans transformation.

| Catalogue | Nature | Gravité |
|---|---|---|
| `cis-v8` | Descriptions = descriptions officielles des Controls CIS v8. **Licence CC BY-NC-SA 4.0 : la clause NC est incompatible avec `LICENSE.commercial`, la clause SA contamine.** | **La plus nette — termes publics et explicites** |
| `soc2-tsc` | CC1.1–CC1.5 au moins = énoncés AICPA TSC, eux-mêmes identiques aux principes COSO 2013 | Haute |
| `iso31000-2018` | Les 8 énoncés de principes (clause 4) recopiés sans transformation | Haute |
| `iso27001-2022` | A.7.1-7.4, A.7.6, A.8.6, A.8.17, A.8.19, A.8.22, A.8.23, A.8.28, A.8.31… dé-modalisés ; les 93 intitulés repris mot pour mot | À confirmer sur copie licenciée |
| `iso22301-2019` | Descriptions FR qui suivent la traduction AFNOR, « doit » retiré | À confirmer sur copie licenciée |

**Pourquoi :** une dé-modalisation n'est pas une reformulation, c'est une œuvre
dérivée. Et l'affirmation de provenance inexacte **inscrite dans le dépôt par ses
auteurs** aggrave la position en cas de litige — elle est à corriger même si
l'usage s'avère licite.

**Comment l'appliquer :** traiter CIS v8 en premier (seul cas où la licence est
publique, écrite et non négociable) ; ne rien réécrire en masse sur l'ISO avant
confirmation contre une copie **licenciée** — mon analyse ISO s'appuie sur ma
connaissance des normes, pas sur une copie en main, et il faut le dire à chaque
fois. Réécrire une description = partir de l'**intention** du contrôle, pas de sa
phrase. Voir [[frameworks-verified-2026-10-30]].
