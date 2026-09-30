-- #849 — a TOTP code is accepted once.
--
-- The server checked a code against its ±30 s window and never recorded that it
-- had been used, so the same code could open a second session, confirm a second
-- sensitive action, or turn MFA off within about 90 s of being typed.
-- last_totp_step holds the time step (unix seconds / 30) of the last code
-- accepted for this secret; a code is accepted only for a later step, in one
-- conditional UPDATE (GormMFARepository.ConsumeTOTPStep).
--
-- Nullable, no default: NULL means "no code accepted yet", which is the true
-- state of every existing row. AutoMigrate adds the same column from
-- domain.MFASecret on boot; this file carries it for the migration CLI path.

ALTER TABLE mfa_secrets ADD COLUMN IF NOT EXISTS last_totp_step bigint;
