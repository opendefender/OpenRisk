# Marketing Claim Matrix — OpenRisk

Every public statement about what OpenRisk does lives here with its proof.
Only the `product-verifier` agent may set a status to `VERIFIED`.

Status: `VERIFIED` · `PARTIAL` · `MOCKED` · `ABSENT` · `PLANNED`

| ID | Claim (EN) | Claim (FR) | Status | Evidence (file:line + test) | Surface | Verified on |
|----|-----------|-----------|--------|------------------------------|---------|-------------|
| C-001 | _seed row — run /verify-claims to populate_ | — | ABSENT | — | — | — |
| C-002 | A new organization reaches its first real risk insight — a cyber score on its own data with at least one compliance gap identified — in under 8 minutes. | Une nouvelle organisation atteint sa première information de risque réelle — un score cyber sur ses propres données avec au moins un écart de conformité identifié — en moins de 8 minutes. | PARTIAL | `tests/e2e/activation.spec.ts` (`AHA_BUDGET_MS = 8 * 60 * 1000`, assertion sur la durée totale signup → Aha) ; définition de l'Aha : `backend/internal/domain/activation.go` (`AhaSignal.IsAha()`) ; mesure : `openrisk_time_to_aha_seconds` (`backend/pkg/monitoring/activation.go`) | Site, docs produit | 2026-08-31 — non promu : la suite E2E n'a pas été exécutée dans cet environnement (pas de backend ni de frontend démarrés). `product-verifier` doit la lancer avant tout passage à `VERIFIED`. |
| C-003 | The onboarding checklist reflects what the organization has actually done — including work completed before the checklist existed. | La checklist d'onboarding reflète ce que l'organisation a réellement fait — y compris le travail accompli avant l'existence de la checklist. | PARTIAL | `backend/internal/infrastructure/repository/gorm_activation_backfill.go` + `gorm_activation_backfill_test.go` (7 tests : amorçage, idempotence sur redémarrage, absence de preuve = absence de coche, isolation multi-tenant) ; `go test ./internal/infrastructure/repository/ -run BackfillDerived` | Docs produit | 2026-08-31 — tests unitaires verts ; l'étape `profile` reste non dérivable (aucun enregistrement ne la prouve). |

## Blocking rule

`MOCKED` and `ABSENT` claims must not appear on any public surface.
`PLANNED` claims must be future tense and visually marked.

## Time to first value — one number

The committed promise is **8 minutes**, and it is the only number that may
appear on a public surface. Two other numbers exist in the tree and are NOT the
promise:

- **12 minutes** — `SlowTimeToAha` in `deployment/monitoring/alerts.yml`. An
  operational warning threshold, deliberately set above the promise so the page
  has headroom and does not fire on every ordinary bad day. It is not a target
  and must never be quoted as one.
- **5 minutes** — appeared in an old `ROADMAP.md` row (17.6) and in the wording
  of issue #234. Corrected: no test, metric or alert has ever asserted it.

Changing the promise means changing `AHA_BUDGET_MS` in
`tests/e2e/activation.spec.ts`, this row, and the alert's `description` in the
same commit. See `docs/DECISIONS.md` D-007.
