# OpenRisk — Plan de lancement 2026 (carte de référence unique)

> **Établi le 2026-08-14.** Cible : **produit prêt le 1er décembre 2026**, préparation marketing en
> décembre, **lancement commercial janvier 2027**. Source : Master Prompt V5, `ROADMAP.md`,
> et les trois prompts de campagne (`PROMPT_AUDIT_UX_PRODUIT`, `PROMPT_CLAUDE_RÉSOLUTION`,
> `PROMPT_UX_IMPLÉMENTATION`) + grille tarifaire (`fixes-plan.md`).
>
> **Objectif produit non négociable** : le meilleur GRC francophone/africain, rivalisant avec
> Vanta/Drata/OneTrust, avec une **UX parfaite** et les **fonctions cybersécurité indispensables**.
> **Anti-objectifs (règles de discipline)** : pas de dispersion, pas de qualité inégale, pas de
> feature bâclée, pas de sous-estimation de charge, **zéro bug critique en production**.

---

## 0. Instantané d'état (mise à jour à chaque phase)

- **P0 — Intégration & vérité de base : ✅ FAIT (2026-08-14).** Les 16 branches ont été intégrées
  dans `origin/master` via PRs **#168→#185**. `origin/master` = `a06982a`. Arbre **identique au tip
  validé vert** : backend `go build`/`go vet`/`go test ./...` **0 FAIL**, frontend `tsc -b`/`vite build`
  verts. Master local synchronisé, branches redondantes supprimées. → `master` reflète la réalité.
- **P0.5 — Migrations : ❌ dette ouverte (non bloquante boot, bloquante prod).** Voir §2.
- **Phases suivantes : à faire**, dans l'ordre ci-dessous.

---

## 1. Verdict & les 3 dettes structurelles

OpenRisk a **plus de surface fonctionnelle construite que la plupart des GRC financés** (Score/Risk/
Mitigation/Compliance 16 référentiels/Assets+dépendances/CRQ FAIR/Smart Score/SOAR/Gouvernance/
Scanner+Agent/Vuln management/IA 5 capacités/Exec dashboard). Le risque de lancement **n'est pas** un
manque de features — c'est l'inverse. Trois dettes menacent exactement les anti-objectifs :

1. **Dette d'intégration** — *résorbée par P0.* (Historiquement : 16 branches, master local périmé.)
2. **Dette de vérité (« faux ✅ »)** — la majorité des modules est prouvée par `tsc`/`vite`/endpoint
   headless, **rarement par un vrai navigateur sur base vierge**. Inscription/MFA soupçonnés d'être
   des façades (P0 audit interne). **Instrument de résolution : audit UX live (Phase 1).**
3. **Dette commerciale** — **Billing ❌, Onboarding backend ❌, Super-Admin ❌, Landing/Pricing ❌,
   round-trip OAuth réel ❌.** On ne lance pas un SaaS sans ça. **Résolution : Phase 5.**

**Conclusion : geler l'ajout de features Wave-2/3, et consolider → prouver → construire le commercial.**

---

## 2. Le programme (aujourd'hui → prêt 01/12 → lancement janvier ≈ 15 semaines)

| Phase | Sem. | Objectif | Instrument |
|---|---|---|---|
| **P0 — Vérité de base** | ✅ | `master` = réalité, suite verte. | *fait* |
| **P0.5 — Migrations** | 1 | Rail SQL canonique reproductible (voir ci-dessous). | *prompt §4* |
| **P1 — Audit UX live** | 1 | Rejouer le produit au navigateur, base vierge, 34 parcours, 4 personas → registre S1–S4. **Ne rien coder.** | `PROMPT_AUDIT_UX_PRODUIT.md` |
| **P2 — Lot 0 sécurité & chiffres** | 2 | Fuites cross-tenant restantes, score 100/100 sur vide, `mitigated` case-fold, sérialisation Notification, **OAuth round-trip réel**, state store Redis. | `PROMPT_CLAUDE_RÉSOLUTION.md` §1–2 |
| **P3 — Auth prouvée live** | 1,5 | MFA + OAuth2 + SAML2 + refresh + switch-org **pilotés bout-en-bout**. | *prompt §4* |
| **P4 — Implémentation UX (Lots 1–7)** | 4 | Thème/a11y, nav & recherche, onboarding < 8 min, édition fantôme/autosave, conformité↔rapports, notifs/radiographie, polish. | `PROMPT_UX_IMPLÉMENTATION.md` |
| **P5 — Couche commerciale** | 3 | **Billing Stripe + Mobile Money** + gating de plan + Onboarding backend + Super-Admin + **Landing & Pricing**. | *prompt §4* |
| **P6 — Durcissement lancement** | 1,5 | `/metrics`, `/ready`/`/deep`, load test, DR, checklist pré-prod V5, canary E2E hebdo, code-splitting (bundle 3 Mo). | V5 §6 |

**Chemin critique = P0.5 → P1 → P2/P3.** P5 (commercial) peut démarrer en parallèle de P4 dès P3 verte.

