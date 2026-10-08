---
name: split-bundled-p0-issues
description: Recurring backlog pattern — a P0 bug issue that also specifies a new cross-cutting mechanism; split the bug out so it ships, park the mechanism behind a DECISIONS.md entry
metadata:
  type: project
---

Wave 1 launch-gate issues written early (the `OR-P0-xx` series) tend to bundle a small,
verifiable bug fix with an ambitious architectural mechanism that would cure the same
symptom more generally. #335 was the exemplar: a missing DB transaction plus a 500-after-commit
(both S, both self-contained) bundled with an `Idempotency-Key` mechanism for *all* mutation
endpoints plus schema-level uniqueness constraints.

**Resolution that worked (2026-09-03):** keep the bug in the original issue and get it to
`status:ready`; open the mechanism as its own issue at `status:needs-refinement`; append the
design question to `docs/DECISIONS.md` as a pending entry with a recommendation and an honest
cost-of-delay; cross-link both ways; and state the reasoning *inside* the issue body, not only
in a comment, so the next reader does not re-litigate the split. #335 → #517, D-028.

**Why:** the bug fix is a launch gate and the mechanism needs an owner decision. Bundled, the
P0 waits on an unanswered question for no gain. Also worth checking before opening the new
issue: an umbrella issue may already nominally own the mechanism (#239 W8-06 listed
"idempotency" in a scope sentence) — comment on it rather than let two designs appear.

**How to apply:** when refining any `OR-P0-xx` or launch-gate issue, first ask which acceptance
criteria the *narrow* fix actually satisfies. Criteria that survive only with the big mechanism
are the split line. Say plainly in the body what the narrow fix does and does not cure — for
#335, it makes a retry-after-error safe but cannot cover a response the client never received.
Relates to [[label-taxonomy-drift]] and [[verify-agent-claims-before-drafting]].
