// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package reportjob

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/pkg/report"
)

// complianceReportRenderer is the existing per-framework PDF use case, narrowed
// to what this generator needs so the package does not depend on the whole
// compliance application layer.
type complianceReportRenderer interface {
	Execute(ctx context.Context, tenantID, frameworkID, requestedBy uuid.UUID, locale report.Locale) (*report.ComplianceReportData, error)
}

// ComplianceGenerator renders the per-framework compliance PDF as a job
// artifact. It wraps the same use case the synchronous download endpoint uses,
// so both paths produce byte-identical documents.
type ComplianceGenerator struct {
	uc complianceReportRenderer
}

// NewComplianceGenerator builds the generator.
func NewComplianceGenerator(uc complianceReportRenderer) *ComplianceGenerator {
	return &ComplianceGenerator{uc: uc}
}

// Kind implements Generator.
func (g *ComplianceGenerator) Kind() domain.ReportKind { return domain.ReportKindComplianceFramework }

// Generate renders the framework's compliance PDF.
//
// Params: framework_id (uuid, required), locale ("fr" | "en", default fr).
func (g *ComplianceGenerator) Generate(
	ctx context.Context,
	tenantID, requestedBy uuid.UUID,
	params map[string]any,
) (GeneratedReport, error) {
	raw, _ := params["framework_id"].(string)
	frameworkID, err := uuid.Parse(strings.TrimSpace(raw))
	if err != nil {
		return GeneratedReport{}, domain.NewValidationError("framework_id must be a valid UUID")
	}

	locale := report.LocaleFR
	if l, _ := params["locale"].(string); strings.EqualFold(l, "en") {
		locale = report.LocaleEN
	}

	data, err := g.uc.Execute(ctx, tenantID, frameworkID, requestedBy, locale)
	if err != nil {
		return GeneratedReport{}, err
	}

	pdf, err := report.RenderCompliancePDF(*data)
	if err != nil {
		return GeneratedReport{}, domain.NewInternalError("failed to render the compliance report")
	}

	title := data.FrameworkName
	if data.FrameworkVersion != "" {
		title += " " + data.FrameworkVersion
	}
	return GeneratedReport{
		Title:       title,
		Filename:    ReportFilename(data.FrameworkName, data.FrameworkVersion),
		ContentType: "application/pdf",
		Bytes:       pdf,
	}, nil
}

// ReportFilename builds a safe, descriptive PDF filename from the framework
// identity, e.g. "compliance-report-iso-iec-27001-2022.pdf".
func ReportFilename(name, version string) string {
	slug := func(s string) string {
		var b strings.Builder
		prevDash := false
		for _, r := range strings.ToLower(s) {
			switch {
			case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
				b.WriteRune(r)
				prevDash = false
			default:
				if !prevDash && b.Len() > 0 {
					b.WriteByte('-')
					prevDash = true
				}
			}
		}
		return strings.Trim(b.String(), "-")
	}
	out := "compliance-report"
	if s := slug(name); s != "" {
		out += "-" + s
	}
	if s := slug(version); s != "" {
		out += "-" + s
	}
	return out + ".pdf"
}
