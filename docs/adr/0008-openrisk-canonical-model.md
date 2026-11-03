# 0008 — OpenRisk Canonical Model v1: the envelope, the relationship vocabulary, the GRC slice

Status: **accepted**
Proposed: 2026-11-03, on #541
Accepted: 2026-11-03 by the owner, as written — D-087 (`docs/DECISIONS.md`)
Decided by: D-030 (a canonical model, ADR before code, schema after), D-044 and D-045 (the
`Vendor`/`Assessment` slice in ADR 0004), D-056 (`control_evidences` is retired), D-083 (option
A: the GRC slice first, 19 entities)
Epic: #535 · Follows: ADR 0001, ADR 0003, ADR 0004
Blocks (on acceptance): the Go types and tests (follow-up issue, opened on acceptance) · #542
schema · #543 data contracts · #553 control chain · #554 evidence record · #555 live status ·
#1217 the ten other entities

## Context

Every module records the same ideas in its own shape. A finding from the scanner is a row of
`vulnerabilities`. A finding from threat intelligence is a risk with `source = cti_auto` and a
`source_cve_id`. A gap found by an audit is free text in `compliance_audits.summary`. A control
is two different Go types (`domain.ComplianceControl` and the integration contract
`domain.Control`), and it is also the requirement it answers, because the catalogue entry and the
tenant's control are the same row. When an auditor asks where a piece of evidence came from, when
it was true and how sure we are, each module answers differently, and some cannot answer.

This ADR fixes one shape. It is a contract, not a schema migration: nothing here changes a
table. It says what every canonical record carries, which verbs connect them, and how each
column persisted today maps onto it. D-030 forbids code before it is accepted.

Seven things were read rather than assumed.

**1. ADR 0004 already carries the envelope for two entities.** `vendor_assessments` has
`source`, `source_id`, `source_version`, `observed_at`, `confidence` and `provenance`, with
semantics for one source class (self-attested, 0.50). This ADR generalises those semantics and
must not contradict them (D-044).

**2. The evidence library already separates "when it was true" from "when we stored it".**
`domain.Evidence.CollectedAt` (`backend/internal/domain/evidence.go:155`) is documented as "when
the proof was *taken*, which is not when the row was created". That is `observed_at` under
another name. The model keeps the idea and renames the field once, here.

**3. The vulnerability register already has two kinds of confidence, and they are different
things.** `Vulnerability.MatchConfidence` (`vulnerability.go:154`) says how sure the correlator is
that *this asset* is the one affected. It does not say how sure we are that the vulnerability is
real. Collapsing the two into `confidence` would make one number answer two questions. The
envelope's `confidence` is about the record; the attribution confidence stays on the `affects`
relationship (D5).

**4. Re-ingest identity already exists, but only in one module.** `Vulnerability.DedupKey()`
(`vulnerability.go:206`) upserts on `source|external_id`. The envelope generalises it as
`(tenant_id, source, source_id)`, which is what lets a source correct itself (D2).

**5. A requirement and a control are not separated today.** `domain.ComplianceControl`
(`compliance.go:62`) holds the catalogue text (`reference_code`, `name`, `description`,
`source_reference`) **and** the tenant's claim (`status`). One requirement answered by two
controls, or one control answering requirements from three frameworks, cannot be expressed
except through `control_crosswalks`, which links copies of requirement text to each other.

**6. 30 files in `backend/internal/domain` import GORM or Fiber.** A purity test on that package
fails the day it lands. The canonical types need a package of their own (D9).

**7. A pure child package and a boundary test already exist, separately.**
`internal/domain/scoring` shows that a child of `domain` can stay free of GORM, but nothing tests
it. `backend/pkg/vendorscore/imports_test.go` is the only import-boundary test: it parses the
imports of every non-test file and fails on a forbidden one. D9 reuses that pattern, with an
allowlist rather than a denylist.

## Decision

### D1 — The entity list: 19, of which v1 specifies 7 and adopts 2

The canonical entity set is D-030's 17 plus two that #553's chain needs as records of their own
(D-083):

| Entity | v1 | Why it is an entity |
|---|---|---|
| `Requirement` | **specified, D6** | an obligation stated by a source (a standard, a regulation, a contract), independent of how a tenant answers it |
| `Control` | **specified, D6** | the tenant's measure that answers one or more requirements |
| `ControlImplementation` | **specified, D6** | see below |
| `ControlTest` | **specified, D6** | see below |
| `Evidence` | **specified, D6** | a proof artifact, reusable across controls and tests |
| `Risk` | **specified, D6** | a business risk with its frozen score |
| `Finding` | **specified, D6** | an observed fact that something is wrong: a vulnerability, a failed test |
| `Vendor` | adopted, ADR 0004 (D8) | a third party; today an asset of category `vendor` |
| `Assessment` | adopted, ADR 0004 (D8) | a questionnaire sent to a vendor |
| `Asset`, `Threat`, `Incident`, `Mitigation`, `Action`, `Owner`, `System`, `Service`, `Policy`, `Signal` | out of scope v1 (D10) | held by #1217 |

**Why `ControlImplementation` is an entity and not a sub-record of `Control`.** A control is
implemented in several places, differently, by different people. "Access reviews are quarterly"
is implemented one way on the core banking system and another way on the e-mail tenant, each
with its own owner, status and date. As a sub-record it could not be the target of an
`applies_to` edge, could not own tests, and could not carry its own `source` when a connector
reports it.

**Why `ControlTest` is an entity and not a sub-record of `Control`.** A test is what makes a
control's status derivable rather than declared (#555). It has its own cadence, method, pass
criteria and results, and the envelope on its results (`observed_at`, `confidence`, `source`) is
the field the Evidence Fabric needs most (D-083, option C rejected). Evidence attaches to a
test's result (#553 criterion 4), so the test must be addressable.

**`ControlTestResult` is a sub-record of `ControlTest`, not a twentieth entity.** A result has no
meaning outside its test, is append-only, and is never the subject of an edge except as the
qualifier of `evidenced_by` (D5). It carries its own `observed_at`, `source`, `source_id` and
`confidence`, because those are what freshness and the audit trail read. Promoting it to an entity
later is an amendment, and adding an entity is cheaper than removing one once #542 publishes.

### D2 — The envelope: eleven fields, on every entity, specified once

Every canonical entity carries these fields. The entity specifications in D6 do not repeat them;
they only state where an entity narrows an optionality.

| Field | Type | Null | Meaning |
|---|---|---|---|
| `id` | uuid | **no** | OpenRisk's identity for the record. Assigned in Go, never by the database (the `ControlCrosswalk` and `AuditEvent` rule: a Postgres-only default cannot migrate under sqlite). |
| `tenant_id` | uuid | **no, on every entity, without exception** | the owning tenant. `uuid.Nil` is invalid. There is no global canonical record: catalogue content becomes canonical only once imported into a tenant. ABSOLUTE RULE 2. |
| `source` | string, from the source registry (D3) | **no** | the system or actor class that produced the record, e.g. `openrisk_user`, `nessus`, `cisa_kev`, `vendor_questionnaire` |
| `source_id` | string ≤ 255 | **no** | the record's identity *in the source*. For records whose source is OpenRisk itself, it is the record's own `id`, as ADR 0004 does for assessments. |
| `source_version` | string ≤ 64 | yes | the source's revision of the record: a plugin output version, a catalogue version, a template version. Null when the source does not version. |
| `created_at` | timestamptz, UTC | **no** | when OpenRisk first stored the record. Never changes. |
| `updated_at` | timestamptz, UTC | **no** | when OpenRisk last changed any field. |
| `observed_at` | timestamptz, UTC | per entity (D4) | when the world was in the state the record describes |
| `confidence` | decimal, 0.00–1.00, two places | **no** | how likely the record is true as of `observed_at` (D3) |
| `provenance` | object (D3) | **no** | how the record reached OpenRisk |
| `relationships` | array of edges (D5) | **no**, may be empty | the record's outgoing edges, in the fixed vocabulary |

