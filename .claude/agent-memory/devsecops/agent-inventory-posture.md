---
name: agent-inventory-posture
description: Security posture of the #910 agent package inventory (OSV/NVD lookups, host_inventories, CloseAbsent), reviewed 2026-10-14 on feat/910-agent-package-scan
metadata:
  type: project
---

#910 was reviewed on 2026-10-14 (branch feat/910-agent-package-scan, diff against feat/911-threat-intel). No blocking findings.

These areas were clean:
- Tenant isolation: every host_inventories/host_packages query and CloseAbsent filter on tenant_id. The pending-key replay uses the authenticated agent's TenantID.
- The 'agent' source can only be written when FromAgent is true. Neither the HTTP import nor the webhook sets it, and ParseVulnSource rejects "agent".
- Egress sends only name, version and ecosystem. The online_vuln_lookup policy fails closed and covers the daily re-match.
- The Redis cache is keyed only on public data.

Open MEDIUM findings:
1. Host identity comes from the hostname in the request body (store_inventory.go). Any agent in a tenant can overwrite another host's inventory and close that host's 'agent' findings as remediated. HostKey does not reject "|", which lets one host's CloseAbsent prefix bleed into another host's findings.
2. POST /scanner/agent/inventory has no rate limit. Each push starts a background match that can run 30 minutes. All tenants share the process-wide OSV/NVD limiters, so one tenant can starve the rest.

LOW findings: UTF-8 split by byte caps, HostKey computed before the hostname is capped, external_id can exceed varchar(255), one NVD error aborts the whole host's match, the OSV advisory id is not path-escaped, OSV_API_URL is not validated, the policy is checked only once per match (TOCTOU), and auto-closing a finding writes no audit entry.

**Why:** agent endpoints trust body fields because the bearer and HMAC prove the tenant, but they do not prove which host is reporting.
**How to apply:** when a later PR touches agent endpoints, check whether these were fixed, and look for new body-asserted identity fields. See [[recurring-defect-areas]].
