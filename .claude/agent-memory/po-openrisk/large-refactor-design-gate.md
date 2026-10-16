---
name: large-refactor-design-gate
description: How to refine a behaviour-neutral refactor far above the 200-line autonomy limit (precedent #991/#992, router extraction from main()) without escalating to the owner
metadata:
  type: project
---

When an issue is a huge refactor that agents can't do on their own authority (#991: 3,235-line `main()`, 368 routes, the authz chain), split out a design issue. It is a `status:ready` ADR issue (#992) with a one-module spike as evidence. The parent stays `status:needs-refinement` (not `blocked`) because it will be restructured into the child PRs the ADR lists.

**Why:** a tracked, behaviour-neutral refactor reviewed by tech-lead and devsecops is not "autonomous", so the 200-line rule doesn't push it to the owner. What does push it to the owner is the design crossing into redesign. So the design issue gets explicit tripwires that turn it into a D-entry:
- enforcement moves somewhere new, such as a route table becoming the enforcer;
- a new dependency, such as a DI framework.

**How to apply:** re-count the issue's figures on `origin/master`, not the local branch. The local `agent` branch showed 363/82/21 against master's correct 368/87/22. Look for the hidden question the author missed. On #991 there were two:
- no "before" snapshot exists for an identical-before/after criterion;
- dependency construction is interleaved with route mounting.

See [[verify-agent-claims-before-drafting]].
