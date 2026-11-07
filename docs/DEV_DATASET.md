# Development dataset

`make seed-dev` fills a running local stack with one complete organisation, so
every screen opens on real content instead of its empty state. It also creates
a second, small organisation so you can check org switching and tenant
isolation by hand (#1337).

The data is in `dev/fixtures/dev-dataset.json`. The loader is
`scripts/seed-dev.mjs`.

## Run it

```bash
make docker-up      # or any stack whose API answers on localhost
make seed-dev
```

`make seed-dev` reads the seed admin password from the backend container
(`/app/secrets/initial_admin_password`) and calls the API on
`http://localhost:${OPENRISK_DEV_BACKEND_PORT:-8080}/api/v1`. Without the
compose stack, set the variables yourself:

```bash
SEED_ADMIN_PASSWORD='<the INITIAL_ADMIN_PASSWORD the backend started with>' \
SEED_API_URL=http://localhost:8080/api/v1 \
node scripts/seed-dev.mjs
```

| Variable | Default | Purpose |
|---|---|---|
| `SEED_API_URL` | `http://localhost:8080/api/v1` | API to fill |
| `SEED_ADMIN_EMAIL` | `admin@opendefender.io` | account the script signs in with |
| `SEED_ADMIN_PASSWORD` | read from the container by `make` | its password |
| `SEED_ADMIN_TOTP_SECRET` | none | needed only once MFA is enrolled on that account |
| `SEED_DEV_ALLOW_REMOTE` | unset | `true` lets the script target a host other than localhost |

A run takes about a minute on a fresh stack: the API rate-limits writes, and
the script waits and retries when it answers 429.

## What it does, and what it does not

- **Everything goes through the public API.** Scores, bands and criticalities
  come from the Score Engine. The audit log, activity feed and notifications
  fill up because the real use cases ran. The script never writes to the
  database, and never sets a score or a criticality itself.
- **It is idempotent.** Each record is looked up by its natural key (title,
  name, email) before it is created. A second run creates nothing, and its
  per-module totals match the first run's, moved from "créés" to "existants".
- **It reports what it could not create.** A record the API refuses is listed
  at the end with the status and body the API returned. The script does not
  work around the refusal.
- **It refuses remote targets.** It creates accounts whose passwords are in
  this repository and prints them. Against any host other than `localhost`,
  `127.0.0.1`, `::1` or the compose service name `backend`, it stops unless
  `SEED_DEV_ALLOW_REMOTE=true` is set. Use that only on a throwaway stack.
- **It ends with an isolation check.** It reads the first organisation's risk
  register and fails with exit code 2 if a risk from the second organisation
  shows up.

The ids it created are written to `dev/.seed-dev-state.json` (git-ignored), to
help when you script something on top of the dataset.

## The organisation

**Kalao Paiements (dev)** is a fictional payment processor in Dakar and
Abidjan. It is the same estate as `dev/fixtures/demo.json`, so the screens tell
one story.

| Module | What you get |
|---|---|
| Members and teams | 8 personas (below), 3 teams: SOC, GRC, Infrastructure |
| Custom fields | 2 fields on risks, 2 on assets |
| Assets | 26 assets (applications, servers, network, endpoints, data processing, 4 vendors), 20 dependencies, 5 vendor-to-asset links |
| Risks | 28 risks: every lifecycle state from `draft` to `reopened`, all four bands, 2 residual risks accepted on a signed governance approval with a renewal date, 2 risks with no framework, owners spread across the personas |
| Mitigations | 13 mitigations from 0 to 100 %, some with sub-actions partly done |
| Compliance | ISO 27001, COBAC, BCEAO, RGPD and PCI DSS imported from their catalogues, a spread of control statuses, 7 cross-framework mappings, 3 risk-to-control mappings |
| Evidence | a statement of applicability per framework, plus approved, rejected, pending and expired evidence; many controls have none |
| Audits | a completed ISO 27001 internal audit with its score, summary and remediation plans generated from its gaps; a COBAC mission in progress; a PCI DSS certification planned |
| Remediation | 3 hand-written remediation plans on top of the generated ones |
| Vulnerabilities | 14 findings, KEV and non-KEV, most matched to an asset by hostname, some left unassigned |
| Incidents | 6 incidents across open, in progress, resolved and closed; two carry response actions; one resolved incident has a published post-mortem, one closed incident a draft one |
| Third parties | a questionnaire template, 2 vendor assessments sent |
| Reports | 2 board reports (one approved), 3 generated reports |
| Governance | an approval workflow, a pending approval request, a delegation |
| Automation | 3 rules adopted from the built-in templates |
| Saved views | 2 saved views on the risk register |
| MFA | 90-day grace period, so the privileged personas sign in with a password |

## Personas

Every account belongs to Kalao Paiements, except the last one. The passwords
are development fixtures: never reuse them.

| Persona | Email | Org role | Business role | Password |
|---|---|---|---|---|
| Seed admin (owner) | `admin@opendefender.io` | root | none | the seed admin password |
| Fatou Ndiaye, RSSI | `fatou.ndiaye@kalao.dev.local` | admin | none, the wildcard grants everything | `DevSeed!Rssi2026` |
| Ibrahima Fall, risk manager | `ibrahima.fall@kalao.dev.local` | user | `risk_manager` | `DevSeed!Risk2026` |
| Awa Diallo, SOC analyst | `awa.diallo@kalao.dev.local` | user | `security_analyst` | `DevSeed!Soc2026` |
| Koffi Mensah, compliance | `koffi.mensah@kalao.dev.local` | user | `compliance_officer` | `DevSeed!Compliance2026` |
| Aminata Touré, internal audit | `aminata.toure@kalao.dev.local` | user | `auditor` | `DevSeed!Audit2026` |
| Moussa Koné, infrastructure | `moussa.kone@kalao.dev.local` | user | `asset_owner` | `DevSeed!Infra2026` |
| Aïssatou Ba, executive | `aissatou.ba@kalao.dev.local` | user | `executive` | `DevSeed!Board2026` |
| Yao Kouassi, finance (read only) | `yao.kouassi@kalao.dev.local` | user | `viewer` | `DevSeed!Viewer2026` |
| Mariam Sow, owner of Sahel Assurances | `mariam.sow@sahel-assur.dev.local` | root of the second organisation | none | `DevSeed!Sahel2026` |

Fatou Ndiaye is also the second signature on the two risk acceptances: the
four-eyes rule forbids the requester from approving their own request.

## The second organisation

**Sahel Assurances (dev)** is registered through `POST /auth/register` and holds
3 assets and 3 risks. Sign in as Mariam Sow to see it. None of its records may
appear while signed in to Kalao Paiements, and the reverse; the script checks
the first direction on every run.

## Starting over

The dataset only adds. To start from an empty stack, remove the stack's volumes
(`make docker-clean` deletes the local database) and bring it back up.
