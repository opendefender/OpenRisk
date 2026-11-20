# 0013 — Content packs: manifest, versioning, provenance and upgrade semantics

Status: accepted (agent decision under D-089, recorded as D-121 in `docs/DECISIONS.md`)
Date: 2026-11-20
Refs: #1477 · Parent: #369
Builds on: ADR 0008 (D2 envelope and identity rule, D3 `regulatory_catalogue`, D7 split of
`compliance_controls`), ADR 0012 (stale marks), D-057 (PCI and CIS at family level), D-120 (catalogue
review status)
Implements nothing. No code, no migration, no query is added by this ADR.

## Context

A "pack" today is a `pkg/compliance.Catalog` (`backend/pkg/compliance/catalog.go`): `Key`, `Name`,
`Version`, `Description`, `Available`, `Review` (D-120), `Withdrawn`, `WithdrawalReason`,
`TrackingTicket` and `Controls []CatalogControl` (`ReferenceCode`, `Name`, `Description`,
`SourceReference`). Curated cross-framework correspondences live apart, in
`backend/pkg/compliance/crosswalks.go` (`CrosswalkEntry`: `SourceCatalog`, `SourceCode`,
`TargetCatalog`, `TargetCode`, `Coverage`, `Rationale`).

What the code does with them, read on 2026-11-20:

1. **Import only adds.** `ImportCatalogUseCase.Execute`
   (`backend/internal/application/compliance/import_catalog.go`) lists the framework's controls,
   skips every catalogue entry whose `ReferenceCode` already exists, creates the others with
   `Status = not_implemented`, then stamps `compliance_frameworks.catalog_key`
   (`backend/internal/domain/compliance.go`). A corrected description never reaches a tenant who
   already imported; there is no notion of "changed upstream".
2. **No transaction over the import.** Each `CreateControl` opens its own transaction (with the
   control-chain mirror, `controlchain.MirrorRequirementCreated`), the framework stamp is a
   best-effort `UpdateFramework`, and crosswalks are materialised best-effort afterwards
   (`materialise_crosswalks.go`). A failure halfway leaves a partial import that a re-run completes.
3. **Deletions are not remembered.** `ListControlsByFramework` and
   `GormControlCrosswalkRepository.Exists` read with GORM's default soft-delete filter, so a tenant
   who deleted a catalogue control or a curated crosswalk link gets it back on the next import of
   that framework (or, for a link, of either side).
4. **No content version.** `Catalog.Version` is the standard's edition (`"2022"` for
   `iso27001-2022`). #1478 (in progress) stores it as `compliance_frameworks.catalog_version` and
   documents it as "the edition of the STANDARD, not a revision of OpenRisk's own wording". Nothing
   records which revision of our own content a tenant holds.
5. **Only the latest content exists.** The registry holds one `Catalog` per key; earlier wording
   lives only in git history.
6. **ADR 0008 already states the identity rule**: a requirement imported from a catalogue has
   `source = openrisk_catalogue` (D7) and `source_version =` "the catalogue version", and
   `(tenant_id, source, source_id)` is unique; a second ingest of the same triple updates the record
   (D2). It does not say which version: edition or content revision.

#369 asks for packs that carry eight content kinds and survive upgrade without losing tenant work;
#556 criteria 3 to 6 ask for idempotent import, upgrade that preserves edits, a report, provenance
on every imported object, and reversibility. Each pack issue would otherwise invent its own answer.

## Decision

### D1 — A pack is a `Catalog`; the manifest is its typed fields

The pack stays Go source in `backend/pkg/compliance`, one file per pack, registered by `register()`
(hand-written or generated, as `cmd/catalogsync` already does for NIST 800-53). `Catalog` is extended,
not replaced, and the manifest is its struct: a content kind exists in a pack when its field is
non-empty. There is no separate manifest file to drift from the content.

Pack-level fields added by the follow-up issues (D7): `ContentVersion` (D3), `Changelog` (one
entry per content version: version, date, summary, the item keys added, changed and retired), and
`Review.ContentVersion` (D6). #370 adds jurisdiction metadata at this level; it is not a content kind.

### D2 — Manifest: the eight content kinds, required or optional, and tenant scoping (AC1)