**Identity rule.** `(tenant_id, source, source_id)` is unique per entity type. A second ingest of
the same triple **updates** the record and records the source's new `source_version`. It never
creates a second record. That is how a source that corrects itself (a scanner withdrawing a
false positive, a catalogue errata) is re-applied: same triple, new version, the old version kept
in the audit trail. Today only `Vulnerability.DedupKey()` does this; the envelope makes it the rule.

**Who writes the envelope.** `tenant_id` comes from the authenticated context (or, on a public
route, from the token row, ADR 0004 D4), and from nowhere else. No envelope field is read from a
request body: `source`, `source_id`, `source_version`, `confidence` and `provenance` are set by
the use case or the ingest path that owns the source. An upsert never changes `id`, `tenant_id`
or `created_at`. A `user_entered` record's confidence is computed from its review state (D3), never
supplied. Otherwise a user posting `source = nessus` with a scanner's `source_id` would overwrite
that scanner's finding through the identity rule.

**What the envelope deliberately does not carry.**
- **No soft-delete field.** A deleted record is absent from the canonical view. Persistence may
  keep `deleted_at`; the contract does not expose it.
- **No `organization_id`.** It is a legacy alias of `tenant_id` on `risks` (`risk.go:192`) and is
  not carried forward.
- **No personal data in `provenance`** beyond the internal user id of an actor who has an account.
  No IP address, no user agent (ADR 0004 D3a, applied to every entity).

### D3 — `source`, `confidence` and `provenance`

**The source registry.** `source` is a flat identifier from a registry that the Go types define
and #542 publishes. Each identifier belongs to exactly one **source class**. The class, not the
identifier, fixes the confidence semantics. Where a persisted value can be a
registry identifier as it stands, it keeps its spelling, so ADR 0004's `vendor_questionnaire` is
valid as written. Where it cannot (a bare `manual`, `scanner` or `agent` says nothing about which
product), D7 names the renaming, column by column.

**`confidence` semantics.** One question: *how likely is it that this record is true, as of its
`observed_at`?* It is not severity, not priority and not importance. It is **never an input to
`Risk.Score` or SmartScore**. A test pins that boundary (D9).

| Source class | Examples (registry identifiers) | Default | Rises to | Why |
|---|---|---|---|---|
| `user_entered` | `openrisk_user`, `openrisk_import_file` | **0.70** | **0.90** when a second person accepts it: `review.verdict = accepted` **and** `review.reviewed_by` is set **and** differs from `collected_by` and from `provenance.actor.id`. Today's default `review = accepted` with no reviewer does not qualify. | a person with an account stated it; one person's statement is discounted until a second one reviews it |
| `automated_scanner` | `nessus`, `openvas`, `qualys`, `ms_defender`, `aws_inspector`, `azure_defender`, `crowdstrike`, `openrisk_scanner`, `openrisk_agent` | **0.80** | — | a tool observed it directly on the estate; scanners raise false positives, and `false_positive` is a status the register already needs (`VulnStatusFalsePositive`) |
| `threat_intelligence` | `nvd`, `cisa_kev`, `mitre_attack`, `opencti` | **0.40** | — | a feed says something exists in the world; that it concerns *this* tenant is inferred from CPEs, not observed. When a scanner confirms the CVE on an asset, the confirmation is the scanner's own `Finding` at 0.80, linked `caused_by` the threat; the intelligence record itself stays at 0.40. |
| `connector_import` | `jira`, `servicenow`, future #543 contracts | **0.60** | the value the source states, when its data contract (#543) maps a source field onto `confidence`, **capped at 0.80** | another system of record holds it; OpenRisk did not observe it |
| `third_party_attestation` | `vendor_questionnaire` | **0.50**, a cap | nothing in v1 | ADR 0004 D3a, adopted unchanged: self-attested and unverified |
| `regulatory_catalogue` | `openrisk_catalogue` | **0.90** for a catalogue with `Available = true`; such catalogues are verified by the compliance officer against the source text before they ship | — | the requirement's *text* is a transcription of a cited source. 1.00 is not used, because a transcription can be wrong and the compliance doctrine treats every control as guilty until sourced |
| `derived` | `openrisk_derived`, `openrisk_starter`, `openrisk_ai` | **the minimum of its inputs' confidences**, per the table below | — | a record OpenRisk computed from others is never surer than its weakest input |

**What a `derived` record names as its inputs.** `provenance` must carry at least one of
`derived_from` (same-tenant canonical records) or `external_refs` (global references).

| Registry id | Written when | `derived_from` | `external_refs` | Confidence |
|---|---|---|---|---|
| `openrisk_derived` | a vulnerability proposes a risk (`scan_auto`) | the `Finding` | — | the finding's |
| `openrisk_derived` | threat intelligence proposes a risk (`cti_auto`) | — | `{cve, <id>}` | 0.40, the threat-intelligence default |
| `openrisk_derived` | a vendor assessment proposes a risk (`vendor`) | the `Assessment` | — | the assessment's, so at most 0.50 |
| `openrisk_derived` | a failing `ControlTestResult` raises a gap | the `ControlTest` (with `result_id` on `raised_by`) | — | the result's |
| `openrisk_derived` | evidence written by an automation | the records the automation read | — | the minimum of theirs |
| `openrisk_starter` | onboarding writes a selected starter risk (#438) | — | `{starter_catalogue, <key>}` | 0.70: the input is the user's selection |
| `openrisk_ai` | the assistant proposes a risk | the records in the prompt context, when any | `{ai_proposal, <id>}` | 0.50 until a person accepts it, then the `user_entered` rule. The governed-AI ADR (Release 1.5) may lower it; it may not raise it above `user_entered`. |

**Rules that bind every class.**
- `1.00` is reserved. No default produces it. Nothing in v1 writes it.
- `0.00` means "recorded and believed false". Marking a finding `false_positive` **writes**
  0.00 under the current `confidence_rule`, as an audited update like any other field change; it
  is a rule applied at a write, not a recomputation. It is not "unknown". Unknown is not
  representable: every source class has a default.
- A value outside a class's default or ceiling needs a reason written into `provenance`. The
  validation test rejects a `user_entered` record above 0.90, a `connector_import` record above
  0.80 and a `third_party_attestation` record above 0.50.
- Confidence is stored on the record and is never silently recomputed. A rule change gets a new
  `provenance.confidence_rule` version, exactly as `scoring_version` and `residual-v1` do.

**`provenance`** is a JSON object with this shape. Keys not listed are refused by validation,
except the source-specific keys a source's registry entry allowlists (ADR 0004's assessment keys
are the first such allowlist, D8).

