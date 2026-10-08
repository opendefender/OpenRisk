---
name: ssrf-outbound-posture
description: SSRF / outbound-request posture — pkg/netguard review of PR #743 (#573), the zone-suffix deny-list bypass, and the unguarded tenant-URL sinks still open as of 2026-09-21
metadata:
  type: project
---

On 2026-09-21 I reviewed PR #743 (#573). `backend/pkg/netguard` is the single outbound guard. Only three paths use it: integration test probe, vuln live-pull, ticketing (Jira/ServiceNow).

**Verdict at that review: BLOCK.** netip `Prefix.Contains` returns false for zoned IPv6. A URL like `https://[64:ff9b::a00:1%251]/` passed both ValidateURL and the dial-time Control check. I proved it with a scratch `go run`. The fix is `ip = ip.Unmap().WithZone("")` in AddrAllowed. Also missing from the deny-list: `::/96`, `::ffff:0:0:0/96`, `2001::/32` (Teredo) and `100::/64`.

Decimal, octal and hex host encodings (e.g. `https://2852039166/`) pass ValidateURL. The cgo resolver turns them into IPs, and the dial-time check then blocks them. Verified.

Tenant-controlled outbound sinks that did not go through netguard at that review:
- automation channels: Slack/Teams/webhook/SMS, via `pkg/notify` `httpDo`. `/automation/channels/test` returns err.Error().
- scanner collectors that run in-process: Docker (tcp and unix socket), Kubernetes api_server (Insecure TLS when there is no CA), AD `ldap.DialURL`, VMware, and GitHub/GitLab base_url.
- GCP collector: `option.WithCredentialsJSON` accepts untrusted JSON (external_account credential_source.url, token_uri). If the JSON is empty it falls back to ADC, which is the pod's own identity.

**Why:** SSRF is the second recurring defect class after tenant isolation. Every new integration adds a fresh `&http.Client{}`.
**How to apply:** in any audit, grep for `http.Client{`, `DefaultClient`, `ldap.DialURL`, `WithBaseURL` and `WithCredentialsJSON`. Check that each tenant URL goes through netguard. Re-check whether the items above were fixed before repeating them. Related: [[recurring-defect-areas]]
