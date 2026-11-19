// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package compliance

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

// Limits of an imported framework document, enforced before any write.
const (
	MaxFrameworkImportBytes = 2 << 20 // 2 MiB
	maxImportControls       = 2000
	maxImportRowErrors      = 100
	maxFwName               = 255
	maxFwVersion            = 50
	maxFwDescription        = 10000
	maxCtrlCode             = 50
	maxCtrlName             = 255
	maxCtrlDescription      = 10000
	maxCtrlSourceRef        = 255
)

// ImportFrameworkResult is what a successful import returns.
type ImportFrameworkResult struct {
	Framework       *domain.ComplianceFramework
	ControlsCreated int
}

// ImportFrameworkUseCase creates a custom framework and its controls from an
// exchange document, all or nothing.
type ImportFrameworkUseCase struct {
	repo domain.ComplianceRepository
}

func NewImportFrameworkUseCase(repo domain.ComplianceRepository) *ImportFrameworkUseCase {
	return &ImportFrameworkUseCase{repo: repo}
}

// Execute parses and validates raw, then writes the framework and every control
// in one transaction. The tenant is the session's: the file cannot name one.
// Every control starts not_implemented and the framework is hand-built
// (CatalogKey ""), whatever the file says. Nothing is written when any row is
// invalid.
func (uc *ImportFrameworkUseCase) Execute(ctx context.Context, tenantID uuid.UUID, raw []byte) (*ImportFrameworkResult, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewUnauthorizedError("tenant context required")
	}
	if len(raw) > MaxFrameworkImportBytes {
		return nil, domain.NewValidationError("file exceeds the 2 MiB limit")
	}
	var doc FrameworkDocument
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, domain.NewValidationError("invalid JSON document")
	}
	if rows := validateFrameworkDocument(&doc); len(rows) > 0 {
		return nil, domain.NewValidationErrorWithFields("framework document is invalid", rows)
	}

	fw := &domain.ComplianceFramework{
		ID:          uuid.New(),
		TenantID:    tenantID,
		Name:        strings.TrimSpace(doc.Framework.Name),
		Version:     doc.Framework.Version,
		Description: doc.Framework.Description,
		CatalogKey:  "",
	}
	controls := make([]*domain.ComplianceControl, 0, len(doc.Controls))
	for _, c := range doc.Controls {
		controls = append(controls, &domain.ComplianceControl{
			ID:              uuid.New(),
			TenantID:        tenantID,
			FrameworkID:     fw.ID,
			ReferenceCode:   strings.TrimSpace(c.ReferenceCode),
			Name:            c.Name,
			Description:     c.Description,
			SourceReference: c.SourceReference,
			Status:          domain.ControlStatusNotImplemented,
		})
	}
	if err := uc.repo.CreateFrameworkWithControls(ctx, fw, controls); err != nil {
		return nil, fmt.Errorf("application.ImportFramework: %w", err)
	}
	return &ImportFrameworkResult{Framework: fw, ControlsCreated: len(controls)}, nil
}

// validateFrameworkDocument returns every problem found, capped at
// maxImportRowErrors, with the 0-based control index for row problems.
func validateFrameworkDocument(doc *FrameworkDocument) []domain.FieldError {
	var errs []domain.FieldError
	add := func(idx *int, field, msg string) {
		if len(errs) < maxImportRowErrors {
			errs = append(errs, domain.FieldError{Index: idx, Field: field, Message: msg})
		}
	}
	tooLong := func(s string, max int) bool { return utf8.RuneCountInString(s) > max }
	// Postgres rejects a NUL in text; catch it here as a 400, not a 500.
	nul := func(idx *int, field, s string) {
		if strings.ContainsRune(s, 0) {
			add(idx, field, "contains a NUL character")
		}
	}
	nul(nil, "framework.name", doc.Framework.Name)
	nul(nil, "framework.version", doc.Framework.Version)
	nul(nil, "framework.description", doc.Framework.Description)

	if doc.SchemaVersion != FrameworkDocumentSchemaVersion {
		add(nil, "schema_version", fmt.Sprintf("must be %d", FrameworkDocumentSchemaVersion))
	}
	if strings.TrimSpace(doc.Framework.Name) == "" {
		add(nil, "framework.name", "is required")
	} else if tooLong(doc.Framework.Name, maxFwName) {
		add(nil, "framework.name", fmt.Sprintf("must be %d characters or less", maxFwName))
	}
	if tooLong(doc.Framework.Version, maxFwVersion) {
		add(nil, "framework.version", fmt.Sprintf("must be %d characters or less", maxFwVersion))
	}
	if tooLong(doc.Framework.Description, maxFwDescription) {
		add(nil, "framework.description", fmt.Sprintf("must be %d characters or less", maxFwDescription))
	}
	switch n := len(doc.Controls); {
	case n == 0:
		add(nil, "controls", "must contain at least one control")
	case n > maxImportControls:
		add(nil, "controls", fmt.Sprintf("must contain at most %d controls", maxImportControls))
		return errs // do not scan an oversized list row by row
	}

	seen := make(map[string]struct{}, len(doc.Controls))
	for i, c := range doc.Controls {
		i := i
		idx := &i
		nul(idx, "reference_code", c.ReferenceCode)
		nul(idx, "name", c.Name)
		nul(idx, "description", c.Description)
		nul(idx, "source_reference", c.SourceReference)
		code := strings.TrimSpace(c.ReferenceCode)
		switch {
		case code == "":
			add(idx, "reference_code", "is required")
		case tooLong(code, maxCtrlCode):
			add(idx, "reference_code", fmt.Sprintf("must be %d characters or less", maxCtrlCode))
		default:
			if _, dup := seen[code]; dup {
				add(idx, "reference_code", "is duplicated in the file")
			}
			seen[code] = struct{}{}
		}
		if strings.TrimSpace(c.Name) == "" {
			add(idx, "name", "is required")
		} else if tooLong(c.Name, maxCtrlName) {
			add(idx, "name", fmt.Sprintf("must be %d characters or less", maxCtrlName))
		}
		if tooLong(c.Description, maxCtrlDescription) {
			add(idx, "description", fmt.Sprintf("must be %d characters or less", maxCtrlDescription))
		}
		if tooLong(c.SourceReference, maxCtrlSourceRef) {
			add(idx, "source_reference", fmt.Sprintf("must be %d characters or less", maxCtrlSourceRef))
		}
	}
	return errs
}
