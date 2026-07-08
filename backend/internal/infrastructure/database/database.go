// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package database

import (
	"fmt"
	"log"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func Connect() {
	// Build DSN from environment variables with sensible defaults
	host := os.Getenv("DB_HOST")
	if host == "" {
		host = "localhost"
	}

	port := os.Getenv("DB_PORT")
	if port == "" {
		port = "5434"
	}

	user := os.Getenv("DB_USER")
	if user == "" {
		user = "openrisk"
	}

	password := os.Getenv("DB_PASSWORD")

	dbname := os.Getenv("DB_NAME")
	if dbname == "" {
		dbname = "openrisk"
	}

	// Try to use DATABASE_URL if provided (takes precedence)
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = fmt.Sprintf(
			"host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=UTC",
			host, user, password, dbname, port,
		)
	}

	var err error
	DB, err = gorm.Open(postgres.Open(databaseURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
		// TranslateError lets repositories detect constraint violations via
		// sentinel errors (e.g. errors.Is(err, gorm.ErrDuplicatedKey))
		// instead of parsing driver-specific error codes.
		TranslateError: true,
		// GORM's AutoMigrate topologically reorders models to satisfy FK constraints,
		// but with 15+ interrelated domain models (some intentionally cyclic, e.g.
		// User.DefaultOrg <-> Organization.Owner) that ordering isn't reliable and
		// AutoMigrate can crash on a fresh DB. Referential integrity across tenants is
		// already enforced at the application layer (tenant_id filtering on every query,
		// see CLAUDE.md), so DB-level FK constraints aren't load-bearing here.
		DisableForeignKeyConstraintWhenMigrating: true,
	})

	if err != nil {
		log.Fatal("Failed to connect to database! \n", err)
	}

	log.Println("Connected to PostgreSQL database successfully")

	sqlDB, err := DB.DB()
	if err != nil {
		log.Fatal("Failed to get database instance")
	}

	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)
}