### P0.5 — Consolidation des migrations (dette précise identifiée)
Le rail SQL est **mort silencieusement** : `RunMigrations()` lit `file://migrations` (dossier racine
**lacunaire**, saute 0026→0047) ; le vrai `backend/migrations/` (0026–0047) n'est jamais lu ; les
fichiers 0001–0024 sont des `.sql` nus refusés par golang-migrate ; et l'erreur est **avalée**
(`log.Printf`). → `AutoMigrate` (GORM) est de facto la **seule autorité de schéma** ; les backfills de
données et le trigger append-only de l'audit **n'ont jamais tourné**. Non bloquant pour le boot,
**bloquant pour l'intégrité prod multi-tenant**.

---

## 3. Gel Wave-2/3 jusqu'au lancement (décision 2026-08-14) — anti-dispersion

**Gelés jusqu'à février+** (hors chemin critique de vente, réouverts un par un post-lancement, chacun
prouvé live) : Digital Twin (14.13), Attack Path (14.15), Vendor (14.2), Policy (14.3), Trust Center
(14.4), BCP (14.6), Training (14.7), Access Review (14.10), Data Discovery (14.11), Offline (14.17),
Marketplace complet (14.18). *Un moat ne sert à rien autour d'un château qui ne sait pas encaisser un
paiement.*

---

## 4. Prompts par phase

### P0.5 — Migrations
> Reconstruire **un** jeu golang-migrate canonique (un seul dossier, `.up/.down`, séquentiel, sans trou
> ni doublon) reproduisant le schéma `AutoMigrate` **+** les backfills de données **+** le trigger
> append-only audit. Câbler `RunMigrations` pour **échouer bruyamment** (plus d'erreur avalée). Tester
> `migrate up`/`down` sur une base Postgres fraîche (docker-compose). Livrable : `docs/MIGRATIONS.md`
> (autorité de schéma, procédure, réconciliation des 2 dossiers).

### P3 — Auth prouvée live
> Prouver, dans un vrai navigateur + tests d'intégration, la chaîne 7 couches de bout en bout :
> inscription réelle (pas une façade), login, MFA TOTP + code de secours, OAuth2 (Google/GitHub/Entra
> avec PKCE), SAML2, rotation refresh-token (rejeu → 401), switch-org. Un test d'intégration + une
> capture Playwright par flux. Si un flux est une façade → l'implémenter. Livrable :
> `docs/audit/AUTH_LIVE_2026-08.md` (verdict par couche).

### P5 — Couche commerciale
> Construire la couche go-to-market. (1) **Billing** : plans Free/Pro/Business/Enterprise (`fixes-plan.md`,
> tarifs EU **et** XAF/XOF), `Subscription`/`Plan`/`UsageCounter` tenant-scoped, **Stripe** (checkout +
> webhooks signés + portail) **et** Mobile Money (Wave/MTN/Orange), middleware `CheckPlanLimits`
> (users/risques/assets/intégrations). (2) **Gating réel** serveur (remplace l'upsell cosmétique
> `UpsellLock`) : aperçu flouté + bénéfice chiffré. (3) **Onboarding backend** : persister
> org/secteur/pays/devise, wizard reprenable, référentiels suggérés par secteur/pays. (4)
> **Super-Admin** : tenants, impersonation auditée, métriques globales. (5) **Landing + Pricing**
> publiques. Chaque écran prouvé live + test. Jamais de secret de paiement en logs ; webhooks signés.

*(P1, P2, P4 : voir les fichiers `PROMPT_*` déjà rédigés à la racine du projet.)*

---

## 5. Règles d'exécution (Master Prompt V5 — Partie A, non négociables)

- **Sécurité** : filtrer `tenant_id` sur **chaque** query (repo, pas handler) ; objet d'un autre tenant
  → **404** ; JWT **RS256** ; secrets jamais loggés ; credentials **AES-256-GCM** ; `admin_audit_events`
  **APPEND-ONLY** (trigger PG).
- **Architecture** : erreurs typées ; transactions multi-table ; Score Engine appelé **via event Redis**,
  jamais depuis un handler ; **lire tous les fichiers d'un module avant d'écrire**.
- **Qualité/UX** : zéro `any` ; Zod sur les formulaires ; tests min. par use case
  (Success/NotFound/Unauthorized) ; skeletons (pas de spinner plein écran) ; 3 états
  (loading/error/empty) ; optimistic updates.
- **Méthode/module** : LIRE → PLANIFIER → IMPLÉMENTER (backend puis frontend + tests) → **VALIDER LIVE**
  → COMMITER. Une branche par feature, PR vers `master` (le flow du dépôt). **Aucun ✅ sans preuve live.**

---

## 6. Preuve de lancement (P6 — checklist)

`/metrics` Prometheus servi · `/ready` + `/deep` · `AUDIT_EXPORT_KEY` configurée · load test
(`load_tests/`) · plan DR · canary E2E `onboarding-canary.spec.ts` hebdo (échec si Aha > 10 min) ·
bundle front code-splitté. Détail : Master Prompt V5 §6 « Checklist avant mise en production ».
