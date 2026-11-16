# Uploads, input validation and output encoding audit (#1316)

Scope: every route that accepts a file, every route that serves one back, and
every user free-text field that is rendered to another user. Status of each row
is what a passing test proves, nothing more (CLAUDE.md rule 12). Tests live in
`backend/` and run with `go test ./...`.

## 1. Upload endpoints

Common rules for the evidence and import rows: the type is decided from the
bytes with `http.DetectContentType` (`backend/pkg/fileupload`), never from the
client `Content-Type` or the extension; rejections are `ErrValidation`
(400, 413 for oversize) through `writeAppError`; every query filters by
`tenant_id` (evidence repository `GetByID(ctx, tenantID, id)`, controls through
`ControlLookup.GetControlByID(.., tenantID, ..)`).

| Route | Handler | Size limit | Sniffed allowlist | Stored name | Test |
|---|---|---|---|---|---|
| `POST /evidence` (multipart, file optional) | `internal/handler/evidence_handler.go:77` -> `evidence.Service.Create` | 3 MB (`fileupload.EvidenceMaxBytes`, read through `io.LimitReader(max+1)`); below the 4 MB Fiber body limit, no figure is promised in the UI | pdf, png, jpeg, webp, text/plain (UTF-8, CSV/TXT), zip (OOXML). Refused: html, xml/svg, octet-stream/exe, anything else. text/plain must be valid UTF-8 over the whole body (UTF-16 BOMs refused) and, for evidence, must not contain (case-insensitive, anywhere) `<svg`, `:svg` (namespace-prefixed SVG), `<script`, `<html`, `<!doctype`, `<iframe`, `<object`, `<embed` or `javascript:`; CSV imports only refuse a UTF-16 BOM and binary content: encoding past the sniff window is left to the importers, which answer 422 `not_utf8` ("save it as CSV UTF-8") | `<tenant>/<uuid><ext>`, ext from the sniffed type (`pkg/storage/local.go` `Save`); client name kept only in `evidences.filename` | `TestCreate_StoresUnderAServerGeneratedName`, `TestCreate_RejectsWrongTypeBySniffing`, `TestCreate_RejectsOversize`, `TestCreate_RejectsEmptyFile`, `TestCreate_Unauthorized_WithoutTenant` (`internal/application/evidence`); `TestSave_KeyNeverCarriesTheClientFilename` (`pkg/storage`); `TestEvidenceUpload_*` (`internal/handler/upload_audit_test.go`) |
| `POST /compliance/controls/:controlId/evidences` | `evidence_handler.go:171` -> same `Service.Create` | same | same | same | `TestEvidenceUpload_Success_StoredNameIsServerGenerated`, `_WrongType_ScriptBearingFilesAreRejected` (HTML, SVG, XML-SVG, exe declared as `application/pdf`), `_Oversize_Is413`, `_Unauthorized_RoleWithoutCreatePermission` (403), `_CrossTenant_ControlOfAnotherTenantIsNotFound` (404) |
| `POST /risks/import` (CSV, not stored) | `internal/handler/risk_import_handler.go:36` | 2 MB (existing `risk.MaxImportBytes`; 413) | text/plain only (`fileupload.CSV`); the `.csv` extension check is kept as a first gate | not stored | `TestRiskImportHTTP_RefusesBinaryNamedCSV`, `TestRiskImportHTTP_Oversize_Is413`, existing `TestRiskImportHTTP_Unauthorized`, `_RowsLandOnlyInCallersTenant` |
| `POST /assets/import` (CSV, not stored) | `internal/handler/asset_import_handler.go:38` | 2 MB (`asset.MaxImportBytes`; 413) | same | not stored | `TestAssetImportHTTP_RefusesBinaryNamedCSV`, `TestAssetImportHTTP_Oversize_Is413`, existing `TestAssetImportHTTP_Unauthorized`, `_RowsLandOnlyInCallersTenant` |
| `PUT /users/me/avatar` | `internal/handler/profile_handler.go:51` | 1 MB (`pkg/imageupload`) | png, jpeg, webp (svg refused) | server key, served with nosniff (`profile_handler.go:88`) | pre-existing, `pkg/imageupload/imageupload_test.go` (not re-proven here) |
| `PUT /organization/logo` | `internal/handler/organization_member_handler.go:293` | 1 MB (`pkg/imageupload`) | same | server key, nosniff (`:326`) | pre-existing, same package tests |

