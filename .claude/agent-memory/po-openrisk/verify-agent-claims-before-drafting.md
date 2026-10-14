---
name: verify-agent-claims-before-drafting
description: Teammate and upstream-doc claims must be re-checked against code/git before they enter an issue body or an owner escalation — two incidents, six wrong claims
metadata:
  type: feedback
---

Re-verify every teammate claim against the code or git before writing it into an
issue body, an ADR reference, or an escalation to the owner.

**Why:** during the #200 split (2026-08-28) three separate relayed claims were
wrong, each of which would have shipped as fact:
- "the branch is 94 commits ahead with four unmerged PRs, the release line has no
  defined merge target" — an artefact of a local `master` ref 23 commits stale.
  After `git fetch`, the branch was 10 ahead / 23 behind and every wave PR had
  merged to master normally. This was one `git fetch` away from a fabricated
  release-line crisis reaching the owner.
- "the `finding` alias narrows to scan-sourced rows" — relayed from a package
  doc comment; the resolver implements no such filter and a test pins the
  opposite. The doc was wrong, not the code.
- "the timeline cannot filter by vendor/finding, the vocabularies are unbridged"
  — a bridge exists (`auditTypes`/`typeForAuditEntity`) and is tested. The real
  defect was adjacent and subtler: the inverse is lossy, so a vendor deep-links
  into the asset drawer.

Two of the three came from reading a comment instead of the function under it.

Second incident, the `ds-v1` design-system epic (#439 family, 2026-08-31). Its issue
bodies were drafted from the design guide and from upstream library docs rather
than from this repo, and #443 alone carried four false statements of fact: a
resolved decision described as pending and attributed to the wrong issue
(#440 instead of #452); `@tanstack/react-table` called "already a dependency"
when neither it nor Base UI is in `frontend/package.json`; a vendoring target
(`frontend/src/components/ui/`) that had been deleted six days earlier as the
very debt the epic exists to remove; and an acceptance criterion demanding a
keyboard-trap fix on `FeatureGateModal`, a component whose only occurrence in
the repo is a `❌ PLANNED` row in `ROADMAP.md`. A P0 issue is where an invented
capability does the most damage, because it becomes the spec.

**How to apply:** when a teammate hands you a fact to put in an issue, run the
grep/test/git command that would falsify it first. Cheap ones that paid off:
`git fetch origin master` before trusting any ahead/behind count, reading the
function body when a doc comment asserts a guarantee, and running a failing test
at the merge-base to decide whether a defect is the branch's or master's.
Correcting a teammate costs one message; an unverified claim in an issue body
becomes the spec. See [[label-taxonomy-drift]].

For any issue written against an external library or a design doc, verify four
things before `status:ready`: every path it names exists (`ls`), every dependency
it assumes is in the manifest (`grep package.json`), every component it says is
broken exists (`grep -rn` the repo, and check `ROADMAP.md` for a PLANNED row),
and every issue number it cites is the one that actually does the work.
