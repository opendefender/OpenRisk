# 0004 — Vendor and Assessment: the TPRM v1 slice of the canonical model

Status: **proposed**
Proposed: 2026-09-15, on #668
Decided by: D-044 (`docs/DECISIONS.md`) — the order, the public-link model, the scoring
boundary and the plan. This ADR specifies the shapes inside those four answers.
Epic: #214 · Canonical model: #541 · Scope spike: #495
Implemented by (blocked on this ADR's acceptance): #669 register and chain (D1, D2, D7) ·
#670 templates, assessments and public link (D3, D4) · #671 score (D5) · #672 reminders (D6) ·
#673 tenant screens · #674 public questionnaire page

## Context

The owner decided to build TPRM v1: a vendor register, questionnaires sent by public link,
scoring, J-7/J-3/J-1 reminders, and a vendor→asset→risk link. `Vendor` and `Assessment` are two
of the 17 entities of the OpenRisk Canonical Model (#541), and D-030 forbids code on them
before their ADR. This ADR is that ADR for those two entities only.

Five things were read rather than assumed. Three of them change what a blank-page design would
have produced.

**1. A vendor already exists, as an asset.** ADR 0001 (D5b and "Neutral but recorded") settled
that a vendor is an asset of category `vendor` (`domain.CategoryVendor`,
`internal/domain/asset_schema.go:39`). It said no vendors table exists and that inventing one
would be a fiction. The category has a real attribute schema
(`internal/domain/asset_schema_defaults.go`): `legal_name`, `country`, `service_provided`,
`contract_reference`, `contract_end`, `contact_email`, `data_shared`, `dpa_signed`,
`certifications`, `service_criticality` and `last_assessment`. The drawer resolves
`/entities/vendor/:id` and refuses a non-vendor asset (`resolver_asset.go`).

**2. The vendor→asset edge already exists too.** `AssetDependency`
(`internal/domain/asset_dependency.go`) is a directed, tenant-scoped edge between two assets.
Its vocabulary already names the supplier relations: `managed_by` ("an asset is operated/managed
by a supplier/user"), `hosted_by` / `hosted_on`, and `processes_data_of` ("typically a
Data/Processing or Vendor asset"). Its create use case already checks that both ends exist in
the caller's tenant. Risks link to assets through `risk_assets` (`internal/domain/risk.go:352`).

**3. "Assessment" is already taken, twice, in unrelated packages.** `pkg/pwpolicy.Assessment` and
`pkg/crq.FinancialAssessment` exist. A domain type named plain `Assessment` would be ambiguous in
review and in search. The domain name is therefore **`VendorAssessment`**. The canonical entity
name in #541 stays `Assessment`: this is its vendor-questionnaire kind.

**4. A hashed bearer-token pattern exists and is sound.** `domain.NewInvitationToken`
(`internal/domain/invitation.go:147`) draws 32 bytes from `crypto/rand`, encodes them base64url,
and stores only a SHA-256 hex digest. `InvitationTokenMatches` compares in constant time. The
invitation service answers 404 for unknown, malformed and foreign tokens alike.

**5. A deadline-reminder pattern exists.** `MitigationDueWorker`
(`internal/infrastructure/workers/mitigation_due_worker.go`) ticks hourly. It applies "past the
threshold and not yet sent", not "exactly on the day", and stamps each reminder once
(`ReminderD7SentAt`, `ReminderD1SentAt`).

## Decision

### D1 — A vendor stays an asset of category `vendor`. No vendors table.

TPRM v1 adds no table for the vendor itself. The register lists assets where
`category = 'vendor'`. Everything TPRM adds hangs off the vendor's asset id.

Why, against a `vendors` table:
- It would duplicate `name`, `owner`, criticality and the eleven attributes above, and the two
  copies would drift.
- It would split the drawer: ADR 0001's `vendor` type would have to choose a table, and every
  existing vendor asset would need migrating.
- An asset already carries exactly what a vendor needs in order to be linked, scored against and
  shown in the Asset Universe.

**The consequence, stated rather than hidden:** the database cannot enforce that an assessment's
`vendor_asset_id` points at a vendor. A foreign key reaches `assets`, not "assets of category
vendor". **The use case enforces it on every write**, and a test proves that sending a
questionnaire to a server asset answers 404, which is D5b's rule applied to writes.

**Canonical envelope for Vendor.** The vendor's envelope is the asset's. `assets` does not carry
`observed_at`, `confidence` or `provenance` today. **This ADR does not add them.** Retrofitting
the asset envelope is #541's job, for all assets at once. Doing it here for vendors alone would
create two asset shapes.

### D2 — The vendor→asset→risk chain uses the edges that exist. No new vocabulary.

A **vendor link** is an `AssetDependency` edge with the vendor at one end. Only these verbs
qualify:

| Edge as stored | Reads | Vendor end |
|---|---|---|
| `asset --managed_by--> vendor` | the asset is operated by the vendor | target |
| `asset --hosted_by--> vendor` (and its alias `hosted_on`) | the asset is hosted by the vendor | target |
| `vendor --processes_data_of--> asset` (and its alias `stores_data_in`) | the vendor processes this asset's data | source |
| `asset --depends_on--> vendor` | generic fallback | target |

The TPRM link-creation endpoint offers exactly these verbs. It creates the edge through the
**existing** `AssetDependency` use case, so the cross-tenant, self-reference and duplicate
guards are the ones already tested. It adds one guard: the vendor end must be category `vendor`.

**Reading the chain.** One hop from the vendor to assets, then assets to risks:
1. The vendor is loaded by id **and** `tenant_id` **and** `category = 'vendor'`. Otherwise 404.
2. Edges come from `asset_dependencies` filtered by `tenant_id` and by the qualifying verbs, with
   the vendor as the matching end.
3. Linked assets come from `assets` filtered by `tenant_id` (`OwnColumn`).
4. Risks come from `risk_assets`, **which has no `tenant_id`**. They are gated **through the
   parent**: `JOIN risks r ON r.id = ra.risk_id AND r.tenant_id = ?` (`ThroughParent{Parent:
   risk}`, in ADR 0001's terms). Stated here, as RULE 2 requires of a table without its own column.

Multi-hop chains (a vendor's vendors, concentration) are out of scope: #376.

### D3 — `VendorAssessment` and its children: four new tables, all with their own `tenant_id`

Every new table carries `tenant_id uuid NOT NULL`, indexed, and every repository query filters
on it (`OwnColumn`). Children carry their own `tenant_id` even though a parent could gate them.
That makes the filter visible in every query text, which is where reviewers and
`internal/security/isolation` look. No `ThroughParent` gate is needed for any TPRM table.

**`vendor_questionnaire_templates`** — a reusable questionnaire the tenant authors.

| Field | Type | Null | Notes |
|---|---|---|---|
| `id` | uuid | no | |
| `tenant_id` | uuid | no | |
| `name` | text | no | |
| `description` | text | yes | |
| `language` | varchar(8) | no | a code from the locale registry (ADR 0002); the language the questions are written in |
| `version` | int | no | incremented on every edit to its questions |
| `archived_at` | timestamptz | yes | archived templates cannot be sent; they are never deleted once used |
| `created_by`, `created_at`, `updated_at` | | | |

**`vendor_questionnaire_questions`** — the questions of a template's current version.

| Field | Type | Null | Notes |
|---|---|---|---|
| `id`, `tenant_id`, `template_id` | uuid | no | |
| `position` | int | no | order |
| `text` | text | no | |
| `help` | text | yes | |
| `answer_type` | varchar(16) | no | `choice` or `text` — see D5 |
| `options` | jsonb | yes | required for `choice`: `[{value, label, points}]`, `points ∈ [0,1]`, where 1 is the fully favourable answer |
| `weight` | numeric(4,1) | no | `0.0–10.0`; forced to 0 for `text` |
| `required` | bool | no | |
| `na_allowed` | bool | no | whether "not applicable" is an accepted answer |
| `control_ref` | text | yes | an optional citation such as a control identifier. Free text, **never generated**: an empty field is honest, a fabricated mapping is not |

A yes/no question is a `choice` with two options, Yes → 1 and No → 0 by default, and editable,
because for some questions "No" is the favourable answer.

**`vendor_assessments`** — one questionnaire sent to one vendor. The canonical `Assessment`.

| Field | Type | Null | Notes |
|---|---|---|---|
| `id`, `tenant_id` | uuid | no | |
| `vendor_asset_id` | uuid | no | FK `assets`; category `vendor` enforced by the use case (D1) |
| `template_id` | uuid | no | the template it came from |
| `template_version` | int | no | the version snapshotted at send |
| `status` | varchar(16) | no | `sent` · `in_progress` · `submitted` · `revoked` |
| `owner_user_id` | uuid | no | the tenant user responsible; receives in-app reminders |
| `contact_email` | text | no | defaults to the vendor's `contact_email` attribute; editable at send |
| `contact_language` | varchar(8) | no | the language of the e-mails; a locale-registry code |
| `due_at` | timestamptz | no | the deadline, stored in UTC |
| `sent_by`, `sent_at` | | no | |
| `submitted_at` | timestamptz | yes | |
| `revoked_at`, `revoked_by` | | yes | |
| `score` | numeric(5,2) | yes | D5; null until submitted, and null when nothing is scorable |
| `tier` | varchar(16) | yes | D5 |
| `score_breakdown` | jsonb | yes | D5; the per-question arithmetic |
| `scoring_version` | varchar(32) | yes | `vendorscore/1` |
| `reminder_d7_sent_at`, `reminder_d3_sent_at`, `reminder_d1_sent_at` | timestamptz | yes | D6 |
| `source` | varchar(32) | no | `vendor_questionnaire` |
| `source_id` | text | no | the assessment's own id: the questionnaire is the source record |
| `source_version` | text | no | `template_version`, as text |
| `observed_at` | timestamptz | yes | **= `submitted_at`**: when the vendor stated their posture |
| `confidence` | numeric(3,2) | no | D3a |
| `provenance` | jsonb | no | D3a |
| `created_at`, `updated_at` | | no | `created_at` = when we sent it, which is not when the world was in that state |

**`vendor_assessment_items`** — the questions **snapshotted at send**, with the answers.

| Field | Type | Null | Notes |
|---|---|---|---|
| `id`, `tenant_id`, `assessment_id` | uuid | no | |
| `position`, `text`, `help`, `answer_type`, `options`, `weight`, `required`, `na_allowed`, `control_ref` | | | copied from the question at send |
| `answer_value` | text | yes | the chosen option's `value`, or the free text |
| `answer_na` | bool | no | default false |
| `answer_comment` | text | yes | the vendor's comment on this question |
| `answered_at` | timestamptz | yes | |

**Why snapshot.** Editing a template after sending it must not change what a vendor was asked,
nor the score of an assessment already submitted. A GRC record that rewrites itself is not a
record. The snapshot costs one row per question per assessment, and it is the only design where
the breakdown in D5 stays true forever.

**Status transitions.**
- `sent → in_progress` on the first saved answer.
- `in_progress | sent → submitted` on submit, only if every `required` item has an answer or,
  where allowed, N/A.
- `sent | in_progress → revoked` by a tenant user.

`submitted` and `revoked` are terminal. **Expired is not a status.** It is derived from `due_at`
plus the grace period (D4), exactly as `Invitation.State(now)` derives it. So no job has to flip
rows on time, and no row can be stale.

All multi-table writes run in one transaction: send (assessment, items and token), submit
(items, assessment and score) and resend (supersede and mint).

#### D3a — `confidence` and `provenance` for a vendor assessment

`confidence` uses #541's 0.0–1.0 scale, with this ADR fixing the semantics **for this one source
class**:

- **0.50: self-attested.** The vendor answered about themselves and nobody has verified it.
  **Every v1 assessment is 0.50**, and v1 has no path that raises it.
- Reviewer verification and evidence-backed answers are the reasons it could rise. Both are out
  of scope (D8), and this ADR does not pre-assign them values.

`provenance` records how the answers arrived, and no personal data beyond what the tenant
already holds:

```json
{ "channel": "public_link", "template_id": "…", "template_version": 3,
  "submitted_with_token_id": "…" }
```

**No IP address and no user agent.** They are personal data of a person who has no account and
no privacy notice from us. Storing them is a data-protection decision this ADR does not take.

**`observed_at` vs `created_at`, worked example.** A questionnaire is sent on 1 March
(`created_at`). The vendor submits on 20 March, describing controls as they stood that day
(`observed_at = 20 March`). A reviewer reading the record on 1 June sees a posture observed 73
days earlier, not 92.

### D4 — The public link

**Token.** 32 bytes from `crypto/rand`, base64url, SHA-256 hex stored, constant-time comparison.
The construction is `domain.NewInvitationToken`'s. The implementation may share that helper or
mirror it, but it must not weaken it.

**Tokens get their own table, because a relaunch must send a working link.** Only a hash is
stored (D-044), so the server can never mail the same link twice. Reminders (D6) and resends
therefore issue a **new** token and supersede the previous one:

**`vendor_assessment_tokens`**

| Field | Type | Null | Notes |
|---|---|---|---|
| `id`, `tenant_id`, `assessment_id` | uuid | no | |
| `token_hash` | char(64) | no | unique index |
| `reason` | varchar(16) | no | `send` · `reminder` · `resend` |
| `issued_at` | timestamptz | no | |
| `superseded_at` | timestamptz | yes | set when a newer token is issued |

At most one token per assessment has `superseded_at IS NULL`. A partial unique index enforces it.

**The token never touches a URL the server sees.** A token in a path or query string lands in
access logs, reverse-proxy logs and `Referer` headers. That would put a secret in logs, which
RULE 6 forbids.
- The e-mailed link is `https://<host>/vendor-questionnaire#<token>`. The fragment is not sent
  to the server when the page loads.
- The page reads the fragment and calls the API with the header
  `X-Vendor-Assessment-Token: <token>`.
- The page is served with `Referrer-Policy: no-referrer`. Every API response carries
  `Cache-Control: no-store`.

**Routes.** They are mounted on `app` **before** the JWT gate, like
`/api/v1/invitations/accept`. They take **no** auth middleware of any kind.

| Route | Does |
|---|---|
| `GET /api/v1/public/vendor-assessment` | returns the questionnaire |
| `PUT /api/v1/public/vendor-assessment/answers` | saves a draft: a partial set of `{item_id, answer_value, answer_na, answer_comment}` |
| `POST /api/v1/public/vendor-assessment/submit` | locks the answers and computes the score (D5) |

**What `GET` exposes, and nothing more:**
- the requesting organisation's display name;
- the vendor's name as registered;
- `due_at`, `status` and the template language;
- the items (text, help, type, options without `points`, required, N/A allowed) and the saved
  answers.

**`points`, `weight`, `score`, `tier`, the owner and every internal field are not exposed.** A
vendor who can see the weights can answer to the score.

**The tenant comes from the token row, and from nowhere else.** The handler does not read a
tenant from a header, the body, a cookie or a session. Every query after the lookup uses
`token.tenant_id`, and the assessment is re-loaded by `(id, tenant_id)`.

**Status contract.** Every answer that says nothing about a real record is indistinguishable,
in status and body:

| Situation | Status |
|---|---|
| header missing, malformed, unknown, or a token of another deployment | **404** |
| token superseded by a newer link | **410** "a newer link was sent to this address" |
| assessment revoked | **410** |
| now > `due_at` + **7 days** grace, and not submitted | **410** expired |
| tenant no longer holds the TPRM entitlement | **410**, generic; the vendor is not told about plans |
| `PUT` or `POST` after submission | **409**; `GET` still answers 200, read-only, until the grace period ends |
| rate limit | **429** |

Superseded tokens answer 410, not 404: a vendor clicking an older e-mail was a legitimate holder
of that link and must be told what to do. That is the same reasoning the invitation service
records for revoked and expired invitations.

**Rate limits**, with `middleware.RateLimit` on the Redis store:
- 60 requests per minute per client IP, on all three routes;
- 300 per hour per token hash;
- 10 submits per hour per token hash.

**Audit.** The mutation middleware skips requests with no organisation (`OrganizationID ==
uuid.Nil`), and these routes have none. **The use cases therefore write the audit events
themselves**, with the tenant from the token row and a system actor of the form
`vendor_contact:<assessment_id>`. Events: `vendor_assessment.answers_saved` (debounced to one per
token per 10 minutes) and `vendor_assessment.submitted`.

**Isolation registry.** The three routes are added to `internal/security/isolation` with the
decision "tenant derived from bearer token row".

### D5 — The vendor score: `pkg/vendorscore`, pure, and never an input to `Risk.Score`

A pure, deterministic function, with no database, clock or HTTP:

```
for each item with answer_type = choice, weight w > 0, and answer_na = false:
    p = points of the chosen option        (an unanswered item scores p = 0)
score = 100 × Σ w·(1 − p) / Σ w           over those items
```

- **Higher is riskier**, on the same reading as every other number in the product: a vendor
  answering every weighted question favourably scores 0, and one answering none favourably scores 100.
- **N/A** removes the item from both sums. It neither helps nor hurts.
- **Unanswered** optional items count as `p = 0`, the least favourable answer. Silence is not
  assurance.
- **`text` items** have weight 0 and never score.
- **Nothing scorable** (`Σ w = 0`) makes `score` and `tier` null, shown as "not scorable". It
  is never shown as 0.

**Tier**, using the Score Engine's own band proportions (7.0 / 4.0 / 2.0 on its 0–10 scale),
multiplied by ten so that the two scales read alike:

| Tier | Score |
|---|---|
| `critical` | ≥ 70 |
| `high` | ≥ 40 |
| `medium` | ≥ 20 |
| `low` | < 20 |

**Breakdown**, stored in `score_breakdown` and returned to tenant users:
`[{item_id, weight, points, na, contribution}]`, where `contribution = 100·w·(1 − p) / Σ w`. The
contributions sum to the score. The UI shows the arithmetic and never recomputes it.

`scoring_version = "vendorscore/1"`. A future formula gets a new version, and stored scores
are never silently recomputed.

**The boundary (D-044):**
- `pkg/vendorscore` is not imported by `pkg/scoring`, and nothing in the risk scoring path
  imports it.
- Submitting an assessment writes to `vendor_assessments` and `vendor_assessment_items` only,
  plus the vendor's `last_assessment` attribute (a date the schema already defines).
- It writes **no** risk, no SmartScore input, and no asset criticality. A test asserts that every
  linked risk's `Score` is unchanged after a submission.
- `service_criticality` is shown **beside** the score, never combined with it. Combining them
  would be a second formula, and it is not decided (D8).

### D6 — J-7 / J-3 / J-1 reminders

A `VendorAssessmentDueWorker`, modelled on `MitigationDueWorker`. It sweeps hourly, and once at
boot.

- **Clock.** `days = ceil((due_at − now) / 24h)`, in UTC.
- **Offsets.** 7, 3 and 1. The rule is "past the threshold and not yet sent", not "exactly on
  the day".
- **At most one reminder per sweep per assessment.** If several thresholds have been passed (a
  long outage, or an assessment sent 2 days before its deadline), **the most imminent** is sent,
  and it and every larger offset are stamped. A vendor never receives J-7, J-3 and J-1 in the
  same hour.
- **Eligible:** status `sent` or `in_progress`, `now < due_at`, and a tenant holding the TPRM
  entitlement. Submitted, revoked and expired assessments are never reminded.
- **Cross-tenant by necessity.** A scheduled job has no session, so the store query spans tenants.
  Every row carries its own `tenant_id`, and that id, and nothing else, addresses the
  notification and the audit event. The same statement is in `MitigationDueStore`.
- **Recipients.**
  - The vendor contact gets an e-mail in `contact_language`, with a **new** token (D4, reason
    `reminder`) that supersedes the previous one.
  - The owner gets an in-app notification through the existing notification use case.
- **Idempotence.** Minting the token, stamping the reminder and the audit event are one
  transaction. The e-mail is sent after commit. A failed send is logged with the assessment id,
  never the token, and is not retried by stamping.

  The consequence, stated: a lost e-mail means a missed reminder. It never means a duplicate,
  and the vendor's previous link then answers 410 while the new one never arrived. The owner's
  in-app notification still fires, and the owner can resend (D4).

### D7 — Gating and permissions

- **Entitlement.** `FeatVendorRisk Feature = "vendor_risk"` in `pkg/entitlements`, added to
  `AllFeatures`: `LevelOn` on Business and Enterprise, off on Free and Pro (D-044). Every
  authenticated TPRM route sits behind `RequireFeature(FeatVendorRisk)` and answers 402. The
  frontend shows `FeatureGate` / `UpsellLock`, as the existing premium modules do.
- **Downgrade.** Data is kept. Authenticated TPRM routes answer 402. Public links answer 410
  (D4). The worker skips the tenant. Vendor **assets** stay visible in the inventory, because
  assets are not gated.
- **Permissions.** Two new keys, mirroring the asset permissions rather than inventing a role
  model:
  - `vendors:read`, granted wherever `assets:read` is: the register, the chain, and reading
    assessments;
  - `vendors:manage`, granted wherever `assets:update` is: links, templates, sending, revoking
    and resending.

  Adding keys to the existing RBAC is feature work. It changes no part of the auth design.

### D8 — Out of scope for TPRM v1, and where each item lives

| Not in v1 | Why | Held by |
|---|---|---|
| File or evidence uploads through the public link | an unauthenticated upload surface needs malware scanning, size and type policy, and storage quotas: its own design | new issue, to open when v1 ships |
| Reviewer verification, and any `confidence` above 0.50 | needs a review workflow and evidence | same issue as above |
| Vendor accounts, portal or one-time code | declined by D-044 | — |
| Vendor score feeding `Risk.Score`, SmartScore or criticality | declined by D-044; would need an engine ADR | — |
| Combining `service_criticality` with the score (inherent vs residual vendor risk) | a second formula, not decided | #373 |
| Automated onboarding and inherent-risk tiering | | #373 |
| Continuous monitoring and re-scoring from external signals | | #375 |
| Fourth-party and concentration risk, multi-hop chains | | #376 |
| Peer benchmarking | | #321 |
| Third-party intelligence network and connectors | | #372, #374 |
| A built-in regulatory questionnaire (for example derived from ISO 27001 or COBAC) | content must be sourced and verified by the compliance officer before it ships; v1 ships the authoring tool and **no pre-filled template** | new issue, compliance-officer owned |
| `observed_at`, `confidence` and `provenance` on `assets` | the envelope retrofit is for all assets at once | #541 |
| IP or user-agent capture on submission | a data-protection decision | — |

## Consequences

**Easier**
- No second shape for a vendor. The register, the drawer and the Asset Universe all show the
  same row.
- The chain inherits the dependency-edge guards already proven live (ADR 0001, JOURNAL item 22).
- A submitted assessment is a permanent record: it is snapshotted, it keeps its breakdown, and
  its formula is versioned.
- The public surface is three routes with one gate: the token row.

**Harder**
- "An assessment belongs to a vendor" is a use-case rule, not a database constraint (D1). It is
  tested, but a raw SQL write could break it.
- Every reminder invalidates the vendor's previous link (D4 and D6). A vendor who bookmarked the
  first e-mail is told, with a 410, to use the latest one. That is the price of hash-only storage,
  paid knowingly.
- Four new tables plus a token table, each with its own `tenant_id` and an isolation test.
- The mutation middleware does not cover the public routes, so their audit is written by hand
  (D4) and can be forgotten by a later route. A test asserts that each public mutation writes an
  event.

## Alternatives rejected

**A — A `vendors` table.** Rejected by D1. It contradicts ADR 0001, duplicates eleven attributes,
and forces a migration of every existing vendor asset for no capability the category lacks.

**B — Store the token encrypted, so reminders can re-send the same link.** Rejected: D-044 chose
"stored hashed only". An encrypted copy means a database dump plus the key yields live links. The
token table (D4) keeps hash-only storage and still lets every reminder carry a working link.

**C — The token in the URL path (`/q/<token>`), as simpler routing.** Rejected: path segments are
logged by every proxy and access log, and leak through `Referer`. It would violate RULE 6 the
first day it ran.

**D — Questions referenced live from the template, not snapshotted.** Rejected by D3: editing a
template would rewrite what a vendor was asked and invalidate a stored breakdown.

**E — A new edge verb such as `supplied_by`.** Rejected by D2: `managed_by`, `hosted_by` and
`processes_data_of` already describe the supplier relations, and edges in the wild already carry
them. A synonym would split the graph.

**F — Score as assurance, higher is better.** Rejected by D5: every other score in OpenRisk rises
with risk. A vendor score that falls as risk rises would be read backwards on the first dashboard
that places them side by side.

**G — Expired as a stored status flipped by a job.** Rejected by D3: a derived state cannot go
stale, and `Invitation.State(now)` already proves the pattern.
