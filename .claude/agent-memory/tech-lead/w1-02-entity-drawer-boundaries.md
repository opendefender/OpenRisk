---
name: w1-02-entity-drawer-boundaries
description: Module boundaries of the universal entity drawer (W1-02, issue #200) and its 200a/200b/200c split
metadata:
  type: project
---

Issue #200 stays an umbrella; split into 200a (polymorphic contract + registry + tenant
isolation, backend, ADR required), 200b (drawer frontend), 200c (global timeline).

The cut is **per-entity vs tenant-wide**, not backend vs frontend. That is the only cut
that separates the two historical defect classes: parent-gated child IDOR goes to 200a,
all-tenants collection read goes to 200c. See [[defect-pattern-tenant-isolation]].

Per-entity timeline and audit belong to 200a, not 200c — their `ThroughParent` gating is
the decision the ADR exists to make.

**Why:** decomposition was retro-applied onto ~10 commits already on
`feat/w1-02-universal-entity-drawer`, so each child issue is mostly "verify + harden +
test", not "build". The two real gaps as of 2026-08-28: the registry cannot refuse an
ungated resolver, and **no register page opens the universal drawer** — the only import
outside `features/entity-drawer/` is the host mount in `App.tsx`.

**How to apply:** when scoping follow-on drawer work, check whether the register-wiring
(200b) has landed before assuming "inspect any entity in context" is true. Verify with
`grep -rn "from '.*entity-drawer" frontend/src --include=*.tsx | grep -v features/entity-drawer`.

See [[adr-0001-entity-registry]].
