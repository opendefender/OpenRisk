// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package compliance

import (
	"context"

	pkgcompliance "github.com/opendefender/openrisk/pkg/compliance"
)

// CatalogSummary is what a client needs to offer catalogs for import — no tenant/framework
// context, since catalogs are static reference data shared by every tenant.
// JSON tags matter here: they must match docs/openapi.yaml's ComplianceCatalogSummary
// schema (snake_case) exactly, since the frontend's generated types are keyed off that spec.
type CatalogSummary struct {
	Key          string `json:"key"`
	Name         string `json:"name"`
	Version      string `json:"version"`
	Description  string `json:"description"`
	Available    bool   `json:"available"`
	ControlCount int    `json:"control_count"`
}

// ListCatalogsUseCase lists every registered regulatory catalog (available and
// not-yet-available placeholders alike, see pkg/compliance) so the UI can offer them for
// import and show what's still pending review.
type ListCatalogsUseCase struct{}

func NewListCatalogsUseCase() *ListCatalogsUseCase {
	return &ListCatalogsUseCase{}
}

func (uc *ListCatalogsUseCase) Execute(_ context.Context) []CatalogSummary {
	catalogs := pkgcompliance.List()
	out := make([]CatalogSummary, 0, len(catalogs))
	for _, c := range catalogs {
		out = append(out, CatalogSummary{
			Key:          c.Key,
			Name:         c.Name,
			Version:      c.Version,
			Description:  c.Description,
			Available:    c.Available,
			ControlCount: len(c.Controls),
		})
	}
	return out
}
