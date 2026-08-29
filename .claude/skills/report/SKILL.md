---
name: report
description: The OpenRisk weekly report to the owner — velocity, milestone burn-down, defect clusters, competitive note, pending decisions and the recommendation for next week. Run at the end of the week.
---

# Weekly report

Delegate to `po-openrisk`.

Read GitHub only — issues closed in the last 7 days, PRs merged, milestone
state, `docs/DECISIONS.md`, and the security and verifier agents' memory files
under `.claude/agent-memory/`. Do not read source.

## Format

```
# OpenRisk — week of <date>

## Milestone
<title> · <closed>/<total> · due <date> · verdict: on track | at risk | slipping
One sentence on why.

## Velocity
Issues closed this week: <n> (previous: <n>) · trend: <up|flat|down>
At this rate the milestone closes on <date>. Target is <date>.

## Shipped
One line per merged PR: #issue — what a user can now do.

## Defect clusters
Top 3 recurring problem areas, with the count and the structural fix.

## Competitive
Anything that moved with Archer, ServiceNow GRC, LogicGate, OneTrust.
Skip this section if nothing moved. Do not pad it.

## Decisions waiting on you
Numbered. Each: the question · the recommendation · the cost of delay.

## Recommendation for next week
The three issues that move the product furthest, with the reasoning.
```

Under 60 lines. No preamble, no "I hope this helps".
