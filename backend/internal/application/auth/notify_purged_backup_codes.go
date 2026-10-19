// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth

import (
	"context"

	"github.com/opendefender/openrisk/internal/domain"
)

// PurgedBackupCodeLedger hands out the people still to be told. ClaimPending
// stamps the rows it returns as notified in the same statement, so two
// replicas booting together, or a restart mid-way, never tell anyone twice.
// A row is claimed whether or not the mail then goes out: a security notice
// sent twice is worse than one lost to a dead SMTP server, and the codes were
// unusable either way.
type PurgedBackupCodeLedger interface {
	ClaimPending(ctx context.Context, limit int) ([]domain.MFABackupCodePurge, error)
}

// BackupCodesPurgedMailer sends the notice. Satisfied by authmail.Async.
type BackupCodesPurgedMailer interface {
	SendMFABackupCodesPurged(ctx context.Context, to, fullName, locale string) error
}

// NotifyPurgedBackupCodesUseCase tells each person whose backup codes were
// purged that they are gone and how to get new ones. It runs once at boot,
// after the SQL layer, and does nothing when the ledger is empty.
type NotifyPurgedBackupCodesUseCase struct {
	ledger PurgedBackupCodeLedger
	users  MFALockUserLookup
	mailer BackupCodesPurgedMailer
}

func NewNotifyPurgedBackupCodesUseCase(ledger PurgedBackupCodeLedger, users MFALockUserLookup, mailer BackupCodesPurgedMailer) *NotifyPurgedBackupCodesUseCase {
	return &NotifyPurgedBackupCodesUseCase{ledger: ledger, users: users, mailer: mailer}
}

// notifyBatch bounds one claim. Execute loops until the ledger is drained.
const notifyBatch = 100

// Execute mails everyone still pending and returns how many were sent. A
// person whose account is gone or deactivated is skipped: their row is claimed
// and nobody is mailed. The codes belong to the account, not to one
// organisation, so membership is not checked: a person in two organisations
// is told all the same.
func (uc *NotifyPurgedBackupCodesUseCase) Execute(ctx context.Context) (int, error) {
	sent := 0
	for {
		pending, err := uc.ledger.ClaimPending(ctx, notifyBatch)
		if err != nil {
			return sent, err
		}
		for _, p := range pending {
			user, err := uc.users.GetByID(ctx, p.UserID)
			if err != nil {
				return sent, err
			}
			if user == nil || !user.IsActive {
				continue
			}
			if err := uc.mailer.SendMFABackupCodesPurged(ctx, user.Email, user.FullName, normaliseLocale(user.Locale)); err != nil {
				return sent, err
			}
			sent++
		}
		if len(pending) < notifyBatch {
			return sent, nil
		}
	}
}
