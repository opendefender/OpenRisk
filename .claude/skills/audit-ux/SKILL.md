---
name: audit-ux
description: Run the OpenRisk UX audit protocol on a screen or flow — four personas, keyboard traversal, axe-core, Core Web Vitals and the 20-criterion grid. Use before closing a milestone with UI changes.
argument-hint: <screen, route or flow>
---

# UX audit: $ARGUMENTS

`qa-automation` runs the automated pass, `ux-designer` and `art-director` the
judgement pass.

## Automated (Playwright MCP)

1. Walk the flow as each of the four personas.
2. Capture at 1440px, 768px and 414px.
3. axe-core — every violation with severity and selector.
4. Keyboard-only traversal: focus always visible · nothing unreachable by Tab ·
   Escape closes every overlay · focus returns to the trigger.
5. LCP, INP, CLS on a throttled mid-range mobile profile.

## Judgement

Score the 20-criterion grid. Every score under 4 gets a concrete fix naming
the file.

## Output

Table: criterion · score · evidence · fix · owning agent.
Then `UX GATE: PASS` or `UX GATE: FAIL — <n> blockers`, and an issue opened
for each blocker.

State honestly what could not be proven headlessly. Never assert an
interaction works because the build is green.
