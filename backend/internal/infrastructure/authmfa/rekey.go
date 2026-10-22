// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package authmfa

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/pkg/crypto"
)

// RekeyResult reports one pass of RekeySecrets.
type RekeyResult struct {
	Rekeyed    int // moved from the old key to the new one
	Unreadable int // readable with neither key; left as they were
}

// RekeySecrets re-encrypts, under `to`, every TOTP secret that only `from`
// can decrypt (#1062).
//
// An install that ran without MFA_ENCRYPTION_KEY encrypted its secrets with
// the public development key. Production now requires a real key; this moves
// those secrets onto it at boot, so nobody has to enrol again.
//
// Cross-tenant by design: a boot maintenance pass over every organisation's
// rows, like the #1001 purge, never reachable from a request. Soft-deleted
// rows are included: they are still decryptable data at rest.
//
// Each row is updated only if it still holds the ciphertext that was read, so
// replicas booting together cannot overwrite each other, and a second pass
// finds nothing left to do.
func RekeySecrets(ctx context.Context, db *gorm.DB, from, to []byte) (RekeyResult, error) {
	var res RekeyResult
	var rows []struct {
		ID              uuid.UUID
		SecretEncrypted string
	}
	if err := db.WithContext(ctx).Unscoped().Model(&domain.MFASecret{}).
		Select("id", "secret_encrypted").Find(&rows).Error; err != nil {
		return res, err
	}
	for _, row := range rows {
		if _, err := crypto.DecryptAES256GCM(row.SecretEncrypted, to); err == nil {
			continue
		}
		plain, err := crypto.DecryptAES256GCM(row.SecretEncrypted, from)
		if err != nil {
			res.Unreadable++
			continue
		}
		next, err := crypto.EncryptAES256GCM(plain, to)
		if err != nil {
			return res, err
		}
		upd := db.WithContext(ctx).Unscoped().Model(&domain.MFASecret{}).
			Where("id = ? AND secret_encrypted = ?", row.ID, row.SecretEncrypted).
			UpdateColumn("secret_encrypted", next)
		if upd.Error != nil {
			return res, upd.Error
		}
		res.Rekeyed += int(upd.RowsAffected)
	}
	return res, nil
}
