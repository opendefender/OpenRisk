---
name: red-ci-gates-are-not-gates
description: Security/CI jobs that "exist" in workflows may be red on master for weeks and not required; check run status and branch protection before writing criteria
metadata:
  type: project
---

A workflow job existing is not a control. On 2026-09-24, while refining #487, both
`security.yml` and `security-scanning.yml` were red on master (govulncheck + Trivy finding
real HIGH vulns) and none of those jobs was a required check, so nothing was blocked.
Trivy was pinned on `@master`, the published image was built from a different Dockerfile
than the one scanned.

**Why:** Issue drafts that say "add a blocking scan" miss that one already exists and is ignored;
the real work is making it green and required.

**How to apply:** When refining any CI/security issue, run `gh run list --workflow=<f> --branch master`
and `gh api repos/opendefender/OpenRisk/branches/master/protection` first, and put "green at merge"
plus "owner adds it to required checks" in the criteria/Next. Related: [[verify-agent-claims-before-drafting]].
Also: I split #487 into #487 (ready) + #795/#796 (blocked on D-054/D-055) so owner-dependent
parts do not freeze the tool-only work — same pattern as [[split-bundled-p0-issues]].
