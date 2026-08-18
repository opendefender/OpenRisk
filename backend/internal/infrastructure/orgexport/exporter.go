// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package orgexport produces the full tenant data export required (RGPD / Cameroon
// law 2024/017) BEFORE an organization can be deleted, and served on demand from
// the danger zone. It is tenant-scoped and best-effort per collection: a missing
// table degrades that slice rather than failing the whole export.
package orgexport

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"gorm.io/gorm"
)

// Exporter writes a JSON bundle of a tenant's data to disk.
type Exporter struct {
	db  *gorm.DB
	dir string
}

// New builds an exporter writing under dir (created if missing).
func New(db *gorm.DB, dir string) *Exporter {
	if dir == "" {
		dir = "uploads/exports"
	}
	return &Exporter{db: db, dir: dir}
}

// Bundle is the exported document.
type Bundle struct {
	Version      string      `json:"version"`
	ExportedAt   string      `json:"exported_at"`
	Organization interface{} `json:"organization"`
	Members      interface{} `json:"members"`
	Risks        interface{} `json:"risks"`
	Assets       interface{} `json:"assets"`
	Mitigations  interface{} `json:"mitigations"`
	Incidents    interface{} `json:"incidents"`
	Subscription interface{} `json:"subscription"`
	Invoices     interface{} `json:"invoices"`
}

// Export writes the tenant's bundle and returns its path. Building the bundle
// tenant-scopes every query.
func (e *Exporter) Export(ctx context.Context, tenant uuid.UUID) (string, error) {
	db := e.db.WithContext(ctx)
	bundle := Bundle{Version: "1", ExportedAt: time.Now().UTC().Format(time.RFC3339)}

	var org domain.Organization
	if err := db.Where("id = ?", tenant).First(&org).Error; err == nil {
		bundle.Organization = org
	}
	bundle.Members = findSlice[domain.OrganizationMember](db, "organization_id = ?", tenant)
	bundle.Risks = findSlice[domain.Risk](db, "tenant_id = ?", tenant)
	bundle.Assets = findSlice[domain.Asset](db, "tenant_id = ?", tenant)
	bundle.Mitigations = findSlice[domain.Mitigation](db, "tenant_id = ?", tenant)
	bundle.Incidents = findSlice[domain.Incident](db, "tenant_id = ?", tenant)
	bundle.Subscription = findSlice[domain.Subscription](db, "organization_id = ?", tenant)
	bundle.Invoices = findSlice[domain.Invoice](db, "organization_id = ?", tenant)

	if err := os.MkdirAll(e.dir, 0o750); err != nil {
		return "", err
	}
	path := filepath.Join(e.dir, fmt.Sprintf("org-export-%s-%d.json", tenant.String(), time.Now().Unix()))
	data, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, 0o640); err != nil {
		return "", err
	}
	return path, nil
}

// findSlice runs a best-effort tenant-scoped Find; a query error (e.g. a table
// that is not migrated in this build) degrades to an empty slice.
func findSlice[T any](db *gorm.DB, cond string, args ...interface{}) []T {
	var out []T
	if err := db.Where(cond, args...).Find(&out).Error; err != nil {
		return []T{}
	}
	return out
}
