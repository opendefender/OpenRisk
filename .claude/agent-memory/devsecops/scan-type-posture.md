---
name: scan-type-posture
description: #1164 review 2026-10-29 — scan_type on scan jobs/agent dispatch; PASS, no findings; pre-existing replay ignores AgentIDs
metadata:
  type: project
---

#1164 (branch feat/1164-scan-types, base feat/912-infrastructure-page) reviewed 2026-10-29: SECURITY PASS, no findings.

- scan_type only picks one of two hard-coded nmap arg lists in agent/scan.go nmapArgs; unknown => vulnerability. Not injectable.
- Server validates type before the tenant-scoped config lookup, provider check after it, so a foreign config still reads as not-found (no provider oracle).
- No app BodyLimit set: Fiber default 4 MB applies to every route.
- Pre-existing, not new: queuedJobFrames replay drops AgentIDs (intra-tenant agent restriction ignored on reconnect); agent target validation (validateTarget) is the real argv guard.

**How to apply:** if a third scan type or user-supplied nmap options appear, re-audit nmapArgs. See [[agent-inventory-posture]].
