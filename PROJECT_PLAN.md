# OpenRisk — Plan d'Exécution (le « système d'exploitation » du projet)

> **Rôle de ce document.** Il définit **COMMENT** exécuter. Le **QUOI / QUAND** vit exclusivement dans `ROADMAP.md` (source de vérité unique des vagues et jalons).
> **Documents de pilotage vivants = deux seulement : `PROJECT_PLAN.md` (ce fichier) + `ROADMAP.md`.** Tout autre `.md` de planification est archive (`/docs/archive/`). Règle anti-bloat : aucun nouveau document stratégique ne se crée tant qu'un jalon n'est pas livré et prouvé.

---

## 0. Cadre — Vision vs Exécution (à ne jamais confondre)

| | Contenu | Horizon | Rôle |
|---|---|---|---|
| **North Star** | Leader GRC open-source ; niveau Vanta/Drata/ServiceNow ; COBAC/BCEAO/ANSSI puis mondial | 3 ans | Destination. Jamais un scope de sprint. |
| **Focus** | #1 GRC francophone africain sur **UN parcours bout-en-bout** : Compliance engine + contenu COBAC/BCEAO + reporting officiel FCFA | 12 mois | Ce qui se construit *maintenant*. |
| **Assumé « later »** | NIS2, DORA, PCI-DSS, global, modules « waouh » (Digital Twin, War Room, Attack Path) | > 12 mois | Écrit, non négociable, gelé. |

**Loi de scope.** L'ambition se prouve par la **profondeur d'un créneau**, pas par la largeur des référentiels. Aucune décision de scope ne peut contredire `ROADMAP.md`. Un référentiel ajouté qui divise l'énergie sans multiplier les clients est une régression, pas un progrès.

---

## 1. Charte d'ingénierie (verrouillée — non négociable)

Reprend `CLAUDE.md` (règles 1→11) et sert de référence à toute PR.

- **Backend** : Go 1.25, Clean Architecture stricte. Domaine pur (zéro Fiber/GORM). `tenant_id` filtré **au repository** sur chaque query. Erreurs typées uniquement (`ErrNotFound`, `ErrForbidden`, `ErrConflict`, `ErrValidation`). Transactions sur toute opération multi-table.
- **Frontend** : React 19, TypeScript strict, **zéro `any`**. 3 états UI obligatoires (loading skeleton / error / empty). Optimistic updates sur toute mutation critique. Zod sur tous les formulaires. Client API **généré depuis OpenAPI** (contract-first).
- **Sécurité** : JWT **RS256** (jamais HMAC). AES-256-GCM câblé sur **tous** les champs sensibles (credentials, secrets MFA, PII) ; clé via env/KMS, jamais en dur. Aucun secret dans les logs.
- **Naming** : « **OpenRisk** » partout (code, UI, copy, OpenAPI). Décision close — purger toute occurrence de « Karath ».
- **UI/UX** : minimalisme façon Apple/Google, animations fluides (respect `prefers-reduced-motion`), WCAG 2.1 AA. La beauté est un **multiplicateur** appliqué *après* la preuve de valeur — jamais avant.
- **Qualité produit** : tests minimum par use case → `TestXxx_Success` + `TestXxx_NotFound` + `TestXxx_Unauthorized` + **test cross-tenant (404)**. i18n FR/EN systématique.

---

## 2. Modèle opératoire Claude Code (la boucle de chaque session)

Toute session suit **5 étapes, dans l'ordre, sans en sauter une**. Deux STOP à HITL (human-in-the-loop) sont non contournables.

1. **READ** — Lire *tous* les fichiers du périmètre + points d'intégration (migrations, OpenAPI, Redis, PostgreSQL). Résumer l'état réel. **Ne coder aucune ligne.**
2. **PLAN** — Proposer un plan étape par étape (schéma DB → domaine → use cases → repo tenant-scoped → handlers → OpenAPI → frontend → tests) + risques. → **STOP #1 : attendre `GO plan`.**
3. **IMPLEMENT** — Backend d'abord, frontend ensuite. Un commit = un fix logique, message conventionnel en anglais, atomique et auditable.
4. **TEST** — Les 4 tests minimum par use case + build vert + `go vet` + lint.
5. **VERIFY & DOCUMENT** — CI locale verte ; OpenAPI + `ROADMAP.md` (statut **avec preuve**) + `CHANGELOG.md` à jour. → **STOP #2 : revue humaine avant merge.**

