# OpenRisk — Charte UX (Annexe A)

> Référence permanente, à citer dans chaque PR au même titre que `CLAUDE.md`.
> Chaque règle conserve son identifiant `UX-xx` et son **critère d'acceptation vérifiable**.
> Cible de qualité : une console AWS ou une app Google — un utilisateur qui n'a
> jamais fait de cybersécurité doit réussir seul.
>
> Statut de vérification : les critères marqués sont mesurés par la suite E2E
> (`tests/e2e/`) ; l'état constaté au 2026-07-24 est consigné dans
> [`UX_AUDIT_2026-07.md`](UX_AUDIT_2026-07.md). Cette charte est la cible, pas l'état.

## A. Activation et onboarding

| ID | Règle | Critère d'acceptation |
|----|-------|-----------------------|
| **UX-01** | L'onboarding fait accomplir, il ne montre pas. Il conduit à une première action réelle qui prouve la valeur. | Un nouvel utilisateur atteint sa première action de valeur (risque créé / contrôle évalué) en **< 10 minutes**, mesuré par le test E2E `journey.newcomer`. |
| **UX-02** | L'inscription demande le strict minimum. La qualification vient après. | Le formulaire d'inscription ne contient **pas plus de 3 champs**. |
| **UX-13** | Le premier engagement précède l'onboarding : on crée le compte, puis on onboarde. | **Aucune question de qualification** n'est posée avant la création du compte. |
| **UX-14** | Pas de visite guidée. Aide contextuelle à la première rencontre d'un élément. | Aucun composant de « product tour » séquentiel. Les infobulles apparaissent au premier survol et **ne se répètent pas**. |
| **UX-17** | Personnalisation de l'espace après l'Aha moment (logo, thème, couleur). | La proposition de personnalisation **n'apparaît qu'après** la première action de valeur. |

## B. États et retours

| ID | Règle | Critère d'acceptation |
|----|-------|-----------------------|
| **UX-03** | Un message d'erreur dit ce qui s'est passé, pourquoi, et quoi faire ensuite. | **Zéro message générique** (« une erreur est survenue », « échec »). Chaque erreur propose une action. |
| **UX-04** | Un écran vide donne envie d'agir : exemple, suggestion, bouton. | Chaque état vide contient **au moins une action primaire**. Vérifié en E2E sur tous les écrans de liste. |
| **UX-05** | Aucun élément interactif inerte. | **Zéro bouton/lien sans effet.** Un élément indisponible est désactivé avec une infobulle explicative. |
| **UX-08** | Toute action confirme visuellement son résultat. | Retour visible en **< 100 ms** ; succès et échec sont distinguables sans lire le texte. |
| **UX-09** | Le temps d'attente est exploité, pas subi. | Toute opération **> 1,5 s** affiche une progression informative (étape en cours, statistique utile). |
| **UX-23** | Autosave par défaut ; bouton « Enregistrer » seulement quand la validation est réellement nécessaire. | Aucun bouton d'enregistrement sur un champ de préférence simple. Indicateur « **Enregistré ✓** » présent. |
| **UX-25** | Historique des modifications consultable (time travel). | Chaque entité majeure expose son **historique daté et attribué**. |

## C. Navigation et charge cognitive

| ID | Règle | Critère d'acceptation |
|----|-------|-----------------------|
| **UX-06** | Édition fantôme : modifier une valeur simple ne fait jamais quitter l'écran. | Les modifications simples se font **en place (inline)**, sans navigation. |
| **UX-07** | L'interface guide toujours vers l'action suivante. | **Aucune fin d'étape** ne laisse l'utilisateur devant un écran sans suite proposée. |
| **UX-10** | La hiérarchie visuelle met en avant ce qui compte. | **Une seule action dominante** par écran, identifiable en < 3 s par un testeur naïf. |
| **UX-16** | Sidebar groupée par intention, 7 espaces maximum. | Le compte d'entrées de premier niveau est **≤ 7** ; regroupements justifiés dans `IA_NAVIGATION_PROPOSAL.md`. |
| **UX-22** | Recherche universelle : tout se trouve depuis une seule barre. | **⌘K** retourne des résultats pour risque, actif, contrôle, CVE, membre, rapport, audit. |
| **UX-26** | Raccourcis clavier pour les actions fréquentes, découvrables. | Une aide `?` liste les raccourcis ; les **5 actions principales** en ont un. |
| **UX-27** | Responsive de 360 px au 4K, sans perte de fonction. | Les tests E2E **Mobile Chrome** passent sur tous les parcours principaux. |

## D. Frictions et décisions

| ID | Règle | Critère d'acceptation |
|----|-------|-----------------------|
| **UX-11** | Radiographie d'impact avant toute action structurante : conséquences visibles + alternative proposée. | Les suppressions/rétrogradations d'utilisateur, de risque possédé ou de référentiel **affichent les objets liés** et proposent au moins une issue alternative (transfert, désactivation). |
| **UX-12** | Action mineure : exécution immédiate + toast « Annuler ». Pas de modale. | Suppression réversible pendant **≥ 7 s** ; aucune confirmation modale sur un élément mineur. |
| **UX-28** | Système 1 / Système 2 (Kahneman) : réflexe pour le fréquent et réversible, délibération pour le rare et irréversible. | **Chaque confirmation modale est justifiée par l'irréversibilité** ; toute autre est supprimée. |

## E. Notifications

| ID | Règle | Critère d'acceptation |
|----|-------|-----------------------|
| **UX-20** | Notifications catégorisées : Sécurité · Conformité · Tâches · Collaboration · Produit · Facturation. | Préférences réglables **par catégorie et par canal** (in-app / e-mail). |
| **UX-21** | Bon moment, bon volume : jamais de rafale, jamais de notification sans action possible. | Regroupement et **limite de fréquence** par catégorie ; chaque notification porte une action. |
| **UX-29** | Relance après inactivité prolongée et annonce de nouveauté, envoyées quand l'utilisateur est disponible. | Envoi calé sur le **fuseau et les heures d'activité** observées du destinataire. |

## F. Conversion

| ID | Règle | Critère d'acceptation |
|----|-------|-----------------------|
| **UX-18** | Une fonctionnalité payante se montre floutée avec son bénéfice, elle ne se bloque pas brutalement. | **Zéro message « vous avez atteint votre limite »** sans aperçu de ce qui est gagné. |
| **UX-19** | Trois moments de sollicitation, et seulement trois : après l'Aha moment, à l'atteinte d'une limite, après une victoire significative. | **Aucune sollicitation ailleurs** dans le parcours. |
| **UX-30** | Essai court, ou basé sur l'usage (loi de Parkinson). | Durée d'essai et **compteur d'usage explicités** dans l'UI à tout moment. |

## G. Fondations produit

| ID | Règle | Critère d'acceptation |
|----|-------|-----------------------|
| **UX-24** | Tableau de bord différencié par rôle métier (RSSI, Analyste, Auditeur, Direction). | Chaque rôle métier **atterrit sur un tableau de bord distinct** et pertinent. |
| **UX-31** | Une fonctionnalité principale, tout le reste la sert. | `ROADMAP.md` **nomme explicitement** cette fonctionnalité ; toute nouvelle entrée de navigation justifie son lien avec elle. |
| **UX-32** | Micro-victoires célébrées le long du parcours. | Chaque jalon (premier risque, premier contrôle, premier rapport) déclenche un **retour positif explicite**. |
| **UX-33** | L'onboarding est testé automatiquement au moins une fois par semaine. | Job CI **hebdomadaire** exécutant `journey.newcomer.spec.ts` avec alerte en cas d'échec. |
