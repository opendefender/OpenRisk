---
name: defect-pattern-merge-side-drop
description: "Merge master into feature" resolved by keeping one side whole silently drops the other side's work, in Go and in Helm (fe9422c5, #1115)
metadata:
  type: project
---

Merge fe9422c5 (master into security/879) settled conflicts by keeping the #879 side whole. It dropped two things:
1. HasLiveSession, RevokeAllExceptFamily and the sweep in gorm_session_repository.go (#1059/#727). master stopped building, which is how it got noticed.
2. The `mfa-encryption-key` entry in helm secrets.yaml and `mfaEncryptionKey` in values.yaml (#1062). Nothing caught this one: the
   deployment still has a secretKeyRef to that key, so inline-secret Helm installs hit CreateContainerConfigError. Restored on the #1115 branch
   in c096b5e3 after review. Also hidden: `helm lint` turns `required` into a WARN and still exits 0, so only `helm template` or the install fails.
3. A test harness that lacked the new `users` table (token_pg_test.go). It only failed on the Postgres CI run, as a timeout.

**Why:** a dropped Go symbol breaks the build, but a dropped YAML key renders without error and only fails at rollout.

**How to apply:** when you review a fix for a bad merge, check every file both parents changed: for each line a parent added, confirm it is
still in the merge result (`git diff base P -- f | grep '^+'` against `git show M:f`). Don't stop at the file named in the issue. See [[defect-pattern-ci-retirement]].
