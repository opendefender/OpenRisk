# OpenRisk Scanner Agent

A single, tiny (< 15 MB), dependency-free executable that runs on-premise and
performs the scans the SaaS can never run itself (nmap / osquery). It is the
**only** supported way to scan on-prem / hybrid infrastructure.

- Enrols with a 24h **registration token** issued by the SaaS (Infrastructure →
  Deploy Agent).
- Runs continuously in the background (systemd / Windows Service / launchd).
- Holds an SSE stream for jobs, heartbeats to stay `online`, and on a job runs
  **nmap** (`-sV -O --script vuln`, `-O` when root) and **osquery** (if present)
  **locally**, then pushes results back over an **RS256 (scoped `scanner`) + HMAC-SHA256**
  signed channel.
- **Stateless w.r.t. scan data**: only its own credentials are persisted
  (owner-only `state.json`); scan output is discarded after each push.
- Targets are refused if wider than **/24** (scope guard), mirroring the SaaS.
- Checks GitHub Releases every 24h and logs when a newer version is available.

The SaaS pipeline never writes assets/risks: results land in a Redis **preview**
(48h) that a user imports or ignores from the Scan Preview page.

## Build

```sh
cd agent
go build -ldflags="-s -w" -o openrisk-agent .   # ~6.5 MB, stdlib only
# cross-compile, e.g. Windows:
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o openrisk-agent.exe .
```

## Run (first enrolment)

```sh
./openrisk-agent -server https://app.openrisk.io -token <REGISTRATION_TOKEN>
```

The token is single-use for enrolment (valid 24h). After the first run the agent
saves its scoped credentials to the state file and no longer needs `-token`.

| Flag/env | Default | Meaning |
|---|---|---|
| `-server` / `OPENRISK_SERVER` | `http://localhost:8080` | SaaS base URL |
| `-token` / `OPENRISK_TOKEN` | — | 24h registration token (first run only) |
| `-name` | hostname | Agent display name |
| `-state` | OS config dir `/openrisk-agent/state.json` | Credentials file (0600) |
| `-install` | | Print a systemd unit and exit |
| `-version` | | Print version and exit |

## Install as a service

**Linux (systemd):**
```sh
./openrisk-agent -install > /etc/systemd/system/openrisk-agent.service
# edit ExecStart to add -token <TOKEN> for the first boot, then:
sudo systemctl daemon-reload && sudo systemctl enable --now openrisk-agent
```
Grant `AmbientCapabilities=CAP_NET_RAW` to enable nmap OS-detection (`-O`).

**Windows:** run once with `-token` to enrol, then register the binary as a
service (e.g. `sc create OpenRiskAgent binPath= "C:\openrisk-agent.exe -server …"`).

**macOS:** wrap in a `launchd` plist under `~/Library/LaunchAgents`.

**Docker:**
```sh
docker run -d --network host -e OPENRISK_SERVER=https://app.openrisk.io \
  -e OPENRISK_TOKEN=<TOKEN> -v openrisk-agent:/root/.config \
  opendefender/openrisk-agent:latest
```

## Requirements on the host

- **nmap** in `PATH` (required for network scans). `--script vuln` provides CVE
  matching; `-O` (OS detection) needs root / `CAP_NET_RAW`.
- **osquery** (`osqueryi`) optional — augments discovery with local inventory.

## Security

- The agent holds **no cloud credentials** — only its own scoped `scanner` token
  (rotates every 7 days on re-enrolment) and a per-agent HMAC push secret.
- Every push is HMAC-SHA256 signed; the SaaS verifies it before ingesting.
- Revoking the agent from the SaaS invalidates its token immediately (the next
  heartbeat/push/stream call gets 401 and the agent exits).