Rule for every kind that becomes a database row: it is **imported into the tenant** and carries
`tenant_id uuid NOT NULL` directly, plus the provenance of D4. ADR 0008 D2 already says there is no
global canonical record; it is also the only way the merge of D5 has a tenant row to compare.
No kind introduces a global (tenant-less) table. Pack content itself is compiled-in static data and
is never stored in a shared table.

| # | Kind | Required | Pack content shipped today | Lands in (table exists today unless stated) | Tenant scoping |
|---|---|---|---|---|---|
| 1 | Requirements | **required** (at least one row when `Available`) | **yes**: `Catalog.Controls []CatalogControl` | `compliance_controls` (`domain.ComplianceControl`); ADR 0008 D7 reads each row as one `Requirement` | `tenant_id NOT NULL` on the row; its `framework_id` points at a `compliance_frameworks` row that also carries `tenant_id` |
| 2 | Controls | **required**, carried by the same entries as 1 | **yes**, as the split: `controlchain.MirrorRequirementCreated` writes one `controls` row per requirement, joined by an `addressed_by` edge in `canonical_edges` (migration 0086) | `controls`, `canonical_edges` | `tenant_id NOT NULL` on both (0086; listed in `security/isolation/tables.go`) |
| 3a | Mappings: cross-framework crosswalks | optional | **yes**: `crosswalks.go`, materialised by `CrosswalkMaterialiser.ForFramework` with `Origin = curated` | `control_crosswalks` (`domain.ControlCrosswalk`) | `tenant_id NOT NULL` on the row; both control ids are resolved in the same tenant |
| 3b | Mappings: requirement to risk | optional | no pack content — #556 (the target table exists) | `risk_control_mappings` (`domain.RiskControlMapping`), mirrored to a `mitigated_by` edge by `controlchain.MirrorMappingCreated` | `tenant_id NOT NULL` on the row and the edge |
| 3c | Mappings: control to evidence collector, default test definitions with cadence | optional | no pack content — #556 (`control_tests` exists) | test definitions: `control_tests` (0086, has `cadence`); collector mapping: no table yet, designed by #556 | `tenant_id NOT NULL` on `control_tests`; #556's table must carry it too |
| 4 | Risks (typical risks of the framework) | optional | no pack content — #556. The nearest existing content is the generic starter catalogue `backend/pkg/onboarding/starter.go`, which is not per framework | `risks`, written **only for the risks the tenant selects**, as onboarding does with `SourceStarter`; never written in bulk into a register | `tenant_id NOT NULL` on `risks` |
| 5 | Questionnaires | optional | no pack content — #1481 (whether it reuses `vendor_questionnaire_templates` is #1481's refinement question) | if reused: `vendor_questionnaire_templates` and `vendor_questionnaire_questions` | both carry `tenant_id NOT NULL` (`domain/vendor_assessment.go`) |
| 6 | Evidence expectations | optional | no pack content — #1480 | no table yet: a child table of the tenant's imported requirement (one control has several), designed by #1480 | `tenant_id NOT NULL` on the row **and** the parent `compliance_controls` row resolved in the same tenant |
| 7 | Assessment templates | optional | no pack content — #1481 (no assessment entity exists in `domain/compliance.go`) | no table yet: imported per tenant, table designed by #1481 | `tenant_id NOT NULL` |
| 8 | Report templates | optional | no pack content — #1481 | **not imported**: pack-resident, read-only layout rendered through `generate_compliance_report.go` and `pkg/report` | no row, so nothing to scope; the data it renders is read by that tenant-scoped use case |

Decisions inside the table:
- **Evidence expectations attach to the tenant's imported control, editable** (answers #1480 open
  question 3). A shared catalogue-level expectation could not be edited without editing it for every
  tenant, and an upgrade would have nothing to merge against.
- **Report templates are the only pack-resident kind.** A layout the tenant cannot edit has no
  merge problem, and importing it would copy static text into every tenant for nothing. If a tenant
  later needs to customise one, it becomes an imported kind under D4 and D5, by amendment.
  A pack-resident report template is a typed layout value rendered by `pkg/report`. It is never a
  `text/template` or `html/template` source, and tenant values are only ever data inserted into it,
  never parsed as template text.
- **Requirement-to-risk mappings and pack risks are written in one go with the risks the tenant
  selected**, so a pack never asserts a risk the tenant did not accept.

### D3 — Two version identifiers, never confused

