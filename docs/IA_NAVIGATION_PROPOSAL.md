# OpenRisk — Proposition d'architecture d'information (navigation)

> **Statut : proposition. À valider par le fondateur avant toute implémentation (session 5).**
> Contrainte directrice : la navigation doit refléter le **chemin naturel de
> l'utilisateur** — *identifier → évaluer → traiter → prouver* — pas l'arborescence
> technique du code. Cible : ≤ 5 intentions de premier niveau (UX-16).

---

## 1. Inventaire de l'existant

Source : `frontend/src/shared/navModel.ts`. **7 groupes · ~20 entrées** de premier
niveau, dont **4 placeholders** `soon:true` et plusieurs doublons d'intention.

| Groupe | Entrées | Remarque |
|--------|---------|----------|
| `g_overview` | Dashboard, Analytics (exécutif), Financier, **Leaderboard** `soon` | 3 vues « de haut » qui se chevauchent |
| `g_security` | Risques, Vulnérabilités, Mitigations, Incidents, Automatisation, **Infrastructure** `soon` | mélange *identifier* (vulns/infra) et *traiter* (mitigations/incidents) |
| `g_intel` | Conformité, Intel Threat (CTI), **Simulations** `soon` | Conformité (*prouver*) mal rangée avec du *renseignement* |
| `g_assets` | Actifs, **Universe** `soon` | 1 vraie entrée + 1 placeholder |
| `g_report` | Rapports, Assistant IA, Risques émergents (IA) | 2 entrées IA séparées |
| `g_admin` | Gouvernance, Rôles & accès, Paramètres | utilitaire |

**Doublons / dispersion constatés :**
- **3 « vues de haut »** (Dashboard, Analytics exécutif, Financier) éclatées → l'utilisateur ne sait pas laquelle est *sa* page.
- **IA en 2 entrées** (Assistant `/recommendations`, Émergents `/ai/emerging-risks`) + un onglet IA dans le drawer de risque → l'IA est partout et nulle part.
- **Conformité** (le cœur du persona Awa) enterrée dans « Intel ».
- **4 placeholders** (`Leaderboard`, `Infrastructure`, `Simulations`, `Universe`) au même rang visuel que des features réelles → promesse creuse (OR-BUG-007, UX-05).
- `Rôles & accès` (`/settings/roles`) est **à la fois** une entrée de nav **et** un onglet de Paramètres (`rbac`) → deux portes pour la même pièce.

---

## 2. Cible — 5 intentions

Ordonnées selon le parcours GRC réel. Chaque intention = un « espace », lu de gauche
à droite / de haut en bas dans l'ordre du travail.

