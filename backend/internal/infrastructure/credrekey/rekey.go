// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package credrekey moves tenant credentials encrypted with one
// SCANNER_CREDENTIAL_KEY onto another at boot (#1200).
package credrekey

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/tenantguard"
	"github.com/opendefender/openrisk/pkg/crypto"
)

// Column is one encrypted column written by scanner.CredentialCipher.
type Column struct {
	Model  any    // GORM model; its table holds the column
	Table  string // for reports only
	Column string // a constant from this file, never user input
}

// Columns lists every column encrypted with SCANNER_CREDENTIAL_KEY. A new
// column encrypted with scanner.CredentialCipher belongs here, or an install
// that sets its first key loses it.
var Columns = []Column{
	{Model: &domain.ScanConfig{}, Table: "scan_configs", Column: "encrypted_credentials"},
	{Model: &domain.ScannerAgent{}, Table: "scanner_agents", Column: "push_secret_enc"},
	{Model: &domain.VulnIntegration{}, Table: "vuln_integrations", Column: "encrypted_credentials"},
	{Model: &domain.VulnTicketingConfig{}, Table: "vuln_ticketing_configs", Column: "encrypted_credentials"},
}

// Result reports one pass over one column.
type Result struct {
	Table      string
	Column     string
	Rekeyed    int // moved from the old key to the new one
	Unreadable int // readable with neither key; left as they were
}

// Rekey re-encrypts, under `to`, every value in cols that only `from` can
// decrypt. `from` and `to` are the derived 32-byte AES keys.
//
// An install that ran without SCANNER_CREDENTIAL_KEY encrypted its cloud,
// agent, vulnerability-tool and ITSM credentials with the public development
// key. Production now requires a real key; this moves those values onto it at
// boot, so nobody has to enter them again.
//
// Cross-tenant by design: a boot maintenance pass over every organisation's
// rows, like authmfa.RekeySecrets, never reachable from a request. Soft-deleted
// rows are included: they are still decryptable data at rest.
//
// No transaction: each row is updated only if it still holds the ciphertext
// that was read, so every row is at all times readable with one of the two
// keys, replicas booting together cannot overwrite each other, and a pass
// interrupted half-way is finished by the next boot.
func Rekey(ctx context.Context, db *gorm.DB, from, to []byte, cols []Column) ([]Result, error) {
	results := make([]Result, 0, len(cols))
	for _, c := range cols {
		res, err := rekeyColumn(ctx, db, from, to, c)
		if err != nil {
			return results, fmt.Errorf("%s.%s: %w", c.Table, c.Column, err)
		}
		results = append(results, res)
	}
	return results, nil
}

func rekeyColumn(ctx context.Context, db *gorm.DB, from, to []byte, c Column) (Result, error) {
	res := Result{Table: c.Table, Column: c.Column}
	var rows []struct {
		ID    uuid.UUID
		Value string
	}
	if err := tenantguard.CrossTenant(db.WithContext(ctx), "credential key rotation re-encrypts every stored credential in the deployment").Unscoped().Model(c.Model).
		Select("id, " + c.Column + " AS value").
		Where(c.Column + " IS NOT NULL AND " + c.Column + " <> ''").
		Find(&rows).Error; err != nil {
		return res, err
	}
	for _, row := range rows {
		if _, err := crypto.DecryptAES256GCM(row.Value, to); err == nil {
			continue
		}
		plain, err := crypto.DecryptAES256GCM(row.Value, from)
		if err != nil {
			res.Unreadable++
			continue
		}
		next, err := crypto.EncryptAES256GCM(plain, to)
		if err != nil {
			return res, err
		}
		upd := tenantguard.CrossTenant(db.WithContext(ctx), "credential key rotation re-encrypts every stored credential in the deployment").Unscoped().Model(c.Model).
			Where("id = ? AND "+c.Column+" = ?", row.ID, row.Value).
			UpdateColumn(c.Column, next)
		if upd.Error != nil {
			return res, upd.Error
		}
		res.Rekeyed += int(upd.RowsAffected)
	}
	return res, nil
}
