---
name: audit-hash-key-posture
description: #879 crit. 2 review 2026-10-21 — AUDIT_HASH_KEY HMAC for address hashes (D-065); PASS, residual plain-SHA-256 rows and Helm plaintext env
metadata:
  type: project
---

Reviewed #879 criterion 2 (branch security/879-audit-hash-key, stacked on 686) on 2026-10-21. Verdict PASS, no blocking finding.

- domain.HashEmailForReset = HMAC-SHA256 under SetAddressHashKey; dev key literal in source when unset. main.go sets it right after config.LoadConfig, before database.Connect; no init()/goroutine hashes earlier. Production (IsProductionEnv: APP_ENV not dev/test => prod) refuses empty or <32-char key (TrimSpace, byte len).
- Residuals raised (MEDIUM/LOW, non-blocking): password_reset_tokens.email_hash rows are never purged, so pre-switch plain SHA-256 of every requested address (unknown ones too) persists — DECISIONS "Applied" only mentions audit rows; Helm chart passes the key as a literal pod env value (no secretKeyRef, no `required`), same pre-existing pattern as MFA_ENCRYPTION_KEY; docker-compose.staging.yaml has no APP_ENV so it now fails to boot; root .env.example omits the key; installer `.+` regex treats `export AUDIT_HASH_KEY=` as absent and appends a second line.
- Note: IsProductionEnv (fail-closed) vs `APP_ENV == "production"` checks in database.go/main.go:477 are inconsistent — APP_ENV=staging is "prod" for one and not the other.

**Why:** the Helm chart repeatedly lacks a Secret path for new backend keys; each new key repeats the plaintext-env pattern.
**How to apply:** for any new server key, check helm secrets.yaml + deployment secretKeyRef, every compose file's APP_ENV, and leftover data hashed under the old scheme. See [[recurring-defect-areas]].
