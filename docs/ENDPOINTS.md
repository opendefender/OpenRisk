# OpenRisk — HTTP API reference

All routes are registered in `backend/cmd/server/main.go` and served under the base
prefix **`/api/v1`**. Unless noted, endpoints are **JWT-protected** (RS256 bearer) and
**tenant-scoped** — every query filters by the caller's `tenant_id`. Write routes are
gated by RBAC permissions (`middleware.RequirePermission("<resource>:<action>")`) or by
role (`admin`/`analyst`); admin (`root`) satisfies any gate.

> Contract-first modules (Compliance, Board Report, …) are also described in
> `docs/openapi.yaml`. This file is the exhaustive route map, kept in sync with
> `main.go`; ~190 routes total.

Last updated: 2026-07-13.

---

## Auth (public)
| Method | Path | Notes |
|---|---|---|
| POST | `/api/v1/auth/register` | Create an account + org |
| POST | `/api/v1/auth/login` | → `{ user, token_pair:{access_token,refresh_token,expires_in}, organization }` |
| POST | `/api/v1/auth/refresh` | Rotate access token from a refresh token |
| POST | `/api/v1/auth/logout` | Revoke the current session |
| GET  | `/api/v1/auth/me` | Current user claims |
| GET  | `/api/v1/auth/oauth2/login/:provider` · `/auth/oauth2/callback/:provider` | OAuth2 (design/partial) |
| GET  | `/api/v1/auth/saml2/login` · `/auth/saml2/metadata` · POST `/auth/saml2/acs` | SAML2 (design/partial) |
| GET  | `/api/v1/health` | Liveness → `{db,status,version}` (public) |

## Risks
| Method | Path | Perm | Notes |
|---|---|---|---|
| GET | `/api/v1/risks` | risk:read | List (page,limit,filters) |
| POST | `/api/v1/risks` | risk:create | Create (Score = Probability×Impact×AssetCriticality) |
| GET | `/api/v1/risks/:id` | risk:read | |
| PATCH | `/api/v1/risks/:id` | risk:update | |
| DELETE | `/api/v1/risks/:id` | risk:delete | |
| GET | `/api/v1/risks/:id/timeline` (+ `/score-changes`, `/status-changes`, `/trend`, `/since/:timestamp`, `/changes/:type`) | risk:read | History |
| GET | `/api/v1/risks/:id/incidents` | | Incidents linked to a risk |

## Mitigations
Mitigations are **risk-scoped**: create under a risk, not at `/mitigations`.
| Method | Path | Perm | Notes |
|---|---|---|---|
| GET | `/api/v1/mitigations` | mitigations:read | List all (board) |
| POST | `/api/v1/risks/:id/mitigations` | mitigations:create | **Create** (body: `title, description, priority, due_date` RFC3339, `assigned_to[]`, `sub_actions[]`) |
| GET | `/api/v1/risks/:id/mitigations` | mitigations:read | List for a risk |
| GET | `/api/v1/mitigations/:id` | mitigations:read | |
| PATCH | `/api/v1/mitigations/:id` | mitigations:update | |
| DELETE | `/api/v1/mitigations/:id` | mitigations:delete | |
| PATCH | `/api/v1/mitigations/:id/validate` | mitigations:update | |
| POST/PATCH/DELETE | `/api/v1/mitigations/:id/sub-actions[...]` | mitigations:* | Sub-actions CRUD + `/complete`, `/revert`, `/reorder-subactions` |

## Incidents (M5)
Tenant-scoped register. `:id` is the numeric incident id.
| Method | Path | Role | Notes |
|---|---|---|---|
| GET | `/api/v1/incidents` | | List (status, severity, type, limit, offset) |
| POST | `/api/v1/incidents` | writer | Create (`title, description, incident_type, severity, source, reported_by`) |
| GET | `/api/v1/incidents/stats` | | Totals: total/open/resolved/critical + resolution_rate |
| GET | `/api/v1/incidents/:id` | | |
| PUT | `/api/v1/incidents/:id` | writer | Update (title/description/status/severity/assigned_to/resolution) |
| DELETE | `/api/v1/incidents/:id` | writer | |
| GET | `/api/v1/incidents/:id/timeline` | | Timeline events |
| POST/GET/PUT | `/api/v1/incidents/:id/actions[...]` | writer | Mitigation actions |
| POST | `/api/v1/incidents/:id/risks/:riskId` | writer | Link a risk (legacy; uint↔uuid mismatch — see ROADMAP 14.1) |