Import error body is unchanged for the frontend: `{"error":"validation_failed","message":...}`;
the refusal is built from a typed domain error (`importUploadError`).

### Dead upload code (not routed, not fixed)

- `internal/handler/organization_handler.go:262` `MigrationHandler.UploadMigrationFile`: only reachable through
  `RegisterMigrationRoutes` (`:367`), which nothing calls (grep over `backend/`: definition only).
- `internal/api/http/handlers/risks.go:515` `ImportRisks`: package `internal/api/http/handlers` is imported by no file.

If either is ever wired it must go through `fileupload` first.

## 2. Downloads

| Route | Handler | Headers | Test |
|---|---|---|---|
| `GET /evidence/:evidenceId/download`, `GET /compliance/evidences/:evidenceId/download` | `evidence_handler.go:394` | `Content-Disposition` from `fileupload.AttachmentHeader` (RFC 6266: ASCII fallback with quotes, backslashes, `%`, `;`, CR/LF and control chars removed, UTF-8 name in `filename*=`), `Content-Type: application/octet-stream`, `X-Content-Type-Options: nosniff`, `Cache-Control: no-store`, `Content-Security-Policy: default-src 'none'; sandbox` | `TestEvidenceDownload_HeadersWithHostileFilename` (`a"\r\nX-Evil: 1.pdf`), `TestEvidenceDownload_UTF8FilenameIsPercentEncoded`, `TestEvidenceDownload_CrossTenant_NotFound` (compliance_handler_test.go), `TestEvidenceDownload_CrossTenant_NoBytesLeak`; `TestAttachmentHeader_*` (`pkg/fileupload`) |
| `GET /reports/:reportId/download`, `GET /reports/jobs/:jobId/download`, compliance report, board report | `report_handler.go`, `report_job_handler.go`, `compliance_handler.go`, `board_report_handler.go` | server-generated bytes; the filename (framework name, period label, stored report name) now goes through `AttachmentHeader` | `TestAttachmentHeader_*`; no per-route handler test added |
| exports with fixed names (`export_handler`, `governance_handler`, `analytics_handler`, `trend_handler`, `org_deletion_handler`) | as named | constant filenames or a date/number built server-side; unchanged | none needed (no user-derived value) |
| every API response | `middleware.SecurityHeaders` (`cmd/server/main.go:527`, global `app.Use`) | `X-Content-Type-Options: nosniff` (helmet default) | `TestSecurityHeaders_NosniffOnEveryResponse` |

## 3. Free-text fields rendered back to users

Frontend: no `dangerouslySetInnerHTML` / `innerHTML` in `frontend/src` (grep), so
React renders every value as text. API: Fiber's JSON encoder is `encoding/json`,
which writes `<`, `>` and `&` as `\u003c`, `\u003e` and `\u0026`; the bytes are
stored verbatim and encoded on output, losslessly. Responses are
`application/json`. Payloads used: `<script>alert(1)</script>` and
`"><img src=x onerror=alert(1)>`.

