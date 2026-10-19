---
name: notification-email-posture
description: #757 notice dispatcher review 2026-10-15 — tenant gate via GetMember, Subject header injection NOT exploitable (Q-encoding), LogService logs addresses, unthrottled notice e-mail
metadata:
  type: project
---

Review of feat/757-notification-system (2026-10-15): SECURITY PASS, 0 blocking.

- Tenant gate for every work notice is `notification.Dispatcher.Deliver` -> `noticeDirectory.NoticeRecipient` (cmd/server/notice_wiring.go) -> membership `GetMember(org,user)` + member.IsActive + user.IsActive. Fails closed with no directory. Any new notice producer must go through the Dispatcher, never call NotifyInApp/SendEmail directly.
- Subject header injection: `email.Transport.message` uses `mime.QEncoding.Encode`; Go's needsEncoding treats any byte < 0x20 (CR/LF) as needing encoding, so CRLF becomes `=0D=0A` inside an encoded-word. Verified empirically. NOT exploitable. The `To:` header is raw but net/smtp Rcpt rejects CR/LF first.
- Mail body: authmail `Mailer.render` html.EscapeString on every field; CTA built from UUID routes only.
- LOW debts filed as suggestions: LogService (EMAIL_TRANSPORT=log) prints `to=<address>` + subject for every notice; no per-recipient throttle on assignment e-mails; inline SMTP in the deadline sweep (500 rows x 15s timeout); pre-existing GORM SQL trace logging addresses in invitation flows deserves its own issue.

**Why:** e-mail fan-out is a new egress of tenant data; the dispatcher is the single choke point.
**How to apply:** on future notification PRs, grep for direct `SendEmail(`/`NotifyInApp(` calls bypassing the Dispatcher. See [[recurring-defect-areas]].