## Compliance (M1–M4)
See `docs/openapi.yaml` for schemas. Perms: `compliance:{frameworks,controls,evidences}:{read,create,update,delete}`.
| Method | Path | Notes |
|---|---|---|
| GET | `/api/v1/compliance/catalogs` | Regulatory catalogs (ISO 27001, BCEAO, COBAC, ANTIC…) |
| GET/POST | `/api/v1/compliance/frameworks` | List / create (tenant-scoped) |
| GET/DELETE | `/api/v1/compliance/frameworks/:frameworkId` | Get / delete (cascades controls) |
| GET/POST | `/api/v1/compliance/frameworks/:frameworkId/controls` | List / create controls (`evidence_count` on each) |
| POST | `/api/v1/compliance/frameworks/:frameworkId/import-catalog` | Bulk import a catalog |
| GET | `/api/v1/compliance/frameworks/:frameworkId/progress` | % complete by status |
| GET | `/api/v1/compliance/frameworks/:frameworkId/report?locale=fr\|en` | One-click PDF report |
| GET/PATCH/DELETE | `/api/v1/compliance/controls/:controlId` | Get / update (strict rule: `implemented` needs ≥1 evidence) / delete |
| GET/POST | `/api/v1/compliance/controls/:controlId/evidences` | List / upload evidence (multipart) |
| GET/DELETE | `/api/v1/compliance/evidences/:evidenceId[/download]` | Download / delete |

## Reports — Board (M4)
| Method | Path | Notes |
|---|---|---|
| GET/POST | `/api/v1/reports/board` | List / generate a monthly board report (AI narrative + FCFA exposure) |
| GET/PATCH/DELETE | `/api/v1/reports/board/:reportId` | Get / edit draft / delete |
| POST | `/api/v1/reports/board/:reportId/approve` | Freeze the draft |
| GET | `/api/v1/reports/board/:reportId/pdf` | Rendered PDF |

## Score engine
`POST /api/v1/score-engine/{compute,classify}` · `GET /api/v1/score-engine/{matrix,metrics,configs}` · `GET/POST /score-engine/configs` · `GET/PUT /score-engine/configs/:id`.
> `ScoreEngineVisualizer` (front) tolerates null `matrix`/`risk_stats` — a null there used to white-screen the New-Risk modal.

## Assets
`GET/POST /api/v1/assets` · `GET/PATCH/DELETE /api/v1/assets/:id` · `GET /api/v1/assets/:id/history`.

## Dashboard / Analytics / Stats
- `GET /api/v1/stats` and `/api/v1/stats/{risk-distribution,risk-matrix,trends,top-vulnerabilities,mitigation-metrics}`.
- `GET /api/v1/dashboard/{complete,metrics,top-risks,risk-trends,severity-distribution,mitigation-status,mitigation-progress}`.
- `GET /api/v1/analytics/{dashboard,export,frameworks,risks/metrics,risks/trends,mitigations/metrics}`.
- `GET /api/v1/timeline/recent`, `GET /api/v1/gamification/me`.
- **Not implemented** (front falls back gracefully): `/analytics/security-score`, `/analytics/assets/statistics`.

## Notifications
`GET /notifications/unread-count` · `GET/PATCH /notifications/preferences` · `PATCH /notifications/read-all` · `PATCH /notifications/:notificationId/read` · `DELETE /notifications/:notificationId` · `POST /notifications/test`.

## RBAC · Users · Teams · Tokens (admin)
- Users: `GET/POST /api/v1/users` · `PATCH/DELETE /api/v1/users/:id` · `PATCH /api/v1/users/:id/{role,status}` · `GET /api/v1/users/me`.
- RBAC users: `GET /api/v1/rbac/users/:user_id[/permissions]` · `PATCH /api/v1/rbac/users/:user_id/role` · `DELETE /api/v1/rbac/users/:user_id`.
- RBAC roles: `GET/PATCH/DELETE /api/v1/rbac/roles/:role_id` · `GET/POST/DELETE /api/v1/rbac/roles/:role_id/permissions`.
- RBAC tenants: `GET/PATCH/DELETE /api/v1/rbac/tenants/:tenant_id` · `GET /api/v1/rbac/tenants/:tenant_id/{users,stats}`.
- Teams: `GET/POST /api/v1/teams` · `GET/PATCH/DELETE /api/v1/teams/:id` · `POST/DELETE /api/v1/teams/:id/members/:userId`.
- API tokens: `GET/POST /api/v1/tokens` · `GET/PUT/DELETE /api/v1/tokens/:id` · `POST /api/v1/tokens/:id/{revoke,rotate}`.
- Audit: `GET /api/v1/audit-logs` · `/audit-logs/action/:action` · `/audit-logs/user/:user_id`.

## Score (the single authority)
The calculation lives **only** in `backend/internal/domain/scoring/`. There is no scoring formula, threshold or band mapping in the frontend — see `docs/scoring/SCORE_MODEL.md`.
- `GET /api/v1/score?scope=tenant|risk|asset&id=<uuid>` — the canonical score. **The band is computed with the value, server-side, and travels with it**, which is what makes label/value desynchronisation structurally impossible. Returns `value` · `band` · `band_label_i18n_key` · `inherent`/`residual` (+ their bands) · `mitigation_effectiveness` · `computed_at` · `formula_version` · `inputs` (the assumptions used) · `breakdown` (`factor`/`weight`/`raw`/`contribution`, contributions summing to the value). `id` required for risk/asset. Any authenticated member.
- `POST /api/v1/score/preview` — live figure for a form (client debounces 300 ms). Runs the **same** model; persists nothing.
- `GET /api/v1/score/model` — the model's self-description (scale, band boundaries, per-scope weights, input bounds), so the explainer states the assumptions without the client carrying a second copy of the thresholds.