| # | Intention | Question de l'utilisateur | Contenu |
|---|-----------|---------------------------|---------|
| **0** | **Piloter** | « Où en suis-je ? » | Tableau de bord **par rôle** (RSSI/Analyste/Auditeur/Direction, UX-24), fusionnant Dashboard + Exécutif + Financier en une vue à densité réglable. |
| **1** | **Identifier** | « Qu'est-ce que je possède et qu'est-ce qui me menace ? » | Actifs (+ dépendances/Universe en divulgation progressive), Vulnérabilités, Intel Threat (CTI), Scanner d'infrastructure. |
| **2** | **Évaluer** | « Quel est mon risque, en clair et en argent ? » | Registre des risques (cycle de vie ISO 31000, quantification, pondération smart), Simulations. |
| **3** | **Traiter** | « Que fais-je pour réduire ? » | Mitigations, Incidents / War Room, Automatisation (SOAR). |
| **4** | **Prouver** | « Comment je le démontre à un régulateur ? » | Conformité (référentiels, gap-analysis, audits, remédiations), Rapports (Board, PDF, IA), Gouvernance (piste d'audit, approbations, délégations). |

**Hors des 5 intentions** — utilitaire, en pied de sidebar, jamais compté comme une
« intention » : **Paramètres** (dont Rôles & accès, Facturation, Intégrations) et le
menu de compte. **Recherche universelle ⌘K** (UX-22) reste transverse en entête.

Résultat : **5 espaces** (0–4) + 1 zone utilitaire → conforme à UX-16 (≤ 7, ici 5).

---

## 3. Table de correspondance ancien → nouveau

| Ancienne entrée | Ancien groupe | → Nouvelle intention | Traitement |
|-----------------|---------------|----------------------|------------|
| Dashboard | Overview | **0 Piloter** | **fusionne** (vue par défaut, par rôle) |
| Analytics (exécutif) | Overview | **0 Piloter** | **fusionne** (onglet/densité « Direction ») |
| Financier | Overview | **0 Piloter** | **fusionne** (onglet « Argent ») |
| Leaderboard `soon` | Overview | **0 Piloter** | **divulgation progressive** (gamification, plus tard) |
| Actifs | Assets | **1 Identifier** | déplace |
| Universe `soon` | Assets | **1 Identifier** | **fusionne** dans Actifs (vue « carte ») |
| Vulnérabilités | Security | **1 Identifier** | déplace |
| Intel Threat (CTI) | Intel | **1 Identifier** | déplace |
| Infrastructure `soon` | Security | **1 Identifier** | déplace (Scanner) |
| Risques | Security | **2 Évaluer** | déplace (cœur) |
| Simulations `soon` | Intel | **2 Évaluer** | déplace |
| *(pondération, import)* | (sous-routes) | **2 Évaluer** | divulgation progressive (dans Risques) |
| Mitigations | Security | **3 Traiter** | déplace |
| Incidents | Security | **3 Traiter** | déplace |
| Automatisation | Security | **3 Traiter** | déplace |
| Conformité | Intel | **4 Prouver** | déplace (remonte en visibilité) |
| *(gap, audits, remédiations)* | (sous-routes) | **4 Prouver** | regroupe sous Conformité |
| Rapports | Report | **4 Prouver** | déplace |
| Assistant IA | Report | **4 Prouver** | **fusionne** avec Émergents en un « IA » |
| Risques émergents (IA) | Report | **4 Prouver** ou contextuel | **fusionne** dans IA / contextualise dans Risques |
| Gouvernance | Admin | **4 Prouver** | déplace (preuve = gouvernance) |
| Rôles & accès | Admin | **Utilitaire** | **fusionne** dans Paramètres (supprime le doublon nav) |
| Paramètres | Admin | **Utilitaire** | pied de sidebar |

### Ce qui fusionne
- **Piloter** = Dashboard + Exécutif + Financier (3 → 1, par rôle + onglets).
- **Actifs** absorbe **Universe** (liste ⇄ carte).
- **IA** = Assistant + Émergents (2 → 1).
- **Rôles & accès** rentre dans **Paramètres** (supprime la double porte).

### Ce qui passe en divulgation progressive
- **Leaderboard / gamification** : masqué tant que non livré (plus de placeholder au 1ᵉʳ niveau).
- **Pondération smart-risk, Import** : sous-actions du Registre, pas des entrées de nav.
- **Universe (graphe)** : bascule de vue dans Actifs.

### Ce qui disparaît de la navigation (pas du produit)
- Les **4 entrées `soon:true`** au premier niveau (repliées ou masquées) — fin de la promesse creuse (OR-BUG-007).
- Le **doublon** `Rôles & accès`.

---

## 4. Nombre de clics — parcours principal avant / après

**Parcours de référence** (le chemin GRC canonique, = les 4 intentions dans l'ordre) :
*constater une vulnérabilité critique → l'évaluer en risque → planifier une mitigation
→ prouver la couverture dans un référentiel.*

| Étape | Avant (7 groupes, dispersion) | Après (5 intentions ordonnées) |
|-------|-------------------------------|--------------------------------|
| Trouver Vulnérabilités | groupe *Security*, 4ᵉ item — repérage | **1 Identifier** (1 clic) |
| Passer au Risque | autre écran, *Security* — retrouver | **2 Évaluer** (1 clic, voisin de droite) |
| Planifier une Mitigation | *Security* encore, item séparé | **3 Traiter** (1 clic) |
| Prouver en Conformité | change de groupe (*Intel*) — repérage | **4 Prouver** (1 clic) |
| **Total (navigation)** | **~7 clics** + 3 changements de groupe + repérage visuel | **4 clics**, ordre linéaire gauche→droite, 0 repérage |

**Gain : 7 → 4 clics** sur le parcours principal, et surtout **0 saut de contexte** :
les intentions sont dans l'ordre du travail, donc la prochaine étape est toujours la
voisine (satisfait UX-07 « toujours guider vers l'action suivante »).

---

## 5. Impacts et garde-fous
- **UX-16** : 5 espaces (< 7). ✅
- **UX-24** : « Piloter » est *le* point de différenciation par rôle — chaque rôle métier y atterrit sur sa vue.
- **UX-31** : chaque intention se rattache à la fonctionnalité principale nommée dans `ROADMAP.md` ; toute future entrée doit se ranger dans l'une des 5.
- **Migration douce** : conserver toutes les routes actuelles (les 11 redirections existantes montrent que c'est déjà la pratique) ; seule la **présentation** de la sidebar change. Aucune URL cassée.
- **À trancher par le fondateur** : (a) garder Financier comme onglet de Piloter ou comme espace à part pour le CFO ? (b) « IA » = espace transverse ou capacité contextuelle dans chaque écran ? (c) Gouvernance sous *Prouver* ou dans l'utilitaire ?
