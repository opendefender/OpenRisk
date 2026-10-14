# Spec — #910 Vulnerabilities from the agent's package inventory

Status: **draft, awaiting owner review** (2026-10-14) · Branch: `feat/910-agent-package-scan`
(stacked on `feat/911-threat-intel` → `feat/909-vulnerabilities`) · Parent: #899 ·
Owner decision this rests on: D-067 point 2 ("auto vuln scan = agent package
inventory + OSV/NVD, no paid service").

## Objective

A tenant that runs the OpenRisk agent should see vulnerabilities in its queue
without anyone exporting and importing a scanner report. The agent reports the
packages installed on its host. The backend checks those exact versions
against OSV, writes what it finds to the vulnerability register on the right
asset, and closes a finding once a later inventory shows a fixed version.

Who: the RSSI or analyst using the vulnerability queue (#909), and the
administrator who installs agents.

## Current state (read 2026-10-14)

What the issue assumes, checked against the code:

| Issue says | Code says | Consequence |
|---|---|---|
| The agent "already embeds" osquery | `agent/scan.go` runs `osqueryi` **only if it is installed** (`exec.LookPath`), and today only reads `system_info` | Collection needs osquery installed, or a fallback (Q1) |
| Packages are sent "with its scan results" | osquery runs **inside the nmap branch**: a job with no valid network target never calls it | A package-only job type is needed |
| "Linux and Windows hosts" | osquery sees **only the machine the agent runs on**. Remote hosts would need authenticated (SSH/WinRM) scanning, which does not exist | Coverage = hosts running an agent (stated in UI and docs) |
| OSV for packages, NVD CPE as fallback | OSV has Debian/Ubuntu/Alpine/RHEL-family/SUSE ecosystems, **no Windows ecosystem**. Our NVD data is the local `cti_vulnerabilities` (24 h window + KEV), not a full CPE dictionary | Windows matching via NVD would be weak and noisy (Q2) |
| "KEV and EPSS enrichment from the existing feeds" | KEV yes (`CTIRepoEnricher`). **No EPSS feed exists**: `cti_enrich.go` says EPSS "stays 0 until an EPSS feed is added" | EPSS stays empty unless a feed is added (Q3) |
| A "vuln-autoscan" design screen | **No such screen in the design bundle.** The nearest is the Infrastructure "Lancer un scan" block (type "Vulnérabilités" + scan history), which is #912's screen | UI scope must be agreed (Q4) |
| Pushes create findings | Agent pushes go to a Redis preview and **never write assets** (human-in-the-loop, `push_results.go`). The vulnerability register, though, is written directly by `/vulnerabilities/ingest` | Inventory and findings are written directly; assets are never created by this feature |

Reused as is: agent auth (bearer + HMAC), job dispatch over SSE, the vulnerability
ingest pipeline (asset resolution via `pkg/assetmatch`, priority, KEV enrichment,
SLA, SOAR event, risk rule), the "unassigned" queue for hosts with no matching
asset, `Vulnerability.SetStatus` with `remediated_at` (#909), and the Redis scan lock.

## Capability map (Phase 0: four separately testable parts)

| Module id | Responsibility | Depends on |
|---|---|---|
| `agent-inventory` | Agent: an `inventory` job type, collects OS release + packages (osquery, fallback per Q1), pushes them HMAC-signed | — |
| `inventory-store` | Backend: receives the inventory, resolves the host to an asset, stores it per tenant, replaces the previous snapshot of that host | `agent-inventory` (wire contract) |
| `osv-matcher` | Backend: OSV `querybatch` client, maps packages→vulns→CVEs, writes findings through the ingest pipeline, closes findings no longer affected | `inventory-store` |
| `scan-trigger` | Backend + UI: manual "Lancer un scan" and a daily re-match, both tenant-scoped, locked and rate-limited; status visible to the user | `osv-matcher` |

Build order: `agent-inventory` + `inventory-store` (one contract, one PR slice) →
`osv-matcher` → `scan-trigger`. One PR for the issue, one commit per module at least.

## Design

### Wire contract (agent → backend)

New job `type: "inventory"`, dispatched on the existing SSE stream with no
targets. The agent answers on a new endpoint, `POST /api/v1/scanner/agent/inventory`,
with the same bearer + HMAC as `/agent/push`:

```json
{
  "job_id": "…",
  "host": { "hostname": "srv-core-01", "fqdn": "srv-core-01.corp", "ips": ["10.20.0.7"],
            "os": "rhel", "os_version": "9.4", "platform": "linux" },
  "packages": [ { "name": "openssl", "version": "3.0.7-27.el9", "source": "rpm", "arch": "x86_64" } ],
  "collector": "osquery|native",
  "errors": []
}
```

Limits are enforced server-side: ≤ 20 000 packages and ≤ 5 MB body. Names and
versions are trimmed and length-capped. The tenant comes from the agent
credential, never from the body.

### Ecosystem mapping (server-side, pure function, table-tested)

| osquery table / source | OS (from `os_version`) | OSV ecosystem |
|---|---|---|
| `deb_packages` | Debian 12 | `Debian:12` |
| `deb_packages` | Ubuntu 22.04 | `Ubuntu:22.04:LTS` (LTS suffix only for LTS releases) |
| `rpm_packages` | RHEL / Rocky / Alma 9 | `Red Hat`, `Rocky Linux:9`, `AlmaLinux:9` |
| `apk_packages` | Alpine 3.19 | `Alpine:v3.19` |
| `programs` (Windows) | any | **none in OSV** → see Q2 |

A package whose ecosystem is unknown is stored and counted as "not checkable",
never silently dropped.

### Storage (migration 0075, additive only)

- `host_inventories`: id, tenant_id, agent_id, host_key (normalised hostname),
  asset_id (nullable), os, os_version, ecosystem, collector, package_count,
  collected_at, matched_at, match_error. Unique (tenant_id, host_key).
- `host_packages`: tenant_id, inventory_id, name, version, ecosystem, arch.
  A new inventory **replaces** the host's package rows in one transaction.

Both tables carry `tenant_id`, and every query filters on it.

### Matching

- OSV `POST https://api.osv.dev/v1/querybatch`, up to 1 000 queries per call,
  then `GET /v1/vulns/{id}` for details (aliases → CVE ids, affected ranges,
  fixed versions). Plain `net/http`, no new Go module. 10 s timeout. OSV
  results are cached by (ecosystem, name, version) for 24 h in Redis, so a
  re-match does not re-ask OSV for what has not changed.
- One finding per (host, package, advisory) with CVE = the first CVE alias. An
  advisory with no CVE alias (some distro-only advisories) is kept with its
  OSV/DSA id in `external_id` and `cve_id` empty, so it is listed rather than lost.
- Written through the existing `IngestUseCase` with a new `VulnSource`,
  `agent` (label "Agent OpenRisk"). That gives KEV enrichment, priority, SLA,
  asset resolution and the risk rule for free.
- **Close on fix**: after a match, every unresolved `agent` finding of that host
  that the new result no longer contains moves to `remediated` through
  `SetStatus` (stamps `remediated_at`). Accepted or false-positive findings are
  never touched. A finding that reappears is reopened by the existing ingest upsert.
- OSV unreachable: the match fails, `match_error` is stored, and existing
  findings are left unchanged. A failed match never closes anything.

### Triggers

- **Manual**: the vulnerability page's "Lancer un scan" (today it starts the
  discovery configs) dispatches an `inventory` job to the tenant's online agents.
  Each arriving inventory is matched. Permission `scanner:scan`.
- **Scheduled**: once a day, re-match every stored inventory against OSV (new
  advisories land without the agent doing anything), tenant by tenant.
  Agents are not woken up.
- **Locks and limits**: one manual scan per tenant at a time (Redis lock, as
  for scan configs), at most one every 10 minutes per tenant (429 with
  `Retry-After` otherwise). A global OSV client limit of 10 requests/s.

### UI (pending Q4)

The minimum this issue owns, all FR/EN with the three states:
- "Lancer un scan" on the vulnerability page starts the package scan. A toast
  reports how many hosts were asked, and then the result.
- A compact **"Inventaire logiciel"** status (per host: OS, package count,
  last collected, last matched, error, "non vérifiable" count) where the user
  can see coverage. Placement is Q4.
- Findings from this source show "Agent OpenRisk" as source and the package +
  installed version + fixed version in the open row (#909).

## Commands

```
cd backend && go build ./... && go test ./...
cd backend && DATABASE_URL=postgres://… go test ./internal/infrastructure/repository/ -run Inventory   # pg-gated
cd agent && go build ./... && go test ./...
cd frontend && npx tsc -b --noEmit && npx eslint <touched files> && npx vitest run --maxWorkers=4
```

## Project structure

```
agent/inventory.go                                   collection (osquery + fallback), OS release
agent/inventory_test.go                              parsing of osquery / native output
backend/internal/domain/host_inventory.go            entities, ecosystem mapping (pure)
backend/internal/application/vulnscan/…              use cases: store inventory, match, trigger (one per file)
backend/internal/infrastructure/osv/client.go        OSV HTTP client + cache
backend/internal/infrastructure/repository/gorm_host_inventory_repository.go
backend/internal/handler/inventory_handler.go        agent endpoint + manual trigger
migrations/0075_host_inventories.{up,down}.sql
frontend/src/features/vulnerabilities/…              trigger + inventory status
```

## Code style

Same as #909/#911: domain stays pure, one use case per file, typed errors,
codes and figures in responses (the console words them), every query
`WHERE tenant_id = ?`, comments explain why. Example of the expected shape:

```go
// OSVEcosystem maps an inventory's OS and package source to the OSV ecosystem
// name, or "" when OSV has none (Windows programs). Pure; table-tested.
func OSVEcosystem(os, osVersion, source string) string
```

## Testing strategy

- Domain: the ecosystem mapping is table-tested (Debian, Ubuntu LTS and non-LTS, RHEL family, Alpine, Windows → "").
- Agent: parsing of recorded osquery JSON and native `dpkg-query`/`rpm` output, plus the HMAC push.
- Use cases, each with `TestXxx_Success` / `_NotFound` / `_Unauthorized`:
  - store-inventory: an unknown job is not found, and an agent of tenant A cannot write to tenant B.
  - match: a fixed version closes the finding, an OSV outage closes nothing, accepted findings are untouched.
  - trigger: the lock and the rate limit hold.
- Cross-tenant isolation: two tenants with the same hostname and packages, where each sees only its own inventory and findings. Postgres-gated repository test plus registry entries.
- OSV client: `httptest` server with recorded responses, covering batching, timeout and cache hit. **No test calls the real OSV.**
- Live: one agent run on a Debian/Ubuntu container with a deliberately old package, showing a finding that appears and then closes after an upgrade.

## Boundaries

- **Always**: tenant_id on every query; tenant taken from the agent credential;
  limits on body size and package count; never close a finding on a failed match;
  FR/EN strings; three UI states.
- **Ask first**: anything in the open questions below; any new Go module; any
  change to the agent's install script; any automatic risk creation beyond the
  existing, tenant-configured risk rule.
- **Never**: create assets from an inventory; call OSV from tests; send
  anything but package name, version and ecosystem to OSV (no hostnames, IPs or
  tenant identifiers); fabricate EPSS or a Windows match.

## Success criteria

1. On a host with the agent (and osquery, or the fallback per Q1), "Lancer un scan" stores its packages and OS within one job.
2. A package with a known OSV advisory at that exact version produces a finding on the matching asset (or in "unassigned" when no asset matches), with CVE, severity, KEV flag, installed and fixed version.
3. Upgrading the package and scanning again closes the finding as remediated, with `remediated_at` set.
4. OSV being unreachable shows an error state and closes nothing.
5. The daily job re-matches stored inventories without contacting agents.
6. A second manual scan within 10 minutes is refused with a clear message.
7. Tenant A never sees tenant B's inventory or findings (automated test).
8. `go test ./...`, the agent tests, `tsc`, `eslint` and `vitest` all pass.

## Open questions (need the owner)

1. **osquery not installed.** Options:
   - (a) require it, and the installer installs it;
   - (b) a native fallback on Linux: `dpkg-query`, `rpm -qa`, `apk info -v`. No new dependency on the host, and osquery is still used when present.

   *Recommended: (b).*
2. **Windows.** OSV has no Windows ecosystem, and our NVD data is not a CPE dictionary. Options:
   - (a) collect and store Windows programs now, show them as "non vérifiable", and match in a follow-up issue;
   - (b) attempt CPE matching against the local catalogue now, accepting false positives and gaps.

   *Recommended: (a).* Nothing claimed that isn't computed.
3. **EPSS.** No feed exists today. Options:
   - (a) ship without it, so EPSS stays empty as it does for imports;
   - (b) add the free FIRST EPSS daily feed to `pkg/cti` in a follow-up issue. That is a new external source, so it's your call.

   *Recommended: (a) now, (b) as a follow-up.*
4. **UI.** There is no vuln-autoscan screen in the design. Options:
   - (a) this issue only wires "Lancer un scan" on the vulnerability page and adds an "Inventaire logiciel" status block in the asset drawer's analysis (#937). The Infrastructure launcher's "Vulnérabilités" type then lands with #912.
   - (b) build the status block on the vulnerability page.

   *Recommended: (a).*
5. **Data leaving the premises.** OSV is run by Google. Each match sends package names and versions, never hostnames or IPs. For banks in the UEMOA/COBAC scope, should it be:
   - on by default, with a per-tenant switch and a line in the privacy docs?
   - off by default (opt-in)?

   *Recommended: on by default, with the switch.*
6. **Air-gapped self-hosting.** Without internet, OSV is unreachable and the feature shows "source injoignable". An offline OSV mirror is out of scope.

## Plan — tasks in dependency order (after approval)

1. `inventory-store` domain + migration 0075 + repository (pg-gated test) — *Verify*: `go test ./internal/domain ./internal/infrastructure/repository`.
2. Agent `inventory` job: collection + push + tests — *Verify*: `cd agent && go test ./...`.
3. Agent endpoint + store use case + isolation entries — *Verify*: use-case tests, registry ratchet.
4. OSV client with cache and limiter (`httptest`) — *Verify*: client tests.
5. Match use case: ingest through the pipeline + close on fix — *Verify*: Success/NotFound/Unauthorized + fix-closes + outage-closes-nothing.
6. Triggers: manual endpoint (lock + 429) and daily job — *Verify*: trigger tests; `go test ./...`.
7. Frontend: "Lancer un scan" wiring + inventory status + finding detail — *Verify*: tsc/eslint/vitest.
8. Live run on a container with an old package; issue comment; PR.
