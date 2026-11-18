// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

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
	// Review is the sign-off state of the catalog's content (D-120). Available only means
	// importable; Review.Status says whether anyone checked the content.
	Review CatalogReview `json:"review"`
}

// CatalogReview mirrors pkg/compliance.Review on the wire. Sources and Gaps are always
// arrays (never null) so a client can iterate without a nil check.
type CatalogReview struct {
	Status   string                `json:"status"`
	Date     string                `json:"date"`
	Reviewer string                `json:"reviewer"`
	Sources  []CatalogReviewSource `json:"sources"`
	Gaps     []string              `json:"gaps"`
}

// CatalogReviewSource is one document a catalog was reviewed against.
type CatalogReviewSource struct {
	Document string `json:"document"`
	URL      string `json:"url"`
}

func toCatalogReview(r pkgcompliance.Review) CatalogReview {
	out := CatalogReview{
		Status:   string(r.Status),
		Date:     r.Date,
		Reviewer: r.Reviewer,
		Sources:  make([]CatalogReviewSource, 0, len(r.Sources)),
		Gaps:     append([]string{}, r.Gaps...),
	}
	for _, src := range r.Sources {
		out.Sources = append(out.Sources, CatalogReviewSource{Document: src.Document, URL: src.URL})
	}
	return out
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
			Review:       toCatalogReview(c.Review),
		})
	}
	return out
}
