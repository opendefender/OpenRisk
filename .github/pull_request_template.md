<!--
  Title format: type(scope): subject (#<issue>) — Conventional Commits, English, imperative.
  One issue → one branch → one PR. Never merge your own PR; merging is the owner's decision.
-->

Closes #

## What changed

<!-- What this does, in user terms. File paths for the substantive changes. -->

## Verification

<!-- The exact commands you ran and their output. "Should work" is not verification. -->

```
```

## Acceptance criteria

<!-- One line per numbered criterion from the issue: ✅ met, ❌ not met with the reason. -->

## Honest remainders

<!-- What is NOT done, and why. Write "nothing" only if that is true. -->

## Checklist

- [ ] **Every commit carries a `Signed-off-by:` trailer**, accepting the
      [Contributor License Agreement](../blob/HEAD/CLA.md) §7. Use `git commit -s`;
      to fix a branch already written, run `git rebase --signoff <base>` and update
      the remote. The **DCO** check enforces this.
- [ ] Every DB query filters by `tenant_id`, or the change touches no query.
- [ ] Tests cover success, not-found and unauthorized, or the change has no use case.
- [ ] No `any` in TypeScript; FR + EN strings present for any new user-facing text.
- [ ] Loading, error and empty states handled for any new UI.
- [ ] No secret, token, password or key is logged or committed.
- [ ] Documentation and `ROADMAP.md` updated if this changes what a user can reach.
