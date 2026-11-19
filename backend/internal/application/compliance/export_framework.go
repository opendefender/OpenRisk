// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package compliance

import (
	"context"
	"sort"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

// FrameworkDocumentSchemaVersion is the only version of the exchange document
// this build reads and writes.
const FrameworkDocumentSchemaVersion = 1

// FrameworkDocument is the JSON exchange format of a custom framework: what
// export writes and import reads. It is a dedicated DTO, never a domain entity:
// it carries no id, tenant, status, evidence or timestamp, so a file can neither
// leak nor assert any of them.
type FrameworkDocument struct {
	SchemaVersion int                   `json:"schema_version"`
	Framework     FrameworkDocumentMeta `json:"framework"`
	Controls      []ControlDocument     `json:"controls"`
}

// FrameworkDocumentMeta is the framework metadata of a FrameworkDocument.
type FrameworkDocumentMeta struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
}

// ControlDocument is one control of a FrameworkDocument.
type ControlDocument struct {
	ReferenceCode   string `json:"reference_code"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	SourceReference string `json:"source_reference"`
}

// ExportFrameworkUseCase builds the exchange document of a tenant's framework.
type ExportFrameworkUseCase struct {
	repo domain.ComplianceRepository
}

func NewExportFrameworkUseCase(repo domain.ComplianceRepository) *ExportFrameworkUseCase {
	return &ExportFrameworkUseCase{repo: repo}
}

// Execute returns the document, or ErrNotFound when the framework does not
// exist for this tenant (a foreign framework looks exactly like a missing one).
func (uc *ExportFrameworkUseCase) Execute(ctx context.Context, tenantID, frameworkID uuid.UUID) (*FrameworkDocument, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewUnauthorizedError("tenant context required")
	}
	fw, err := uc.repo.GetFrameworkByID(ctx, frameworkID, tenantID)
	if err != nil {
		return nil, err
	}
	if fw == nil {
		return nil, domain.NewNotFoundError("framework", frameworkID)
	}
	controls, err := uc.repo.ListControlsByFramework(ctx, tenantID, frameworkID)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(controls, func(i, j int) bool {
		if controls[i].ReferenceCode != controls[j].ReferenceCode {
			return controls[i].ReferenceCode < controls[j].ReferenceCode
		}
		return controls[i].Name < controls[j].Name
	})
	doc := &FrameworkDocument{
		SchemaVersion: FrameworkDocumentSchemaVersion,
		Framework:     FrameworkDocumentMeta{Name: fw.Name, Version: fw.Version, Description: fw.Description},
		Controls:      make([]ControlDocument, 0, len(controls)),
	}
	for _, c := range controls {
		doc.Controls = append(doc.Controls, ControlDocument{
			ReferenceCode: c.ReferenceCode, Name: c.Name, Description: c.Description, SourceReference: c.SourceReference,
		})
	}
	return doc, nil
}
