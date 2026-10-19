// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"runtime"
	"sync"
	"time"

	_ "github.com/lib/pq" // "postgres" driver, the one golang-migrate uses
	"golang.org/x/crypto/bcrypt"
)

// purgeBeforeVersion is the migration that used to purge every MFA backup
// code (0056). The exact purge below runs just before it.
const purgeBeforeVersion = 56

// purgeLedgerDDL is the table the purge writes its victims to, so the backend
// can tell each of them once it is up (application/auth
// NotifyPurgedBackupCodesUseCase). The purge can run before 0078 declares the
// table, so it creates it too; the shape is the same.
const purgeLedgerDDL = `CREATE TABLE IF NOT EXISTS mfa_backup_code_purges (
	id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
	tenant_id   uuid        NOT NULL,
	user_id     uuid        NOT NULL,
	purged_at   timestamptz NOT NULL DEFAULT now(),
	notified_at timestamptz
)`

// legacyUniversalBackupCodes rebuilds the eight backup codes every user got
// before ff6961ba (#240). The old generator seeded an LCG with a constant, so
// the codes were the same for everyone and derivable from the public source.
// They are reproduced here only to recognise and delete them. Never hand
// these out.
func legacyUniversalBackupCodes() []string {
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567"
	codes := make([]string, 8)
	seed := int64(0x1234567890abcdef)
	for i := range codes {
		code := make([]byte, 12)
		for j := range code {
			seed = (seed*1103515245 + 12345) & 0x7fffffff
			code[j] = chars[seed%int64(len(chars))]
		}
		codes[i] = string(code)
	}
	return codes
}

// backupCodeBatch is the codes one enrolment wrote: same user, same tenant,
// created in the same second. One generator call made all of them, so they
// are either all universal or all random, and testing one hash settles the
// whole batch.
type backupCodeBatch struct {
	userID, tenantID string
	second           time.Time
	sampleHash       string
}

// purgeUniversalBackupCodes deletes the backup codes the old generator made,
// and only those (#1001). 0056 used to delete every code. That was right on
// the deployments it first ran on, but compose installs ran without the SQL
// layer, so their first run of 0056 happens now, on codes that are mostly
// real CSPRNG credentials. Deleting those would leave a user who lost their
// phone with no way in.
//
// Codes are bcrypt-hashed with a per-row salt, so SQL cannot tell them apart.
// This compares one hash per batch against the eight known codes: at most
// eight bcrypt checks per enrolment, spread over the CPUs. It is a one-time,
// cross-tenant maintenance pass like the SQL files it sits between; every
// delete is scoped by user_id and tenant_id.
func purgeUniversalBackupCodes(ctx context.Context, db *sql.DB) error {
	var exists bool
	if err := db.QueryRowContext(ctx, `SELECT to_regclass('mfa_backup_codes') IS NOT NULL`).Scan(&exists); err != nil {
		return fmt.Errorf("migrations: look for mfa_backup_codes: %w", err)
	}
	if !exists {
		return nil
	}

	rows, err := db.QueryContext(ctx, `
		SELECT DISTINCT ON (user_id, tenant_id, date_trunc('second', created_at))
		       user_id::text, tenant_id::text, date_trunc('second', created_at), code_hash
		  FROM mfa_backup_codes
		 ORDER BY user_id, tenant_id, date_trunc('second', created_at), id`)
	if err != nil {
		return fmt.Errorf("migrations: list backup code batches: %w", err)
	}
	var batches []backupCodeBatch
	for rows.Next() {
		var b backupCodeBatch
		if err := rows.Scan(&b.userID, &b.tenantID, &b.second, &b.sampleHash); err != nil {
			rows.Close()
			return fmt.Errorf("migrations: read backup code batch: %w", err)
		}
		batches = append(batches, b)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("migrations: list backup code batches: %w", err)
	}
	if len(batches) == 0 {
		return nil
	}

	universal := legacyUniversalBackupCodes()
	compromised := make([]bool, len(batches))
	work := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < runtime.NumCPU(); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range work {
				for _, code := range universal {
					if bcrypt.CompareHashAndPassword([]byte(batches[i].sampleHash), []byte(code)) == nil {
						compromised[i] = true
						break
					}
				}
			}
		}()
	}
	for i := range batches {
		work <- i
	}
	close(work)
	wg.Wait()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("migrations: purge universal backup codes: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, purgeLedgerDDL); err != nil {
		return fmt.Errorf("migrations: create the backup-code purge ledger: %w", err)
	}
	var purged, kept int
	told := map[[2]string]bool{}
	for i, b := range batches {
		if !compromised[i] {
			kept++
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			DELETE FROM mfa_backup_codes
			 WHERE user_id = $1 AND tenant_id = $2
			   AND date_trunc('second', created_at) = $3`,
			b.userID, b.tenantID, b.second); err != nil {
			return fmt.Errorf("migrations: purge universal backup codes: %w", err)
		}
		purged++
		// One notice per person, however many batches they had. Written in the
		// same transaction as the delete: nobody loses codes without a record
		// that they must be told.
		if who := [2]string{b.tenantID, b.userID}; !told[who] {
			told[who] = true
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO mfa_backup_code_purges (tenant_id, user_id) VALUES ($1, $2)`,
				b.tenantID, b.userID); err != nil {
				return fmt.Errorf("migrations: record a backup-code purge: %w", err)
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("migrations: purge universal backup codes: %w", err)
	}
	log.Printf("migrations: universal MFA backup codes purged from %d enrolment(s), %d kept", purged, kept)
	return nil
}