Scale **0–100, higher is worse**. Bands: `low` [0,25) · `medium` [25,50) · `high` [50,75) · `critical` [75,100], floors inclusive. Formula version **2.1**.

## Activation & onboarding (the newcomer journey)
Activation state is **derived from server events**, never from client state — there is deliberately **no endpoint that lets a client declare a step complete**. The only writes are the celebration acknowledgement and the wizard's own answers.
- `GET /api/v1/activation/state` — the checklist: steps (with FR/EN copy, deep link, order), `completed` + `completed_at` (the FIRST occurrence of the step's single event key), `percent`, `aha_reached_at`, `time_to_aha_seconds`, and `celebrate` (the server's once-per-step-per-user instruction to fire the burst). Any authenticated member.
- `POST /api/v1/activation/celebrated` — `{step_key}`; acknowledges a shown celebration. Idempotent (unique on user+step), so a double call is a no-op, not a 500. Returns 204; 400 on an unknown step key.
- `GET /api/v1/onboarding/state` — the resumable wizard state and, crucially, `completed`, which the frontend route guard reads. Never 404s: a newcomer's first call returns a valid empty state.
- `PUT /api/v1/onboarding/steps/:step` — save one step (`organization|profile|goal|framework|team`). Body `{answers, next}`; `next` **may point backwards** (correcting an answer is a supported move). Answers are stored verbatim so the form repopulates exactly as left.
- `POST /api/v1/onboarding/complete` — lifts the route guard. Idempotent: `completed_at` does not move on a repeat submit.
- `GET /api/v1/onboarding/suggestions[?industry=&country=&goal=]` — sector/goal-driven content: sector + goal option lists, the **three pre-filled first-risk drafts**, and the suggested framework catalog keys. Query params preview a choice before it is saved. **Creates nothing** (spec §5: the first risk is guided, never automatic).

**Events emitted by the domain** (`activation_events`, append-only): `signup` (t0 anchor, at registration) · `profile.completed` · `risk.created` · `framework.imported` · `asset.connected` · `mitigation.created` · `member.invited` · `report.generated` · `aha.reached`. Each checklist step maps to **exactly one** key (`domain.ValidateActivationSteps` enforces the bijection — this is what stops one import from striking two rows through).

**Metrics**: `GET /metrics` (root app, outside `/api/v1` and outside the auth gate) now serves the real Prometheus exposition format, including `openrisk_time_to_aha_seconds` (histogram), `openrisk_activation_events_total{event_key}` and `openrisk_aha_reached_total`. Alerts `SlowTimeToAha` (P50 > 12 min) and `NoActivationDespiteSignups` live in `deployment/monitoring/alerts.yml`.

## Other modules
- Custom fields: `GET/POST /api/v1/custom-fields` · `GET/PATCH/DELETE /custom-fields/:id` · `GET /custom-fields/scope/:scope` · `POST /custom-fields/templates/:id/apply`.
- Bulk ops: `GET/POST /api/v1/bulk-operations` · `GET /bulk-operations/:id`.
- Marketplace: `GET/POST /api/v1/marketplace/apps` · `GET/PUT/DELETE /marketplace/apps/:id` · `POST /marketplace/apps/:id/{enable,disable,sync}` · `GET /marketplace/apps/:id/logs` · connectors `GET /marketplace/connectors[/:id][/search]` · `POST /marketplace/connectors/:id/reviews`.
- Risk-management lifecycle (ISO 31000): `POST /api/v1/risk-management/{identify,analyze,evaluate,decisions,monitoring-reviews,treatment-plans,audit-reports}` · `POST /risk-management/decisions/:id/approve` · `POST /risk-management/treatment-plans/:id/actions` · `GET /risk-management/risks/:id/lifecycle-status`.
- Scanner: `POST /api/v1/scanner/mitigations/auto-complete`.
- Integrations: `POST /api/v1/integrations/:id/test`.
- Export: `GET /api/v1/export/pdf`.

---

## Planned / not yet implemented
- **Incident War Room collaboration** (participants, in-app + email/Slack/Teams invites & notifications, action tracking that resolves the incident): the per-incident War Room (`/incidents/:id/war-room`) reads real incident header + timeline and can close the incident, but the roster/chat/task board are still fixtures. Making them real needs: an `incident_participants` table + invite endpoints (`POST /incidents/:id/participants`, `DELETE /incidents/:id/participants/:userId`), a war-room note/action model persisted to the timeline, and wiring `pkg/notify` (email/Slack/Teams) to fan out invites + status changes. Tracked in ROADMAP 14.1.
- `/analytics/security-score`, `/analytics/assets/statistics` (dashboard widgets fall back gracefully today).