| Field | Where rendered | Encoding | Test |
|---|---|---|---|
| risk `title`, `description` | `GET /risks/:id`, lists | JSON-escaped | `TestOutputEncoding_RiskThroughHandler` (CSV import -> `RiskHandler.GetRisk`), `TestOutputEncoding_EveryFreeTextField` |
| asset `name`, `type`, `owner` | asset endpoints | JSON-escaped | `TestOutputEncoding_EveryFreeTextField` (entity level) |
| evidence `title`, `description`, `filename`, `source_detail`, `review_note`, `external_url` | evidence create/list/get | JSON-escaped; filename in `Content-Disposition` per section 2 | `TestOutputEncoding_EvidenceThroughHandler` (title + description through the handler), `TestOutputEncoding_EveryFreeTextField` (all six, entity level) |
| organization `name`, `industry` | organization endpoints | JSON-escaped | `TestOutputEncoding_EveryFreeTextField` (entity level) |
| user `full_name`, `bio`, `department`, `phone` | profile/member endpoints | JSON-escaped | `TestOutputEncoding_EveryFreeTextField` (entity level) |
| mitigation `title`, `description` | mitigation endpoints | JSON-escaped | `TestOutputEncoding_EveryFreeTextField` (entity level) |
| compliance framework / control `name`, `description` | compliance endpoints | JSON-escaped | `TestOutputEncoding_EveryFreeTextField` (entity level) |
| user `full_name` in the welcome email | HTML email (`pkg/notify/email_service.go`) | `html/template` auto-escape; the plain-text fallback, which is also sent as the HTML body, now `html.EscapeString`s the name | `TestSendWelcomeEmail_EscapesTheFullName` |

There is no comment entity in `internal/domain`.

## 4. Findings

- Fiber's JSON encoder does escape HTML characters: no change to global behaviour was needed or made.
- Fixed: evidence uploads had no type check at all and kept the client name in the stored key; the download header
  interpolated the raw filename (quote and CR/LF injection); CSV imports trusted the extension; the welcome-email
  fallback body interpolated the name unescaped.
- Filename `%q` in the report/board/compliance downloads was safe against header splitting but not RFC 6266 and
  broke non-ASCII names; replaced by the shared helper.

## 5. Limits / not covered

- Free-text fields outside the list above (any not named in section 3) were not enumerated; the entity-level test
  proves the encoder, not that no handler writes raw HTML or a non-JSON content type elsewhere. Only risk and
  evidence are proven through their real handler; asset, organization, user, mitigation and compliance rows are
  proven at the entity level.
- Sniffing reads the first 512 bytes (`http.DetectContentType`). A valid PDF or ZIP header followed by arbitrary
  content passes: the files are served only as `application/octet-stream` attachments with nosniff and a sandbox
  CSP, never inline. There is no malware scan and no deep OOXML/PDF structure validation. `application/zip` also
  admits any zip (jar, apk), not only Office files.
- Text evidence that legitimately mentions one of the refused markers (for example a runbook quoting `<script>`) is
  refused: a deliberate false positive. The marker list is a denylist on top of the allowlist, not a parser.
- Open MEDIUM: the downloaded file keeps the uploader's name and extension (for example `report.hta`, or a name with
  bidi-override characters), and the frontend uses the raw `filename` JSON field for `a.download`
  (`frontend/src/features/evidence/EvidenceLibraryPage.tsx:105`, `frontend/src/services/complianceService.ts:174`).
  Tracked as #1448.
- Existing stored evidence keeps its old key (`<uuid>-<client name>`); only new uploads get the server name. No
  migration was written.
- Fiber's 4 MB body limit still applies first; an oversize body is refused by Fiber before the handler (its own 413).
- No live-environment proof was produced from this change: every claim above is a unit or in-process API test
  (sqlite, local storage). The live-proof note required by the Definition of Done is not written here.
- OpenAPI spec not updated: no route or request shape changed, only new 400/413 responses on existing routes.

## Live proof (2026-11-16)

Real server, real PostgreSQL, curl only. Every status and header below was copied from the run; request bodies
are the fixtures named in each row. HEAD `502f1c5a` plus the uncommitted audit doc.

### Environment

- Go 1.26.8; `postgres:16-alpine` (PostgreSQL 16.15) as `or1316-pg` on `127.0.0.1:55432`; `redis:7-alpine`
  (7.4.12) as `or1316-redis` on `127.0.0.1:56379`; server on `:18316` (8080 was taken by another container).
