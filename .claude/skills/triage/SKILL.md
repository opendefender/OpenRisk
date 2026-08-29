---
name: triage
description: Triage the OpenRisk backlog — label unlabeled issues, detect duplicates, assess readiness, route to owners. Run weekly or whenever new issues have arrived. Cheap.
argument-hint: "[optional: number of issues, defaults to all unlabeled]"
---

# Backlog triage

Delegate to `issue-triage`.

```
gh issue list --state open --limit 100 --json number,title,labels,milestone,body
```

Triage every issue missing a `type:`, `area:`, `priority:` or `status:` label.
Apply labels. Flag duplicates. Never close anything.

Then hand the `status:needs-refinement` list to `po-openrisk` with this
instruction: refine the top 5 by writing the full issue body template into each,
then flip them to `status:ready`.

Output: the triage table, then a one-line summary, then the refined issue numbers.