- **Edition** — `Catalog.Version`, unchanged: the standard's own edition (`2022`, `2.0`, `4.0`).
  Stored by #1478 as `compliance_frameworks.catalog_version`. It never changes for a given `Key`.
- **Content version** — new `Catalog.ContentVersion`, semver `MAJOR.MINOR.PATCH`, our revision of
  the pack's content:
  - PATCH: wording or citation corrected on existing items, rationale of a crosswalk changed.
  - MINOR: items or content kinds added, crosswalk entries added.
  - MAJOR: an item retired (D5), or an item's identity changed.
  Every existing catalogue starts at `1.0.0` when the field lands. Stored on the framework as a new
  column `compliance_frameworks.catalog_content_version` (`varchar(32) NOT NULL DEFAULT ''`, expand
  only, no backfill). Empty means "imported before content versions existed" and is shown as
  unknown, the rule #1478 applies to the edition.
- **Crosswalks version** — new `CrosswalksVersion` in `crosswalks.go`, same semver rules, because a
  crosswalk entry belongs to a pair of packs, not to one. Stored on each framework as a new column
  `compliance_frameworks.catalog_crosswalks_version` (`varchar(32) NOT NULL DEFAULT ''`, expand only):
  the crosswalks version last materialised for that framework's pairs. A 3a row's `pack_version` is
  the `CrosswalksVersion` that last wrote it. An upgrade is available when **either**
  `catalog_content_version` or `catalog_crosswalks_version` differs from the registry, so a
  crosswalks-only bump triggers one; idempotence (D5) holds only when both are equal.
- **ADR 0008 reading.** For `source = openrisk_catalogue`, `source_version` is the content version,
  not the edition: the edition is constant per key, so it cannot say which revision a row holds.

### D4 — Provenance recorded on every imported row

Every row a pack writes (kinds 1, 3a, 3b, 3c, 4, 5, 6, 7) carries seven columns, added expand-only
to existing tables and created with new ones:

| Column | Type | Meaning |
|---|---|---|
| `pack_key` | `varchar(64) NOT NULL DEFAULT ''` | `Catalog.Key`, or `crosswalks` for 3a. Empty: not from a pack (hand-made). |
| `pack_item_key` | `varchar(255) NOT NULL DEFAULT ''` | the item's identity inside the pack. Requirements: the `ReferenceCode` as shipped. Crosswalks: `<catalog>:<code>|<catalog>:<code>`, the two ends in key order. Separate from `reference_code` so a tenant who renumbers a control does not lose its identity. |
| `pack_version` | `varchar(32) NOT NULL DEFAULT ''` | the content version that last wrote the row |
| `pack_base` | `jsonb` | snapshot of the pack-managed fields as the pack last wrote them |
| `pack_base_hash` | `char(64)` | SHA-256 (stdlib) of the canonical JSON of `pack_base`, prefixed by a hash-rule version |
| `pack_retired_in` | `varchar(32) NOT NULL DEFAULT ''` | the content version that retired the item (D5); empty while live |
| `pack_reverted_by` | `uuid NULL` | set only on a row a revert soft-deleted (D5, Reversibility): the id of that revert's `content_pack_upgrades` row. It is what tells a revert tombstone from a tenant deletion. |

Uniqueness is enforced by a partial unique index leading on `tenant_id`, deleted rows included:
`(tenant_id, framework_id, pack_key, pack_item_key) WHERE pack_key <> ''` on rows that have a
framework, `(tenant_id, pack_key, pack_item_key) WHERE pack_key <> ''` on `control_crosswalks`.

