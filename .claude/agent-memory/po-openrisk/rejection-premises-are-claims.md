---
name: rejection-premises-are-claims
description: When an issue is bounced to me for refinement, the rejection reasons are claims to falsify like any other — twice now the wave-family labels were reported as invalid taxonomy
metadata:
  type: feedback
---

A refinement request's *rejection reasons* get the same falsification pass as its
factual claims. They arrive framed as findings, which makes them easy to act on
without checking.

**Why:** on #235 (2026-09-07) the `/work` loop bounced the issue with four reasons,
and reason 2 was wrong in a way that would have caused damage: `area:foundation`
and `priority:P2` were reported as "label taxonomy violations against the fixed
table in CLAUDE.md ... both labels exist in the repo, so this is drift", with an
instruction to relabel to `area:frontend`/`area:backend`/`priority:P2-medium`.
Those are the **wave family**, made permanent by D-003. Executing the instruction
as written would have stripped both — the exact mistake I already made once on
#201. The instruction even offered "or flag a backlog-wide hygiene pass", which
would have escalated a non-problem to the owner. Counts settled it in one command:
`area:foundation` 14 open users, `priority:P2` 34. The other three reasons (no
`status:`, no numbered criteria, epic-sized) were all correct — the wrongness was
localised, not systemic, which is why skimming would have missed it.

**How to apply:** for every bounced issue, before editing anything, run the command
that would falsify each rejection reason. For label claims that is
`gh issue list --state open --limit 300 --json number,labels` piped through `jq`
group-by — a label with a double-digit user base is a convention, not drift. Say
plainly in the issue comment which rejection reasons were wrong and why, so the
next agent through the loop does not re-file the same one. `CLAUDE.md`'s label
table is the Constitution family only; it does not enumerate the wave family, so
"not in the table" is never by itself evidence a label is invalid.
See [[label-taxonomy-drift]] and [[verify-agent-claims-before-drafting]].
