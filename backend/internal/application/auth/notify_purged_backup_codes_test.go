// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
)

type fakePurgeLedger struct {
	pending []domain.MFABackupCodePurge
	claims  int
}

// ClaimPending hands each row out once, like the UPDATE … RETURNING it stands for.
func (l *fakePurgeLedger) ClaimPending(_ context.Context, limit int) ([]domain.MFABackupCodePurge, error) {
	l.claims++
	n := min(limit, len(l.pending))
	out := l.pending[:n]
	l.pending = l.pending[n:]
	return out, nil
}

type fakeUsersByID map[uuid.UUID]*domain.User

func (f fakeUsersByID) GetByID(_ context.Context, id uuid.UUID) (*domain.User, error) {
	return f[id], nil
}

type fakePurgeMailer struct{ sent []string }

func (m *fakePurgeMailer) SendMFABackupCodesPurged(_ context.Context, to, _, locale string) error {
	m.sent = append(m.sent, to+"|"+locale)
	return nil
}

func purgeRow(user uuid.UUID) domain.MFABackupCodePurge {
	return domain.MFABackupCodePurge{ID: uuid.New(), TenantID: uuid.New(), UserID: user}
}

func TestNotifyPurgedBackupCodes_Success(t *testing.T) {
	awa, ben := uuid.New(), uuid.New()
	ledger := &fakePurgeLedger{pending: []domain.MFABackupCodePurge{purgeRow(awa), purgeRow(ben)}}
	users := fakeUsersByID{
		awa: {ID: awa, Email: "awa@example.test", IsActive: true, Locale: "fr"},
		ben: {ID: ben, Email: "ben@example.test", IsActive: true, Locale: "en"},
	}
	mailer := &fakePurgeMailer{}

	sent, err := NewNotifyPurgedBackupCodesUseCase(ledger, users, mailer).Execute(context.Background())
	require.NoError(t, err)
	require.Equal(t, 2, sent)
	require.ElementsMatch(t, []string{"awa@example.test|fr", "ben@example.test|en"}, mailer.sent)

	// A second boot finds nobody left: no one is told twice.
	sent, err = NewNotifyPurgedBackupCodesUseCase(ledger, users, mailer).Execute(context.Background())
	require.NoError(t, err)
	require.Zero(t, sent)
	require.Len(t, mailer.sent, 2)
}

func TestNotifyPurgedBackupCodes_DrainsMoreThanOneBatch(t *testing.T) {
	ledger := &fakePurgeLedger{}
	users := fakeUsersByID{}
	for i := 0; i < notifyBatch+3; i++ {
		id := uuid.New()
		ledger.pending = append(ledger.pending, purgeRow(id))
		users[id] = &domain.User{ID: id, Email: id.String() + "@example.test", IsActive: true}
	}
	mailer := &fakePurgeMailer{}
	sent, err := NewNotifyPurgedBackupCodesUseCase(ledger, users, mailer).Execute(context.Background())
	require.NoError(t, err)
	require.Equal(t, notifyBatch+3, sent)
	require.Equal(t, 2, ledger.claims)
}

func TestNotifyPurgedBackupCodes_NotFound(t *testing.T) {
	ledger := &fakePurgeLedger{pending: []domain.MFABackupCodePurge{purgeRow(uuid.New())}}
	mailer := &fakePurgeMailer{}
	sent, err := NewNotifyPurgedBackupCodesUseCase(ledger, fakeUsersByID{}, mailer).Execute(context.Background())
	require.NoError(t, err, "a deleted account is not a boot failure")
	require.Zero(t, sent)
	require.Empty(t, mailer.sent)
}

func TestNotifyPurgedBackupCodes_Unauthorized(t *testing.T) {
	gone := uuid.New()
	ledger := &fakePurgeLedger{pending: []domain.MFABackupCodePurge{purgeRow(gone)}}
	users := fakeUsersByID{gone: {ID: gone, Email: "gone@example.test", IsActive: false}}
	mailer := &fakePurgeMailer{}
	sent, err := NewNotifyPurgedBackupCodesUseCase(ledger, users, mailer).Execute(context.Background())
	require.NoError(t, err)
	require.Zero(t, sent, "a deactivated account is not mailed")
	require.Empty(t, mailer.sent)
}