| Key | Type | Null | Meaning |
|---|---|---|---|
| `method` | enum | no | `manual_entry` · `file_import` · `api` · `scan` · `feed` · `questionnaire` · `derivation` · `migration` |
| `actor` | `{kind, id}` | no | `kind` ∈ `user` · `system` · `vendor_contact`; `id` is a user id, a worker name or `vendor_contact:<assessment_id>` (ADR 0004 D4) |
| `via` | string | yes | the integration, connector or catalogue key that carried it, e.g. a `vuln_integrations` id or `iso27001-2022`. A registry key or an id, **never a URL**. |
| `ingest_id` | string | yes | the run that wrote it (a scan job id, an import batch id) |
| `derived_from` | array of `{type, id}` | yes; **required** when the source class is `derived` | the canonical records it was computed from. **Same-tenant canonical records only.** |
| `external_refs` | array of `{scheme, value}` | yes | references to global catalogues that are not tenant records, e.g. `{cve, CVE-2024-3094}` for a risk proposed from threat intelligence |
| `rule` | string | yes | the rule and its reason, frozen at the time it fired. Today's `Risk.SourceRuleReason` lands here. |
| `confidence_rule` | string | no | the version of the confidence rule applied, `confidence/1` for this ADR |
| `raw_ref` | string | yes | a pointer to the raw payload held by persistence (`vulnerabilities.raw_data`). The payload itself is not part of the contract. |

**Never in `provenance`:** tokens, credentials, API keys, IP addresses, user agents, e-mail
addresses of people without an account. ABSOLUTE RULE 6. Enforced, not only stated: every string
in `provenance` is capped at 255 characters, and a value that parses as a URL carrying userinfo
or a signature or token query parameter (`X-Amz-Signature`, `sig`, `token`, `access_token` and the
like) is refused. The same refusal applies to `Evidence.external_url`, where presigned links are
most likely to be pasted.

### D4 — Time: `observed_at` against `created_at`

- **`observed_at`** is when the world was in the state the record describes.
- **`created_at`** is when OpenRisk learned it.

**Worked example.** A Nessus scan runs on **Monday 2 November 2026 at 22:00 UTC** and finds
CVE-2024-3094 on a host. The export is uploaded on **Thursday 5 November at 09:00 UTC**. The
canonical `Finding` has `observed_at = 2026-11-02T22:00Z` and `created_at = 2026-11-05T09:00Z`. A
compliance officer reading it on Friday sees an observation four days old, not one day old. If
the host was patched on Tuesday, Thursday's import does not prove it vulnerable on Thursday, and
the record must not say so.

A second scan on **Monday 9 November** finds it again. Same `(tenant_id, source, source_id)`, so
the record is updated: `observed_at` moves to 9 November, `first_observed_at` stays at 2
November, `created_at` stays at 5 November.

**Optionality by entity.**

| Entity | `observed_at` | What it is |
|---|---|---|
| `Requirement` | **null** | a requirement is a statement, not an observation; `source_version` dates it |
| `Control` | **null** | a design, not an observation |
| `ControlImplementation` | yes, nullable | when the implementation was last confirmed in place; null until confirmed |
| `ControlTest` | **null** on the test; **required** on each `ControlTestResult` | the definition is not observed; each run is |
| `Evidence` | **required** | when the proof was taken (today `collected_at`) |
| `Risk` | yes, nullable | when the risk was last assessed (today `last_reviewed_at`) |
| `Finding` | **required** | the most recent time the source observed it |
| `Assessment` (ADR 0004) | null until submitted, then `submitted_at` | unchanged from ADR 0004 D3 |

