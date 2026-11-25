# Key rotation and production secrets (#1317)

In production (`APP_ENV` unset, or anything other than `development`, `dev`,
`test`, `testing`, `local`) the backend checks every secret at boot, before any
of them is used (`backend/cmd/server/production_secrets.go`). It refuses to
start and lists **every** unusable variable in one message, never a value.

A secret is refused when it is empty (where required), when it is a public
development default or an example placeholder (`change-me`, `changeme`,
`secret`, `test`, `password`, `your-api-key`, anything containing
`change-me`/`changeme`, the built-in development keys), or when it is too weak
(`AUDIT_HASH_KEY` under 32 characters, a JWT key under 2048 bits).

## Every key, what it protects, how to rotate

| Variable | Protects | Required | Rotation |
|---|---|---|---|
| `RSA_PRIVATE_KEY` / `RSA_PRIVATE_KEY_PATH` (+ public) | Signs the RS256 access tokens | Yes (the entrypoint generates a per-volume pair if none is given) | Replace the pair on **every replica at once**. Every issued access token stops verifying, so sessions are re-issued: users sign in again. No data is encrypted with it. |
| `MFA_ENCRYPTION_KEY` | TOTP secrets at rest (AES-256-GCM) | Yes | Online, see below. Never just replace it: a new key alone cannot read existing enrolments. |
| `MFA_ENCRYPTION_PREVIOUS_KEY` | The key being rotated away from | Only while rotating | Set for one boot, then remove. |
| `AUDIT_HASH_KEY` | Keyed hash of addresses in the sign-in audit trail and the per-address limits | Yes (32+ characters) | No previous-key support: changing it means audit entries from before the change can no longer be matched with those after it, and per-address counters restart. Do it deliberately, on every replica together. |
| `DEVICE_COOKIE_KEY` | Signs the trusted-device cookie | No (derived from `AUDIT_HASH_KEY` when empty) | Move the old value to `DEVICE_COOKIE_PREVIOUS_KEY`, set the new one, keep both for 90 days (the cookie lifetime), then drop the previous. |
| `AUDIT_EXPORT_KEY` | Signs audit exports | No (exports are unsigned without it) | Replace it; exports made earlier still verify only with the old key, so keep that key with the exports it signed. |
| `SCANNER_CREDENTIAL_KEY` | Scanner and integration credentials at rest | Handled by #1200 / PR #1510 | Not covered here. Production refusal and rekey from the development key ship with #1510; a previous-key rotation is a follow-up once it merges (`credrekey.Rekey` already takes the `from` key). |

## Rotating `MFA_ENCRYPTION_KEY` (online, no one enrols again)

1. Generate the new key: `openssl rand -hex 32`.
2. Set `MFA_ENCRYPTION_PREVIOUS_KEY` to the **current** value and
   `MFA_ENCRYPTION_KEY` to the new one, on **every** replica.
   - Docker Compose self-hosting: both lines in `deploy/selfhost/.env`.
   - Helm: `secrets.inline.mfaEncryptionKey` (new) and
     `secrets.inline.mfaEncryptionPreviousKey` (old).
3. Restart. At boot the backend re-encrypts every two-factor secret that only
   the previous key can read (`authmfa.RekeySecrets`, idempotent, safe while
   replicas boot together) and logs counts only:
   `MFA: key rotation: N two-factor secret(s) re-encrypted ... M readable with neither key`.
4. `M` should be 0. A non-zero `M` means some rows were written under a third
   key: keep the previous key set and find it before removing anything.
5. Remove `MFA_ENCRYPTION_PREVIOUS_KEY` once `N` is 0 on a second boot.

Proof: `TestRekeySecrets_DecryptsDataWrittenWithThePreviousKey` in
`backend/internal/infrastructure/authmfa/rekey_test.go` writes a secret under
the previous key and reads it with the new one after the pass.

## Replicas and the key check

Replicas compare fingerprints of `AUDIT_HASH_KEY` and `MFA_ENCRYPTION_KEY`
through Redis (#1063) and log a mismatch. During a rolling rotation a brief
mismatch is expected.
