---
name: defect-pattern-ci-retirement
description: Traps when retiring or renaming CI jobs in OpenRisk — branch-protection required contexts, TruffleHog on schedule, CodeQL action EOL, monthly selfhost job
metadata:
  type: project
---

Found in the 2026-10-14 review of #980–#983 (owner goal: master CI always green).

1. **Branch protection on master requires job-name contexts, with enforce_admins on.**
   On 2026-10-14 they were: "Software Composition Analysis", "Secret Detection",
   "Code Quality & Security", "SAST Analysis" (all from security-scanning.yml).
   If you delete or rename one of those jobs, every PR waits forever (#980 showed BLOCKED).
   CI Pipeline, Tenant Isolation and Dependency gate were NOT required at that date.
   **Why:** a renamed job does not report under the old context. **How to apply:** before approving any
   workflow job removal or rename, run `gh api repos/opendefender/OpenRisk/branches/master/protection`
   and list the contexts that change. Changing the protection is an owner action.

2. **TruffleHog with explicit base/head fails on `schedule`.** `base: event.before || default_branch`,
   `head: event.after || github.sha` resolve to the same commit on cron, and the action exits 1
   ("BASE and HEAD commits are the same"). That made Secret Detection red every night.
   Fix: drop base/head and let the action choose by event, or skip the job on schedule.

3. **CodeQL action EOL.** v2 refused since Jan 2025; v3 deprecated December 2026. Use upload-sarif@v4.

4. **selfhost-install.yml runs monthly on master (1st, 06:00 UTC) and on PRs touching Dockerfiles/install.**
   Step 7 has been red since it was written (#650: it greps for a string install.sh no longer prints).
   Any Go/Dockerfile bump PR will show it red. Until #650 lands, master is not green on the 1st of each month.

5. `docker-compose.test.yaml` still uses golang:1.21-alpine while go.mod needs ≥1.25.13 (1.26.9 after #982),
   and README documents it. No workflow uses it.

Related: [[defect-pattern-tenant-isolation]] (Tenant Isolation Coverage Gate is the one security job that is a real gate).
