---
name: license-ci-gates-are-theatre
description: The license-check CI job asserts nothing and always passes; the frontend has no licence gate at all and ships react-leaflet under Hippocratic-2.1
metadata:
  type: project
---

Do not treat a green CI run as evidence of inbound licence compliance in this repo.

- `.github/workflows/security.yml`, job `license-check`: writes `modules.json` from
  `go list -json -m all`, never reads it, then `echo "✅ License check completed"`. The comment
  describes a GPL/AGPL deny-list that was never written. It has always passed.
- `frontend/` has **no** licence gate. A manual audit of direct deps on 2026-09-01 found
  **`react-leaflet` under `Hippocratic-2.1`** — not OSI-approved, imposes use restrictions, and is
  not compatible with AGPL redistribution or with most enterprise OSS policies. Also present:
  `react-use` (Unlicense), `@axe-core/playwright` + `axe-core` (MPL-2.0, dev/test only, fine).
- `frontend/package.json` declares no `license` field.

**Why:** a named, green, licence-compliance job that checks nothing is worse than no job — it is
evidence of a control that does not exist, and it is what gets cited in a customer security
questionnaire. The `react-leaflet` finding is the proof the gap is not theoretical.

**How to apply:** when auditing inbound licences, read `go.mod` / `package.json` and the installed
`node_modules/*/package.json` yourself. Never cite the `license-check` job as verification. As of
2026-09-01 this had no issue open; it was flagged as finding F5 on issue #452.

Related: [[openrisk-copyright-ownership]]
