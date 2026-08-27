---
name: ship
description: The OpenRisk milestone close gate — build, tests, cross-tenant proof, security, UX, claim verification, infra readiness and release notes. Run before closing a milestone or tagging a release.
argument-hint: <milestone title or version tag>
---

# Release gate: $ARGUMENTS

Every gate runs. Any failure stops the release. Report the table at the end.

1. **Backlog** — `gh issue list --milestone "$ARGUMENTS" --state open`.
   Any open issue blocks, unless explicitly moved out of the milestone.
2. **Build** — `go build ./...` and `npx vite build`. Paste output.
3. **Tests** — `go test ./... -race -count=1` and the frontend suite. Paste output.
4. **Lint** — `golangci-lint run`, `npx tsc -b`, `npx eslint . --max-warnings=0`.
5. **Tenant isolation** — delegate to `devsecops`: prove every endpoint added in
   this milestone has a passing cross-tenant test. Missing test = blocker.
6. **Dependencies** — `govulncheck ./...`, `npm audit --audit-level=high`.
7. **Security** — `devsecops` full pass. CRITICAL or HIGH blocks.
8. **UX** — `/audit-ux` on every screen changed in this milestone.
9. **Truth** — `product-verifier`. `NO-GO` blocks. Update `ROADMAP.md` module
   statuses to match reality.
10. **Infra** — `devops-sre`: rollback command documented, migration job present.
11. **Release notes** — `copywriter`, FR and EN, every claim verified.

Final line: `SHIP: GO` or `SHIP: NO-GO — <blocking gate>`.
If GO, prepare the release but **do not tag or publish** — that is my call.
