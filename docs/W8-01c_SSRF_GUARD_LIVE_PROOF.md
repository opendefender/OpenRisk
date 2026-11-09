# W8-01c — SSRF guard for every outbound fetch · Live Proof Record

Issue #1315 (part of #229). Everything below was observed by running code from
this branch. Where a claim rests on reading code rather than running it, it says
so and is listed under [Known Limitations](#known-limitations).

---

## Environment

| | |
| --- | --- |
| Date | 2026-11-06 (first run); 2026-11-09 (re-run after review round 1) |
| Go | go1.26.9 linux/amd64 |
| Branch | `security/1315-ssrf-guard`, cut from `develop` at `8f7bbaec` |
| Host | Linux workstation, LAN address `192.168.0.224`, loopback `127.0.0.1` |
| Cloud metadata | none on this host — `169.254.169.254` attempts prove the request is refused before any dial, not that a metadata server answered |

The full API was not booted. The proof drives the **production code paths**
(`pkg/notify` with its default client, `automation.ChannelService.Save`,
`automation.ChannelTester.Test`, `netguard.Client`) from a throwaway `go run`
harness against real listening servers. The harness source is reproduced at the
end; it was deleted after the run (it contains an unguarded control client that
the bypass-detection test would rightly reject).

---

## The shared guard

`backend/pkg/netguard/netguard.go` (#573, extended in #750) is the one guard.
This issue routes every remaining tenant-URL call through it; the guard itself
is unchanged.

| Layer | What it refuses |
| --- | --- |
| `ValidateURL` (save and send time) | any scheme but `https`; credentials in the URL; `localhost` / `*.localhost`; literal loopback, link-local (`169.254.0.0/16`, `fe80::/10`), private (`10/8`, `172.16/12`, `192.168/16`, `fc00::/7`), CGNAT, unspecified, multicast, NAT64/6to4/Teredo/IPv4-mapped forms; encoded hosts (`2130706433`, `0x7f.1`, `127.1`) |
| `Client` dialer `Control` hook (connect time) | the **resolved** address, on every connection and every redirect hop — the DNS-rebinding window is closed because the check runs on the IP actually being connected to |
| `Client` `CheckRedirect` | a redirect to another host; a redirect that fails `ValidateURL` (http downgrade, `file://`, literal internal host); more than 5 hops |
| `Client` transport | ignores `HTTP(S)_PROXY`, so a proxy cannot fetch a denied target on our behalf |

**Scheme decision.** The issue text says "non-http(s) schemes"; the guard is
**https-only**, which is stricter. No inventoried tenant-URL caller needs plain
http: Slack and Teams webhooks are https, and a generic webhook or SMS gateway
carries an HMAC signature or an API key that must not travel in clear.

**Behaviour changes** an operator or a tenant can see:

1. `automation.ChannelService.Save` (`PUT /automation/channels`) now answers 400
   for a Slack, Teams, webhook or SMS gateway URL that is `http://` or points at
   a denied address. Such URLs used to be stored. An on-premise SMS gateway that
   only speaks plain http can no longer be saved: `OUTBOUND_ALLOWED_CIDRS` reopens
   private ranges but never reopens `http://`.
2. `automation.ChannelTester.Test` (`POST /automation/channels/test`) answers 400
   for a refused URL, including one stored before the guard existed. It used to
   answer 200 with `delivered: false` and the transport error.
3. `scanner.cloudScanner.Validate` refuses a GCP service-account key whose
   `token_uri` is `http://` or an internal address.
4. `webhooks.WebhookProvider.Validate` no longer accepts `http://`. It has no
   caller outside its own package.
5. The guarded client ignores `HTTP(S)_PROXY` (a design choice of netguard since
   #573). A deployment whose only egress is an HTTP proxy loses Slack, Teams,
   webhook and SMS delivery, M365 Graph paging and the GCP token fetch, as it
   already did for every other guarded path.

The refusal message returned to the API caller is fixed:
`<field>: target address is not allowed (must be a public https host)`. It names
the field to fix, never the stored URL (for Slack and Teams the path is the
secret) nor the internal address it resolved to. A delivery failure that is not
a refusal reports the operation and a reason class (`timed out`, `connection
failed`), not the URL.

**Operator escape hatch.** `OUTBOUND_ALLOWED_CIDRS` can open specific private
ranges (an on-prem Jira, an internal bus). Loopback, link-local and unspecified
addresses stay denied whatever it says.

**Typed error.** The rule is that `pkg/` does not import `internal/` (one
existing exception predates this issue: `pkg/notify` imports
`internal/infrastructure/email`). So the guard keeps its own
sentinel `netguard.ErrDenied`, and the application layer maps it with the new
`domain.NewOutboundDeniedError(field, cause)` (`backend/internal/domain/errors.go`):
an `*AppError` with status 400 whose `Err` is `errors.Join(ErrValidation, cause)`.
`errors.Is(err, domain.ErrValidation)` **and** `netguard.IsDenied(err)` are both
true on the result.

---

## Inventory — every outbound call

### Calls that take a tenant- or user-supplied host — all guarded

| # | Call site | URL source | Save-time check | Send-time guard | Status |
| --- | --- | --- | --- | --- | --- |
| 1 | `pkg/notify/chatops.go` `PostSlack` | tenant automation channel `slack_webhook_url` | `automation/channels.go` `checkChannelURLs` | `notify.send` → `ValidateURL` + `netguard.Client` | **fixed here** (was `&http.Client{}`) |
| 2 | `pkg/notify/chatops.go` `PostTeams` | tenant `teams_webhook_url` | same | same | **fixed here** |
| 3 | `pkg/notify/outbound.go` `PostWebhook` | tenant `webhook_url` | same | same | **fixed here** |
| 4 | `pkg/notify/outbound.go` `SendSMS` | tenant `sms_gateway_url` | same | same | **fixed here** |
| 5 | `internal/infrastructure/automation/notifier.go` (rule actions, incident notifications) | stored tenant Slack/Teams URL | — (stored config) | passes `nil` doer → #1/#2 guarded path | **fixed here** (via #1/#2) |
| 6 | `internal/application/automation/channel_test_usecase.go` (channel test) | stored tenant config, possibly saved before the guard | — | #1–#4; refusal returned as `NewOutboundDeniedError` (400) | **fixed here** |
| 7 | `internal/infrastructure/integrations/notifications/slack_provider.go` | webhook URL given to `NewSlackProvider` | `Validate` → `ValidateURL` | `guardedDo` → `ValidateURL` + `netguard.Client` | **fixed here** (no production caller today) |
| 8 | `internal/infrastructure/integrations/webhooks/webhook_provider.go` | `url` argument / `webhook_url` config | `Validate` → `ValidateURL` (was http-or-https prefix check) | `ValidateURL` + `netguard.Client`; a denial is not retried | **fixed here** (no production caller today) |
| 9 | `internal/scanner/collectors/gcp.go` service-account token exchange | `token_uri` inside the tenant's `service_account_json` | `scanner/cloud.go` `validateGCPTokenURI` | `gcpCredentials` → `credentials.DetectOptions{Client: egress.client(...)}` | **fixed here** (was the SDK default client — a real gap, see below) |
| 10 | `internal/scanner/collectors/m365.go` Graph paging | fixed `graph.microsoft.com`, but each next page is the response's `@odata.nextLink` | — | `egress.client` | **fixed here** (was `http.DefaultClient`) |
| 11 | `pkg/ticketing` (Jira, ServiceNow) | tenant base URL | `ticketing.go:64` `ValidateURL` | `ticketing.go:47` `netguard.Client` | already guarded (#573) |
| 12 | `internal/vulnscan/livepull` (Tenable, Qualys, Rapid7, …) | tenant `base_url`, `token_url` | `application/vulnerability/integrations.go:77` | `livepull.go:50` `netguard.Client`; re-check `application/vulnerability/livepull.go:46` | already guarded (#573) |
| 13 | `internal/handler/integration_handler.go` test-connection probe | tenant `api_url` | `:84`, `:105` `ValidateURL` | `:57` `netguard.Client{NoRedirects}` | already guarded (#573) |
| 14 | `internal/scanner/collectors/kubernetes.go` | tenant `api_server` | `cloud.go` `httpsEndpoint` | `egress.dialer()` | already guarded (#750) |
| 15 | `internal/scanner/collectors/dockerapi.go` (tcp/http/https) | tenant `host` | `cloud.go` `validateDockerHost` | `egress.dialer()` in the transport | already guarded (#750) |
| 16 | `internal/scanner/collectors/vmware.go` | tenant `url` | `httpsEndpoint` | `egress.dialer()` + guarded `DialTLSContext` | already guarded (#750) |
| 17 | `internal/scanner/collectors/active_directory.go` (LDAP, not HTTP) | tenant `url` | `endpoint("url", …, "ldap","ldaps")` | `ldap.DialWithDialer(egress.dialer())` | already guarded (#750) |
| 18 | `internal/scanner/collectors/github.go`, `gitlab.go` | tenant `base_url` | `endpoint("base_url", …)` | `egress.client(...)` | already guarded (#750) |

### Calls whose host is fixed or operator-configured — exempt, with reason

The allow-list in `backend/pkg/netguard/bypass_test.go` holds 8 files: the guard
itself (`pkg/netguard/netguard.go`) and the 7 files named in the first 7 rows
below (cti, billing, hibp, osv, TheHive, oauth2 `fetchJSON`, dockerapi). The
other rows are SDK-internal clients and non-HTTP egress the AST test cannot see.

| Call site | Host | Why exempt |
| --- | --- | --- |
| `pkg/cti/client.go`, `pkg/cti/nvd_cpe.go` | `services.nvd.nist.gov`, `www.cisa.gov` | constants in code |
| `pkg/billing/billing.go` / `gateways.go` | `api.stripe.com`, `api.notchpay.co`, `api-checkout.cinetpay.com` | constants; keys from env |
| `pkg/hibp/hibp.go` | `api.pwnedpasswords.com` | constant; `WithEndpoint` is only called by tests |
| `internal/infrastructure/osv/client.go` | `OSV_API_URL` env, default `api.osv.dev` | operator setting; refuses cross-host redirects |
| `internal/infrastructure/integrations/thehive/client.go` | TheHive URL from server config | operator setting, typically on-prem |
| `internal/handler/oauth2_handler.go` `fetchJSON` | Google, GitHub, Microsoft Graph user-info | constants |
| `internal/handler/oauth2_handler.go` `config.Exchange` (x/oauth2, default client) | Google / GitHub / `login.microsoftonline.com/<OAUTH2_AZURE_TENANT_ID>` | provider endpoints in code; Azure tenant from env |
| `internal/scanner/collectors/dockerapi.go` unix socket | host Docker daemon | refused unless the operator sets `SCANNER_DOCKER_SOCKET_ENABLED=true` |
| `internal/scanner/collectors/aws.go` (aws-sdk-go-v2) | AWS regional endpoints | SDK-resolved from region; credentials only from the tenant |
| `internal/scanner/collectors/azure.go`, `m365.go` token (azidentity) | `login.microsoftonline.com`, `management.azure.com` | SDK constants; the tenant's `tenant_id` is a path segment, not a host |
| `internal/scanner/collectors/gcp.go` Compute API calls | `compute.googleapis.com` | endpoint from client options, not the key file (the key's token exchange is row 9, guarded) |
| `pkg/ai` (anthropic-sdk-go) | `api.anthropic.com` | SDK default; key from env |
| `internal/infrastructure/email/smtp.go` (SMTP, not HTTP) | `SMTP_HOST` env | operator setting |
| Redis, PostgreSQL | env | operator setting |
| SAML | — | no metadata fetch exists: SAML refuses every request since #866 |
| `agent/` (separate Go module) | the OpenRisk server it is enrolled with | runs on customer hosts, not in the API pod |

Inventory method (reproducible):

```
cd backend
grep -rnE 'http\.Client\{|http\.DefaultClient|http\.(Get|Post|Head|PostForm)\(|http\.DefaultTransport|http\.Transport\{' --include=*.go . | grep -v _test.go
grep -rlnE 'http\.NewRequest' --include=*.go . | grep -v _test.go
grep -rnE '\.Do\(req|\.Do\(httpReq' --include=*.go . | grep -v _test.go
grep -rnE 'ldap\.Dial|smtp\.|tls\.Dial|net\.Dial' --include=*.go . | grep -v _test.go
```

followed by reading each hit and tracing its URL to its source.

---

## The gap the inventory found in GCP

The GCP collector passed the tenant's key file to the SDK with
`option.WithAuthCredentialsJSON`. For a service account the SDK POSTs a signed
JWT assertion to the key's `token_uri` with its default client. Running the new
test against the **previous** `gcp.go` (`git show HEAD:…/gcp.go`, test
unchanged) shows the API process connecting to a loopback server named by the
key — the TLS handshake happened; only the test certificate's name mismatch
stopped the request:

```
$ go test ./internal/scanner/collectors/ -count=1 -run GCPCollect     # gcp.go from HEAD
    Error:  "gcp: aggregated list: Get \"https://compute.googleapis.com/compute/v1/projects/p/aggregated/instances\":
             auth: cannot fetch token: Post \"https://localhost:45927/token\": tls: failed to verify certificate:
             x509: certificate is valid for example.com, *.example.com, not localhost"
             does not contain "outbound target not allowed"
--- FAIL: TestGCPCollect_RefusesLoopbackTokenURI
```

With a `token_uri` of `http://169.254.169.254/...` or an internal `http://`
host there is no certificate to fail. With this branch the same test passes:
the token exchange goes through the guarded client and is refused at dial time.

---

## Live attempts

Command, from `backend/` with the harness at `backend/internal/zzssrfproof/main.go`:

```
$ go run ./internal/zzssrfproof
```

Output of the re-run on 2026-11-09, after review round 1, unedited. `api message`
is what `domain.MessageFromError` returns, i.e. the body the API caller sees:

```
control: unguarded GET https://127.0.0.1:35813 -> 200 (server reachable, hits=1)

== send path: pkg/notify (production default client) ==
PostWebhook https://127.0.0.1:35813                            denied=true  validation=false status=500
    err: post webhook: outbound target not allowed: address 127.0.0.1
PostWebhook https://169.254.169.254/latest/meta-data/          denied=true  validation=false status=500
    err: post webhook: outbound target not allowed: address 169.254.169.254
PostSlack http://169.254.169.254/latest/meta-data/             denied=true  validation=false status=500
    err: post webhook: outbound target not allowed: "http://169.254.169.254/latest/meta-data/" must use https
SendSMS https://10.0.0.5/send                                  denied=true  validation=false status=500
    err: send SMS to ***0000: outbound target not allowed: address 10.0.0.5
PostTeams gopher://127.0.0.1:6379/_x                           denied=true  validation=false status=500
    err: post webhook: outbound target not allowed: "gopher://127.0.0.1:6379/_x" must use https

== dial-time check (DNS path / rebinding): netguard.Client ==
guarded GET https://localhost:35813/ (name -> 127.0.0.1)       denied=true  validation=false status=500
    err: Get "https://localhost:35813/": dial tcp 127.0.0.1:35813: outbound target not allowed: address 127.0.0.1

== save path: automation ChannelService.Save ==
Save webhook_url=https://169.254.169.254/latest/meta-data/iam/ denied=true  validation=true  status=400
    err: webhook_url: target address is not allowed (must be a public https host)
    api message: webhook_url: target address is not allowed (must be a public https host)
Save webhook_url=https://127.0.0.1:35813/hook                  denied=true  validation=true  status=400
    err: webhook_url: target address is not allowed (must be a public https host)
    api message: webhook_url: target address is not allowed (must be a public https host)
Save webhook_url=http://hooks.slack.com/services/T/B/x         denied=true  validation=true  status=400
    err: webhook_url: target address is not allowed (must be a public https host)
    api message: webhook_url: target address is not allowed (must be a public https host)
rows stored: 0

== test-send path: automation ChannelTester on a row stored before the guard ==
Test webhook (stored https://169.254.169.254/latest/meta-data/iam/) result=false denied=true  validation=true  status=400
    err: webhook_url: target address is not allowed (must be a public https host)
    api message: webhook_url: target address is not allowed (must be a public https host)
Test webhook (stored https://127.0.0.1:35813/hook) result=false denied=true  validation=true  status=400
    err: webhook_url: target address is not allowed (must be a public https host)
    api message: webhook_url: target address is not allowed (must be a public https host)

== redirect: allowed-by-policy server redirecting inward ==
policy: Allowed=[192.168.0.224/32]  server=https://192.168.0.224:44945
GET https://192.168.0.224:44945/ok                             status=200
GET https://192.168.0.224:44945/to-loopback                    denied=true  validation=false status=500
    err: Get "https://127.0.0.1:35813/": outbound target not allowed: redirect to another host "127.0.0.1:35813"
GET https://192.168.0.224:44945/to-metadata                    denied=true  validation=false status=500
    err: Get "https://169.254.169.254/latest/meta-data/": outbound target not allowed: redirect to another host "169.254.169.254"
GET https://192.168.0.224:44945/to-http                        denied=true  validation=false status=500
    err: Get "http://192.168.0.224:44945/ok": outbound target not allowed: "http://192.168.0.224:44945/ok" must use https

loopback server hits during all guarded attempts: 0
```

Reading it:

- **127.0.0.1** — the control line proves a real server was listening and
  reachable. Every guarded attempt against it — direct (`PostWebhook`), by name
  (`localhost`, refused at **dial** time on the resolved `127.0.0.1`), saved as a
  channel, tested from a stored row, or reached by redirect — is refused, and
  the server's hit counter stays at **0**.
- **169.254.169.254** — refused on the send path, the save path, the test-send
  path and as a redirect target.
- **Redirect** — `192.168.0.224` is opened with `Policy{Allowed}`, and `/ok`
  answers 200 (the allowed case). Its redirects to `127.0.0.1`, to
  `169.254.169.254`, and its same-host downgrade to `http://` are all refused.
- **Typed error** — at the application boundary (`Save`, `Test`) the refusal is
  `ErrValidation`, status 400, and still `netguard.IsDenied`. Inside `pkg/`
  (notify, netguard) it is `netguard.ErrDenied` only — `pkg/` cannot import the
  domain — which is why `validation=false status=500` is printed for those rows.
  No handler returns a `pkg/` error unmapped on these paths.
- **Nothing stored** — `rows stored: 0` after three refused saves.
- **No URL or address in the API message** — the `api message` lines carry the
  field name and a fixed reason only. The internal `err` keeps the cause for
  `errors.Is`; it is never sent to the caller.

---

## Bypass detection

`backend/pkg/netguard/bypass_test.go` parses every non-test `.go` file of the
backend module with `go/parser` and fails on an `http.Client{…}` /
`http.Transport{…}` literal, `new(http.Client)`, a zero-value `http.Client`
variable, `http.DefaultClient`, `http.DefaultTransport`, or
`http.Get/Post/Head/PostForm`, under any import alias, outside an allow-list of
files that each carry the reason their host is not tenant-supplied. An
allow-list entry pins the exact number of unguarded uses in its file: one more
(a new client added to an already-listed file) or one fewer (a stale entry)
fails the test.

Re-run on 2026-11-09 with the count-pinned allow-list. First a new file with two
bypasses, then a second client added to an already allow-listed file, then both
probes removed:

```
$ cat internal/application/automation/zz_bypass_probe.go
package automation
import "net/http"
func probeBypass(u string) (*http.Response, error) { return http.Get(u) }
var probeClient = &http.Client{}

$ go test ./pkg/netguard/ -count=1 -run TestNoUnguardedOutboundHTTP   # new file
--- FAIL: TestNoUnguardedOutboundHTTP (0.09s)
    bypass_test.go:202: unguarded outbound HTTP at internal/application/automation/zz_bypass_probe.go:3: http.Get(...) on the default client — use netguard.Client/Dialer, or allow-list the file with the reason its host is not tenant-supplied
    bypass_test.go:202: unguarded outbound HTTP at internal/application/automation/zz_bypass_probe.go:4: http.Client{...} literal — use netguard.Client/Dialer, or allow-list the file with the reason its host is not tenant-supplied
FAIL
FAIL	github.com/opendefender/openrisk/pkg/netguard	0.097s
FAIL

$ rm internal/application/automation/zz_bypass_probe.go
$ printf '\nvar probeExtra = &http.Client{}\n' >> pkg/hibp/hibp.go
$ go test ./pkg/netguard/ -count=1 -run TestNoUnguardedOutboundHTTP   # second client in an allow-listed file
--- FAIL: TestNoUnguardedOutboundHTTP (0.18s)
    bypass_test.go:202: unguarded outbound HTTP at pkg/hibp/hibp.go: 2 unguarded use(s), allow-list expects 1:
        	55: http.Client{...} literal
        	141: http.Client{...} literal — use netguard.Client/Dialer, or allow-list the file with the reason its host is not tenant-supplied
FAIL
FAIL	github.com/opendefender/openrisk/pkg/netguard	0.181s
FAIL

$ cp <backup>/hibp.go pkg/hibp/hibp.go   # restore the original file
$ go test ./pkg/netguard/ -count=1 -run TestNoUnguardedOutboundHTTP   # both probes removed
ok  	github.com/opendefender/openrisk/pkg/netguard	0.101s
```

---

## Tests added

| Test | Proves |
| --- | --- |
| `pkg/netguard/bypass_test.go` `TestNoUnguardedOutboundHTTP`, `TestHTTPBypasses_DetectsEveryShape` | no unguarded client outside the allow-list; the detector sees all ten shapes, aliases included |
| `pkg/netguard/redirect_test.go` `TestClient_RedirectFromAllowedServer` | allowed-by-policy host answers; its redirects to loopback, metadata, `10.0.0.5`, `http://` and `file://` are refused; the loopback server is never hit |
| `pkg/notify/ssrf_test.go` `TestSenders_RefuseInternalTargets` | Slack, Teams, webhook and SMS refuse 127.0.0.1, localhost, 169.254.169.254, 10.x, ::1, IPv4-mapped, encoded IP, `file://`, `gopher://`, `http://`; server never hit |
| `pkg/notify/ssrf_test.go` `TestSenders_InjectedDoerStillValidated` | an injected doer cannot bypass the URL check: a refused URL never reaches it |
| `internal/application/automation/channels_ssrf_test.go` | `Save` refuses every internal target with `ErrValidation` + `IsDenied` and stores nothing; `Save_Success`, `Save_Unauthorized`; `ChannelTester` refuses a stored internal URL with `ErrValidation`; `ChannelTester_Unauthorized` (no tenant: 401, repository never read); `ChannelTester_DeliveryErrorDoesNotEchoURL`; `NotFound`, `Success` |
| `internal/scanner/cloud_endpoint_test.go` `TestCloudScannerValidate_GCPTokenURI` | a key whose `token_uri` is internal or http is refused at save with `ErrValidation` + `IsDenied` |
| `internal/scanner/collectors/gcp_test.go` `TestGCPCollect_RefusesLoopbackTokenURI` | the token exchange goes through the guarded dialer (DNS path) |
| `internal/infrastructure/integrations/{notifications,webhooks}/*_test.go` | the two providers refuse internal targets at validate and send time |
| `internal/domain/errors_test.go` `TestNewOutboundDeniedError_MatchesValidationAndCause` | 400, `ErrValidation`, cause preserved, fixed message without URL or address |

Pre-existing guard tests (`pkg/netguard/netguard_test.go`) already cover
`::1`, `fd00:ec2::254`, NAT64/6to4/Teredo, zone-hidden IPv6, the
`OUTBOUND_ALLOWED_CIDRS` allow-list (and that it cannot open loopback or
link-local), and public IP literals such as `https://8.8.8.8:8834` being allowed.

---

## Known Limitations

- **SDK-internal clients are outside the AST test.** Third-party SDKs that build
  their own client (aws-sdk-go-v2, azidentity, x/oauth2 `Exchange`, the
  anthropic SDK, the Google Compute client) are not `net/http` literals in our
  source, so `bypass_test.go` cannot see them. Each is listed in the exempt
  table above from **reading** its configuration, not from a test. A new SDK
  given a tenant URL needs a guarded client or dialer by review, not by test.
- **Non-HTTP egress and raw dials** (`net.Dial`, `tls.Dial`, `&net.Dialer{`,
  `ldap.DialURL`) are inventoried but not covered by the bypass test, which
  looks only at `net/http`. LDAP already uses the guarded dialer (#750); SMTP is
  operator-configured. Extending the test to them is a follow-up.
- **The allow-list counts uses per file, not per call site.** Replacing one
  allow-listed client with a different unguarded one in the same file keeps the
  count and passes. Review is the control there.
- **Logged URLs.** A transport error from Go's `net/http` (`*url.Error`) quotes
  the full request URL, and `automation/notifier.go` logs it at warn level.
  For a Slack or Teams webhook the URL path is the secret. This predates #1315
  and is not made worse by it (a refusal logged there quotes the same URL); it
  needs its own issue. The API response no longer carries it (see above).
- **No full-stack run.** The routes (`PUT /automation/channels`,
  `POST /automation/channels/test`, scanner config save) were not exercised
  over HTTP. They call `ChannelService.Save`, `ChannelTester.Test` and
  `cloudScanner.Validate` and pass their errors to `writeAppError`, which maps
  `*AppError.Code` — established by reading `internal/handler/automation_handler.go`,
  `automation_ops_handler.go` and `compliance_handler.go`.
- **No metadata server on the test host**, so the `169.254.169.254` rows prove
  refusal before any connection, not a blocked response.

---

## Harness source (deleted after the run)

```go
// W8-01c live-proof harness (#1315). Drives the production code paths against
// real listening servers. Not committed.
package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"sync/atomic"

	"github.com/google/uuid"

	appauto "github.com/opendefender/openrisk/internal/application/automation"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/pkg/netguard"
	"github.com/opendefender/openrisk/pkg/notify"
)

type repo struct{ saved int }

func (r *repo) Upsert(context.Context, *domain.AutomationChannelConfig) error { r.saved++; return nil }
func (r *repo) Get(context.Context, uuid.UUID) (*domain.AutomationChannelConfig, error) {
	return nil, nil
}

type storedRepo struct{ url string }

func (r storedRepo) Upsert(context.Context, *domain.AutomationChannelConfig) error { return nil }
func (r storedRepo) Get(_ context.Context, t uuid.UUID) (*domain.AutomationChannelConfig, error) {
	return &domain.AutomationChannelConfig{TenantID: t, WebhookEnabled: true, WebhookURL: r.url}, nil
}

func show(label string, err error) {
	fmt.Printf("%-62s denied=%-5v validation=%-5v status=%d\n    err: %v\n",
		label, netguard.IsDenied(err), errors.Is(err, domain.ErrValidation), domain.HTTPStatusFromError(err), err)
	if errors.Is(err, domain.ErrValidation) {
		fmt.Printf("    api message: %s\n", domain.MessageFromError(err))
	}
}

func main() {
	ctx := context.Background()
	msg := notify.ChatMessage{Title: "proof", Text: "proof"}

	var internalHits atomic.Int32
	internal := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { internalHits.Add(1) }))
	defer internal.Close()
	iu, _ := url.Parse(internal.URL)

	// Control: the loopback server is really up and answering.
	pool := x509.NewCertPool()
	pool.AddCert(internal.Certificate())
	resp, err := (&http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool}}}).Get(internal.URL)
	if err == nil {
		resp.Body.Close()
		fmt.Printf("control: unguarded GET %s -> %d (server reachable, hits=%d)\n\n", internal.URL, resp.StatusCode, internalHits.Load())
	}
	internalHits.Store(0)

	fmt.Println("== send path: pkg/notify (production default client) ==")
	show("PostWebhook "+internal.URL, notify.PostWebhook(ctx, internal.URL+"/hook", "s", msg, nil))
	show("PostWebhook https://169.254.169.254/latest/meta-data/", notify.PostWebhook(ctx, "https://169.254.169.254/latest/meta-data/", "s", msg, nil))
	show("PostSlack http://169.254.169.254/latest/meta-data/", notify.PostSlack(ctx, "http://169.254.169.254/latest/meta-data/", msg, nil))
	_, err = notify.SendSMS(ctx, notify.SMSConfig{GatewayURL: "https://10.0.0.5/send"}, []string{"+237600000000"}, "x", nil)
	show("SendSMS https://10.0.0.5/send", err)
	show("PostTeams gopher://127.0.0.1:6379/_x", notify.PostTeams(ctx, "gopher://127.0.0.1:6379/_x", msg, nil))

	fmt.Println("\n== dial-time check (DNS path / rebinding): netguard.Client ==")
	_, err = netguard.Client(netguard.Options{}).Get("https://localhost:" + iu.Port() + "/")
	show("guarded GET https://localhost:"+iu.Port()+"/ (name -> 127.0.0.1)", err)

	fmt.Println("\n== save path: automation ChannelService.Save ==")
	r := &repo{}
	svc := appauto.NewChannelService(r)
	for _, u := range []string{"https://169.254.169.254/latest/meta-data/iam/", internal.URL + "/hook", "http://hooks.slack.com/services/T/B/x"} {
		_, err := svc.Save(ctx, uuid.New(), appauto.ChannelInput{WebhookEnabled: true, WebhookURL: u})
		show("Save webhook_url="+u, err)
	}
	fmt.Printf("rows stored: %d\n", r.saved)

	fmt.Println("\n== test-send path: automation ChannelTester on a row stored before the guard ==")
	for _, u := range []string{"https://169.254.169.254/latest/meta-data/iam/", internal.URL + "/hook"} {
		res, err := appauto.NewChannelTester(storedRepo{u}).Test(ctx, uuid.New(), uuid.New(), domain.ChannelWebhook)
		show(fmt.Sprintf("Test webhook (stored %s) result=%v", u, res != nil), err)
	}

	fmt.Println("\n== redirect: allowed-by-policy server redirecting inward ==")
	var lanIP netip.Addr
	addrs, _ := net.InterfaceAddrs()
	for _, a := range addrs {
		if n, ok := a.(*net.IPNet); ok {
			if ip, ok := netip.AddrFromSlice(n.IP); ok && ip.Unmap().Is4() && !ip.IsLoopback() && ip.Unmap().IsPrivate() {
				lanIP = ip.Unmap()
				break
			}
		}
	}
	ln, err := net.Listen("tcp", net.JoinHostPort(lanIP.String(), "0"))
	if err != nil {
		panic(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/ok", func(http.ResponseWriter, *http.Request) {})
	mux.HandleFunc("/to-loopback", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, internal.URL+"/", http.StatusFound) })
	mux.HandleFunc("/to-metadata", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://169.254.169.254/latest/meta-data/", http.StatusFound)
	})
	mux.HandleFunc("/to-http", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://"+ln.Addr().String()+"/ok", http.StatusFound)
	})
	srv := httptest.NewUnstartedServer(mux)
	srv.Listener.Close()
	srv.Listener = ln
	srv.StartTLS()
	defer srv.Close()

	policy := netguard.Policy{Allowed: []netip.Prefix{netip.PrefixFrom(lanIP, 32)}}
	c := policy.Client(netguard.Options{})
	spool := x509.NewCertPool()
	spool.AddCert(srv.Certificate())
	c.Transport.(*http.Transport).TLSClientConfig = &tls.Config{RootCAs: spool, ServerName: "example.com"}
	fmt.Printf("policy: Allowed=[%s/32]  server=%s\n", lanIP, srv.URL)
	for _, p := range []string{"/ok", "/to-loopback", "/to-metadata", "/to-http"} {
		resp, err := c.Get(srv.URL + p)
		if err == nil {
			resp.Body.Close()
			fmt.Printf("%-62s status=%d\n", "GET "+srv.URL+p, resp.StatusCode)
			continue
		}
		show("GET "+srv.URL+p, err)
	}
	fmt.Printf("\nloopback server hits during all guarded attempts: %d\n", internalHits.Load())
}
```