- Build and boot: `cd backend && go build -o <scratch>/server ./cmd/server`, then run with
  `PORT=18316 APP_ENV=development OPENRISK_DEPLOYMENT=self_hosted DATABASE_URL=<elided> MIGRATIONS_DIR=<repo>/migrations
  DB_HOST/DB_PORT/DB_USER/DB_PASSWORD/DB_NAME=<throwaway> REDIS_URL=redis://127.0.0.1:56379/0 REDIS_HOST REDIS_PORT
  RSA_PRIVATE_KEY_PATH/RSA_PUBLIC_KEY_PATH=<throwaway 2048-bit pair> STORAGE_DRIVER=local STORAGE_LOCAL_PATH=<scratch>/uploads
  PASSWORD_HIBP_DISABLED=true EMAIL_TRANSPORT=log`. All secrets were generated for this run and discarded.
- Two tenants through the app's own `POST /api/v1/auth/register` (201 each), then `POST /api/v1/auth/login`
  (200). The session is cookie based (`or_access`, `or_csrf`); mutating calls send the `or_csrf` value in the
  `X-CSRF-Token` header. Tenant A = `5f36260a-...` (owner role), tenant B = `f9b27bbe-...`.
- Routes are under `/api/v1` (`/evidence`, `/compliance/controls/:id/evidences`, `/risks/import`, `/assets/import`,
  `/users/me/avatar`).

### Evidence upload (`POST /evidence`, multipart)

| Case | Status | Result |
|---|---|---|
| real 80-byte PDF, client name `client-secret-name.pdf` | 201 | `file_ref` = `5f36260a-.../1cb80338-....pdf`; `filename` field keeps the client name |
| storage dir after the upload (`find <scratch>/uploads -type f`) | n/a | `uploads/5f36260a-e152-4a1c-86ec-aed54439edc5/1cb80338-ed5e-4960-822c-7219e253debc.pdf`: `<tenant>/<uuid>.pdf`, no client name |
| HTML named `x.pdf`, declared `application/pdf` | 400 | `{"error":"This file type is not allowed. Attach a PDF, PNG, JPEG, WebP, text or Office (docx, xlsx, pptx) file."}` |
| SVG with `<script>`, named `x.pdf`, declared `application/pdf` | 400 | same body |
| UTF-8 BOM + `<html>`, named `x.pdf` | 400 | same body |
| 300 random bytes named `.csv` declared `text/csv` | 400 | same body |
| 3,500,000-byte text file (over 3 MB cap) | 413 | `{"error":"The file is larger than 3 MB."}` (handler's own refusal, below Fiber's limit) |
| 4,600,000-byte text file (over Fiber's 4 MB body limit) | 413 | `{"error":true,"msg":"Request Entity Too Large"}` (Fiber's own response, before the handler) |
| storage dir after all rejections | n/a | unchanged: the rejected files wrote nothing |

curl prints an interim `HTTP/1.1 100 Continue` for the two large bodies; the final codes above were taken with
`-w %{http_code}`.

### Download headers (`GET /evidence/:id/download`, also `/compliance/evidences/:id/download`)

Every download, whatever the stored name, returned `200`, `Content-Type: application/octet-stream`,
`X-Content-Type-Options: nosniff`, `Cache-Control: no-store`, `Content-Security-Policy: default-src 'none'; sandbox`.
The body of the stored PDF was byte-identical to the upload (`cmp`).

| Uploaded filename | `Content-Disposition` |
|---|---|
| `client-secret-name.pdf` | `attachment; filename="client-secret-name.pdf"` |
| `résumé-é.pdf` | `attachment; filename="r_sum_-_.pdf"; filename*=UTF-8''r%C3%A9sum%C3%A9-%C3%A9.pdf` |
| `a\"x; X-Evil: 1.pdf` (raw multipart, quote and semicolon) | `attachment; filename="ax X-Evil: 1.pdf"; filename*=UTF-8''a%22x%3B%20X-Evil%3A%201.pdf` |
| `a"\r\nX-Evil: 1.pdf` sent through curl `-F` | curl percent-encoded CR/LF to `%0D%0A` in the multipart header, so the server stored the literal text `a%22%0D%0AX-Evil: 1.pdf`; header was `filename="a220D0AX-Evil: 1.pdf"; filename*=UTF-8''a%2522%250D%250AX-Evil%3A%201.pdf` |

