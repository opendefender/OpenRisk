-- Security fix (audit-2026, issue #240): every MFA backup code issued before the
-- crypto/rand fix was generated from a hard-coded constant seed and was therefore
-- identical across all users and derivable from public source. Purge them so the
-- compromised universal codes can no longer satisfy an MFA challenge. Affected
-- users regenerate fresh, unique codes by re-running MFA setup.
DELETE FROM mfa_backup_codes;
