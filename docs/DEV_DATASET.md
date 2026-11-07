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

- **Everything goes through the public API.** Risk scores and bands come from
  the Score Engine. The script sends only what a user would type: probability,
  impact and financial inputs on a risk, and the declared criticality of an
  asset. The audit log, activity feed and notifications fill up because the
  real use cases ran. The script never writes to the database, and never sends
  a risk score, band or criticality.
- **It is idempotent.** Each record is looked up by its natural key (title,
  name, email) before it is created. A second run creates nothing, and its
  per-module totals match the first run's, moved from "créés" to "existants".
- **It reports what it could not create.** A record the API refuses is listed
  at the end with the status and body the API returned. The script does not
  work around the refusal.
- **It does not repair a half-built record.** Follow-up steps (a risk's owner,
  a mitigation's progress, an audit's status, an incident's post-mortem) run
  only when the record is created. If one of them is refused, the next run
  finds the record and leaves it as it is. Fix the cause, reset the stack and
  run again.
- **It refuses remote targets.** It creates accounts whose passwords are in
  this repository and prints them. Against any host other than `localhost`,
  `127.0.0.1` or `::1`, it stops unless `SEED_DEV_ALLOW_REMOTE=true` is set.
  Use that only on a throwaway stack.
- **Your stack is still reachable from your network.** The compose file
  publishes its ports on every interface, so anyone on the same Wi-Fi can sign
  in with these passwords. On a shared network, bind to loopback:
  `OPENRISK_DEV_BACKEND_PORT=127.0.0.1:8080 OPENRISK_DEV_FRONTEND_PORT=127.0.0.1:5173 make docker-up`,
  and give `make seed-dev` `SEED_API_URL=http://localhost:8080/api/v1`.
- **Threat Intel needs the Internet.** The catalogue is public data, filled
  only by a sync from `services.nvd.nist.gov` and `www.cisa.gov`, which the
  script asks the API for once. Offline, the run reports "catalogue CTI" as
  skipped and Threat Intel stays empty. The sync also runs the product's own
  matching of newly published NVD CVEs against every tenant's asset CPEs, which
  can create risks titled after a CVE; the script never calls `POST /cti/match`.
- **It ends with an isolation check.** It reads the first organisation's risk
  register and exits with code 2 if a risk from the second organisation shows
  up, or if the check could not run (the second organisation's risks are
  missing, or the register could not be read).

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
| Assets | 26 assets (applications, servers, network, endpoints, data processing, 4 vendors), 20 dependencies, 5 vendor-to-asset links; 9 assets marked internet-exposed and 7 carrying CPEs, for the topology and threat-intel screens |
| Infrastructure | one nmap scan configuration, never triggered: the seed scans nothing |
| Threat Intel | the shared CVE catalogue, synced once from NVD and CISA KEV through `POST /cti/sync` when it is empty; the KEV entries matching the seeded findings' CVEs then show as threats on Kalao's assets |
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
| Onboarding | completed for every persona through their own session, so a first sign-in opens the app, not the setup wizard. To try the wizard, invite a new member. |

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
