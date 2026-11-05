---
name: deployment-flag-posture
description: OPENRISK_DEPLOYMENT (self_hosted|hosted) review of #1185/D-081 on 2026-10-30 — required at boot and in Helm; residual fall-open via layered in-repo values
metadata:
  type: project
---

#1185 (branch feat/1185-require-deployment, reviewed 2026-10-30): OPENRISK_DEPLOYMENT is
required. Empty or unknown refuses boot (`ParseDeployment`); Helm `openrisk.deployment`
helper requires `backend.deployment` and fails on unknown values or on
`backend.env.OPENRISK_DEPLOYMENT`. Read once at boot from the env only; no tenant or request
path reaches it. A Service built without WithDeployment means hosted, which fails closed. Verdict PASS, 0 blocking.

Fall-open direction is self_hosted: every tenant gets uncapped entitlements, so revenue and
plan bypass. It is not a data leak.

Residual risks to re-check whenever this area moves:
- `helm/values-{prod,staging,dev}.yaml` declare self_hosted. If a SaaS layers one of them
  under its private file and misspells or omits the key, it renders self_hosted silently.
  Helm has no values.schema.json, so unknown keys pass (verified with `backend.deplyoment=hosted`). MEDIUM.
- Compose `${OPENRISK_DEPLOYMENT:-self_hosted}` (root docker-compose.yaml, deploy/selfhost)
  and the Makefile `dev` target turn empty into self_hosted. Judged acceptable, since every
  in-repo file is self_hosted by design. LOW.
- ParseDeployment runs at main.go ~L1192, after the migrations, the admin bootstrap and the
  MFA rekey, so a missing variable still mutates the DB before the fatal. LOW.
- The D-081 entry in docs/DECISIONS.md is still Open with options A and B only, while the code cites "D-081 C". LOW.

**Why:** the SaaS config lives outside the repo, so the in-repo guarantees are the only ones I can verify.
**How to apply:** on any PR that touches entitlements, Helm values or compose, re-check the fall-open paths above. Related: [[recurring-defect-areas]].
