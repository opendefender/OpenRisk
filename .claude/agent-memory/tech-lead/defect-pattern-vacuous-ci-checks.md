---
name: defect-pattern-vacuous-ci-checks
description: CI gates that pass on an empty fixture DB or ignore status codes (#1114 DAST no-store curl, helm lint vs required); ZAP baseline facts for 2.17.0
metadata:
  type: project
---

CI checks in this repo keep going green for the wrong reason. Seen in the 2026-10-23 review of #1114/#1133:

- The #1114 header check curls each GET route and reads Cache-Control but never checks the status. On the empty DAST DB, /organization/logo and /users/me/avatar (the only handlers that set `private, max-age=300`) return 404, which carries the middleware's default no-store, so the check never sees the values it exists to catch.
- `helm lint --strict` does not fail on a missing `required` value (helm v4.3.1 logs WARN "missing required values" and exits 0). Only `helm template` fails. #1133 tests by rendering.
- ZAP 2.17.0 zap-baseline classic path (--autooff): `Total of N URLs` = len(zap.core.urls()), printed only when N>0. Reports are not written when N=0. Informational alerts are skipped in zap_get_alerts. The target is cut back to the host root. Without -silent it adds `-addonupdate`. ZAP_AUTH_HEADER_VALUE goes on *every* request unless ZAP_AUTH_HEADER_SITE is set. ZAP's own stdout goes to /zap/zap.out, not into the uploaded log.

**Why:** a gate that cannot fail is worse than no gate. It gets cited as proof under rule 12.
**How to apply:** for any new CI assertion, ask what the fixture DB holds and whether a 4xx or 5xx response would still pass. Also run the gate once with its guard taken out (a mutation). Related: [[defect-pattern-ci-retirement]].