No response in any of these carried an `X-Evil` header (the header filter printed none).

### Cross-tenant (tenant B against tenant A's objects)

| Request by B | Status | Body |
|---|---|---|
| `GET /evidence/<A id>/download` | 404 | `{"error":"evidence not found"}` |
| `GET /evidence/<A id>` | 404 | `{"error":"evidence not found"}` |
| `GET /compliance/evidences/<A id>/download` | 404 | `{"error":"evidence not found"}` |
| `POST /compliance/controls/<A control>/evidences` with a valid PDF | 404 | `{"error":"control not found"}` |
| `GET /compliance/controls/<A control>/evidences` | 404 | `{"error":"control not found"}` |
| `GET /evidence` (B's own list) | 200 | `{"items":[],"total":0,...}`: none of A's rows |
| `GET /users/<A id>/avatar` | 404 | `{"error":"avatar not found"}` |

The same upload by A to its own control returned 201. After B's attempts the storage dir held only A's tenant
directory (no directory for B).

### CSV imports

| Request | Status | Body |
|---|---|---|
| `POST /risks/import`, 300 random bytes named `.csv` | 400 | `{"error":"validation_failed","message":"Only CSV (UTF-8 text) files can be imported."}` |
| `POST /risks/import`, valid CSV | 200 | `{"created":1,"rejected":0,"risk_ids":[...],"errors":[]}` |
| `POST /risks/import`, 2.2 MB file | 413 | not body-captured (`-w %{http_code}` only) |
| `POST /assets/import`, binary named `.csv` | 400 | same `validation_failed` body |
| `POST /assets/import`, valid CSV | 200 | `{"created":1,"rejected":0,"asset_ids":[...],"errors":[]}` |

### Output encoding

`POST /risks` with `title` = `<script>alert(1)</script>` and `description` = `"><img src=x onerror=alert(1)>`
returned 201; `GET /risks/<id>` returned `200`, `Content-Type: application/json`. Raw response bytes contain
`"title":"\u003cscript\u003ealert(1)\u003c/script\u003e"`; `grep -c '<script'` on the raw response = 0 and
`grep -o '\\u003cscript'` = 2 occurrences. So the literal `<script` does NOT appear on the wire: Fiber's
encoder escapes it as `\u003cscript`, as section 3 states (a JSON parser decodes it back losslessly). The first
create attempt with `probability` 3 was refused 400 (`Probability is too large (maximum 1)`), a payload error,
not an encoding result.

### Avatar (`PUT /users/me/avatar`, field `file`)

| Request | Status | Body |
|---|---|---|
| SVG with `<script>` declared `image/png` | 400 | `{"error":"avatar: image must be PNG, JPEG or WebP"}` |
| HTML declared `image/png` | 400 | same |
| 1x1 PNG | 200 | profile JSON with `has_avatar:true` |

`GET /users/<id>/avatar` (owner): `200`, `Content-Type: image/png`, `X-Content-Type-Options: nosniff`.

### Limits

- Not run: `PUT /organization/logo`, the report, board-report and compliance-report downloads, UTF-16 BOM text,
  a text file containing each refused marker, a zip/OOXML upload, a JPEG/WebP avatar. Covered only by the unit and
  in-process tests listed above.
- A literal CR/LF in a filename cannot be produced through curl `-F` (it percent-encodes them) and the
  multipart parser did not hand the server a raw CR/LF in my raw-body attempt; the CR/LF case is proven only by
  `TestEvidenceDownload_HeadersWithHostileFilename`, not live. The live run proves quote, semicolon, `%`, and
  non-ASCII handling.
- Single run on one host, one request per case: no concurrency, no repeat. Role/permission refusals (403) were not
  exercised live (both users are organisation owners).
- Free-text encoding was proven live for risk title/description only, not for the other fields in section 3.
- The frontend (`a.download` use of the raw filename, #1448) is not exercised: curl only.
- Server log checked for `panic`: 0 occurrences. No ZAP/fuzzing, no k6.