**The freshness rule (#555).** Freshness is computed from **`observed_at` and never from
`created_at`.** For a control, it is the `observed_at` of the most recent `ControlTestResult` and
of the newest covering `Evidence`. A record that should carry `observed_at` and does not is
**not fresh**: an absent signal never reads as a good one, the rule ADR 0003 applies to coverage.
Uploading old proof today must not make a control look tested today.

### D5 — The relationship vocabulary

An edge is a record of its own, with this envelope:

| Field | Type | Null | Meaning |
|---|---|---|---|
| `id` | uuid | no | assigned in Go |
| `tenant_id` | uuid | no | inherited from the source end |
| `from` | `{type, id}` | no | the source end |
| `verb` | enum, the table below | no | |
| `to` | `{type, id, tenant_id}` | no | the target end. `to.tenant_id` is filled by persistence when it resolves the target, never by a caller, so a pure validator can compare it with `tenant_id`. |
| `qualifier` | uuid | yes | `result_id` where the verb takes one; part of the identity |
| `attributes` | object | no, may be empty | per verb, closed set |
| `source`, `source_id`, `confidence`, `provenance` | as D2 and D3 | no | |
| `created_at`, `updated_at` | timestamptz | no | |

It is stored on its **source** end and read in both directions. Every verb has exactly one direction.
The inverse is a reading, never a second stored verb: storing both is how two vocabularies drift
(ADR 0001 D6b).

**Edge rules.**
- Both ends belong to the same tenant. An edge whose target resolves in another tenant is
  invalid, and the write answers 404, as `AssetDependency` already does.
- An edge carries its own `source` and `confidence` from the registry (D3), because "the scanner
  says this finding affects that host" and "a person said so" are different claims about the same
  pair.
- `(tenant_id, from, verb, to, qualifier)` is unique. Re-asserting an edge updates it. The
  qualifier is in the key so that one artifact can back two runs of the same test.
- Every verb is directed except **`maps_to`, which is symmetric**: it is stored once per unordered
  pair, and A→B and B→A are the same edge, as `control_crosswalks` already enforces.
- An edge inherits `tenant_id` from its source end. **On every edge write, persistence resolves
  the target with `WHERE id = ? AND tenant_id = ?`**, or with ADR 0001 D5's `ThroughParent` join
  for a target table without `tenant_id`, for every verb, deferred ones included. A miss answers
  404. A join table without `tenant_id` (today `risk_assets`) is gated on **both** ends, not only
  on the end that owns the column.
- A verb whose target is outside the v1 slice is kept, with the target marked **shape deferred**.
  The edge stores the target's type and id. **Deferral postpones validation of the target's
  fields, never the tenant check above.** The target's fields are validated once #1217 fixes
  them.

**Cardinality** is written `source : target`.

| Verb | Source | Target | Cardinality | Reads | Attributes | Origin |
|---|---|---|---|---|---|---|
| `addressed_by` | `Requirement` | `Control` | N : M | the requirement is addressed by the control | `coverage` ∈ `full` · `partial`; `rationale` | #553 chain |
| `implemented_by` | `Control` | `ControlImplementation` | 1 : N, each implementation has exactly one control | the control is implemented by this implementation | — | #553 chain |
| `tested_by` | `ControlImplementation` | `ControlTest` | 1 : N, each test has exactly one implementation | the implementation is tested by this test | — | #553 chain |
| `evidenced_by` | `ControlTest` | `Evidence` | N : M | the test's result is evidenced by this artifact | `result_id` (required): the `ControlTestResult` it supports; `note` | #553 chain, criterion 4 |
| `evidenced_by` | `Finding` | `Evidence` | N : M | the finding is evidenced by this artifact | `note` | original note |
| `evidenced_by` | `Control` | `Evidence` | N : M | **transitional**: today's `evidence_control_links`, for links that have no test yet | `note` | #553 criterion 6 |
| `maps_to` | `Requirement` | `Requirement` | N : M, stored once per unordered pair | the two requirements correspond | `coverage` ∈ `full` · `partial`; `rationale` (required); `origin` ∈ `curated` · `manual` | today's `control_crosswalks` |
| `mitigated_by` | `Risk` | `Control` | N : M | the risk is mitigated by the control | `note` | today's `risk_control_mappings` |
| `indicates` | `Finding` | `Risk` | N : M | the finding indicates the risk | — | original note |
| `violates` | `Finding` | `Control` | N : M | the finding violates the control | — | original note |
| `raised_by` | `Finding` | `ControlTest` | N : 1 | the gap was raised by a failing result of this test | `result_id` (required) | #553 criterion 5 |
| `affects` | `Finding` | `Asset` *(shape deferred)* | N : M | the finding affects the asset | `match_confidence`, `match_method`, `match_ambiguous`, `match_candidates`, `resolved_by`, `asset_criticality_at_observation` | original note; today's `vulnerabilities.asset_*` |
| `affects` | `Risk` | `Asset` *(shape deferred)* | N : M | the risk affects the asset | — | today's `risks.asset_id` and `risk_assets` |
| `applies_to` | `ControlImplementation` | `Asset`, `System`, `Service` *(shape deferred)* | N : M | the implementation is in place on that target | — | #553 criterion 2 |
| `caused_by` | `Finding` | `Threat` *(shape deferred)* | N : M | the finding is caused by the threat | — | original note |
| `requires` | `Finding` | `Action` *(shape deferred)* | 1 : N | the finding requires the action | — | original note; today's `vulnerabilities.ticket_*` |
| `treated_by` | `Risk` | `Mitigation` *(shape deferred)* | 1 : N | the risk is treated by the mitigation | — | today's `mitigations.risk_id` |
| `assesses` | `Assessment` | `Vendor` | N : 1 | the assessment assesses the vendor | — | ADR 0004 D3 (`vendor_asset_id`) |

**Two notes on the table.**
- `maps_to` connects **requirements**, not controls. Today `control_crosswalks` links two
  `compliance_controls` rows, which hold requirement text (Context 5). Once a requirement and a
  control are separated, a crosswalk says that ISO A.5.15 and SOC 2 CC6.1 ask for the same thing,
  which is what it always meant.
- ADR 0004 D2's vendor link verbs (`managed_by`, `hosted_by`, `processes_data_of`, `depends_on`)
  are edges **between assets**. They join this vocabulary with `Asset` in #1217, unchanged. This ADR
  defines no competing verb for them.

### D6 — The seven entities

Each table lists the entity's own fields; the envelope (D2) is implied. "Null" says whether a
field may be absent. Enumerations are closed: adding a value is an amendment of this ADR.

#### `Requirement`

| Field | Type | Null | Rationale |
|---|---|---|---|
| `framework` | `{key, name, version}` | no | which framework states it. `key` is the catalogue key (`compliance_frameworks.catalog_key`), empty for a tenant-authored framework. A framework is a grouping, not an entity: nothing points at it except through its requirements. |
| `reference_code` | string ≤ 50 | no | the identifier the source uses, e.g. `A.5.15`. Empty is allowed for a tenant-authored requirement. |
| `title` | string ≤ 255 | no | short name |
| `statement` | text | no | the obligation as the source states it |
| `citation` | string ≤ 255 | **no for `regulatory_catalogue`**, yes otherwise | where exactly it comes from (section, article, circular). ISO/IEC 27001:2022 7.5.3 asks for the identification and source of documented information; this is that field. |
| `obligation` | enum `shall` · `should` · `may` · `unspecified` | no | the strength of the source's wording. `unspecified` is the honest value for imported catalogues that do not record it yet; it is never guessed. |
| `parent_id` | uuid | yes | a requirement that this one refines (a clause and its sub-clause) |
| `language` | locale code (ADR 0002) | no | the language the statement is written in |

#### `Control`

| Field | Type | Null | Rationale |
|---|---|---|---|
| `name` | string ≤ 255 | no | how the tenant names the control; may differ from every requirement it answers |
| `description` | text | yes | what the control does, in the tenant's words |
| `objective` | text | yes | what it is meant to achieve; what a test checks against |
| `owner_id`, `assignee_id`, `reviewer_id` | uuid | yes | the `domain.Ownership` block, unchanged. `Owner` is an entity in #1217; until then these are user ids. |
| `applicability` | enum `applicable` · `not_applicable` | no | scoping. ADR 0003 excludes `not_applicable` from both sums; this is the field it reads. |
| `applicability_justification` | text | **no when `not_applicable`** | an auditor asks why a control is out of scope, and a Statement of Applicability without a reason is not one |
| `declared_status` | enum `not_implemented` · `in_progress` · `implemented` | no | the tenant's claim, as today's `compliance_controls.status` minus `not_applicable`, which moved to `applicability`. #555 computes an effective status from tests and evidence; that computed status is **derived and not stored** in the contract. |
| `tags` | array of string | no, may be empty | free tenant labels; never a substitute for `addressed_by` |

#### `ControlImplementation`

| Field | Type | Null | Rationale |
|---|---|---|---|
| `control_id` | uuid | no | the control it implements; mirrors the `implemented_by` edge, so the 1 : N cardinality is a field, not a convention |
| `description` | text | no | *how* the control is implemented here. "Implemented" without a how is the declared status this entity exists to replace. |
| `status` | enum `planned` · `in_progress` · `in_place` · `retired` | no | its own lifecycle. `retired` keeps history when a system is decommissioned. |
| `in_place_since` | date | yes | the date an auditor asks for when sampling the period a control operated |
| `owner_id`, `assignee_id`, `reviewer_id` | uuid | yes | the `Ownership` block; the person who runs it here, not the control's owner |

Its targets (where it is in place) are `applies_to` edges (D5), because they are assets, systems
or services, whose shapes #1217 holds.

#### `ControlTest`

| Field | Type | Null | Rationale |
|---|---|---|---|
| `implementation_id` | uuid | no | mirrors `tested_by` |
| `name` | string ≤ 255 | no | shown in the test list and in the audit file |
| `checks` | text | no | what the test checks (#553 criterion 3) |
| `method` | enum `automated` · `manual` | no | how |
| `procedure` | text | **no when `manual`** | the steps a person follows |
| `collector` | string | **no when `automated`** | the collector or connector that runs it; its shape is #554's |
| `cadence` | ISO 8601 duration, e.g. `P30D`, `P3M` | yes | null means run on demand. ISO/IEC 27001:2022 9.1 asks what is monitored and when; this is when. |
| `pass_criteria` | text | no | what a pass means, written before the first run |
| `results` | array of `ControlTestResult` | no, may be empty | append-only, newest last |

`ControlTestResult` (sub-record, D1):

| Field | Type | Null | Rationale |
|---|---|---|---|
| `result_id` | uuid | no | the qualifier `evidenced_by` and `raised_by` point at |
| `outcome` | enum `pass` · `fail` · `inconclusive` | no | `inconclusive` is a run that could not decide (collector error, missing access). It never counts as a pass. |
| `observed_at` | timestamptz | **no** | when the test ran against the world |
| `recorded_at` | timestamptz | no | when OpenRisk stored it; the result-level `created_at` |
| `source`, `source_id`, `confidence` | as D2 and D3 | no | a manual run is `user_entered`, a collector run is `automated_scanner` or `connector_import` |
| `executed_by` | `{kind, id}` | no | the `provenance.actor` shape |
| `note` | text | yes | |

#### `Evidence`

| Field | Type | Null | Rationale |
|---|---|---|---|
| `title` | string ≤ 255 | no | how the artifact is found in the library |
| `kind` | enum `document` · `capture` · `configuration` · `attestation` · `log` | no | today's `EvidenceType`, unchanged |
| `description` | text | yes | what the artifact shows; may be the whole evidence for an inline statement |
| `file_ref` | string | yes | storage key of bytes OpenRisk holds; served only through the download path |
| `filename` | string ≤ 255 | yes | |
| `external_url` | URL | yes | evidence held in another system of record. At least one of `file_ref`, `external_url` or `description` is non-empty. |
| `valid_until` | timestamptz | yes | expiry; null never expires |
| `collected_by` | uuid | yes | the person who took the proof, which may differ from `provenance.actor` (who stored it) |
| `review` | `{verdict, note, reviewed_by, reviewed_at}` | no | `verdict` ∈ `pending` · `accepted` · `rejected`, today's `EvidenceReview`. A human verdict outranks the calendar (`Evidence.EffectiveStatus`). |
| `owner_id`, `assignee_id`, `reviewer_id` | uuid | yes | the `Ownership` block |

`observed_at` is required (D4). The effective status (`valid`, `expiring_soon`, `expired`,
`rejected`, `pending`) is derived from `review`, `valid_until` and the clock, exactly as
`Evidence.EffectiveStatus` derives it, and is not stored. The content hash, observation period and
chain of custody are #554's fields and are added by amendment there.

#### `Risk`

| Field | Type | Null | Rationale |
|---|---|---|---|
| `name` | string ≤ 255 | no | the register's label |
| `description` | text | yes | the scenario in words |
| `probability` | decimal 0.000–1.000 | no | Score Engine input |
| `impact` | decimal 0.0–10.0 | no | Score Engine input |
| `score` | `{value: decimal, criticality: enum low · medium · high · critical}` | no | **as computed by `pkg/scoring/engine.go`**, read-only in the contract. `value` is `Risk.Score`, `criticality` its band. No engine version is carried: none is persisted today, and the formula is frozen. The canonical model carries the score; it does not define it, and nothing that writes a canonical `Risk` may write `score`. |
| `residual` | `{value, band, formula_version}` | yes | ADR 0003's residual, `residual-v1` |
| `smart_score` | `{value: decimal 0–100, level: enum, factors: array of {key, label, weight, value, contribution, detail}, computed_at}` | yes | SmartScore, additive, from `pkg/scoring/smart.go`. Read-only like `score`. |
| `lifecycle_state` | enum, today's `RiskState` | no | the single source of truth for where a risk stands |
| `treatment` | enum `accept` · `mitigate` · `transfer` · `avoid` | no | the ISO 27005 treatment option; the plan itself is a `Mitigation` (#1217) |
| `acceptance` | `{accepted_at, approval_id, approved_by, expires_at, expired_at}` | yes | present only in or after `RESIDUAL_ACCEPTED` (#811) |
| `review_cadence` | `{interval_days, next_review_at, last_reviewed_at}` | yes | |
| `category` | `{id, name}` | yes | the tenant's controlled taxonomy. `RiskCategory` is not a canonical entity: it is a tenant vocabulary, like a tag list. |
| `tags` | array of string | no, may be empty | free labels, kept apart from `category` (`risk_taxonomy.go`) |
| `quantification` | `{currency, sle, aro, downtime_hours, hourly_downtime_cost, data_loss_cost, fines, other_direct_cost, remediation_cost, mitigation_effectiveness}` | yes | the CRQ inputs. Amounts carry an ISO 4217 `currency`, `XAF` for every value stored today (the `_xaf` columns); conversion for display is the presenter's job. |
| `custom_fields` | object | yes | tenant-defined fields, unvalidated by the contract |
| `owner_id`, `assignee_id`, `reviewer_id` | uuid | yes | the `Ownership` block |
| `created_by` | uuid | no | the account that created it; kept beside `provenance.actor` because legacy rows have it and no provenance |

`observed_at` is the last assessment date (today `last_reviewed_at`), nullable (D4). Assets,
mitigations and the findings that indicate it are edges (D5).

#### `Finding`

| Field | Type | Null | Rationale |
|---|---|---|---|
| `kind` | enum `vulnerability` · `control_gap` | no | `vulnerability`: today's register. `control_gap`: raised by a failing `ControlTestResult` (#553 criterion 5). Other kinds (audit observation, misconfiguration) are added by amendment when a module records them; inventing them now would be a shape with no data. |
| `title` | string ≤ 512 | no | the source's title |
| `description` | text | yes | the source's description |
| `severity` | enum `critical` · `high` · `medium` · `low` · `info` | no | today's `VulnSeverity` |
| `status` | enum `open` · `triaged` · `in_remediation` · `remediated` · `accepted` · `false_positive` | no | today's `VulnStatus`. `false_positive` sets `confidence` to 0.00 (D3). |
| `identifiers` | array of `{scheme, value}` | no, may be empty | `scheme` ∈ `cve` · `cwe` · `vendor_advisory`. Today `cve_id`. An array because one finding often carries a CVE and a vendor advisory id. |
| `cvss` | `{score: decimal 0.0–10.0, vector: string ≤ 128, version: string}` | yes | the technical severity as the source scored it |
| `exploitability` | `{epss: decimal 0–1, kev: bool, exploit_available: bool, exploit_maturity: enum none · poc · functional · high}` | yes | what makes a severe finding urgent |
| `priority` | `{score, tier, version}` | yes | from `pkg/vulnprio`, read-only like `Risk.score` |
| `first_observed_at` | timestamptz | no | when the source first observed it (today `first_seen`) |
| `sla_due_at` | timestamptz | yes | the remediation deadline under the tenant's policy (#902) |
| `remediated_at` | timestamptz | yes | what time-to-remediate is measured on (#909) |
| `remediation_hint` | text | yes | the source's fix advice |

`observed_at` is required and is the **most recent** observation (today `last_seen`); D4's second
example shows why both times exist.

### D7 — The mapping from today's persisted models

Every column is mapped to a canonical field, listed as **no canonical home** (persistence keeps
it, the contract does not expose it), or listed as **lost** (it does not survive into the
canonical record, and nothing reads it after migration). "Envelope" means the D2 field of the same
name. "Edge" means a D5 relationship.

#### `domain.Risk` — `backend/internal/domain/risk.go:186`, table `risks`

| Column | Canonical |
|---|---|
| `id`, `tenant_id`, `created_at`, `updated_at` | envelope |
| `organization_id` | **lost**: legacy alias of `tenant_id` |
| `name` | `name` |
| `title` | **lost**: alias of `name` |
| `description` | `description` |
| `probability`, `impact` | `probability`, `impact` |
| `score`, `criticality` | `score.value`, `score.criticality` |
| `impact_legacy`, `probability_legacy` | **lost**: the 1–5 scale |
| `status` | **lost**: derived from `lifecycle_state` on every write |
| `level` | **lost**: legacy band |
| `lifecycle_phase` | **lost**: derived from `lifecycle_state` |
| `lifecycle_state` | `lifecycle_state` |
| `owner_id`, `assignee_id`, `reviewer_id` (Ownership) | same names |
| `created_by` | `created_by`; also `provenance.actor` when the source is `openrisk_user` |
| `assigned_to` | **lost**: deprecated, backfilled into `assignee_id` (migration 0044) |
| `owner` | **lost**: deprecated free text |
| `asset_id` | edge `Risk --affects--> Asset` |
| `tags` | `tags` |
| `category_id` | `category.id` |
| `frameworks` | **lost**: frozen since migration 0046 |
| `control_ids` | **lost**: deprecated, migrated into `risk_control_mappings` by 0046 |
| `treatment_plan` | `treatment` |
| `residual_risk` | `residual.value` |
| `last_mitigated_at` | **no canonical home**: operational; derivable from mitigation history in #1217 |
| `accepted_at`, `acceptance_approval_id`, `acceptance_approved_by`, `acceptance_expires_at`, `acceptance_expired_at` | `acceptance.*` |
| `acceptance_reminder30_at`, `acceptance_reminder7_at` | **no canonical home**: reminder stamps |
| `sle_xaf`, `aro`, `downtime_hours`, `hourly_downtime_cost_xaf`, `data_loss_cost_xaf`, `fines_xaf`, `other_direct_cost_xaf`, `remediation_cost_xaf`, `mitigation_effectiveness` | `quantification.*`, `currency = XAF` |
| `smart_score`, `smart_level`, `smart_factors`, `smart_computed_at` | `smart_score.*` |
| `review_interval_days`, `next_review_at`, `last_reviewed_at` | `review_cadence.*`; `last_reviewed_at` is also `observed_at` |
| `source` | envelope `source`: `manual` → `openrisk_user`, `import` → `openrisk_import_file`, `cti_auto` → `openrisk_derived` with the CVE in `provenance.external_refs`, `scan_auto` → `openrisk_derived` with `derived_from` the finding, `vendor` → `openrisk_derived`, `ai` → `openrisk_ai`, `starter` → `openrisk_starter` |
| `source_cve_id` | `provenance.external_refs[{scheme: cve}]`: the CVE catalogue is global, not a tenant record |
| `source_vulnerability_id` | edge `Finding --indicates--> Risk` |
| `source_rule_reason` | `provenance.rule` |
| `external_id` | envelope `source_id` when the source is external; otherwise `source_id = id` |
| `custom_fields` | `custom_fields` |
| `deleted_at` | **no canonical home** (D2) |
| join `risk_assets (risk_id, asset_id)` — **no `tenant_id`**, gated through `risks` (ADR 0004 D2) | edge `Risk --affects--> Asset` |

`ai` and `starter` are class `derived`, with the inputs and confidences of D3's derived table:
`starter` at 0.70 (the user's selection), `ai` at 0.50 until a person accepts the proposal.

#### `domain.ComplianceFramework` — `compliance.go:31`, table `compliance_frameworks`

| Column | Canonical |
|---|---|
| `id` | **no canonical home**: a framework is not an entity (D6) |
| `tenant_id` | the `tenant_id` of each `Requirement` imported from it |
| `name`, `version`, `catalog_key` | `Requirement.framework.{name, version, key}` |
| `description` | **no canonical home** |
| `created_at`, `updated_at`, `deleted_at` | **no canonical home** |

#### `domain.ComplianceControl` — `compliance.go:62`, table `compliance_controls`

One row splits into **one `Requirement` and one `Control`**, joined by an `addressed_by` edge
with `coverage = full`. Both keep the row's `tenant_id`. The `Requirement` takes the row's `id`,
because every existing link points at that id and they are all links to requirement text.

**The `Control`'s id is deterministic**: `uuid.NewSHA1(ControlSplitNamespace,
compliance_controls.id)` (UUIDv5), with `ControlSplitNamespace` a constant of the canonical
package. Nothing is persisted until #553, so the projection runs on every read, and a random id
would change each time and break every `mitigated_by` and transitional `evidenced_by` edge that
points at it. #553 persists exactly these ids.

| Column | Canonical |
|---|---|
| `id` | `Requirement.id`; the `Control` gets the UUIDv5 above |
| `tenant_id`, `created_at`, `updated_at` | envelope of both |
| `framework_id` | `Requirement.framework` (through the framework row) |
| `reference_code` | `Requirement.reference_code` |
| `name` | `Requirement.title` and `Control.name` |
| `description` | `Requirement.statement` |
| `source_reference` | `Requirement.citation`. Required for `openrisk_catalogue`; `pkg/compliance` requires it of every catalogue entry (`catalog.go`). A catalogue-imported row that is empty anyway is a catalogue defect: the projection keeps the row, sets `source = openrisk_user`, and lists it for the compliance officer rather than inventing a citation. |
| `status` | `not_applicable` → `Control.applicability = not_applicable`, with `applicability_justification` empty and flagged for the tenant to fill; otherwise `Control.declared_status` |
| `evidence_count` | **not a column** (`gorm:"-"`); derived |
| `deleted_at` | **no canonical home** |

A requirement imported from a catalogue gets `source = openrisk_catalogue` and `source_version =`
the catalogue version. One built by hand gets `source = openrisk_user`.

#### `domain.Control` — `integration_models.go:36`, no table

The integration contract with OpenRMF. **It is not persisted**, so nothing migrates. Its fields
(`ID`, `Name`, `Framework`, `Status`) map to `Control.name`, `Requirement.framework.name` and
`Control.declared_status` when a connector (#543) imports through it. It is superseded by the
canonical `Control` and is retired in the follow-up implementation issue.

#### `domain.ControlCrosswalk` — `control_crosswalk.go:70`, table `control_crosswalks`

| Column | Canonical |
|---|---|
| `id`, `tenant_id`, `created_at`, `updated_at` | edge envelope |
| `source_control_id`, `target_control_id` | edge `Requirement --maps_to--> Requirement` (both ids are requirement ids after the split above) |
| `coverage`, `rationale`, `origin` | edge attributes |
| `created_by` | edge `source = openrisk_user`, `provenance.actor` |
| `deleted_at` | **no canonical home** |

#### `domain.RiskControlMapping` — `risk_taxonomy.go:158`, table `risk_control_mappings`

| Column | Canonical |
|---|---|
| `id`, `tenant_id`, `created_at`, `updated_at` | edge envelope |
| `risk_id`, `control_id` | edge `Risk --mitigated_by--> Control`. `control_id` is a `compliance_controls` id, so it resolves to the **`Control`** created by the split, not to the requirement. |
| `framework_id` with `control_id` set | **lost**: derivable from the control's requirement |
| `framework_id` with `control_id` null | **no canonical home**: "this risk relates to ISO 27001" without naming a control is a statement about a framework, which is not an entity. Persistence keeps it; the contract exposes no edge for it. |
| `note` | edge attribute `note` |
| `created_by`, `source` | edge `source` and `provenance` |
| `deleted_at` | **no canonical home** |

#### `domain.Evidence` — `evidence.go:134`, table `evidences`

| Column | Canonical |
|---|---|
| `id`, `tenant_id`, `created_at`, `updated_at` | envelope |
| `title`, `description`, `file_ref`, `filename`, `external_url`, `valid_until`, `collected_by` | same names |
| `type` | `kind` |
| `collected_at` | **`observed_at`** |
| `review`, `review_note`, `reviewed_by`, `reviewed_at` | `review.{verdict, note, reviewed_by, reviewed_at}` |
| `source` | envelope `source`: `manual` → `openrisk_user`, `scanner` → `openrisk_scanner`, `integration` → the integration's registry id, `automation` → `openrisk_derived` |
| `source_detail` | `provenance.via` |
| `owner_id`, `assignee_id`, `reviewer_id` | same names |
| `reminder_sent_at` | **no canonical home**: reminder stamp |
| `deleted_at` | **no canonical home** |
| computed `status`, `days_until_expiry`, `control_ids`, `controls`, `collected_by_email` | **not columns**; derived |

#### `domain.EvidenceControlLink` — `evidence.go:276`, table `evidence_control_links`

| Column | Canonical |
|---|---|
| `id`, `tenant_id`, `created_at` | edge envelope |
| `evidence_id`, `control_id` | edge `Control --evidenced_by--> Evidence` (**transitional**, D5) to the `Control` created by the split. #553 moves each link onto a test result where one exists and reports the links it could not place (#553 criterion 6). |
| `note` | edge attribute `note` |
| `linked_by` | `provenance.actor` |

#### `domain.ControlEvidence` — `compliance.go:101`, table `control_evidences`

**No canonical mapping.** The table is retired by #813 under D-056, once #813 criterion 1 proves
that every row exists in the evidence library. `domain.Evidence` is the only source the canonical
`Evidence` maps from. A row in `control_evidences` that is not in `evidences` is a defect for #813
to report, not a second input to this model.

#### `domain.Vulnerability` — `vulnerability.go:117`, table `vulnerabilities` (the finding register)

Every row is a `Finding` of kind `vulnerability`. ADR 0001 D5b established that the whole table,
`manual` rows included, is the register of what has been observed. That is why no row is
excluded.

| Column | Canonical |
|---|---|
| `id`, `tenant_id`, `created_at`, `updated_at` | envelope |
| `cve_id` | `identifiers[{scheme: cve}]` when non-empty |
| `title`, `description`, `severity`, `status`, `remediation_hint`, `sla_due_at`, `remediated_at` | same names |
| `cvss_score`, `cvss_vector` | `cvss.score`, `cvss.vector`; `cvss.version` parsed from the vector prefix, null when absent |
| `epss`, `kev`, `exploit_available`, `exploit_maturity` | `exploitability.*` |
| `priority_score`, `priority_tier` | `priority.score`, `priority.tier`; `priority.version` from `pkg/vulnprio` |
| `asset_id` | edge `Finding --affects--> Asset` |
| `match_confidence`, `match_method`, `match_ambiguous`, `match_candidates`, `match_resolved_by` | attributes of that `affects` edge (Context 3: attribution confidence is not the record's confidence) |
| `asset_criticality` | edge attribute `asset_criticality_at_observation` |
| `asset_name` | **lost**: a denormalised copy of the asset's name |
| `affected_assets_count` | **lost**: derived from the number of `affects` edges |
| `source` | envelope `source`, class `automated_scanner`: `nessus`, `openvas`, `qualys`, `ms_defender`, `aws_inspector`, `azure_defender`, `crowdstrike` keep their spelling; `scanner` → `openrisk_scanner`; `agent` → `openrisk_agent`; `manual` → `openrisk_user` (class `user_entered`) |
| `external_id` | envelope `source_id`. Empty → `<cve_id>\|<asset_id>`, the fallback `DedupKey()` already uses, so the identity rule and today's upsert agree. For `manual` rows with neither, the record's own `id`. |
| `risk_id` | edge `Finding --indicates--> Risk`. `risks.source_vulnerability_id` yields the same edge from the other end; the edge's identity (D5) makes the two one edge. If they disagree, both edges are kept, which `indicates`' N : M allows, and the follow-up implementation reports the pair. |
| `ticket_provider`, `ticket_key`, `ticket_url` | **no canonical home** until `Action` (#1217); then edge `Finding --requires--> Action` |
| `first_seen` | `first_observed_at` |
| `last_seen` | **`observed_at`** |
| `raw_data` | **no canonical home**; referenced by `provenance.raw_ref` |
| `deleted_at` | **no canonical home** |
| computed `priority_explanation` | **not a column** |

#### Other finding-bearing sources, and why they do not map in v1

| Model | Path | Canonical in v1 |
|---|---|---|
| `scanner.FindingDiscovery` | `backend/internal/scanner/scanner.go:115` | **none**: a transient preview, never persisted. It becomes a `Finding` only when written to `vulnerabilities`. |
| CTI threats (catalogue CVEs relevant to the tenant) | `backend/internal/domain/threat.go` | **none** as findings: they are intelligence about the world, not observations on the estate (ADR 0001 D5b). They belong to `Threat` (#1217), and the scanner-confirmed case is an edge `Finding --caused_by--> Threat`. |
| `domain.CTIThreatDismissal` | `threat.go:145` | **no canonical home**: a per-tenant view preference on a threat |
| `domain.ComplianceAudit.summary` | `compliance_audit.go` | **no canonical home**: free text. Audit observations become `Finding`s only when an audit module records them one by one (amendment, D6). |
| `domain.RemediationPlan` | `compliance_audit.go` | out of the slice: it is an `Action`, held by #1217 |

### D8 — `Vendor` and `Assessment` are adopted from ADR 0004 by reference

This ADR defines **no shape** for `Vendor` or `Assessment`. ADR 0004 D1 and D3 are their
canonical specification, and this ADR supersedes **no field** of them.

What adoption means, field by field:
- **`Assessment`** is `vendor_assessments`. Its `source`, `source_id`, `source_version`,
  `observed_at`, `confidence` and `provenance` are already the D2 envelope. `vendor_questionnaire`
  joins the registry as class `third_party_attestation`, with ADR 0004's 0.50 as that class's cap
  (D3). `provenance`'s keys (`channel`, `template_id`, `template_version`,
  `submitted_with_token_id`) are ADR 0004's, and are the `vendor_questionnaire` source's
  allowlist (D3). None is removed or renamed. **One addition, named here as D-044 requires:** the
  three keys D3 makes mandatory are added beside them, with fixed values for this source:
  `method = questionnaire`, `actor = {vendor_contact, vendor_contact:<assessment_id>}` (ADR 0004
  D4's own actor) and `confidence_rule = confidence/1`. Existing rows get them in the projection;
  no 0004 field changes meaning.
- **`Vendor`** is an asset of category `vendor`, and its envelope is the asset's. ADR 0004 D1
  deferred the asset envelope retrofit to #541. Under D-083 that retrofit moves with `Asset` to
  #1217 (D10). Until then `Vendor` has no `observed_at`, `confidence` or `provenance`, as ADR 0004
  states.
- The edge `Assessment --assesses--> Vendor` (D5) is ADR 0004's `vendor_asset_id`, read as an
  edge. It is not a new link.

### D9 — Where the Go types live, and the tests that guard them

**Package: `backend/internal/domain/canonical`.** A child of `domain`, not `domain` itself,
because 30 files in `domain` import GORM or Fiber (Context 6). The same arrangement as
`internal/domain/scoring`. It holds the envelope, the source registry, the relationship vocabulary,
the seven entity types and their validation. No repository and no persistence: mapping from GORM
models is written in `internal/infrastructure`, which may import the package, never the reverse.

**Tests named here, written in the follow-up implementation issue.**

| Test | Fails when |
|---|---|
| `TestCanonical_TenantIDRequired` | table-driven over every entity of the slice: constructing or validating any of them with `tenant_id == uuid.Nil` does not return an error satisfying `errors.Is(err, canonical.ErrValidation)`. ABSOLUTE RULE 2. |
| `TestCanonical_EveryEntityCarriesTheEnvelope` | reflection over every exported entity type in the package: one of them does not embed `Envelope`. This is what stops a twentieth entity from skipping `tenant_id`. |
| `TestCanonical_EdgeRefusesAnotherTenant` | an edge whose `to.tenant_id` differs from its `tenant_id` validates (pure; `to.tenant_id` is filled by persistence, D5) |
| `TestEdgeWrite_TargetInOtherTenant_404` | in persistence, against Postgres, table-driven over every verb of D5 including the shape-deferred ones: an edge whose target id exists in another tenant is written, or answers anything but 404. This is the test the pure package cannot prove on its own. |
| `TestCanonical_IdentityKeyIncludesTenant` | the same `(source, source_id)` in two tenants yields one record instead of two |
| `TestCanonicalWrite_EnvelopeNotClientWritable` | a request body carrying `tenant_id`, `source`, `source_id`, `confidence` or `provenance` changes the stored value |
| `TestCanonical_EdgeVocabularyIsClosed` | a verb, or a `(verb, source type, target type)` triple, not in D5 validates |
| `TestCanonical_ConfidenceCeilingPerClass` | a record above its class's ceiling (D3) validates, or any default produces 1.00 |
| `TestCanonical_ObservedAtRequired` | an `Evidence`, a `Finding` or a `ControlTestResult` without `observed_at` validates |
| `TestCanonical_ProvenanceRefusesUnknownKeys` | a `provenance` key outside D3 and outside its source's allowlist validates |
| `TestCanonical_ProvenanceRefusesCredentialShapedValues` | a `provenance` string over 255 characters, or a URL with userinfo or a signature/token query parameter in `provenance` or `Evidence.external_url`, validates |
| `TestCanonical_ImportsOnlyStdlibAndUUID` | any non-test file in `internal/domain/canonical` imports anything other than the standard library and `github.com/google/uuid`. An **allowlist**, so `gorm.io`, `gofiber`, `net/http`, `database/sql` and `internal/domain` itself (which would bring GORM in transitively) are all refused without being named. Built on the `go/parser` pattern of `pkg/vendorscore/imports_test.go`. |
| `TestScoring_DoesNotReferenceConfidence` | `pkg/scoring` or `internal/domain/scoring` imports `internal/domain/canonical`, or names a `Confidence` identifier. Confidence never reaches `Risk.Score` or SmartScore (D3). |
| `TestCanonical_RiskScoreIsReadOnly` | anything in the package computes or sets `Risk.score` or `Risk.smart_score` from canonical fields, or imports `pkg/scoring`. The canonical `Risk` carries the frozen score; it never produces one. |

**Typed errors.** The allowlist forbids `internal/domain`, so the package declares its own
sentinel, `canonical.ErrValidation`. The infrastructure mapping layer wraps it into
`domain.ErrValidation` (`internal/domain/errors.go:19`), so HTTP error handling is unchanged.
Validation is the only error a pure package can raise; `ErrNotFound`, `ErrForbidden` and
`ErrConflict` stay where they are.

Each use case that later reads or writes canonical records keeps the CLAUDE.md minimum
(`_Success`, `_NotFound`, `_Unauthorized`). This ADR adds no use case.

### D10 — Out of scope for v1, and where each item lives

| Not in v1 | Why | Held by |
|---|---|---|
| `Asset` | the largest existing shape (`assets`, its attribute schemas, its dependency edges); its envelope retrofit touches every asset at once | #1217 |
| The `assets` envelope retrofit (`observed_at`, `confidence`, `provenance`), deferred by ADR 0004 D1 and D8 | moves with `Asset` (D-083) | #1217 |
| `Threat` | CTI catalogue entries and their relevance (`threat.go`) are intelligence, not tenant observations; the shape needs the CTI owner's reading | #1217 |
| `Incident` | the incident module has its own lifecycle and timeline (`incident.go`); the Evidence Fabric does not read it | #1217 |
| `Mitigation` | `mitigations` and its sub-actions; the chain reaches it only through `treated_by` | #1217 |
| `Action` | today spread over mitigation sub-actions, remediation plans and external tickets; unifying them is its own design | #1217 |
| `Owner` | today the `Ownership` block of user ids; a canonical owner (person, team, role) is an identity design | #1217 |
| `System` | no model exists today; inventing one here would be a shape with no data | #1217 |
| `Service` | no model exists today; same reason | #1217 |
| `Policy` | no model exists today; same reason | #1217 |
| `Signal` | the event intake of the Risk Data Plane; no model exists today | #1217 |
| `openrisk-schema.json` and its compatibility policy | the irreversible step; after acceptance only (D-030) | #542 |
| Data contracts and source mappings | | #543 |
| OSCAL import and export | | #566 |
| The evidence record's hash, observation period and chain of custody | | #554 |
| Any change to `Risk.Score` or SmartScore | frozen; an engine ADR and the owner's approval | — |
| Migrating any table to the split `Requirement` / `Control` | this ADR is a contract, not a migration | #553 |

## Consequences

**Easier**
- One answer to "where does this come from, when was it true, how sure are we", on every record,
  in the same three fields.
- A source that corrects itself is re-applied by its `(source, source_id)` and a new
  `source_version`, rather than by bespoke matching code per module.
- Freshness has one rule, on one field, and old proof uploaded today cannot read as fresh.
- `Finding` has one shape. A vulnerability and a failed control test are both findings and both
  reach a risk through `indicates`.
- #553 can be refined straight away: its chain is five entities and four verbs that this ADR
  names.

**Harder**
- `compliance_controls` splits into two records. Every screen that shows "a control" today shows
  a requirement and a control after #553. The split is the cost of saying that one control answers
  three frameworks.
- The confidence defaults (0.70, 0.80, 0.40, 0.60, 0.90) are judgements, written down so they
  can be argued with. Changing one later is a versioned rule change, not a silent edit.
- `ControlTestResult` is a sub-record carrying most of an envelope. If results ever need their own
  edges beyond `evidenced_by` and `raised_by`, it becomes the twentieth entity by amendment.
- Ten entities wait for #1217, and `affects`, `applies_to`, `caused_by`, `requires` and
  `treated_by` point at shapes that are not yet fixed. Their edges are valid now; their targets'
  fields are not.

## Alternatives rejected

**A — All 17 entities in one ADR.** Rejected by D-083: it puts the Evidence Fabric behind ten
entities it does not read.

**B — `ControlImplementation` and `ControlTest` as sub-records of `Control`.** Rejected by D-083
(option C) and D1: the envelope on a test result is what the Evidence Fabric needs most, and a
sub-record cannot be the target of an edge.

**C — Put the canonical types in `internal/domain`.** Rejected by D9: the purity test would fail
on 30 existing files the day it lands.

**D — One `confidence` field that also carries asset attribution.** Rejected by Context 3: "is it
real" and "is it on this host" are different questions with different evidence.

**E — `source` as a structured `class/system` string.** Rejected: it would change ADR 0004's
persisted `vendor_questionnaire` value, which D-044 forbids this ADR to supersede silently. A flat
identifier plus a registry gives the class without rewriting a stored value.

**F — Store inverse verbs (`addresses`, `evidences`) as edges of their own.** Rejected by D5:
two stored directions of one relation drift, as ADR 0001 D6b recorded for two vocabularies.

**G — Map `control_evidences` as a second evidence source.** Rejected by D-056: it is retired, and
the evidence library is the only register the product reads.