**Who writes them.** The seven columns are written only by the pack use case. They are absent from
every input DTO (PATCH, bulk, #307 import) and read-only in the API. #307 export does not emit them.

Where a canonical
projection carries the ADR 0008 envelope, `source_id` is derived from it and `source_version` is
`pack_version`; ADR 0008 does not fix a catalogue `source_id` format today, and the follow-up sets it.

**Pack-managed fields** are the fields the pack supplies, and only those. For a requirement:
`name`, `description`, `source_reference`. Never pack-managed, never touched by an upgrade: `status`,
applicability, owners, evidence links, test results, exceptions, the canonical `controls` row's
own name and description (ADR 0008 D6: the tenant's words), and anything the tenant added.
For a crosswalk: `coverage`, `rationale`.

**Tenant edit** = a pack-managed field whose current value differs from `pack_base`. Storing the
base snapshot, not only a hash, gives a field-level three-way merge: a tenant who rewrote a
description still receives an upstream citation fix on the same row.

**Legacy rows** (imported before D4 exists) have empty provenance. They are matched on first upgrade
by `(framework, reference_code)`, **soft-deleted rows included**, so no boot backfill is needed (SQL
backfills do not run on a `DB_HOST` boot here). A live legacy row equal to the new upstream content
is adopted (provenance stamped); a live legacy row that differs is a conflict with "baseline
unknown", because a tenant edit and an upstream change cannot be told apart. It is never
overwritten. A soft-deleted legacy row is `kept deleted` with "baseline unknown": it gets the
provenance stamp so later upgrades recognise it, and it is never re-created.

### D5 — Upgrade merge rules and what the report shows (AC2)

`B` is the row's `pack_base`, `T` its current pack-managed fields, `U` the item in the pack's current
content. Rules apply per item and, for D4's fields, per field. Tombstones count: the upgrade reads
the framework's rows **including soft-deleted ones**, with `tenant_id` in the WHERE.

| Case | Condition | What happens to the row | Report line |
|---|---|---|---|
| **Added** | item in `U`, no live or deleted row with that `pack_item_key` | inserted (through `MirrorRequirementCreated` for requirements), `status = not_implemented`, provenance stamped | `added` · item key, reference code, name |
| Added, collides with a hand-made row | a row without provenance has the same `reference_code` | equal to `U`: adopted. Different: left as is | `adopted` · or `conflict` (hand-made row has the code) |
| Unchanged | `U = B` and `T = B` | nothing written | counted in `unchanged`, not listed |
| **Changed upstream** | `U ≠ B`, `T = B` on every changed field | fields set to `U`, `pack_base := U`, `pack_version :=` new | `updated` · field, before, after |
| **Edited by tenant** | `U = B`, `T ≠ B` | pack-managed fields untouched; only `pack_version :=` new, so the row reads current | `kept (your edit)` · fields you changed |
| Edited by tenant, converged | `T = U ≠ B` | `pack_base := U`, `pack_version :=` new | `already up to date` |
| **Changed upstream AND edited by tenant** | the same field differs in `T` and `U` from `B`, and `T ≠ U` | **conflict**: the conflicting fields are left as the tenant has them; non-conflicting upstream fields are applied; the conflict is recorded open | `conflict` · field, yours, upstream, base |
| **Removed upstream** | row has provenance, item absent from `U` | **never deleted, never status-changed**: `pack_retired_in :=` new version. Evidence links, results, exceptions, mappings, audit history untouched. Gap analysis and progress count it exactly as before until the tenant acts (sets it not applicable, or deletes it through the existing audited delete) | `retired` · item, the changelog reason; flagged "edited by you" when `T ≠ B` |
| Deleted by tenant | soft-deleted row with that `pack_item_key` and `pack_reverted_by` null | **not re-created**, whether upstream changed it or not | `kept deleted` · and "changed upstream since" when `U ≠ B` |
| Reverted earlier | soft-deleted row with that `pack_item_key` and `pack_reverted_by` set, item in `U` | **restored, same row id**: `deleted_at` and `pack_reverted_by` cleared, fields `:= U`, provenance stamped, its control-chain rows restored (the split control id is deterministic, ADR 0008 D7) | `added` (restored after revert) |

**Conflict resolution.** An open conflict offers two actions, each its own audited write:
*keep mine* (`pack_base := U`, so the next upgrade compares against the new content and does not
reopen it) and *take upstream* (fields `:= U`, `pack_base := U`). Until resolved, the conflict
stays listed on the framework and in the next upgrade's report.

**The report** is persisted, not only returned: counts per case, then one line per non-unchanged
item as in the table, plus from-version, to-version, the target content's review status (D6) and
who applied it. It is the control of changes ISO/IEC 27001:2022 7.5.3 asks for on documented information:
what changed, from which version to which, and who decided.

**Preview first.** The same computation runs read-only and returns the report without writing.
Apply is a second call that recomputes (the report it returns is the one that applied, not the
preview's).

**Idempotence.** Re-importing or re-upgrading when both the content version and the crosswalks
version already held equal the registry's writes **no row**
(no update, no `updated_at` change, no upgrade record, no audit entry) and returns a report of
zero changes. Import and upgrade are the same use case: a first import is an upgrade from nothing.

**Transaction (rule 7).** One use case, one transaction for the whole apply: every row insert and
update, the control-chain mirror writes, the ADR 0012 stale marks of the controls it touches, the
crosswalk materialisation (no longer best-effort), the framework's `catalog_version`,
`catalog_content_version` and `catalog_crosswalks_version` stamp, the upgrade record and its items, and one `audit_events` row.
Any failure rolls the whole upgrade back; there is no partial upgrade.

**Locking and conditional writes.** Apply, revert and both conflict resolutions lock the framework
row (`FOR UPDATE`) first, and choose the latest upgrade or read the conflict only after taking the
lock. Crosswalk rows span two frameworks, so an apply or revert that writes 3a rows locks, in one
statement and in `id` order, the path framework **and** every framework of the tenant whose
`catalog_key` forms a crosswalk pair with it; two upgrades of A and B therefore serialise instead of
racing on the same links. The partial unique index of D4 is the backstop: a violation fails the
transaction, which rolls back whole and can be retried. Each row write is conditional:
`UPDATE … WHERE id = ? AND tenant_id = ? AND <fields> = <expected>`. 0 rows affected means the row
changed since; it is reported as a conflict and never forced. Take-upstream carries the `T` it
displayed and fails with `ErrConflict` if `T` moved.

**Reversibility.** Each applied upgrade stores, per row it touched, the before and after values of
the pack-managed fields and provenance. *Revert* targets the latest non-revert upgrade of the
framework that is not yet reverted (older ones revert in order), in one transaction:
- an updated row whose fields still equal the after-image returns to the before-image;
- an added row is soft-deleted only if the tenant has done nothing to it since (status, evidence
  links, results, exceptions, mappings unchanged); otherwise it stays and is reported. The
  soft-delete sets `pack_reverted_by` to the revert's upgrade id, so it never counts as a tenant
  deletion: a later re-apply restores the row (D5, "Reverted earlier"). For a requirement, the
  revert also soft-deletes its split `controls` row and removes its `addressed_by` edge in the same
  transaction. `controlchain/mirror.go` has no function for a requirement deletion today
  (`DeleteControl` in `gorm_compliance_repository.go` does not mirror); the follow-up adds one;
- a retired row has `pack_retired_in` cleared;
- a row the tenant changed since the upgrade is left and reported as a conflict.
A revert is itself recorded as an upgrade (to-version = the previous one), so the history reads
forward. Revert never needs the old pack content, which the code does not keep (Context 5).

**Tenant gate of the new tables.** The upgrade record and its items live in two new tables,
`content_pack_upgrades` (one row per apply or revert: framework, from and to versions, actor, and a
`report` holding counts only) and `content_pack_upgrade_items` (one row per touched item: kind,
row id, action, before, after; the report lines live here). Both carry `tenant_id uuid NOT NULL`
with an index leading on it, every read and write filters on it, the framework is resolved
`WHERE id = ? AND tenant_id = ?` before anything else (miss: `ErrNotFound`), and both are added to
`security/isolation/tables.go`. A missing tenant context is a 401, never a cross-tenant read.

Every target row a revert or resolution writes is re-read `WHERE id = ? AND tenant_id = ? AND
framework_id = ?` (for a crosswalk: `tenant_id` and one end in the framework), soft-deleted rows
included. A conflict id from the client is resolved `WHERE id = ? AND tenant_id = ? AND upgrade_id
IN (upgrades of the path framework)`; a miss is `ErrNotFound`. Items store `(kind, row_id)` and the
use case maps the kind to a fixed repository, never a table name from data.

Items are append-only. A resolution is a new item row (action `resolved_keep_mine` or
`resolved_take_upstream`, actor, before, after) plus an `audit_events` row; a conflict is open while
no resolution item follows it. Items are never UPDATEd or DELETEd. Upgrade records and items follow
their framework's deletion (soft-deleted with it) and are included in the tenant's export and
erasure. They are never written to logs or `audit_events` metadata.

**Routes and permissions.** Preview and reading upgrade history and items:
`compliance:frameworks:read` plus `compliance:controls:read`. Apply, revert and conflict
resolution: `compliance:frameworks:create`, the key the catalogue import route already uses. In
addition, the use case checks the caller's own permission for every other module a pack writes:
kinds 3b and 4 require `risks:create`, kind 5 requires `vendors:manage`, and a revert that
soft-deletes a requirement requires `compliance:controls:delete`. A missing key drops that kind from
the apply with a report line `skipped (permission)`. It is never written with the framework key
alone. The routes sit in the `protected` group, which already applies the general per-tenant budget
(`middleware.TenantRateLimit`, `internal/middleware/apiquota.go`); no mutation-specific limiter
exists today, and the upgrade follow-up adds a dedicated per-tenant limit on preview, apply and
revert.

### D6 — Interaction with D-120 review status

`Review` describes the content it reviewed, so it gains `ContentVersion`. When a catalogue's
`ContentVersion` moves past `Review.ContentVersion`, the status shown is `unreviewed`, with "last
reviewed at X" beside it; `TestCatalogReviewContract` is extended to enforce the field. Upgrading
does not change a framework's review state, because review belongs to the pack, not the tenant row.
The preview shows the review status of the held version and of the target version, and an upgrade to
content that is not `reviewed` asks the same one confirmation as an import does (#1468).

### D7 — ISO 27001:2013 and :2022: two packs, not one pack with two versions (AC4)

**Decision: two packs**, `iso27001-2013` and `iso27001-2022`, each with its own content-version
line. The edition is part of the key, as most keys in the registry already are (`iso27001-2022`,
`nist-csf-2.0`, `pci-dss-4.0`, `dora-2022-2554`). A key without an edition (`soc2-tsc`, `cobac`,
`bceao`, `antic-cm`) keeps meaning the edition it holds today; a second edition gets a new key.

Reasons:
1. **An upgrade must never be an edition change.** The merge of D5 is keyed by item identity. The
   2013 and 2022 Annex A are restructured and renumbered, so a reference code does not name the same
   requirement in both (the 2022 catalogue here is coded `A.5` to `A.8`, the four themes of that
   edition); an upgrade from one to the other would rewrite or retire most rows and
   "migrate" a Statement of Applicability by accident.
2. **Crosswalks between editions need two keys.** `CrosswalksBetween` returns nothing when both
   sides have the same key ("a catalog does not crosswalk to itself"), and the materialiser skips a
   framework whose key equals the imported one. The assisted migration #325 asks for runs over a
   2013 to 2022 crosswalk, which only two packs can carry.
3. **Coexistence is a tenant fact.** During a certification transition a tenant runs both editions,
   each as its own framework row, with its own status, evidence and history. One framework row holds
   one `catalog_key`; two packs map onto that directly.

Moving a tenant from one edition to the next is therefore **a migration, not an upgrade**: import
the new edition as a new framework, materialise the edition-to-edition crosswalks, and let the
tenant carry status and evidence across along them, reviewed. That workflow, the visible changelog
and tenant notifications are #325's. The old framework stays until the tenant deletes it.
No `iso27001-2013` catalogue exists in code today; whether to ship one is #325's call.

### D8 — Out of scope, stated

- **Score Engine and SmartScore are untouched.** No formula, threshold or input changes. Added
  requirements arrive `not_implemented` and move compliance progress and gap analysis, which the
  preview shows; pack risks (#556) are scored by the existing engine on write.
- **No new external dependency.** Hashing is `crypto/sha256`; the format is Go source.
- No tenant-authored pack format: custom frameworks keep the #307 JSON import/export.

## Follow-up issues

Existing, now unblocked or constrained by this ADR:
- **#1478** — records the edition (`catalog_version`), consistent with D3 as implemented on its
  branch. It does not need to change.
- **#556** — criteria 3 to 6 are D5 and D4; kinds 3b, 3c and 4 are its content.
- **#1480** — kind 6; open questions 1 and 3 are answered by D2.
- **#1481** — kinds 5, 7 and 8; report templates are pack-resident (D2).
- **#325** — edition migration, changelog view and notifications (D7); content versions and
  `Changelog` give it its data.
- **#370** — regional packs follow D1 to D5 unchanged; jurisdiction metadata is a pack-level field.

To open:
1. **Pack content version, changelog and per-row provenance** — `Catalog.ContentVersion`,
   `Changelog`, `Review.ContentVersion`, `CrosswalksVersion`; columns of D3 and D4 on
   `compliance_frameworks` (`catalog_content_version`, `catalog_crosswalks_version`),
   `compliance_controls`, `control_crosswalks`, with the partial unique indexes (expand only, next free
   migration number, mirrored in the GORM models); the import made one transaction and
   tombstone-aware (Context 2 and 3).
2. **Pack upgrade: preview, apply, report, conflict resolution, revert** — D5 and D6, the two
   tables, the routes, the per-module permission checks, the requirement-deletion mirror, the
   dedicated rate limit, and the framework page that shows the report and open conflicts. Depends on 1.

## Tests that prove this design when implemented

`TestUpgradePack_Success` (one test per D5 case, report lines asserted),
`TestUpgradePack_NotFound` (framework of another tenant: 404, nothing written),
`TestUpgradePack_Unauthorized` (no tenant context: 401; no permission: 403),
`TestUpgradePack_SameVersion_WritesNothing` (row count and `updated_at` unchanged),
`TestUpgradePack_DeletedRowNotResurrected`, `TestUpgradePack_RetiredKeepsEvidenceAndStatus`,
`TestUpgradePack_FailureRollsBackEverything`, `TestRevertPackUpgrade_RestoresBeforeImage`,
`TestRevertPackUpgrade_LeavesTenantTouchedRows`, `TestRevertThenReapply_RecreatesAddedItems`,
`TestUpgradePack_LegacyDeletedRowKeptDeleted`, `TestUpgradePack_CrosswalksOnlyBumpUpgrades`,
`TestUpgradePack_ConcurrentPairSerialises`, `TestUpgradePack_MissingModulePermissionSkipsKind`,
`TestResolvePackConflict_NotFound` (conflict id of another tenant or another framework: 404),
and a Postgres isolation test over both new tables.

## Consequences

Easier:
- Every pack issue has one answer for format, version, provenance and upgrade; #556, #1480, #1481
  and #370 build content, not mechanisms.
- Catalogue corrections finally reach tenants who imported earlier, without overwriting their words.
- A tenant can see what changed, from which version to which, who applied it, and undo it.

Harder:
- Every new content kind's table carries the provenance columns and the merge case table.
- The import changes behaviour: a deleted catalogue control or curated link is no longer
  re-created by a re-import. That is the intended fix, and it is announced in the follow-up's notes.
- Legacy rows that differ from current content surface as conflicts on first upgrade; the first
  upgrade of an old tenant may show many. That is the honest reading: we cannot know who changed them.
- A legacy control the tenant deleted before provenance existed stays deleted forever, even if it was
  deleted by mistake; restoring it is a manual re-creation. Re-creating it would repeat today's
  resurrection defect.
- An upgrade of one framework locks every framework it shares crosswalks with, so upgrades of
  paired frameworks in one tenant run one after the other.
- Old pack content is not kept in code, so a conflict's "base" comes from the row's own snapshot,
  and legacy rows have none.

## Alternatives rejected

- **YAML or JSON pack files.** YAML needs a new dependency; JSON loses the compile-time typing and the
  existing catalogue tests (`TestNoOrphanControls`, `TestExpectedControlCounts`,
  `TestCatalogReviewContract`) that run on Go values. #307's JSON stays for tenant-authored frameworks.
- **A separate `Pack` type wrapping `Catalog`.** Renames every caller for no gain; the struct is the
  manifest.
- **Overwrite on upgrade (upstream wins).** Destroys tenant edits, which #1477 and #556 criterion 4 forbid.
- **Never update existing rows (today's behaviour).** Corrections never reach anyone; a known-wrong
  citation stays in an audit file.
- **Hash only, no base snapshot.** Detects an edit but cannot say which field, so every edited row
  conflicts in full and a tenant loses upstream citation fixes on rows whose description they rewrote.
- **Delete rows removed upstream.** Deletes evidence links, results and history an auditor relies on.
- **Catalogue-level (shared) evidence expectations and templates.** Not editable per tenant, nothing
  to merge; and a global table would be the first tenant-less compliance table.
- **ISO 27001 as one pack with two versions.** See D7: an upgrade would become an edition migration,
  and edition crosswalks would be self-crosswalks the code refuses.
- **Reusing `Catalog.Version` as the content version.** It is the edition, #1478 stores it as such,
  and it does not move when our wording does.
- **Revert any past upgrade in any order.** Before-images of an older upgrade can be invalid once a
  later one has written the same rows; latest-first keeps every revert well defined.
