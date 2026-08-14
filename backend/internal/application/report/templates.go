// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package report is the reporting engine: it turns a configured request into a
// document, asynchronously, in the format and the language the requester asked
// for — and keeps that document's version, hash and editorial state.
package report

import (
	"fmt"

	"github.com/opendefender/openrisk/internal/domain"
)

// Template is a versioned layout for one report type.
//
// Versioned because a report is a record. A document approved in March under
// template v1 has to keep saying it was produced by v1: if the layout changes in
// June and the version is not recorded, nobody can explain later why two reports
// of "the same" type look different, or reproduce the older one.
//
// The version is bumped by hand when the layout changes in a way a reader would
// notice — a new section, a column removed, a different ordering. Cosmetic
// changes do not need it; a reader comparing two documents does.
type Template struct {
	Key     string
	Version string
	Type    domain.ReportType
	// Formats the template can render into. Not every report makes sense in
	// every container: a board narrative as a spreadsheet is a table of one cell.
	Formats []domain.ReportFormat
	// Title per language. The document's language, not the interface's.
	Titles map[domain.ReportLocale]string
	// Description is what the configurator shows so someone picking a report
	// knows what they will get before they generate it.
	Descriptions map[domain.ReportLocale]string
}

// Supports reports whether the template renders that format.
func (t Template) Supports(f domain.ReportFormat) bool {
	for _, x := range t.Formats {
		if x == f {
			return true
		}
	}
	return false
}

// Title returns the document title in the requested language, falling back to
// French (the product's primary market) rather than to an empty heading.
func (t Template) Title(locale domain.ReportLocale) string {
	if s, ok := t.Titles[locale]; ok && s != "" {
		return s
	}
	return t.Titles[domain.ReportLocaleFR]
}

// Description returns the configurator blurb in the requested language.
func (t Template) Description(locale domain.ReportLocale) string {
	if s, ok := t.Descriptions[locale]; ok && s != "" {
		return s
	}
	return t.Descriptions[domain.ReportLocaleFR]
}

// all is the registry: one template per report type, for now.
//
// The shape allows several per type later (a short board pack and a long one)
// without changing the job model — the request already carries template_key.
var all = []Template{
	{
		Key: "executive-summary", Version: "1.0", Type: domain.ReportTypeExecutiveSummary,
		Formats: []domain.ReportFormat{domain.ReportFormatPDF, domain.ReportFormatDOCX},
		Titles: map[domain.ReportLocale]string{
			domain.ReportLocaleFR: "Synthèse exécutive",
			domain.ReportLocaleEN: "Executive Summary",
		},
		Descriptions: map[domain.ReportLocale]string{
			domain.ReportLocaleFR: "Posture globale en une page : score cyber, exposition financière, risques majeurs, conformité et incidents.",
			domain.ReportLocaleEN: "One-page posture: cyber score, financial exposure, top risks, compliance and incidents.",
		},
	},
	{
		Key: "compliance-framework", Version: "1.1", Type: domain.ReportTypeComplianceByFramework,
		Formats: []domain.ReportFormat{domain.ReportFormatPDF, domain.ReportFormatDOCX, domain.ReportFormatXLSX},
		Titles: map[domain.ReportLocale]string{
			domain.ReportLocaleFR: "Rapport de conformité",
			domain.ReportLocaleEN: "Compliance Report",
		},
		Descriptions: map[domain.ReportLocale]string{
			domain.ReportLocaleFR: "État de conformité d'un référentiel, contrôle par contrôle, avec les preuves rattachées et leur validité.",
			domain.ReportLocaleEN: "A framework's compliance state, control by control, with attached evidence and its validity.",
		},
	},
	{
		Key: "board-pack", Version: "1.0", Type: domain.ReportTypeBoard,
		Formats: []domain.ReportFormat{domain.ReportFormatPDF, domain.ReportFormatDOCX},
		Titles: map[domain.ReportLocale]string{
			domain.ReportLocaleFR: "Rapport au conseil d'administration",
			domain.ReportLocaleEN: "Board of Directors Report",
		},
		Descriptions: map[domain.ReportLocale]string{
			domain.ReportLocaleFR: "Lecture non technique pour le conseil : exposition en FCFA, décisions attendues, tendance.",
			domain.ReportLocaleEN: "Non-technical reading for the board: exposure in XAF, decisions required, trend.",
		},
	},
	{
		Key: "risk-register", Version: "1.0", Type: domain.ReportTypeRiskRegister,
		Formats: []domain.ReportFormat{domain.ReportFormatPDF, domain.ReportFormatXLSX, domain.ReportFormatDOCX},
		Titles: map[domain.ReportLocale]string{
			domain.ReportLocaleFR: "Registre des risques",
			domain.ReportLocaleEN: "Risk Register",
		},
		Descriptions: map[domain.ReportLocale]string{
			domain.ReportLocaleFR: "Le registre complet : score, criticité, responsable, état du cycle de vie et exposition financière.",
			domain.ReportLocaleEN: "The full register: score, criticality, owner, lifecycle state and financial exposure.",
		},
	},
	{
		Key: "incident-report", Version: "1.0", Type: domain.ReportTypeIncident,
		Formats: []domain.ReportFormat{domain.ReportFormatPDF, domain.ReportFormatXLSX, domain.ReportFormatDOCX},
		Titles: map[domain.ReportLocale]string{
			domain.ReportLocaleFR: "Rapport d'incidents",
			domain.ReportLocaleEN: "Incident Report",
		},
		Descriptions: map[domain.ReportLocale]string{
			domain.ReportLocaleFR: "Incidents de la période : gravité, statut, délai de résolution et suites données.",
			domain.ReportLocaleEN: "Incidents in the period: severity, status, time to resolve and follow-up.",
		},
	},
	{
		Key: "audit-report", Version: "1.0", Type: domain.ReportTypeAudit,
		Formats: []domain.ReportFormat{domain.ReportFormatPDF, domain.ReportFormatDOCX, domain.ReportFormatXLSX},
		Titles: map[domain.ReportLocale]string{
			domain.ReportLocaleFR: "Rapport d'audit",
			domain.ReportLocaleEN: "Audit Report",
		},
		Descriptions: map[domain.ReportLocale]string{
			domain.ReportLocaleFR: "Audit de conformité : périmètre, constats, écarts relevés et plans de remédiation ouverts.",
			domain.ReportLocaleEN: "Compliance audit: scope, findings, gaps raised and open remediation plans.",
		},
	},
}

// Templates returns the registry (the configurator's catalogue).
func Templates() []Template {
	out := make([]Template, len(all))
	copy(out, all)
	return out
}

// TemplateFor returns the template for a report type.
func TemplateFor(t domain.ReportType) (Template, error) {
	for _, tpl := range all {
		if tpl.Type == t {
			return tpl, nil
		}
	}
	return Template{}, domain.NewValidationError(fmt.Sprintf("no template for report type %q", t))
}

// TemplateByKey returns a template by its key, for a request that pins one.
func TemplateByKey(key string) (Template, bool) {
	for _, tpl := range all {
		if tpl.Key == key {
			return tpl, true
		}
	}
	return Template{}, false
}
