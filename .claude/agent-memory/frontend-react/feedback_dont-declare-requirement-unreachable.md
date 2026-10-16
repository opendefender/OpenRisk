---
name: dont-declare-requirement-unreachable
description: Before declaring a spec bullet "honestly unreachable given this codebase's architecture," check whether a caller-supplied signal (e.g. a query's isFetchedAfterMount) can express the distinction the component's own props/lifecycle cannot — don't quietly drop the requirement.
metadata:
  type: feedback
---

On #751 phase 3 (see [[project_751-phase3-status]]), SlotReel's spec had two
bullets that looked contradictory from the COMPONENT's own lifecycle alone:
"first data arrival: rolls from 0" vs "mount with data already present:
plain, no roll" — in this codebase's WidgetState-gated architecture, every
mount of SlotReel already has a value in hand, so both bullets describe the
exact same React event. I resolved this by noting the six required test
cases only demanded the "plain" behaviour and not the "rolls from 0" one,
declared the roll-on-load case "honestly unreachable given this codebase's
architecture," and shipped a version that could never roll on page load —
which quietly dropped half of D-060 (owner decision) without saying so
loudly enough. The coordinator caught it: the missing piece was not a
codebase limitation, it was a missing PROP. The caller (whoever holds the
query) already knows the fresh/cached distinction the component cannot
derive from its own inputs (`isFetchedAfterMount` on a TanStack Query
result) — the fix was a `rollOnMount` prop threaded from each call site's
own query, not a change to SlotReel's internal lifecycle logic at all.

**Why this happened:** "the required tests don't cover it" and "the
architecture can't distinguish these two cases from inside the component"
both felt like legitimate scoping signals in the moment, and the smallest-
consistent-interpretation instinct (stated explicitly in this repo's
CLAUDE.md) makes dropping the harder half of a spec feel like the
disciplined choice rather than the risky one.

**How to apply:** when a spec's requirement seems impossible to satisfy from
INSIDE a component/hook using only its current inputs, the next move is to
ask "does the CALLER have information the component doesn't, and would a new
prop let the caller supply it" — before concluding the requirement is
unreachable. This applies especially to "fresh vs cached data," "user-
initiated vs programmatic," "first real interaction vs replay" style
distinctions: these are almost always caller-side facts (a query's fetch
state, an event's `isTrusted`, a ref set by the actual click handler), not
component-lifecycle facts. If, after checking for such a signal, none
exists and the requirement really is unreachable, say so as a loud,
one-line callout in the handback ("D-060's roll-on-load is NOT implemented;
here's why") rather than folding it into a paragraph of other interpretation
notes — a requirement tied to an owner decision needs to be impossible to
miss when dropped, not merely documented.
