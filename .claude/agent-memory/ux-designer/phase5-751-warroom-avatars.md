---
name: phase5-751-warroom-avatars
description: Findings from spec'ing issue #751 phase 5 item "avatar-group-hover" (War Room responder stack) — data model limits, component contract decisions
metadata:
  type: project
---

Spec'd 2026-09-29 on branch `754-securityauth-...` (read-only pass, no code written). Full spec
delivered to caller via SubagentHandback that day and saved at the time to a scratchpad path
(session-scoped, will not persist) — re-derive detail from code if this memory and the code
disagree, this is a snapshot.

**The War Room "responder list" is not backed by a participants table.** `WarRoom.tsx:155-181`
builds it from two plain strings on `Incident` (`incidentService.ts`): `reported_by: string`
(always set) and `assigned_to?: string` (optional). Per person: `name` (raw string, may be an
email), a `role` that is **not data** — fixed by array position ("Assignee"/"Reporter") — and
computed `init`. No `userId`, no avatar photo, no email field distinct from name, no
online/presence, no join-time. Max entries today is 2. `UserAvatar.tsx` (real photo via
`useAvatarUrl`) does not apply here — WarRoom already renders the initials-only `Avatar` from
`shared/ui.tsx`; the avatar stack must stay initials-only, not silently upgrade to photos.

**Real gap the stack fixes, not just decoration.** The rail carrying this list is
`hidden md:block` — below `md` the responder/reporter info disappears entirely today. Spec puts
the compact `AvatarStack` in the top bar at all breakpoints (new placement) plus the rail header
(≥md, rows kept unchanged below it) so the sub-md gap actually closes instead of the stack being
pure top-bar polish.

**Duplicate-person edge case decided explicitly:** if `assigned_to === reported_by`, the entry
list still renders two entries (two roles) rather than deduping to one avatar — the dual-role fact
is itself information a GRC user needs, per [[owner-overrides-on-scope-and-polish]]-adjacent
reasoning (don't let a "cleaner" visual merge erase a real distinction).

**Why:** consistent with `[[phase3-751-dashboard-spec]]` pattern for this issue — INVENT NOTHING
is binding even when a feature name ("avatar-group-hover") implies richer social/presence UI than
the data supports.

**How to apply:** if a future phase adds a real participants table / user IDs to incidents, this
spec's "initials-only, no presence, N capped at 2 today" constraints are stale — re-read
`incidentService.ts`'s `Incident` interface before reusing this spec, don't assume it still holds.
