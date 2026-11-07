# #214 — TPRM v1 · Live Proof Record

Everything below was observed against a running stack on 2026-11-07. Nothing is
inferred from reading code; what was not observed live is listed under
[Not proven here](#not-proven-here).

## Environment

| | |
| --- | --- |
| Date | 2026-11-07 |
| Commit | `2572fd10` (`develop`, every TPRM child PR merged: #675–#684) |
| Backend | go1.26.9 (`go version` on the binary), built from that commit, `:58214`, `OPENRISK_DEPLOYMENT=hosted` |
| Frontend | Vite dev server from the same commit, `:57214`, French UI |
| Database | throwaway `postgres:16-alpine`, migrations applied through `DATABASE_URL` |
| Cache | throwaway `redis:7-alpine`, `REDIS_URL` set |
| Mail | local SMTP sink, `EMAIL_TRANSPORT=smtp` |

## Tenants

Four organisations created through the real sign-up (`POST /auth/register`).
There was no subscription row, so the stored `organizations.plan` decides the
plan; it was set in SQL before the first request, and `GET /entitlements`
confirmed it for each tenant.

| Tenant | Plan | Role in the proof |
| --- | --- | --- |
| A | business | runs the journey |
| B | business | cross-tenant attacker |
| C | free | plan gate |
| D | pro | plan gate |

## Criterion 2 — the journey, in a real browser

Tenant A's owner, logged in through the login screen. Before the journey, an
application asset `Core banking (proof 214)` (criticality HIGH) and a risk on it
(`Core banking outage via hosting provider`, P 0.6 × I 8, score 12) were created
through the API.

1. **Create a vendor.** `/vendors` showed its empty state, then *Référencer un
   fournisseur* opened the asset form with the vendor category. `HostCo
   Datacenter` was saved. Register: `1 fournisseur référencé · 1 service
   critique`, status `Jamais évalué`.
2. **Link it to an asset that carries a risk.** On `/vendors/:id`, *Relier un
   actif* → `Core banking (proof 214)`, relation `Hébergé par le fournisseur`.
   The section became `Actifs et risques associés (1)` and listed the risk with
   `Score 12 · Critique`.
3. **Send a questionnaire.** A template `Évaluation sécurité hébergeur` was
   created on `/vendors/questionnaires/new`: two weighted choice questions
   (weights 8 and 6) and one free-text question. *Envoyer un questionnaire*
   with a due date of 2026-11-21 → `Questionnaires (1) … Envoyé`. The first
   send ran with `EMAIL_TRANSPORT=log`, so the link never left the server; the
   server was restarted on the SMTP sink and *Renvoyer* issued a new link,
   received as mail `Nouveau lien : questionnaire de sécurité de Tenant a 214`.
4. **Answer through the public link in a private window.** The link opened in a
   separate browser context with 0 cookies and an empty `localStorage`. The
   page read `Tenant a 214 vous demande de compléter un questionnaire de
   sécurité pour HostCo Datacenter`. Draft saved: `PUT
   /api/v1/public/vendor-assessment/answers`, with the token in the
   `X-Vendor-Assessment-Token` header and not in the URL.
5. **Submit.** `POST /api/v1/public/vendor-assessment/submit` → 200, and the page
   showed `Merci — vos réponses ont été transmises`.
6. **See the score, tier and breakdown.** Back in tenant A, the vendor showed
   `Soumis · Score 42,9 · Élevé`, and the register showed `Plan d'action requis`.
   The review page showed `42,86 sur 100 · Élevé · Formule vendorscore/1` with
   one row per question: Q1 weight 8, points 1, contribution 0; Q2 weight 6,
   points 0, contribution 42,86; Q3 `Réponse libre : non notée`. That matches
   100 × 6 × (1 − 0) ÷ 14.
7. **See the vendor→asset→risk chain.** On the vendor page: `Hébergé par le
   fournisseur → Core banking (proof 214) → Core banking outage via hosting
   provider, Score 12`. The risk's score stayed at 12 after the vendor was
   scored. The same independence is pinned by
   `TestSubmitPublicAssessment_NeverTouchesLinkedRisksOrAssets`
   (`internal/application/tprm/score_assessment_test.go`) and
   `TestBoundary_NothingInTheRiskScoringPathImportsVendorScore`
   (`pkg/vendorscore/imports_test.go`).

The 43-character token was searched for in both server logs: not found.

## Criteria 3 and 4 — API probe

The script logs in as each tenant and reads tenant A's ids as A. It replays
every authenticated TPRM route in `cmd/server/main.go` as C (Free) and D (Pro),
then replays every route that carries one of A's ids as B, with A's public
token in the `X-Vendor-Assessment-Token` header. Output, verbatim:

```
A ids: vendor=4e38996f-960a-4af8-a4e2-510bcd07ceac assessment=1a983707-58c2-431b-af8d-ff7e92f58e2b template=41eacbd2-a003-44b7-bee5-30dbf30df761 link=52c47490-c5ce-4cec-87e9-9025bba0af41

== Criterion 4: Free (C) and Pro (D) on every authenticated TPRM route ==
tenant c plan=free
  free GET    /vendors                                                               402 OK
  free GET    /vendors/4e38996f-960a-4af8-a4e2-510bcd07ceac/chain                    402 OK
  free POST   /vendors/4e38996f-960a-4af8-a4e2-510bcd07ceac/assets                   402 OK
  free DELETE /vendors/4e38996f-960a-4af8-a4e2-510bcd07ceac/assets/52c47490-c5ce-4cec-87e9-9025bba0af41 402 OK
  free GET    /vendor-questionnaire-templates                                        402 OK
  free POST   /vendor-questionnaire-templates                                        402 OK
  free GET    /vendor-questionnaire-templates/41eacbd2-a003-44b7-bee5-30dbf30df761   402 OK
  free PUT    /vendor-questionnaire-templates/41eacbd2-a003-44b7-bee5-30dbf30df761   402 OK
  free POST   /vendor-questionnaire-templates/41eacbd2-a003-44b7-bee5-30dbf30df761/archive 402 OK
  free GET    /vendors/4e38996f-960a-4af8-a4e2-510bcd07ceac/assessments              402 OK
  free POST   /vendors/4e38996f-960a-4af8-a4e2-510bcd07ceac/assessments              402 OK
  free GET    /vendor-assessments/1a983707-58c2-431b-af8d-ff7e92f58e2b               402 OK
  free POST   /vendor-assessments/1a983707-58c2-431b-af8d-ff7e92f58e2b/revoke        402 OK
  free POST   /vendor-assessments/1a983707-58c2-431b-af8d-ff7e92f58e2b/resend        402 OK
tenant d plan=pro
  pro  GET    /vendors                                                               402 OK
  pro  GET    /vendors/4e38996f-960a-4af8-a4e2-510bcd07ceac/chain                    402 OK
  pro  POST   /vendors/4e38996f-960a-4af8-a4e2-510bcd07ceac/assets                   402 OK
  pro  DELETE /vendors/4e38996f-960a-4af8-a4e2-510bcd07ceac/assets/52c47490-c5ce-4cec-87e9-9025bba0af41 402 OK
  pro  GET    /vendor-questionnaire-templates                                        402 OK
  pro  POST   /vendor-questionnaire-templates                                        402 OK
  pro  GET    /vendor-questionnaire-templates/41eacbd2-a003-44b7-bee5-30dbf30df761   402 OK
  pro  PUT    /vendor-questionnaire-templates/41eacbd2-a003-44b7-bee5-30dbf30df761   402 OK
  pro  POST   /vendor-questionnaire-templates/41eacbd2-a003-44b7-bee5-30dbf30df761/archive 402 OK
  pro  GET    /vendors/4e38996f-960a-4af8-a4e2-510bcd07ceac/assessments              402 OK
  pro  POST   /vendors/4e38996f-960a-4af8-a4e2-510bcd07ceac/assessments              402 OK
  pro  GET    /vendor-assessments/1a983707-58c2-431b-af8d-ff7e92f58e2b               402 OK
  pro  POST   /vendor-assessments/1a983707-58c2-431b-af8d-ff7e92f58e2b/revoke        402 OK
  pro  POST   /vendor-assessments/1a983707-58c2-431b-af8d-ff7e92f58e2b/resend        402 OK

== Criterion 3: tenant B (Business) against tenant A's ids, with A's public token in the header ==
  GET    /vendors -> 200, B sees 0 vendor(s), A's vendor visible: False OK
  GET    /vendor-questionnaire-templates -> 200, A's template visible: False OK
  GET    /vendors/4e38996f-960a-4af8-a4e2-510bcd07ceac/chain                    404 leak=False OK
  POST   /vendors/4e38996f-960a-4af8-a4e2-510bcd07ceac/assets                   404 leak=False OK
  DELETE /vendors/4e38996f-960a-4af8-a4e2-510bcd07ceac/assets/52c47490-c5ce-4cec-87e9-9025bba0af41 404 leak=False OK
  GET    /vendor-questionnaire-templates/41eacbd2-a003-44b7-bee5-30dbf30df761   404 leak=False OK
  PUT    /vendor-questionnaire-templates/41eacbd2-a003-44b7-bee5-30dbf30df761   404 leak=False OK
  POST   /vendor-questionnaire-templates/41eacbd2-a003-44b7-bee5-30dbf30df761/archive 404 leak=False OK
  GET    /vendors/4e38996f-960a-4af8-a4e2-510bcd07ceac/assessments              404 leak=False OK
  POST   /vendors/4e38996f-960a-4af8-a4e2-510bcd07ceac/assessments              404 leak=False OK
  GET    /vendor-assessments/1a983707-58c2-431b-af8d-ff7e92f58e2b               404 leak=False OK
  POST   /vendor-assessments/1a983707-58c2-431b-af8d-ff7e92f58e2b/revoke        404 leak=False OK
  POST   /vendor-assessments/1a983707-58c2-431b-af8d-ff7e92f58e2b/resend        404 leak=False OK

== A's own data unchanged after B's attempts ==
  chain unchanged: True
  assessment status: submitted score: 42.86
  template archived: None

== Public link, no session ==
  no token           -> 404
  forged token       -> 404
  token in query     -> 404 (query string must not authenticate)
  A's token, header  -> 200 (submitted)
  edit after submit  -> 409

== Public payload shape (A's token) ==
  keys: ['due_at', 'items', 'items.[].answer_comment', 'items.[].answer_na', 'items.[].answer_type', 'items.[].answer_value', 'items.[].help', 'items.[].id', 'items.[].na_allowed', 'items.[].options', 'items.[].options.[].label', 'items.[].options.[].value', 'items.[].position', 'items.[].required', 'items.[].text', 'language', 'organization_name', 'read_only', 'status', 'vendor_name']
  contains 'tenant_id': False
  contains 'Core banking': False
  contains 'outage': False
  contains 'asset': False
  contains 'risk': False
  contains 'score': False
  contains 'created_by': False
  contains 'security@hostco.test': False
  Cache-Control: no-store | Referrer-Policy: no-referrer
```

The public payload holds the questionnaire only: no tenant id, asset, risk,
score, contact address, or answer points.

## Not proven here

- **Reminders J-7/J-3/J-1** were not replayed live. They are covered by
  `send_vendor_assessment_reminders_test.go`.
- **Self-hosted access** to TPRM was not booted live. It rests on
  `TestSelfHosted_MatchesD040` and `internal/middleware/hosted_plan_selfhost_test.go`.
- **The token of the first send** could not be checked against the 410 a
  replaced link must return, because the log transport never released it.
  Resend replacing a token is covered by `vendor_assessments_test.go`.
- **Screenshots** of the register, the submitted vendor page, the score
  breakdown and the vendor chain were taken during the run. They are not
  committed, and the GitHub REST API cannot attach an image to an issue
  comment, so they are not on the issue either. The DOM text quoted above is
  the record.
