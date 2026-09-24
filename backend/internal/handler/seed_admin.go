// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"math/big"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/auth"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/database"
)

// InitialAdminEmail is the account the first boot creates. scripts/install.sh
// and the E2E seed sign in with it.
const InitialAdminEmail = "admin@opendefender.io"

// defaultInitialAdminPasswordFile is where a generated first password is
// written when INITIAL_ADMIN_PASSWORD_FILE is unset. Relative to the working
// directory, so /app/secrets in the container, which the dev compose file
// mounts as a volume.
const defaultInitialAdminPasswordFile = "secrets/initial_admin_password"

// retiredDefaultAdminPassword is the value seeded before #485 whenever
// INITIAL_ADMIN_PASSWORD was unset, in every environment but APP_ENV=production.
// It has been public in this repository since 2025. It stays here only so a
// boot can tell an operator their administrator still uses it.
const retiredDefaultAdminPassword = "admin123"

// generatedPasswordLength and generatedPasswordAlphabet give about 190 bits of
// entropy, with no character a shell or a YAML file would reinterpret.
const (
	generatedPasswordLength   = 32
	generatedPasswordAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
)

// passwordHasher is the subset of auth.Argon2idPasswordHasher the seed needs.
type passwordHasher interface {
	Hash(password string) (string, error)
	Verify(hashedPassword, plainPassword string) bool
}

// SeedAdminUser creates the first administrator when the database holds no
// user at all, and warns when an existing one still uses the retired default.
// Called once at boot from main.go.
//
// The password comes from INITIAL_ADMIN_PASSWORD. Without it, a random one is
// generated and written to INITIAL_ADMIN_PASSWORD_FILE with mode 0600. It is
// never logged: only the file path is. A boot that cannot write that file
// stops, since an administrator nobody knows the password of is no use.
func SeedAdminUser() {
	hasher := auth.NewConfiguredArgon2idPasswordHasher()
	seeded, err := seedInitialAdmin(database.DB, hasher, os.Getenv)
	if err != nil {
		var fileErr *passwordFileError
		if errors.As(err, &fileErr) {
			log.Fatalf("FATAL: %v", err)
		}
		log.Printf("initial admin not seeded: %v", err)
		return
	}
	if seeded != nil {
		if seeded.PasswordFile != "" {
			log.Printf("Initial admin %s created. Its generated password is in %s (mode 0600): read it, sign in, change it, then delete the file.",
				InitialAdminEmail, seeded.PasswordFile)
		} else {
			log.Printf("Initial admin %s created with the password from INITIAL_ADMIN_PASSWORD.", InitialAdminEmail)
		}
		return
	}
	warnIfRetiredDefaultPassword(database.DB, hasher)
}

// seededAdmin describes what seedInitialAdmin created.
type seededAdmin struct {
	UserID uuid.UUID
	// PasswordFile is the file holding a generated password, or "" when the
	// operator supplied INITIAL_ADMIN_PASSWORD.
	PasswordFile string
}

// passwordFileError marks the one failure that must stop the boot.
type passwordFileError struct {
	path string
	err  error
}

func (e *passwordFileError) Error() string {
	return fmt.Sprintf("cannot write the generated initial admin password to %s: %v. Set INITIAL_ADMIN_PASSWORD, or point INITIAL_ADMIN_PASSWORD_FILE at a writable path", e.path, e.err)
}

func (e *passwordFileError) Unwrap() error { return e.err }