**STOP obligatoires (jamais franchis seul par l'agent) :** migration destructive/`drop`/altération de données ; tout ce qui touche auth/JWT/chiffrement/secrets ; **contenu réglementaire** (COBAC/BCEAO/ANSSI → validation conformité humaine) ; suppression de fichiers hors périmètre.

**Fencing** : chaque prompt de session **borne explicitement** les fichiers autorisés en écriture. Le reste est interdit ; tout besoin de sortir du périmètre déclenche un STOP + proposition.

---

## 3. Gate 0 — état vérifiable AVANT toute feature (tranché par la preuve, pas par supposition)

Objectif : un `master` qui compile vert avec CI bloquante, reproductible. Aucune feature ne démarre tant que ceci n'est pas ✅.

```bash
# Backend
go build ./...          # doit sortir 0
go vet ./...            # doit sortir 0
go test ./... -race     # vert
# Frontend
npm ci && npm run build && npm run typecheck   # zéro any, zéro erreur TS
```

**CI bloquante (GitHub Actions)** : `build` · `vet` · `test -race` · `gosec` · `govulncheck` · `npm audit` · `gitleaks` · `typecheck`. Un check rouge bloque le merge, sans exception.
**Acceptation** : pipeline vert sur `master`, reproductible par un clone frais.

---

## 4. Séquence d'exécution (les gates ; le détail des modules reste dans `ROADMAP.md`)

L'ordre est **non négociable**. On ne franchit une porte de sortie qu'avec preuve (CI verte + revue humaine).

| Vague | Porte d'ENTRÉE | Contenu (voir `ROADMAP.md`) | Porte de SORTIE |
|---|---|---|---|
| **Gate 0** | — | Build vert + CI bloquante | §3 ci-dessus ✅ |
| **Wave 0** | Gate 0 ✅ | Licence cohérente · schéma Compliance (fondations tenant-scoped) · AES câblé · README honnête · CI sécu | DoD §5 sur chaque item |
| **Wave 1** | Wave 0 ✅ | **M1 Compliance engine → M2 contenu africain → M3 Assets → M4 Reporting/Board → M5 Incident → M6 Offline → M7 polish 3 écrans** (ordre imposé) | **Jalon majeur** : un responsable conformité CEMAC/UEMOA monte seul un programme COBAC/ISO et produit un rapport défendable, bout-en-bout, dans l'UI |
| **Wave 2** | Wave 1 ✅ | 7 différenciateurs (Content-as-Code, Audit Defensibility, AI Control-Mapping, CRQ/FAIR FCFA, RAG…) | par ratio valeur/effort, DoD §5 |
| **Wave 3** | Wave 2 ✅ | Plateforme + Billing/Stripe + Regulator Portal + Benchmarking | DoD §5 |

**Règle anti-dérive (la plus importante).** Aucun module « waouh » ne démarre tant que **M1 + M2 ne sont pas ✅**. Le créneau d'abord. C'est là que se gagne « imbattable » : la profondeur sur *un* parcours, pas 12 modules à moitié faits.

**Fin de Wave 1 = seulement là** on approche 3 institutions pilotes (banque moyenne / microfinance / fintech). Pas avant : la GRC ne s'achète pas à un produit sans parcours complet.

---

## 5. Definition of Done — checklist par module (à coller dans chaque PR)

```
[ ] CI verte : build · vet · test -race · gosec · govulncheck · gitleaks · npm audit · typecheck
[ ] Isolation multi-tenant PROUVÉE par test (404 cross-tenant)
[ ] Aucun secret en clair (logs + base de données)
[ ] Erreurs typées uniquement ; transactions sur opérations multi-table
[ ] Frontend : zéro any · 3 états UI · optimistic update · Zod
[ ] i18n FR/EN complet
[ ] OpenAPI à jour + client typé régénéré (contract-first)
[ ] (si conformité) contenu réglementaire cité à la source + validation humaine tracée
[ ] ROADMAP.md (statut + PREUVE) et CHANGELOG.md à jour
```
Un seul item non coché ⇒ statut **🟡 Partiel**, jamais ✅. **Jamais ✅ sans preuve. Aucun fichier d'auto-félicitation.**

---

## 6. Template de session Claude Code (copier-coller au début de CHAQUE session)

```md
# SESSION CLAUDE CODE — <MODULE / JALON>  (réf. ROADMAP.md §<x>)

## Contexte
Tu es Lead Principal Engineer sur OpenRisk. Respecte CLAUDE.md (règles 1→11) et
PROJECT_PLAN.md (charte §1, DoD §5). Nom du produit : « OpenRisk » partout.

## Périmètre AUTORISÉ en écriture (fencing strict)
- <lister fichiers/dossiers>
Tout le reste est INTERDIT en écriture. Besoin d'en sortir → STOP + proposition.

## Objectif (Definition of Done vérifiable)
- <acceptation observable, ex : « un tenant suit un référentiel bout-en-bout dans l'UI »>

## Protocole — dans l'ordre, sans sauter d'étape
1. READ    — Lis TOUT le périmètre + intégrations (migrations, OpenAPI, Redis, PG).
             Résume l'état réel. Ne code rien.
2. PLAN    — Plan étape par étape (DB → domaine → use cases → repo tenant-scoped →
             handlers → OpenAPI → frontend → tests) + risques. STOP : attends « GO plan ».
3. IMPLEMENT — Backend d'abord. 1 commit = 1 fix logique, message conventionnel EN.
             Zéro any · tenant_id sur chaque query · erreurs typées.
4. TEST    — Success + NotFound + Unauthorized + cross-tenant (404). Build vert + vet.
5. VERIFY  — CI locale verte ; OpenAPI + ROADMAP.md (preuve) + CHANGELOG.md à jour.
             STOP : revue humaine avant merge.

## STOP obligatoires (jamais seul)
migration destructive · auth/JWT/chiffrement/secrets · contenu réglementaire · suppression hors périmètre
```

---

## 7. Rituel hebdomadaire (fondateur solo)

- **Lundi (15 min)** — Un seul module « en vol » à la fois. Vérifier la porte d'entrée de la vague. Écrire le fencing de la session.
- **Vendredi (30 min)** — Démo du parcours, DoD §5 cochée, merge, mise à jour `ROADMAP.md` + `CHANGELOG.md` avec preuve. **Check anti-dérive** : ai-je touché à autre chose que le module en cours ? Si oui → corriger la trajectoire.
- **Interdit permanent** — Reprendre le méta-travail (nouveaux Master Prompts, agents marketing) tant que Wave 1 n'est pas ✅. L'infra multi-agent se dégèle *après* produit + pilote.

---

*Ce plan est l'« OS » d'exécution. Il ne remplace pas `ROADMAP.md` — il l'exécute proprement. Deux fichiers vivants, zéro bloat, chaque ✅ prouvé.*
