// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package assetschema owns the typed attribute schemas that govern each asset
// category. It is the single place that answers "what does a Server look like
// for THIS tenant?" — the form generator, the write-path validator and the
// vulnerability correlator all ask it, so none of them can drift from another.
package assetschema

import (
	"context"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// Service reads and edits a tenant's asset attribute schemas.
type Service struct {
	repo domain.AssetTypeSchemaRepository
}

func NewService(repo domain.AssetTypeSchemaRepository) *Service {
	return &Service{repo: repo}
}

// Get returns the schema governing a category for a tenant.
//
// A tenant that has never edited anything has no row; rather than 404 (which
// would make an unedited category unusable) it is served the shipped default,
// and the row is seeded so the next read is stable and the edit path has
// something to update. Seeding failures are tolerated: serving the correct
// schema matters more than persisting it, and the next call retries.
func (s *Service) Get(ctx context.Context, tenantID uuid.UUID, cat domain.AssetCategory) (*domain.AssetTypeSchema, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewForbiddenError("missing tenant context")
	}
	if _, err := domain.ParseAssetCategory(string(cat)); err != nil {
		return nil, err
	}
	existing, err := s.repo.GetByCategory(ctx, tenantID, cat)
	if err != nil {
		return nil, domain.NewInternalError("failed to load asset schema: " + err.Error())
	}
	if existing != nil {
		return existing, nil
	}
	seeded := domain.DefaultSchemaFor(tenantID, cat)
	_ = s.repo.Upsert(ctx, seeded)
	return seeded, nil
}

// List returns the schema for every supported category, seeding any that the
// tenant has never touched. The result is always the full set of 8, in the
// canonical order, so the UI never has to reason about partial coverage.
func (s *Service) List(ctx context.Context, tenantID uuid.UUID) ([]domain.AssetTypeSchema, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewForbiddenError("missing tenant context")
	}
	rows, err := s.repo.ListByTenant(ctx, tenantID)
	if err != nil {
		return nil, domain.NewInternalError("failed to list asset schemas: " + err.Error())
	}
	byCat := make(map[domain.AssetCategory]domain.AssetTypeSchema, len(rows))
	for _, r := range rows {
		byCat[r.Category] = r
	}

	out := make([]domain.AssetTypeSchema, 0, len(domain.AssetCategories))
	for _, cat := range domain.AssetCategories {
		if r, ok := byCat[cat]; ok {
			out = append(out, r)
			continue
		}
		seeded := domain.DefaultSchemaFor(tenantID, cat)
		_ = s.repo.Upsert(ctx, seeded)
		out = append(out, *seeded)
	}
	return out, nil
}

// UpdateInput is a tenant's edit of one category's schema.
type UpdateInput struct {
	Label      string                `json:"label"`
	Attributes []domain.AttributeDef `json:"attributes"`
}

// Update replaces a category's attribute schema.
//
// The new schema is validated as a whole before anything is written: a schema is
// the contract every asset of that category is checked against, so a half-applied
// or malformed one would make the category unwritable for everybody.
func (s *Service) Update(ctx context.Context, tenantID uuid.UUID, cat domain.AssetCategory, in UpdateInput) (*domain.AssetTypeSchema, error) {
	current, err := s.Get(ctx, tenantID, cat)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateSchema(in.Attributes); err != nil {
		return nil, err
	}

	label := in.Label
	if label == "" {
		label = current.Label
	}

	current.Label = label
	current.Attributes = in.Attributes
	current.Customized = true
	current.Version = current.Version + 1
	if current.ID == uuid.Nil {
		current.ID = uuid.New()
	}
	current.TenantID = tenantID

	if err := s.repo.Upsert(ctx, current); err != nil {
		return nil, domain.NewInternalError("failed to save asset schema: " + err.Error())
	}
	return current, nil
}

// Reset drops a tenant's customization, restoring the shipped default.
func (s *Service) Reset(ctx context.Context, tenantID uuid.UUID, cat domain.AssetCategory) (*domain.AssetTypeSchema, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewForbiddenError("missing tenant context")
	}
	if _, err := domain.ParseAssetCategory(string(cat)); err != nil {
		return nil, err
	}
	if err := s.repo.Delete(ctx, tenantID, cat); err != nil {
		return nil, domain.NewInternalError("failed to reset asset schema: " + err.Error())
	}
	return s.Get(ctx, tenantID, cat)
}

// Validator adapts the schema service into the narrow port the asset write path
// needs: "given this category, are these attributes valid, and what do I store?".
type Validator struct {
	svc *Service
}

// NewValidator returns the write-path validator backed by this service.
func NewValidator(svc *Service) *Validator { return &Validator{svc: svc} }

// ValidateFor validates an attribute bag against the tenant's schema for a
// category and returns the coerced bag to persist, together with the schema used
// (so the caller can refresh the asset's correlation fingerprints from it).
func (v *Validator) ValidateFor(ctx context.Context, tenantID uuid.UUID, cat domain.AssetCategory, in map[string]any) (domain.AssetAttributes, []domain.AttributeDef, error) {
	if v == nil || v.svc == nil {
		// No schema service wired: accept nothing rather than accept anything.
		// An unvalidated attribute bag is worse than an absent one.
		if len(in) == 0 {
			return nil, nil, nil
		}
		return nil, nil, domain.NewInternalError("asset attribute schemas are not available")
	}
	schema, err := v.svc.Get(ctx, tenantID, cat)
	if err != nil {
		return nil, nil, err
	}
	attrs, err := domain.ValidateAttributes(schema.Attributes, in)
	if err != nil {
		return nil, nil, err
	}
	return attrs, schema.Attributes, nil
}
