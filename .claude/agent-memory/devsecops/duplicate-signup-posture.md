---
name: duplicate-signup-posture
description: #686 review 2026-10-21 — register duplicate guard (409/429/alert), D-073 existence disclosure accepted, username-collision oracle, no blocking finding
metadata:
  type: project
---

#686 (branch security/686-duplicate-signup) reviewed 2026-10-21: PASS, no blocking finding.

- D-073 (owner): registration discloses whether an address is registered. Accepted risk — do not re-raise the 409 or the 429 as findings.
- The per-address limit (5/h then 1 h lock) does NOT slow enumeration across many addresses; only the per-IP limiter (auth-register 10/15 min) does. D-073's "slows that down" is only true per address.
- Zero-side-effect oracle: register with a taken `username` + unknown address -> USERNAME_TAKEN; registered address -> EMAIL_ALREADY_REGISTERED (email checked first). Inside D-073, but cheaper than the 201 path. Watch if D-073 is ever reversed: that path must be closed too.
- Alert throttle relies on Redis SETNX (authmfa.AttemptStore.Lock); Redis outage falls back to per-replica memory -> N alerts / N×5 budget. Same as #688 sign-in lock.
- Audit rows store unsalted SHA-256 of the address (same as reset keys) — dictionary-reversible by DB readers. LOW, project-wide pattern.

**How to apply:** if D-073 is reopened or a uniform answer is introduced, check resolveUsername ordering and the 429 in the same change. See [[recurring-defect-areas]], [[auth-token-type-posture]].