// seedInitialAdmin returns (nil, nil) when users already exist.
//
// User, organisation and membership are written in one transaction, so a
// failure leaves no half-created administrator. A generated password is written
// to disk before the transaction and removed if the transaction fails: when
// several replicas boot together, only the one that created the account keeps a
// file, and that file matches the account.
func seedInitialAdmin(db *gorm.DB, hasher passwordHasher, getenv func(string) string) (*seededAdmin, error) {
	var count int64
	if err := db.Model(&domain.User{}).Count(&count).Error; err != nil {
		return nil, fmt.Errorf("count users: %w", err)
	}
	if count > 0 {
		return nil, nil
	}

	password := getenv("INITIAL_ADMIN_PASSWORD")
	passwordFile := ""
	if password == "" {
		generated, err := generateInitialAdminPassword()
		if err != nil {
			return nil, fmt.Errorf("generate initial admin password: %w", err)
		}
		passwordFile = getenv("INITIAL_ADMIN_PASSWORD_FILE")
		if passwordFile == "" {
			passwordFile = defaultInitialAdminPasswordFile
		}
		if err := writeSecretFile(passwordFile, generated); err != nil {
			return nil, &passwordFileError{path: passwordFile, err: err}
		}
		password = generated
	}

	hash, err := hasher.Hash(password)
	if err != nil {
		removeSecretFile(passwordFile)
		return nil, fmt.Errorf("hash initial admin password: %w", err)
	}

	admin := domain.User{
		ID:       uuid.New(),
		Email:    InitialAdminEmail,
		Username: "admin",
		Password: hash,
		FullName: "System Administrator",
		IsActive: true,
	}

	// No legacy role row. The `roles` table belongs to domain.RoleEnhanced
	// (rbac.go) and has no `permissions` column, so inserting a domain.Role
	// there always failed. The old seed ignored that error and left RoleID
	// nil, which is what every deployment has. The administrator's authority
	// comes from the root membership below.
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&admin).Error; err != nil {
			return fmt.Errorf("create admin user: %w", err)
		}

		// The multi-tenant LoginUseCase requires a default organization and a
		// membership to authenticate (application/auth/login.go). Mirror what
		// registration does so the seeded admin can sign in.
		org := domain.Organization{
			ID:       uuid.New(),
			Name:     "OpenDefender",
			Slug:     "opendefender",
			OwnerID:  admin.ID,
			IsActive: true,
		}
		if err := tx.Create(&org).Error; err != nil {
			return fmt.Errorf("create default organization: %w", err)
		}
		if err := tx.Model(&admin).Update("default_org_id", org.ID).Error; err != nil {
			return fmt.Errorf("set default organization: %w", err)
		}
		if err := tx.Create(&domain.OrganizationMember{
			ID:             uuid.New(),
			OrganizationID: org.ID,
			UserID:         admin.ID,
			Role:           domain.RoleRoot,
			IsActive:       true,
			JoinedAt:       time.Now(),
		}).Error; err != nil {
			return fmt.Errorf("create membership: %w", err)
		}
		return nil
	})
	if err != nil {
		removeSecretFile(passwordFile)
		return nil, err
	}

	return &seededAdmin{UserID: admin.ID, PasswordFile: passwordFile}, nil
}

// warnIfRetiredDefaultPassword logs when the initial administrator still signs
// in with the pre-#485 default. Deployments seeded before that change, Helm
// installs included (the chart never set APP_ENV), are exposed until someone
// changes it. The password itself is never logged.
func warnIfRetiredDefaultPassword(db *gorm.DB, hasher passwordHasher) {
	if usesRetiredDefaultPassword(db, hasher) {
		log.Printf("SECURITY WARNING: %s still uses the retired default password published in this repository. Change it now: anyone who has read the source can sign in as this administrator.", InitialAdminEmail)
	}
}

func usesRetiredDefaultPassword(db *gorm.DB, hasher passwordHasher) bool {
	var admin domain.User
	if err := db.Select("id", "password").Where("email = ?", InitialAdminEmail).First(&admin).Error; err != nil {
		return false
	}
	return hasher.Verify(admin.Password, retiredDefaultAdminPassword)
}

// generateInitialAdminPassword draws each character uniformly from
// generatedPasswordAlphabet with crypto/rand.
func generateInitialAdminPassword() (string, error) {
	alphabetSize := big.NewInt(int64(len(generatedPasswordAlphabet)))
	out := make([]byte, generatedPasswordLength)
	for i := range out {
		n, err := rand.Int(rand.Reader, alphabetSize)
		if err != nil {
			return "", err
		}
		out[i] = generatedPasswordAlphabet[n.Int64()]
	}
	return string(out), nil
}

// writeSecretFile writes value to path with mode 0600, creating the parent
// directory with mode 0700 if needed. It goes through a temporary file and a
// rename, so the path never holds a partial value or wider permissions, even
// when a stale file from an earlier failed boot is there.
func writeSecretFile(path, value string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".initial_admin_password-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }

	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		cleanup()
		return err
	}
	if _, err := tmp.WriteString(value + "\n"); err != nil {
		_ = tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		cleanup()
		return err
	}
	return nil
}

func removeSecretFile(path string) {
	if path == "" {
		return
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Printf("could not remove %s after a failed seed: %v", path, err)
	}
}
