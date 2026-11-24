---
name: label-taxonomy-drift
description: One label family since D-128 (2026-11-24) — the wave family (priority:P0–P3, type:hardening, area:foundation…quality) is retired; never apply it again
metadata:
  type: project
---

**D-128 (owner, 2026-11-24) supersedes D-003.** The repository speaks one label
family: the CLAUDE.md table (`type:feature|bug|chore|security|docs|design|debt`,
`area:backend|frontend|infra|design|marketing|docs|db`,
`priority:P0-critical|P1-high|P2-medium|P3-low`, `status:…`, `tier:…`).

The wave family is retired: `priority:P0|P1|P2|P3`, `type:hardening`,
`area:foundation|security|grc|risk|resilience|enterprise|quality`. #1508 moved every
issue, open and closed, off it; the labels stay in the repository (AGENT_RULES 7.2)
with a description starting `RETIRED (D-128)`. The Ready gate
(`scripts/ready/check.mjs`) no longer accepts them.

Mapping used, for reading old history: P1→P1-high, P2→P2-medium, P3→P3-low. A wave
`priority:P0` meant "blocks the launch", so it became `P1-high` ("blocks a milestone"),
never `P0-critical` ("production broken or exposed").

**Why:** D-003 made every issue carry two priorities; they drifted apart (#229 was
`P0` and `P1-high` at once) and the owner could not tell which one counted.

**How to apply:** when creating or editing an issue, apply one `type:`, one `area:`,
one `priority:` from the table, plus `status:` and `tier:`. If you meet a wave label,
replace it by the mapping above and say so in the comment. Never create a label.

**Still true from 2026-09-03 (#335):** bare duplicates (`bug`, `backend`, `p0`) and
single-use synonyms (`launch-blocker` vs `launch-gate`) may be removed with the
reason in the issue comment; topical labels with no family equivalent
(`data-integrity`, `mitigation`, `api`) are kept.
