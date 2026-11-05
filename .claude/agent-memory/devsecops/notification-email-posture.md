---
name: notification-email-posture
description: #757 dispatcher 2026-10-15 + #997 throttle and outbox rework 2026-10-21 (PASS, MEDIUM unbounded digest/poison row) — tenant gate via GetMember, Subject header injection NOT exploitable (Q-encoding), LogService logs addresses, unthrottled notice e-mail
metadata:
  type: project
---

Review of feat/757-notification-system (2026-10-15): SECURITY PASS, 0 blocking.

- Tenant gate for every work notice is `notification.Dispatcher.Deliver` -> `noticeDirectory.NoticeRecipient` (cmd/server/notice_wiring.go) -> membership `GetMember(org,user)` + member.IsActive + user.IsActive. Fails closed with no directory. Any new notice producer must go through the Dispatcher, never call NotifyInApp/SendEmail directly.
- Subject header injection: `email.Transport.message` uses `mime.QEncoding.Encode`; Go's needsEncoding treats any byte < 0x20 (CR/LF) as needing encoding, so CRLF becomes `=0D=0A` inside an encoded-word. Verified empirically. NOT exploitable. The `To:` header is raw but net/smtp Rcpt rejects CR/LF first.
- Mail body: authmail `Mailer.render` html.EscapeString on every field; CTA built from UUID routes only.
- LOW debts filed as suggestions: LogService (EMAIL_TRANSPORT=log) prints `to=<address>` + subject for every notice; no per-recipient throttle on assignment e-mails; inline SMTP in the deadline sweep (500 rows x 15s timeout); pre-existing GORM SQL trace logging addresses in invitation flows deserves its own issue.

- #997 review 2026-10-21 (security/997-notice-email-throttle): PASS, 0 blocking. Throttle table notice_email_throttle keyed tenant_id-first PK, both statements parameterised and tenant-scoped. Pool acquire/Add under mutex vs Drain = correct. Fail-open on throttle-store error accepted (store == main DB). LOW debts: claim not released on SendNotice panic (release not deferred); sweep budget can expire after MarkReminderSent so that row's e-mail is cut; no pruning of throttle rows (no FK, rows outlive deleted users/tenants).

- #997 r2 2026-10-21 (outbox rework, commits 3fa1ea42..05ae3e26): PASS, 0 blocking. Throttle/pool replaced by notice_email_outbox (+throttle row lock, SKIP LOCKED claim, 2-min lease, 12 attempts) and per-tenant notice_email_policies (15/30/60/240/1440 min). ClaimDue/Prune cross-tenant accepted (system jobs, every follow-up write re-scoped by row.tenant_id). Digest current-state read is tenant-filtered, names only active members; no per-item visibility exists beyond tenant + risks:read, which is NOT re-checked at send (LOW, custom roles). RequireRole iterates all org_roles but session tokens carry only the current org (session_wiring.go) so OK. MEDIUM debts: digest events jsonb unbounded per window (events || append, no cap -> O(n^2) rewrites, oversize mail -> 12 retries -> failed = dropped); poison row (panic/OOM) re-leased forever, attempts cap only checked in retry(), 'sending' never pruned. LOW: ClaimDue restamps throttle after its claim commits (race -> single + digest seconds apart), enqueue failure only logged, no per-tenant fairness in global due_at FIFO, recipient/state reads have no timeout vs 2-min lease.

**Why:** e-mail fan-out is a new egress of tenant data; the dispatcher is the single choke point.
**How to apply:** on future notification PRs, grep for direct `SendEmail(`/`NotifyInApp(` calls bypassing the Dispatcher. See [[recurring-defect-areas]].
