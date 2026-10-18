---
name: ci-security-gates-posture
description: CI security gate inventory after #978/#979 (2026-10-14) — branch protection drift, Semgrep loss, unscanned agent module and stray agent binary
metadata:
  type: project
---

State as reviewed 2026-10-14 (PR #980 retiring CI jobs, PR #982 Go 1.26.9 / D-072):
- Branch protection on master (enforce_admins=true) requires the contexts "Software Composition Analysis", "Secret Detection", "Code Quality & Security", "SAST Analysis". #980 deletes two of them, so every PR hangs on checks that never report. The real blocking gates (Tenant Isolation Coverage Gate, Dependency gate, Backend Lint & Format) were never required checks.
- Semgrep was NOT broken: "Run Semgrep" passed on master; only upload-sarif@v2 failed. Retiring it removed the only SAST over frontend TS, Dockerfiles and workflow YAML.
- Blocking gosec is golangci-lint's, with golangci default excludes (G204/G304/G301-302 etc.). Standalone gosec is `-no-fail` (report only).
- agent/ module (go 1.25 floor) is scanned by nothing: dependency-gate.sh covers backend + frontend + images only. A binary agent/openrisk-agent (go1.25.12) is tracked in git with 14 reachable stdlib vulns (govulncheck -mode=binary).
- Unreachable-but-present: fiber v2.52.12 (GO-2026-5585), fasthttp v1.52.0 (GO-2026-4950 double-encoded path authz bypass).
- Third-party actions gosec@master and trufflehog@main are unpinned (dependency-gate.yml pins SHAs).
- Repo is public, default workflow token is read-only, repo secrets are only DOCKER_HUB_* (no ANTHROPIC_API_KEY).

**Why:** CI cleanup keeps changing what actually gates; branch protection is configured outside the repo and drifts silently.
**How to apply:** on any workflow PR, run `gh api repos/opendefender/OpenRisk/branches/master/protection` and diff required contexts against the job names that still exist. Running govulncheck locally on backend needs a govulncheck built with go1.26.9 and GOMEMLIMIT (plain `go run` got OOM-killed). See [[recurring-defect-areas]].
