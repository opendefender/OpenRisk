---
name: label-taxonomy-drift
description: The repo carries TWO parallel label taxonomies (wave family + CLAUDE.md family); both now have real users, but most of the backlog still speaks only the wave family
metadata:
  type: project
---

GitHub has both label families installed. The CLAUDE.md/Constitution family
(`area:backend|frontend|infra|design|marketing|docs|db`, `priority:P0-critical…P3-low`,
`status:needs-refinement|ready|in-progress|blocked|in-review`, `type:bug|chore|security|docs|design|debt`)
was created by `scripts/gh-labels.sh` and had 0 issues using it as of 2026-08-28.
The family actually in use across most of the backlog is the wave family:
`area:foundation|security|grc|risk|resilience|enterprise|quality` (17 on foundation),
`priority:P0|P1|P2|P3` (86 on P1), `type:feature|type:hardening` only.

**Update 2026-08-30:** `status:` labels are no longer unused — by this date the repo
already had `status:ready`(6) `status:in-review`(5) `status:blocked`(3, incl. mine)
`status:in-progress`(3) `status:needs-refinement`(1) in the wild, applied by other
agents/sessions between 2026-08-28 and 2026-08-30, i.e. adoption moved fast. Do not
assume `status:` labels are decorative-only anymore — check current counts with
`gh api -X GET search/issues -f q='repo:opendefender/OpenRisk label:"status:X"' --jq .total_count`
before assuming either way, this drifts quickly.

**This is decided, not open**: `docs/DECISIONS.md` D-003 (2026-08-28) — "Label
taxonomies: keep both." Owner picked Option C over the recommended Option A
(declare wave canonical): both families stay permanently, and **every issue must
carry both** — one `area:`+`priority:` from the wave family, one from the
Constitution family, plus a `status:`. #409-#412 are the reference examples,
e.g. #410: `area:foundation, priority:P1, area:backend, priority:P1-high,
status:in-progress`. This is NOT a transitional migration state to clean up —
do not drop either family's label from an issue, ever, including one you already
have open for other edits. (I did this by mistake on #201 once — stripped its
wave-family `priority:P1` thinking it was leftover cruft — then had to restore
it and post a correction comment. Don't repeat that.)

**Why:** the Constitution's Ready definition demands the `type:`/`area:`/`priority:`
triplet from its own taxonomy, but a lot of Wave 1 history still speaks the wave
family. D-003 keeps both rather than forcing a backlog-wide migration.

**How to apply:** when creating or editing any issue, apply BOTH `area:`+`priority:`
pairs (wave + Constitution) plus a `status:` label. Never create a new label.
Never remove one family's label to "clean up" — that violates D-003, not fixes it.

**Precedent set 2026-09-03 on #335** — D-003 protects the two *families*, not every
stray label. A third class exists: bare duplicates (`bug`, `backend`, `p0`) that are
strictly subsumed by their `type:`/`area:`/`priority:` equivalents, and single-use
synonyms (`launch-blocker` vs `launch-gate`, which 8 issues carry). Those I remove and
justify in the issue comment. Topical labels with no family equivalent
(`data-integrity`, `mitigation`, `api`) I keep — they cost nothing and carry signal.
Rule of thumb: check `gh issue list --state all --limit 200 --json labels` usage counts
first; a label with a real user base is a convention, a singleton is noise.
