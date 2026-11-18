---
name: recurring-defect-patterns
description: Defect patterns to check first when reviewing OpenRisk backend diffs
metadata:
  type: feedback
---

- Postgres-only tests (`t.Skip("PostgreSQL only")`) pass as "ok" locally on SQLite; claims backed only by them (p95, locking, SKIP LOCKED) are unverified in review. Say so.
- Docs drift: ADR prose often not updated when the implementer deviates (lease, placeholder rows). Compare ADR text with code, ask for an amendment.
- Comments naming a test file that does not exist (test lives elsewhere). Grep the cited path.
- "Every write path marks X" claims: grep for other writers (boot reconcilers, workers) of the same tables.
- Multi-row `UPDATE ... IN (...)` inside request transactions: lock-order deadlock risk on legacy write paths; flag as WARNING.

**Why:** these recurred in the #555 review.
**How to apply:** run these greps before reading the diff line by line.
