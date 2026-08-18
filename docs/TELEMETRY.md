# OpenRisk Telemetry

OpenRisk can send **anonymous** usage statistics to help us prioritise. It is
built to be irreproachable — because in security, sneaky telemetry destroys trust
overnight.

## The three guarantees

1. **Opt-in.** Telemetry is **disabled by default**. Nothing is ever sent until an
   admin explicitly enables it in **Settings → Billing → Télémétrie anonyme**.
2. **Kill switch.** Set the environment variable `OPENRISK_TELEMETRY=off` (or
   `0`/`false`/`no`/`disabled`) to hard-disable telemetry **regardless** of any
   in-app consent. This always wins.
3. **Anonymous.** The payload carries a random instance identifier and **coarse
   counts only**. It never contains an organization name, a user, an email, a
   hostname, an IP address, or any risk/asset/compliance content.

## Exactly what is sent

The complete payload is the `Payload` struct in
[`backend/pkg/telemetry/telemetry.go`](../backend/pkg/telemetry/telemetry.go).
Nothing outside this struct is ever transmitted:

| Field               | Example         | Notes |
|---------------------|-----------------|-------|
| `instance_id`       | random UUID     | The only identifier. Maps to no org/user/network. |
| `version`           | `1.4.2`         | OpenRisk version, for compatibility stats. |
| `sent_at`           | RFC3339 UTC     | Send timestamp. |
| `os` / `arch`       | `linux`/`arm64` | Coarse platform facts. |
| `db`                | `postgres`      | Database engine. |
| `orgs`              | `4`             | Number of organizations. |
| `users_bucket`      | `11-50`         | Bucketed count (never a raw number). |
| `risks_bucket`      | `201-1000`      | Bucketed count. |
| `assets_bucket`     | `1-10`          | Bucketed count. |
| `plan_distribution` | `{free:3,pro:1}`| How many orgs per tier. No identity. |

Counts are **bucketed** (`0`, `1-10`, `11-50`, `51-200`, `201-1000`, `1000+`) so a
small deployment cannot be re-identified by an unusually specific number.

## Verifying for yourself

Read the package — it is deliberately tiny and dependency-free. `GET /api/v1/telemetry`
returns the current state (`enabled`, `consent`, `env_forced_off`, `instance_id`).
`PUT /api/v1/telemetry` (admin) toggles consent.
