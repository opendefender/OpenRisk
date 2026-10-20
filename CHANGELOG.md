# Changelog

All notable changes to OpenRisk will be documented in this file.

The format is based on [Keep a Changelog 1.1.0](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning 2.0.0](https://semver.org/spec/v2.0.0.html).
Git tags use the `vMAJOR.MINOR.PATCH[-rc.N]` convention; see [docs/VERSIONING.md](docs/VERSIONING.md).

## [Unreleased]

### Planned
- Board Report mensuel (IA, human-in-the-loop, FCFA) — the second half of M4
- Mobile app (React Native)

## [1.1.0-rc.7] - 2026-10-20

> **Security release: fixes GHSA-6m8w-33j5-ff68, a cross-organisation role change.**
>
> A large release candidate: a run of authentication and session hardening, the
> October 2026 console redesign, e-mail work notices, dated risk acceptances and
> vulnerabilities found from the agent's package inventory. Several changes alter
> how an existing install boots or signs people in, so read the operator actions
> before upgrading.
>
> **Operator actions**
> - Docker Compose refuses to start until `DB_PASSWORD` (and the other passwords the
>   compose files used to default) is set. `.env.example` no longer ships one (#485).
> - With `INITIAL_ADMIN_PASSWORD` unset, the first boot now writes a generated admin
>   password to `INITIAL_ADMIN_PASSWORD_FILE` (default `secrets/initial_admin_password`)
>   and stops if it cannot. An instance still accepting the old published default
>   password logs a `SECURITY WARNING` at every boot: change that password (#485).
> - `JWT_SECRET` and the chart's `secrets.inline.jwtSecret` are no longer read; you
>   can drop them. The RSA key pair is what the backend needs (#789).
> - Accounts whose password is still stored in the first-release SHA-256 format can
>   no longer sign in and must use "Forgot password?". The
>   `openrisk_password_hash_accounts` metric counts them. Argon2id cost is set with
>   `ARGON2ID_MEMORY_KIB`, `ARGON2ID_TIME`, `ARGON2ID_PARALLELISM` and
>   `ARGON2ID_MAX_CONCURRENT`; the server refuses to start on a value below its floor.
>   The Helm chart raises the API limits to `1000m / 1Gi` (#484).
> - SAML sign-in is turned off (#866).
> - Every install now runs the SQL migrations at boot, Docker Compose included (#1001).
>   Members who still held the universal pre-#240 MFA backup codes lose them and are
>   told by e-mail. Rolling back to an earlier image needs the `migrate force` step in
>   `docs/runbooks/migrations.md` (#349).
> - A stored GCP scanner credential that is not a service-account key now fails on the
>   next scan (#957).
> - Building from source needs Go 1.26 (#979).
> - Agents need osquery and a rebuilt binary to report their package inventory. Package
>   versions are then looked up at OSV and NVD, on by default for each organization;
>   it can be switched off in Settings (#910).

### Security
- **An organisation's administrator could change another organisation's roles
  (GHSA-6m8w-33j5-ff68).** The `/api/v1/rbac/roles/:role_id` routes checked that
  the caller administers their own organisation, then read and changed the role
  by its ID alone. Anyone holding the UUID of another organisation's custom role
  could read it, rename or deactivate it, and add or remove its permissions. The
  same routes also let any organisation change the predefined roles (Admin,
  Manager, Analyst, Viewer) that every organisation shares. Every role-specific
  read and write is now bound to the caller's organisation:
  - another organisation's role answers 404, the same as a missing role;
  - changing a predefined role answers 403;
  - a session with no organisation answers 401.

  Along the way:
  - `GET` and `PATCH` on a single role work again; they answered 404.
  - `PATCH` writes only the fields it is sent, so `is_active` can be set back to
    false.
  - A role that users still hold cannot be deactivated (409).

  **Operator action:** permissions added to a predefined role through this hole
  before the upgrade are not removed by it. Compare `role_permissions` for the
  four predefined roles against the seeded baseline. Reported by Sithum Shihara.
- **A revoked token family can no longer leave one token behind (#775).** When
  two requests refreshed the same single-use token at once, the loser detected
  the reuse and deleted the family, but the winner's brand-new token was written
  after that sweep and survived it. The lineage was reported as revoked while a
  usable refresh token stayed in the database. The rotation now re-checks, after
  storing the new token, that the row it consumed is still there; if the family
  was swept meanwhile, it drops what it issued and answers reuse. When a
  revocation is warranted, nothing of the lineage survives it — and the entry
  below decides when it is warranted.
- **A concurrent refresh no longer signs the user out (#777).** Reuse detection
  read "this token was already rotated" as proof of theft, so two browser tabs
  refreshing together, or a retry after a timeout, killed the whole session. A
  token rotated less than 10 seconds ago is now served the token that first
  rotation already issued — the same one, so a session still has exactly one
  live refresh token and the lineage cannot fork. Past that window, when the
  device fingerprints disagree, or once the chain has moved past that step,
  reuse detection is unchanged and still revokes the whole family.
- **Revocations hold on PostgreSQL too (#725).** A refresh could store its successor
  while a revocation's `DELETE` was still running and keep it. Revoking a family, all
  of a user's sessions, or a member's sessions in one organization now repeats the
  delete until a pass finds nothing. The refresh path is unchanged.
- **No more default credentials (#485).** The first boot no longer seeds a published
  default admin password, a Helm install included; the compose files no longer carry
  default database, Redis or Grafana passwords; and the guides stop showing them. See
  the operator actions above.
- **A disclosure policy (#485).** `SECURITY.md` is rewritten: GitHub private reporting
  and `security@opendefender.io`, response targets, coordinated disclosure, safe
  harbour, supported versions, and what is and is not in place today.
- **The second factor can no longer be guessed (#689).** An MFA challenge token allows
  five attempts and is then revoked; ten failures in fifteen minutes lock the account
  for fifteen minutes (`429 MFA_LOCKED`) and the owner gets one e-mail; the route has
  its own per-address limit. Logout now also revokes the access token, not only the
  refresh token.
- **A TOTP code is accepted only once (#849)**, wherever a code is checked: sign-in,
  enrolment, turning MFA off and the step-up gate. A replayed code gets the same answer
  as a wrong one. The code that finishes enrolment cannot be reused to sign in.
- **Turning MFA off requires the password (#754).** Accounts that sign in through an
  identity provider confirm with a current TOTP code instead. Attempts are limited to
  five per fifteen minutes, roles that must keep MFA are refused, and every attempt is
  audited. The owner is told by e-mail and in the app.
- **Sign-in throttling that stops locking out the wrong people (#688).** Sign-in,
  sign-up, password reset, the password-strength meter and re-authentication each have
  their own limit, so typing on the sign-up screen no longer blocks signing in. A 429
  says how long to wait. Ten failed sign-ins on one address lock it for fifteen minutes,
  ended early by a password reset. Unknown and disabled addresses cost the same work as
  a wrong password, and the e-mail lookup ignores case.
- **OAuth sign-in is bound to the browser that started it (#295).** The state now
  travels in an HttpOnly `or_oauth_state` cookie, so an attacker's callback link can no
  longer sign a victim in to the attacker's account.
- **SSO sign-in no longer prints tokens on a page (#803).** A successful Google, GitHub
  or Entra sign-in used to end on a page of raw JSON holding both tokens. It now sets
  the session in HttpOnly cookies, as password sign-in does, and lands in the app. The
  return-address check now refuses control characters and backslashes anywhere.
- **SAML sign-in is turned off until assertions are verified (#866).** The login and
  ACS endpoints redirect to `/login?error=provider_not_configured&provider=saml2` and
  create no user and no session. The code that accepted unsigned assertions is removed.
  SAML had never been proven end to end.
- **An organization can no longer lock someone out of another one (#807).** The
  unused `PATCH /users/:id/status`, `PATCH /users/:id/role` and `DELETE /users/:id`
  routes are removed. If a person's default organization withdraws their access, sign-in
  now falls back to another organization where they are still an active member.
- **Withdrawing access ends sessions only in the organization that acted (#831).**
  Changing a member's role or deactivating them used to sign the person out of every
  organization they belong to.
- **`/teams` is authorized from the session's organization role (#830)**, not the
  global account role. Adding a member requires an active membership in the caller's
  organization, and deleting a team is one transaction.
- **The audit log records the real client address (#877).** It used to store the raw
  `X-Forwarded-For` header, which any client could set; it now trusts that header only
  from a configured proxy.
- Organization exports are written with mode `0600` (#957).
- Dependency upgrades: `google.golang.org/grpc` 1.83.2 and `golang.org/x/crypto`
  0.55.0 (#804); axios 1.20.0, Go 1.26.9 and `golang.org/x/net` 0.60.0 (#979). The
  published images move to alpine 3.24, and the self-host frontend to nginx 1.30
  (#487).

### Added
- **The October 2026 console redesign (#899).** Ten screens move onto the new layout:
  - the shell: sidebar with badges for action items, open KEV flaws, open incidents
    and pending invitations, a new header, notifications panel and command palette
    (#900);
  - the dashboard, one layout for every member, with a twelve-month score history
    (`GET /score/history`, daily snapshots) and the business role choosing the
    counters and priorities (#901);
  - the Action Center, grouped by deadline, with a tick that finishes the work after
    a 5-second undo, and new categories for vulnerability deadlines, vendor follow-ups
    and plans in review. Every finding now gets a remediation deadline by severity,
    at most 14 days for a KEV flaw. Organization admins and owners see every category
    (#902);
  - the executive view, a one-page committee summary where a pending approval can be
    deferred to the next committee (#903);
  - financial quantification, with the loss-exceedance curve, mitigation payback, a
    3-year ROSI and a risk appetite admins can set (#904);
  - the Activity journal, one sentence per entry, filtered by domain (#905);
  - the asset inventory, with open vulnerabilities and KEV per asset and a
    "Lancer une découverte" button that starts a real scan (#906), and an exposure
    analysis in the asset drawer (#937);
  - the topology, with paths from internet-exposed assets to critical ones and a list
    view for keyboard and screen-reader users (#907);
  - the vendor register, as a list beside a vendor profile (#908);
  - the vulnerability queue, with a real MTTR on criticals and SLA breaches (#909);
  - Threat Intel, listing only the NVD, KEV and ATT&CK entries that touch the
    organization's own assets, with an explained relevance score that never feeds the
    Score Engine, and a per-organization "Non pertinent" (#911).
- **Vulnerabilities from the agent's package inventory (#910).** The agent reports
  the packages installed on its host; the backend checks them against OSV and NVD,
  files the findings on the matching asset and closes a finding when a later
  inventory shows a fixed version. "Lancer un scan" on the queue triggers it, and the
  asset drawer lists the installed software. An agent can report only its own host.
- **Work notices by e-mail (#757).** Assignments, D-7 and D-1 deadline reminders, a
  single overdue reminder, plan validation and every change to a risk's state now
  reach people by e-mail as well as in the bell, in their language and within their
  notification preferences. `/health` reports whether e-mail is `smtp`, `log` or
  `disabled`.
- **Residual-risk acceptances expire (#811).** Accepting a risk records an end date
  (12 months by default, capped by an organization maximum), an approver and the
  approval behind it. The owner and approver are reminded at D-30 and D-7; at the end
  date the risk goes back to `assessed` and appears in the Action Center. Renewal needs
  a new approval. A risk can no longer be made accepted through creation, CSV import,
  bulk status change or `PATCH`. There is no screen yet to pick the end date, renew or
  set the policy; the API supports it.
- **Score working and a trail that verifies on PostgreSQL (#486).** A risk's Score tab
  shows how its score was produced, who set each term and when, and whether each
  journal entry is intact. The tamper-evident trail now verifies on PostgreSQL for new
  entries, and records who acted (user, service token or job). Entries sealed on
  PostgreSQL before this release still fail verification.
- **CSV import of risks and assets (#755, #861).** `POST /risks/import` was never
  mounted and reported success on zero rows. Both imports now validate every row,
  report each error with its line and column, and write all or nothing. A risk file can
  link assets by name; files on the old 1–5 scale are refused with a conversion message.
- **Ownership transfer (#302).** `POST /risks/:id/transfer-owner`, and the same for
  mitigations and incidents, audit the previous and new owner and notify the new one.
  No screen calls them yet.
- **Release verification (#487).** The release workflow is set to attach CycloneDX
  SBOMs and a `SHA256SUMS` file, push a versioned image and publish nothing while the
  dependency gate fails. `SECURITY.md` explains how to verify a release.

### Changed
- **One security score (#287).** Every surface reads the same score. An organization
  with no risk and no applicable control shows "non mesuré" instead of 0/100, and a
  failed request shows "indisponible". A factor with no data behind it no longer counts
  as perfect, so organizations with risks but no vulnerability data see their score rise
  a little (score model 2.2).
- **Motion across the console (#751).** Reduced motion now also stops script-driven
  animation; menus, dialogs and drawers share the motion tokens; modals and drawers
  animate on close; the bell shows the real unread count; the notification panel closes
  on Escape and returns focus; toasts use the theme and stay until dismissed when they
  carry an action; MFA enrolment submits when the last digit is typed.
- **One keyboard-accessible select (#935)**, used for member roles, the scan schedule,
  the display currency and the simulator, with type-ahead and accent-insensitive search.
- **A lighter first load (#966, #858).** The initial bundle drops from 223.6 KB to
  161.5 KB gzipped: rarely used icons, panels and screens load on first use, and the
  French strings arrive with the screen that uses them. English readers download the
  English catalogue after the entry.
- The case-insensitive account lookup is indexed (#876).
- The README lists only what ships today; unshipped modules move to a labelled roadmap,
  and vendor risk management, which does ship, is listed (#483).

### Fixed
- **Risk scores include asset criticality from the first save (#792).** Creating or
  updating a risk stored `Probability × Impact` and left asset criticality to a
  background worker. Both now score through the Score Engine with the linked assets, in
  one transaction. A risk with no asset uses a neutral 1.0 everywhere.
- **API tokens from Settings work (#782).** They were kept in memory, lost on restart
  and never checked. Tokens are now stored, authenticate requests, act only in the
  organization they were created in, and are copied correctly.
- **An unfinished MFA enrolment can start over (#889, #714).** A member whose role
  requires MFA and who once left enrolment unfinished could never sign in again.
  Enrolment is now one transaction, and an unexpected error answers with a generic
  message instead of database text.
- **The sign-in screen explains MFA refusals (#872).** After too many codes it returns
  to the password form or says how long to wait, instead of "incorrect code" or a
  silent reload.
- An SSO account's Settings shows that its password lives at the identity provider,
  instead of a form that could only fail (#850).
- An account created with `POST /users` can sign in (#870).
- Owners and admins can edit the risk weights again (#867).
- Reloading or opening a link to `/assets` no longer answers 403 behind nginx (#758).
- Migration 0060's `mfa_policies` constraints were never applied on a booted server;
  they are now added, and a runbook covers the upgrade and rollback (#349).
- Approval deferrals are saved (#903); incidents in progress appear in the Action
  Center (#902); the Activity filter by type is applied by the database (#905);
  Cmd/Ctrl+K opens the palette the first time (#900).
- The accent swatches in Settings show the colour the accent will actually be (#627).

### Removed
- The unreachable HS256 authentication stack and `JWT_SECRET` (#789, #293).
- The legacy in-memory API token service (#782).
- The `cyber_score` field of `/analytics/executive` and its second score formula (#287).
- The five persona dashboards and the dashboard's period control (#901); the P10/P50/P90
  band, 5-year projection and by-criticality card of the financial page, which the
  loss-exceedance curve supersedes (#904).

### Internal
- Agent tooling, CI repairs and retired checks, test hardening, decision records and
  ADR 0005; regressions introduced and fixed between rc.6 and rc.7 are not listed.

### Added after the notes were drafted
These two PRs merged just before the tag, and are part of 1.1.0-rc.7.

- **The Helm chart can now run a backend and a frontend on a current cluster (#1022).**
  Four defects stopped the chart from running a backend, each hiding the next:
  - capabilities were declared at pod level, so the API server refused the Deployments;
  - no RSA keys were mounted;
  - the uploads directory was not writable;
  - the probes targeted the wrong path.

  The frontend no longer hides nginx's `mime.types`, and it runs as uid 1000. HPAs render only when autoscaling is on. PDBs no longer block a node drain on a single replica. A kind-based workflow installs, upgrades and checks the chart.

  **Operator actions:**
  - Provide the RSA key pair, with `--set-file secrets.inline.rsaPrivateKey=…` or `secrets.rsa.existingSecret`.
  - Uploads now live on a `backend-uploads` PVC, which defaults to `ReadWriteMany`. The chart refuses `ReadWriteOnce` with more than one replica or with autoscaling.
- **Trends and financial figures no longer show zeros or reference amounts as if they were data (#756).**
  - **Trends:** a day with no risk is a gap, not a score of 0. Below two dated points, a trend answers `insufficient_data`.
  - **Financial pages:** when no risk has its own loss figures, the financial page, the executive dashboard and the executive counters say so. They no longer show the reference model's amounts as the organisation's exposure.
  - No amount, formula or score changed.

## [1.1.0-rc.6] - 2026-09-23

> A security release. Both fixes below change how an existing scan config
> behaves, so read the operator actions before upgrading.

### Security
- **SSRF through discovery connectors — GHSA-98hp-4h7m-v45m, high (#750).** The
  Kubernetes, Docker, VMware, Active Directory, GitHub and GitLab connectors run
  inside the backend and connected to whatever address a scan config named:
  loopback, the cluster network, cloud metadata, or the host's Docker socket. They
  now go through the same outbound guard as integrations. The address is checked
  when the config is saved and again when each connection opens.
  **Action for operators:** a scan config aimed at a private address fails until
  its range is added to `OUTBOUND_ALLOWED_CIDRS`. Docker configs that use a unix
  socket need `SCANNER_DOCKER_SOCKET_ENABLED=true`. Active Directory accepts only
  `ldap://` and `ldaps://`. Kubernetes, vCenter, GitHub and GitLab endpoints must
  use https. These connections no longer go through `HTTP(S)_PROXY` (see
  `docs/SELF_HOSTING.md`).
- **Kubernetes scans no longer skip TLS verification on their own (#770).** A
  Kubernetes scan config with no `ca_cert` connected to the cluster's API server
  with verification disabled, and sent the tenant's ServiceAccount token over
  that connection. Verification is now on by default, and skipping it takes the
  explicit credential `insecure: "true"`, the same opt-in the vCenter connector
  requires.
  **Action for operators:** a self-signed cluster scanned without a `ca_cert`
  now fails with a certificate error until you supply the cluster's CA in
  `ca_cert` (preferred) or set `insecure` to `true` on that scan config.

## [1.1.0-rc.5] - 2026-09-21

> Supersedes `v1.1.0-rc.4`, which was tagged but never published: the release
> workflow failed to build the frontend, because a conflict resolution in the
> organization-switcher merge (#734) had dropped an import from the sidebar
> (#746). The tag `v1.1.0-rc.4` is left in place with no release attached — a
> pushed tag is never moved or deleted (see [docs/VERSIONING.md](docs/VERSIONING.md) §4).
> Everything listed under `[1.1.0-rc.4]` below ships in this release, including
> the fix for [GHSA-cjh4-89ww-2jpr](https://github.com/opendefender/OpenRisk/security/advisories/GHSA-cjh4-89ww-2jpr).
> The full list of changes since `v1.1.0-rc.3` is in
> [CHANGELOG.md](https://github.com/opendefender/OpenRisk/blob/v1.1.0-rc.5/CHANGELOG.md).

### Security
- **SSRF through integration URLs — GHSA-cjh4-89ww-2jpr, high (#743).** Scanner
  live-pull, Jira / ServiceNow ticketing and `POST /integrations/:id/test` could be
  pointed at loopback, private networks or cloud metadata, and could send stored
  credentials to another host. Outbound requests to tenant-configured URLs now only
  reach public https addresses, and credentials must be re-entered when `base_url`
  changes host.
  **Action for operators:** integrations whose `base_url` is `http://` or a private
  address stop working until fixed. Tools on your own network can be opened with
  `OUTBOUND_ALLOWED_CIDRS` (see `docs/SELF_HOSTING.md`); these requests no longer use
  `HTTP(S)_PROXY`.

### Added
- Change your password while signed in, from the settings screen. Your other devices
  are signed out and this one stays signed in; accounts that sign in through an
  identity provider are sent to it (#726, #720).

### Fixed
- The frontend builds again: the sidebar's "My profile" icon import was restored, and
  so were the organization's branded name and logo in the sidebar, which the #734 merge
  had replaced with the login name and initials (#747, #746).
- Every table production code uses is checked against the tables startup builds; the
  marketplace routes, whose tables are never built and which answered 500, are no
  longer mounted (#711, #707).
- The release workflow builds the frontend on Node 20, which Vite 7 requires (#748).

## [1.1.0-rc.4] - 2026-09-21

> Ships the fix for [GHSA-cjh4-89ww-2jpr](https://github.com/opendefender/OpenRisk/security/advisories/GHSA-cjh4-89ww-2jpr)
> to self-hosted operators, together with everything merged since `v1.1.0-rc.3`.
> Still a release candidate: the release trains that follow Wave 0 remain open.

### Security
- **SSRF through integration URLs — GHSA-cjh4-89ww-2jpr, high (#743, #573).** Scanner
  live-pull, Jira / ServiceNow ticketing and `POST /integrations/:id/test` fetched the
  tenant-configured `base_url` with no restriction, and echoed the remote response back
  through `last_pull_error`, ticket errors and the test endpoint. Changing only
  `base_url` also sent the stored API credentials to the new host. Outbound requests
  now go through `pkg/netguard`: https only; loopback, private, link-local (cloud
  metadata), CGNAT, reserved and IPv4-embedding IPv6 ranges refused when the URL is
  saved and again at connect time; redirects stay on the original host. Stored
  credentials must be re-entered when `base_url` moves to another host, and remote
  bodies are no longer echoed.
  **Action for operators:** integrations whose `base_url` is `http://` or a private
  address stop working until fixed. Tools on your own network can be opened with
  `OUTBOUND_ALLOWED_CIDRS` (see `docs/SELF_HOSTING.md`); these requests no longer use
  `HTTP(S)_PROXY`.
- `audit_logs` scoped to the acting organisation — a P0 cross-tenant read (#534).
- Audit of the 92 protected routes that had no permission guard; 17 were wrong (#577).
- Isolation assessment of the 97 pending collection routes (#533).
- Live permission-denial test matrix over all 52 guards, with a route ratchet (#530).
- Session revocation enforced on live streams (#526); no session for a disabled or
  deleted account (#525); MFA sign-in carries the current business role (#523).

### Added
- **Third-party risk (TPRM v1):** vendor register and the vendor → asset → risk chain on
  Business and Enterprise (#676), questionnaire templates, assessments and a public
  token link for vendors (#677, #682), J-7 / J-3 / J-1 reminders (#679). ADR 0004 (#675).
- **Onboarding tunnel:** posture reveal and recognition (#628), residual score from
  control coverage and a starter catalogue (#625), versioned Aha metric (#622), theme and
  language switch inside the tunnel (#667).
- **Organisation and profile:** editable organisation profile and regional settings
  (#721), organisation logo and accent for every member (#723), self-service profile,
  preferences and avatar (#722), switching between organisations (#734).
- **Action Center:** aggregation API (#431) and the `/action-center` page (#434, #436).
- Server-side saved table views that a tenant can share (#584); governed bulk
  operations for risks, vulnerabilities and assets (#585, #586, #603).
- Polymorphic entity contract, universal drawer and global timeline (#414).
- Locale registry, CLDR pluralisation and locale-aware formatting (#604).
- Tenant-safe response caching replacing the passthrough (#531).
- One-command self-host: first admin and generated entitlement table (#610).
- Design system: canonical token contract and theme foundation (#407, #423), form
  primitives, Spinner, AlertDialog, Popover, Menu, Command, OtpField, Empty, TagInput,
  RiskMatrix (#465–#477, #497, #642), in-house chart layer replacing Recharts (#498),
  scoped SVG entry motion (#515).
- Deferrable MFA enrolment before the Aha moment (#334).

### Changed
- Frontend migrated to Tailwind v4 (#449); Prettier adopted (#506); initial bundle
  reduced from 245.8 KB to 179.7 KB (#466).
- Licence references aligned on AGPL-3.0-only with a CLA and DCO check (#448, #516);
  `frontend/design-system/` and `frontend/src/shared/ds/` relicensed to Apache-2.0 (#456).
- Navigation regrouped into the five ratified intentions (#705).
- Lint, test, build, performance-budget and E2E gates made blocking in CI (#500, #469,
  #661).
- The Helm chart exposes `OUTBOUND_ALLOWED_CIDRS` (#743).

### Fixed
- Auth: session refresh when the access cookie has expired (#692); registration is
  atomic and duplicates nothing (#694); pasted one-time codes accepted (#478).
- Onboarding: the tunnel completes from its last step (#693), keeps one state per
  organisation (#736), stops its load-failure remount loop (#697), and five defects found
  by walking it (#633, #635, #644, #717).
- Transactions: mitigation creation (#518), risk bulk actions (#585), mitigation
  sub-actions built at startup and scoped to the tenant (#708).
- RBAC and UI truthfulness: the sidebar shows the real plan and role (#698, #699),
  read-only members no longer get create controls the server refuses (#740), every
  business role can hold the realtime stream (#742), `events:read` in the permission
  catalogue (#618), notifications open their subject (#733), `/threat-intel` redirect
  (#738).
- Infrastructure: `docker-up` starts a stack you can sign up to (#648); the backend cache
  reaches Redis in Docker (#729); no `record not found` error logs from a healthy backend
  (#617).
- Accessibility: unnamed selects and unreachable scroll regions (#646), tint contrast
  pairs (#460).
- Pending invitations counted against the caller's clock (#418).

### Deprecated
- Three legacy SSE endpoints now carry RFC 8594 `Deprecation`, `Sunset` and `Link` headers and are
  **removed in 1.2.0** (sunset **2026-12-02**) — see
  [docs/API_REFERENCE.md](docs/API_REFERENCE.md#deprecated-endpoints) and #527:
  - `GET /mitigations/events` → `GET /realtime/events?aggregates=mitigation`. This one matters:
    it accepts the access token as a **query parameter**, so the credential reaches access logs,
    proxy logs and browser history. The replacement uses the HttpOnly session cookie.
  - `GET /scanner/events` → `GET /scanner/jobs` (**polling**; the realtime hub has no `scan.*`
    events).
  - `GET /reports/{reportId}/progress` → `GET /reports/{reportId}` (**polling**; no `report.*`
    events).

  `GET /scanner/agent/stream` — the Agent's own job channel — is **not** deprecated.

### Removed
- `frontend/src/hooks/useSSE.ts`, a generic SSE hook with no consumers, superseded by
  `src/lib/realtime.ts`.

## [1.1.0-rc.3] - 2026-08-25

> Supersedes `v1.1.0-rc.2`, which was tagged but never published: the release
> workflow builds and tests from the tag, and three cache tests failed on a
> runner with no Redis alongside them. Those tests now skip when Redis is absent
> instead of reporting a missing dependency as a defect. The tag `v1.1.0-rc.2`
> is left in place — a pushed tag is never moved or deleted (see
> [docs/VERSIONING.md](docs/VERSIONING.md) §4) — and simply has no release
> attached to it.

### Milestone

**Wave 0 — "Audit & proof" is complete** (8/8 issues, W0-01 → W0-07). The wave did
not add product surface for its own sake: it established what is actually true
about the system and closed the gaps the audit found. Waves 1–8 remain open, so
this is a release candidate, not a GA.

### Added
- **W0-07 — Real-time event hub and stream contracts.** One authenticated,
  tenant-scoped SSE backbone (`GET /api/v1/realtime/events`) replacing three
  hand-rolled streams and two WebSocket clients that pointed at endpoints the
  backend never served. A versioned event envelope (id, version, tenant,
  aggregate, per-tenant sequence, correlation and causation ids) over a closed
  catalog of 25 canonical domain events across risks, assets, vulnerabilities,
  incidents, controls, assessments and mitigations. Events are appended to a
  durable per-tenant ordered log (`realtime_events`, migration `0059`) before
  delivery, so a reconnect replays from a cursor instead of refetching
  everything; a cursor older than the retained window gets an explicit resync
  rather than a silent partial replay. Delivery is **at-least-once** with
  server-minted ids and idempotent consumers — exactly-once is not claimed.
  Backpressure is real: bounded per-subscriber buffers, per-instance and
  per-tenant connection limits, and a coalescing resync instruction instead of
  blocking. Stream authorization is two-layered (`events:read` to hold a stream,
  then the read permission of each event's own aggregate to receive it). New
  permission `events:read`, 18 Prometheus series with no unbounded labels, and 8
  alert rules built around the hub's real failure mode — silence. The frontend
  gets one shared transport per tab with jittered backoff, a liveness watchdog,
  duplicate suppression, sequence-gap detection and tenant-switch teardown.
  See [docs/W0-07_REALTIME_EVENT_HUB.md](docs/W0-07_REALTIME_EVENT_HUB.md).
- **W0-04 — Real organization member management.** Inviting a colleague is now an
  invitation: a hashed, expiring, revocable token, instead of `POST /rbac/members`
  creating the account immediately and handing the administrator a temporary
  password to relay by hand. Membership gains an explicit lifecycle (status plus
  the timestamps that say when it changed), backfilled from the boolean that used
  to be the only signal (migration `0058`).
  See [docs/W0-04_ORGANIZATION_MEMBER_MANAGEMENT.md](docs/W0-04_ORGANIZATION_MEMBER_MANAGEMENT.md).
- **Financial risk quantification** (FAIR + Monte Carlo).
- **Open-core entitlements and billing**: a plan matrix enforced in the backend,
  with telemetry consent and a cancelable organization-erasure grace period.
- **Attack-surface module**, and **compliance evidence crosswalks + reporting**.

- **Guided onboarding to the Aha moment.** After signup (email + password only),
  the dashboard runs a 4-step action-driven checklist — create your first risk (the
  Aha), import a framework, invite a teammate, personalize your workspace — with a
  progress bar. Steps auto-tick from real data; the current step is emphasized so no
  one faces an empty dashboard; completion celebrates (reduced-motion-safe confetti).
  Post-Aha personalization (theme + accent) is an inline ghost edit. New
  `features/onboarding`; replaces the single dismissible banner. (UX-01/07/13/17/32.)
- The founder's latest UX guidelines are folded into the living rollout plan
  (`docs/UI_IA_ROLLOUT_PLAN.md`, section 1bis): a sequenced backlog of shared
  primitives (undoable delete, impact radiography, contextual hints, ProgressState,
  categorized notifications, keyboard shortcuts, time-travel, premium peek), each
  mapped to screens and priority — to apply UI_ELEVATION + IA_NAVIGATION screen by
  screen without regressing the healthy surface.
- **End-to-end test harness + UX audit (Gate 0).** A runnable Playwright workspace at
  the repo root (`package.json`, `playwright.config.ts`, `tests/e2e/`) that drives the
  real app: a deterministic seed (`scripts/seed-e2e.mjs` + `dev/fixtures/e2e-dataset.json`),
  API-minted `storageState` auth (no UI login except `auth.login.spec.ts`), and suites
  `smoke.routes` (every route in `App.tsx`/`navModel.ts`), `journey.newcomer`,
  `journey.settings`, `journey.rbac`, `a11y` (axe-core WCAG 2.1 AA) and a real risk
  `workflows` journey. **Local run: 95 passed / 0 failed / 19 fixme** (chromium +
  Mobile Chrome); 42 routes render (0 broken), time-to-value measured at 3.7 s. Broken
  flows are written and quarantined `test.fixme` with a bug id. E2E workflow rebuilt on
  v4/v5 actions (Node 20, PR-blocking chromium + Mobile Chrome, nightly firefox/webkit,
  route×status job summary); `ci.yml` gains a blocking `frontend-typecheck` job. Stable
  `data-testid`s added (attribute-only): `login-*`, `nav-*`, `app-main`, `settings-tab-*`.
  Deliverables: [docs/UX_CHARTER.md](docs/UX_CHARTER.md), [docs/UX_AUDIT_2026-07.md](docs/UX_AUDIT_2026-07.md),
  [docs/IA_NAVIGATION_PROPOSAL.md](docs/IA_NAVIGATION_PROPOSAL.md),
  [docs/UI_ELEVATION_PROPOSAL.md](docs/UI_ELEVATION_PROPOSAL.md) (+ 3 mockups),
  [docs/DOCS_INVENTORY.md](docs/DOCS_INVENTORY.md).
- **Single-source versioning.** The root `VERSION` file is the sole source of truth,
  propagated to the Go binary (via `-ldflags` → reported by `GET /api/v1/health`), the
  Helm chart (`version`/`appVersion`) and the frontend (`package.json`) through
  `make sync-version` / `make check-version`. Tag convention `vX.Y.Z[-rc.N]` (SemVer 2.0.0)
  with a tag-triggered release workflow that fails when the tag and `VERSION` diverge.
  See [docs/VERSIONING.md](docs/VERSIONING.md).
- **M4 — Official compliance report (PDF, 1-click).** New `GET /compliance/frameworks/{id}/report?locale=fr|en`
  streams a print-ready PDF for a framework: cover identity (organization, framework, date, requester),
  executive summary (compliance %, per-status breakdown, progress bar) and a paginated controls table
  (reference, name, colored status, evidence count, source citation). All data strictly tenant-scoped.
  Pure renderer in `backend/pkg/report` (fully unit-tested, no DB/HTTP), `GenerateComplianceReportUseCase`
  in the application layer, `CountEvidencesByFramework` repo method (single grouped query), and a
  "PDF report" button on the Compliance page (FR/EN). Serves the COBAC/BCEAO/ISO one-click statement goal.

- **Member invite** (`POST /rbac/members`, admin): provision a real tenant member
  (user + organization_member) with an org role and an optional business-role preset;
  returns a one-time temporary password. Frontend invite modal on `/settings/roles`.

### Changed
- **W0-06 — The home Security Command Center reads what it displays.** Home is a
  dispatcher over six persona dashboards plus an executive view, which is why the
  earlier cleanups had not finished the job. Every persona now reads real tenant
  data, `/stats` gains a period and one score instead of two, the inventory is
  counted in SQL rather than in the browser, and a rejected period is refused
  instead of being answered with a dashboard.
  See [docs/W0-06_SECURITY_COMMAND_CENTER.md](docs/W0-06_SECURITY_COMMAND_CENTER.md).
- **W0-02 — Release-line integrity.** `master` is established as the single
  trustworthy release line with zero branch divergence; the P1 operational risks
  found (hardcoded `latest` image tags, unlocked AutoMigrate on boot) are recorded
  in [docs/W0-02_RELEASE_LINE_INTEGRITY.md](docs/W0-02_RELEASE_LINE_INTEGRITY.md).

- **Navigation restructured into 5 GRC intentions** (founder-ratified UX proposal).
  The sidebar is regrouped by the user's intention, in the natural order of the work —
  **Piloter → Identifier → Évaluer → Traiter → Prouver** — plus a utility group
  (7 groups/~20 items → 5 intentions + utility, UX-16/UX-07). Infrastructure and Asset
  Universe are un-flagged as "soon" (both shipped); genuine placeholders (Leaderboard,
  Simulations) are withheld from the sidebar; fake count badges removed from
  Risks/Mitigations. `docs/IA_NAVIGATION_PROPOSAL.md` + `docs/UI_ELEVATION_PROPOSAL.md`
  ratified (accent default azure, density Confort, 4K master-detail); mockups polished.

- **UI elevation lot** (`feat/ia-nav-ui-elevation`): design tokens (motion, type
  scale, spacing, radii, elevation), a persisted density system (Confort default) with
  a header control, reusable `DataTable`/`EmptyState` primitives, and confetti
  micro-victories (UX-32). See `docs/UI_ELEVATION_PROPOSAL.md` §10.

### Fixed
- **The cache tests no longer fail a CI job that has no Redis.** Three of the
  four built a client against `localhost:6379` and then asserted on the result
  unconditionally, so an absent server was reported as a defect in the cache
  service. All four now ping first and skip with the reason — which is what the
  first test in the file had always intended. This is what blocked `v1.1.0-rc.2`
  from publishing.
- **UX bug registry — all 12 audit findings** (`fix/ux-audit-bug-registry`, atomic
  commits): **OR-BUG-001/009** registration is real (3 fields → account + org
  membership → auto-login → land; fake MFA façade removed); **OR-BUG-002** first-run
  onboarding card + personalized greeting; **OR-BUG-003** invite a member into the
  tenant (`InviteMemberUseCase` + `POST /rbac/members` + invite modal; live-proven:
  invite → member logs in with tenant + business role → RBAC governs them);
  **OR-BUG-004** Settings shows real session data (org/email/time zone) + honest
  placeholders for un-built areas, with persisted "Saved ✓" preference toggles;
  **OR-BUG-005** sidebar shows the real org name + real cyber score (was hardcoded
  72/"Banque Atlantique"); **OR-BUG-006** scan preview no longer retry-spams the
  console; **OR-BUG-008** auth rate limit relaxed 5/15min → 15/5min; **OR-BUG-010**
  plain-language glossary tooltips for CVE/KEV/EPSS/CVSS; **OR-BUG-011/012** WCAG 2.1
  AA — axe-core reports 0 serious/critical on the 6 key screens (labels + contrast
  fixed, a11y gate un-quarantined).

### Removed
- **W0-05 — Deceptive UI.** Fixtures, placeholders and dead-end actions removed
  across the app, with the audit and the disposition of every finding recorded in
  [docs/W0-05_DECEPTIVE_UI_AUDIT.md](docs/W0-05_DECEPTIVE_UI_AUDIT.md). Two dead
  WebSocket clients and the unrouted page that consumed them go with W0-07.

### Security
- **W0-03 — Authentication security baseline.** The identity surface — login,
  registration, MFA, OAuth2, SAML2, access tokens, refresh-token rotation,
  revocation, logout, sessions, recovery and tenant switching — is verified and
  hardened, with its actual state recorded rather than assumed. Refresh-token
  family rotation (migration `0057`), MFA backup codes moved to a CSPRNG, and
  HttpOnly cookie sessions with double-submit CSRF.
  See [docs/W0-03_AUTHENTICATION_SECURITY_BASELINE.md](docs/W0-03_AUTHENTICATION_SECURITY_BASELINE.md).
- **Tenant-isolation sweep**: every route audited against the tenant predicate,
  with regression probes for the leaks it found.
- The audit trail is made append-only (migration `0055`).

### Known limitations

- The real-time hub is an event log, **not a transactional outbox**: the row is
  appended after the business transaction commits, so a crash in that window
  loses the event. Survivable because the stream is never the source of truth and
  every client has a resync path.
- Stream authorization happens at connect, so a session revoked mid-stream keeps
  its stream until it ends (2 h ceiling) or the client reconnects.
- No load test was run against the real-time hub, so no capacity figure is
  claimed.

## [1.1.0-rc.1] - 2026-07-23

> Release-candidate hardening pass (branch `release/hardening-rc1`, stacked on
> `release/1.0-rc1`). Focus: reliability, security, multi-tenant isolation,
> performance and a green test suite — no new features.
>
> ⚠️ Versioning note: GitHub already carries `1.0.0`–`1.0.8` tags/releases that
> predate most of the current product. They do **not** reflect today's feature
> set. This RC proposes restarting the line at `1.1.0-rc.1`; the stale releases
> should be curated/relabelled (see `docs/RC_HARDENING_REPORT.md`).

### Security
- **Cross-tenant leak fixed** in bulk operations: `/bulk-operations` ran
  delete/update/export/assign filtered only by a user-supplied query with no
  `tenant_id` scope (a bulk delete could hit every tenant). Now tenant-scoped
  end-to-end with isolation tests. (Earlier RC also fixed the analytics/dashboard
  aggregation leak.)
- **Rate limiter is now Redis-backed** so brute-force protection on
  `/auth/login|register|refresh` holds across a horizontally-scaled deployment
  (was per-instance in-memory); degrades gracefully to in-memory if Redis is down.
- Legacy HS256 `/auth/legacy/*` login surface removed; default `admin123` seed
  refused in production (earlier RC).

### Performance
- **Route-based code splitting** (React.lazy): initial JS bundle 1.56 MB → ~651 kB.
- **Composite DB indexes** (migration 0039) on hot `(tenant_id, …)` access paths
  across risks/vulnerabilities/assets/compliance_controls/incidents/mitigations/audit_events.

### Fixed
- **Edit-Risk "Save" was broken in production**: the form declared `tags` as an
  array while the input produced a string, so Zod rejected every save. Fixed.
- Frontend test suite: 7 failing tests + 2 unloadable files → **43 passed / 0 failed**.
- `TestRiskCRUDFlow` green (sqlite DDL re-synced) — backend **36 pkg OK / 0 FAIL**.

### Accessibility
- Shared `Input` now associates `<label>`/`<input>` (screen readers + testability).

### RBAC
- Real per-route `PermissionRoute` guard wired from the shared nav permission map.

### Removed (dead code / duplicates)
- `ai_risk_predictor_service.go` (never wired), tenant-blind `risk_repo.go`,
  duplicate `components/CreateRiskModal.tsx`, dead `shared/fixtures.ts`, and two
  legacy Jest test files.

## [1.0.4] - 2025-01-02

### Added
- Analytics dashboard with real-time risk metrics
- Gamification system with badges and progress tracking
- Custom fields framework (5 field types supported)
- Bulk operations for risks and mitigations
- Advanced search and filtering capabilities
- Risk timeline view (audit trail)

### Improved
- Dashboard load time reduced by 40%
- Mobile responsive design across all pages
- API response times optimized
- Documentation structure reorganized

### Fixed
- API token expiration edge cases
- Search filter bugs with special characters
- Session handling on token refresh
- Mobile menu navigation issues

## [1.0.3] - 2024-12-15

### Added
- OAuth2/SAML2 SSO support (Google, GitHub, Azure AD)
- Role-Based Access Control (RBAC)
- API token management (create, revoke, rotate)
- Comprehensive audit logging

### Improved
- Authentication flow security
- Permission matrix granularity
- Database query optimization

### Fixed
- JWT token refresh bugs
- Permission check edge cases

## [1.0.2] - 2024-12-01

### Added
- Mitigation sub-actions (checklist items)
- Asset relationship management
- Risk scoring engine improvements

### Fixed
- Soft-delete cascade issues
- Asset linking bugs

## [1.0.1] - 2024-11-15

### Added
- Basic CRUD for risks, mitigations, assets
- Initial dashboard
- Documentation structure

## [1.0.0] - 2024-11-01

### Added
- Initial release
- Core risk management features
- React frontend + Go backend
- Docker Compose setup
- Basic authentication

---

[Unreleased]: https://github.com/opendefender/OpenRisk/compare/v1.1.0-rc.7...HEAD
[1.1.0-rc.7]: https://github.com/opendefender/OpenRisk/compare/v1.1.0-rc.6...v1.1.0-rc.7
[1.1.0-rc.6]: https://github.com/opendefender/OpenRisk/compare/v1.1.0-rc.5...v1.1.0-rc.6
[1.1.0-rc.5]: https://github.com/opendefender/OpenRisk/compare/v1.1.0-rc.4...v1.1.0-rc.5
[1.1.0-rc.4]: https://github.com/opendefender/OpenRisk/compare/v1.1.0-rc.3...v1.1.0-rc.4
[1.1.0-rc.3]: https://github.com/opendefender/OpenRisk/compare/v1.1.0-rc.1...v1.1.0-rc.3
[1.1.0-rc.1]: https://github.com/opendefender/OpenRisk/compare/v1.0.8...v1.1.0-rc.1
[1.0.4]: https://github.com/opendefender/OpenRisk/compare/v1.0.3...v1.0.4
[1.0.3]: https://github.com/opendefender/OpenRisk/compare/v1.0.2...v1.0.3
[1.0.2]: https://github.com/opendefender/OpenRisk/compare/v1.0.1...v1.0.2
[1.0.1]: https://github.com/opendefender/OpenRisk/compare/v1.0.0...v1.0.1
[1.0.0]: https://github.com/opendefender/OpenRisk/releases/tag/v1.0.0
