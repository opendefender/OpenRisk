// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package report

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

// Sources are the narrow ports the builders read through. Every one is optional
// and nil-safe: a deployment without the incident module still generates every
// other report, and an incident report on such a deployment says so rather than
// failing.
type Sources struct {
	Compliance ComplianceSource
	Evidence   EvidenceSource
	Risks      RiskSource
	Incidents  IncidentSource
	Audits     AuditSource
	Org        OrgSource
}

// ComplianceSource reads the control register.
type ComplianceSource interface {
	GetFrameworkByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.ComplianceFramework, error)
	ListFrameworks(ctx context.Context, tenantID uuid.UUID) ([]domain.ComplianceFramework, error)
	ListControlsByFramework(ctx context.Context, tenantID, frameworkID uuid.UUID) ([]domain.ComplianceControl, error)
	CountEvidencesByFramework(ctx context.Context, tenantID, frameworkID uuid.UUID) (map[uuid.UUID]int, error)
}

// EvidenceSource reads the evidence library.
type EvidenceSource interface {
	ListByControl(ctx context.Context, tenantID, controlID uuid.UUID) ([]domain.Evidence, error)
}

// RiskSource reads the risk register.
type RiskSource interface {
	ListRisksForFinancial(ctx context.Context, tenantID uuid.UUID) ([]domain.Risk, error)
}

// IncidentSource reads incidents in a period.
type IncidentSource interface {
	ListIncidentsForReport(ctx context.Context, tenantID uuid.UUID, from, to time.Time) ([]domain.Incident, error)
}

// AuditSource reads compliance audits.
type AuditSource interface {
	GetAudit(ctx context.Context, tenantID, id uuid.UUID) (*domain.ComplianceAudit, error)
	ListAudits(ctx context.Context, tenantID uuid.UUID) ([]domain.ComplianceAudit, error)
	ListRemediations(ctx context.Context, tenantID uuid.UUID) ([]domain.RemediationPlan, error)
}

// OrgSource resolves the organisation's identity for the cover.
type OrgSource interface {
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Organization, error)
}

// Request is a configured report, as the builders see it.
type Request struct {
	TenantID uuid.UUID
	Type     domain.ReportType
	Locale   domain.ReportLocale
	Template Template
	// Period bounds the data. Zero values mean "everything".
	From, To time.Time
	// Scope narrows to one framework / audit, depending on the type.
	FrameworkID uuid.UUID
	AuditID     uuid.UUID
	// OrgName is resolved before the build so the builders stay free of lookups
	// they would all repeat.
	OrgName string
	// Recipients is recorded on the document so it says who it was produced for.
	Recipients []string
}

// Progress reports how far a render has got. The worker passes one in so the
// client sees real movement instead of an indeterminate spinner.
type Progress func(percent int, step string)

// Build produces the content for a request. One entry point, dispatching by
// type, so the worker stays free of report-specific knowledge.
func Build(ctx context.Context, s Sources, req Request, progress Progress) (Content, error) {
	if progress == nil {
		progress = func(int, string) {}
	}
	switch req.Type {
	case domain.ReportTypeComplianceByFramework:
		return buildCompliance(ctx, s, req, progress)
	case domain.ReportTypeRiskRegister:
		return buildRiskRegister(ctx, s, req, progress)
	case domain.ReportTypeIncident:
		return buildIncidents(ctx, s, req, progress)
	case domain.ReportTypeAudit:
		return buildAudit(ctx, s, req, progress)
	case domain.ReportTypeExecutiveSummary:
		return buildExecutive(ctx, s, req, progress)
	case domain.ReportTypeBoard:
		return buildBoard(ctx, s, req, progress)
	default:
		return Content{}, domain.NewValidationError(fmt.Sprintf("no builder for report type %q", req.Type))
	}
}

// baseContent fills the identity block every report carries.
func baseContent(req Request) Content {
	l := req.Locale
	c := Content{
		Title: req.Template.Title(l),
	}

	meta := []KeyValue{
		{Label: Localise(l, "Organisation", "Organisation"), Value: req.OrgName},
		{Label: Localise(l, "Généré le", "Generated on"), Value: FormatDate(l, time.Now())},
		{Label: Localise(l, "Langue du document", "Document language"), Value: strings.ToUpper(string(l))},
		{Label: Localise(l, "Modèle", "Template"),
			Value: fmt.Sprintf("%s v%s", req.Template.Key, req.Template.Version)},
	}
	if !req.From.IsZero() || !req.To.IsZero() {
		meta = append(meta, KeyValue{
			Label: Localise(l, "Période", "Period"),
			Value: periodLabel(l, req.From, req.To),
		})
	}
	if len(req.Recipients) > 0 {
		meta = append(meta, KeyValue{
			Label: Localise(l, "Destinataires", "Recipients"),
			Value: strings.Join(req.Recipients, ", "),
		})
	}
	c.Meta = meta
	return c
}

func periodLabel(l domain.ReportLocale, from, to time.Time) string {
	switch {
	case from.IsZero() && to.IsZero():
		return Localise(l, "Depuis l'origine", "All time")
	case from.IsZero():
		return Localise(l, "Jusqu'au ", "Until ") + FormatDate(l, to)
	case to.IsZero():
		return Localise(l, "Depuis le ", "Since ") + FormatDate(l, from)
	default:
		return fmt.Sprintf("%s — %s", FormatDate(l, from), FormatDate(l, to))
	}
}

// ---------------------------------------------------------------------------
// Compliance by framework
// ---------------------------------------------------------------------------

func buildCompliance(ctx context.Context, s Sources, req Request, progress Progress) (Content, error) {
	l := req.Locale
	c := baseContent(req)

	if s.Compliance == nil {
		return c, domain.NewValidationError("the compliance register is not available on this deployment")
	}
	if req.FrameworkID == uuid.Nil {
		return c, domain.NewValidationError(Localise(l,
			"choisissez un référentiel : un rapport de conformité porte sur un référentiel",
			"pick a framework: a compliance report is about one framework"))
	}

	progress(20, "loading framework")
	fw, err := s.Compliance.GetFrameworkByID(ctx, req.FrameworkID, req.TenantID)
	if err != nil {
		return c, err
	}
	if fw == nil {
		return c, domain.NewNotFoundError("framework", req.FrameworkID)
	}

	progress(40, "loading controls")
	controls, err := s.Compliance.ListControlsByFramework(ctx, req.TenantID, fw.ID)
	if err != nil {
		return c, err
	}
	evCounts, err := s.Compliance.CountEvidencesByFramework(ctx, req.TenantID, fw.ID)
	if err != nil {
		return c, err
	}

	c.Title = fmt.Sprintf("%s — %s", req.Template.Title(l), fw.Name)
	c.Subtitle = strings.TrimSpace(fw.Name + " " + fw.Version)

	var implemented, inProgress, notApplicable, notImplemented, evidenced int
	rows := make([][]string, 0, len(controls))

	progress(60, "assembling")
	for _, ctrl := range controls {
		switch ctrl.Status {
		case domain.ControlStatusImplemented:
			implemented++
		case domain.ControlStatusInProgress:
			inProgress++
		case domain.ControlStatusNotApplicable:
			notApplicable++
		default:
			notImplemented++
		}
		n := evCounts[ctrl.ID]
		if n > 0 {
			evidenced++
		}
		rows = append(rows, []string{
			ctrl.ReferenceCode,
			ctrl.Name,
			statusLabel(l, string(ctrl.Status)),
			fmt.Sprintf("%d", n),
			ctrl.SourceReference,
		})
	}

	applicable := len(controls) - notApplicable
	coverage := 0.0
	if applicable > 0 {
		coverage = float64(implemented) / float64(applicable) * 100
	}

	c.Summary = []KeyValue{
		{Label: Localise(l, "Contrôles", "Controls"), Value: fmt.Sprintf("%d", len(controls))},
		{Label: Localise(l, "Conformité", "Compliance"), Value: Percent(coverage),
			Status: coverageStatus(coverage)},
		{Label: Localise(l, "Contrôles avec preuve valide", "Controls with valid evidence"),
			Value: fmt.Sprintf("%d / %d", evidenced, applicable)},
	}

	// The gap between "implemented" and "evidenced" is the sentence an auditor
	// starts from, so the report states it rather than leaving it to be inferred
	// from two numbers on different pages.
	body := []string{}
	if implemented > evidenced {
		body = append(body, Localise(l,
			fmt.Sprintf("%d contrôle(s) déclarés implémentés ne sont pas justifiés par une preuve valide à ce jour.",
				implemented-evidenced),
			fmt.Sprintf("%d control(s) declared implemented are not substantiated by valid evidence today.",
				implemented-evidenced)))
	}

	c.Sections = []Section{
		{
			Heading: Localise(l, "Répartition", "Breakdown"),
			Body: append(body,
				Localise(l,
					fmt.Sprintf("Implémentés : %d · En cours : %d · Non implémentés : %d · Non applicables : %d",
						implemented, inProgress, notImplemented, notApplicable),
					fmt.Sprintf("Implemented: %d · In progress: %d · Not implemented: %d · Not applicable: %d",
						implemented, inProgress, notImplemented, notApplicable))),
		},
		{
			Heading: Localise(l, "Contrôles", "Controls"),
			Table: &Table{
				Columns: []string{
					Localise(l, "Code", "Code"),
					Localise(l, "Contrôle", "Control"),
					Localise(l, "Statut", "Status"),
					Localise(l, "Preuves", "Evidence"),
					Localise(l, "Source", "Source"),
				},
				Rows:         rows,
				StatusColumn: 2,
			},
			Empty: Localise(l, "Ce référentiel ne contient aucun contrôle.", "This framework holds no controls."),
		},
	}
	progress(80, "rendering")
	return c, nil
}

// ---------------------------------------------------------------------------
// Risk register
// ---------------------------------------------------------------------------

func buildRiskRegister(ctx context.Context, s Sources, req Request, progress Progress) (Content, error) {
	l := req.Locale
	c := baseContent(req)
	c.Subtitle = Localise(l, "Registre complet des risques", "Full risk register")

	if s.Risks == nil {
		return c, domain.NewValidationError("the risk register is not available on this deployment")
	}

	progress(30, "loading risks")
	risks, err := s.Risks.ListRisksForFinancial(ctx, req.TenantID)
	if err != nil {
		return c, err
	}

	// Newest first would bury the important ones; the register is read worst
	// first.
	sort.Slice(risks, func(i, j int) bool { return risks[i].Score > risks[j].Score })

	var critical, high int
	rows := make([][]string, 0, len(risks))
	for _, r := range risks {
		crit := strings.ToLower(string(r.Criticality))
		switch crit {
		case "critical":
			critical++
		case "high":
			high++
		}
		rows = append(rows, []string{
			r.Name,
			crit,
			fmt.Sprintf("%.2f", r.Score),
			string(r.LifecycleState),
			formatXAF(r.ALEXAF),
		})
	}

	c.Summary = []KeyValue{
		{Label: Localise(l, "Risques", "Risks"), Value: fmt.Sprintf("%d", len(risks))},
		{Label: Localise(l, "Critiques", "Critical"), Value: fmt.Sprintf("%d", critical), Status: "critical"},
		{Label: Localise(l, "Élevés", "High"), Value: fmt.Sprintf("%d", high), Status: "high"},
	}
	c.Sections = []Section{{
		Heading: Localise(l, "Registre", "Register"),
		Table: &Table{
			Columns: []string{
				Localise(l, "Risque", "Risk"),
				Localise(l, "Criticité", "Criticality"),
				Localise(l, "Score", "Score"),
				Localise(l, "Cycle de vie", "Lifecycle"),
				Localise(l, "Perte annuelle attendue", "Annual loss expectancy"),
			},
			Rows:         rows,
			StatusColumn: 1,
		},
		Empty: Localise(l, "Aucun risque enregistré.", "No risk recorded."),
	}}
	progress(80, "rendering")
	return c, nil
}

// ---------------------------------------------------------------------------
// Incidents
// ---------------------------------------------------------------------------

func buildIncidents(ctx context.Context, s Sources, req Request, progress Progress) (Content, error) {
	l := req.Locale
	c := baseContent(req)
	c.Subtitle = Localise(l, "Incidents de sécurité", "Security incidents")

	if s.Incidents == nil {
		return c, domain.NewValidationError("the incident register is not available on this deployment")
	}

	progress(30, "loading incidents")
	incidents, err := s.Incidents.ListIncidentsForReport(ctx, req.TenantID, req.From, req.To)
	if err != nil {
		return c, err
	}

	var open, resolved, criticalCount int
	rows := make([][]string, 0, len(incidents))
	for _, i := range incidents {
		sev := strings.ToLower(i.Severity)
		if sev == "critical" {
			criticalCount++
		}
		status := strings.ToLower(i.Status)
		if status == "resolved" || status == "closed" {
			resolved++
		} else {
			open++
		}
		rows = append(rows, []string{
			fmt.Sprintf("INC-%d", i.ID),
			i.Title,
			sev,
			status,
			i.CreatedAt.Format("2006-01-02"),
		})
	}

	c.Summary = []KeyValue{
		{Label: Localise(l, "Incidents", "Incidents"), Value: fmt.Sprintf("%d", len(incidents))},
		{Label: Localise(l, "Ouverts", "Open"), Value: fmt.Sprintf("%d", open),
			Status: openStatus(open)},
		{Label: Localise(l, "Critiques", "Critical"), Value: fmt.Sprintf("%d", criticalCount),
			Status: openStatus(criticalCount)},
	}
	c.Sections = []Section{{
		Heading: Localise(l, "Incidents", "Incidents"),
		Table: &Table{
			Columns: []string{
				Localise(l, "Référence", "Reference"),
				Localise(l, "Titre", "Title"),
				Localise(l, "Gravité", "Severity"),
				Localise(l, "Statut", "Status"),
				Localise(l, "Déclaré le", "Reported"),
			},
			Rows:         rows,
			StatusColumn: 2,
		},
		// Saying it beats an empty page: "no incident in the period" is a finding.
		Empty: Localise(l, "Aucun incident sur la période.", "No incident in the period."),
	}}
	progress(80, "rendering")
	return c, nil
}

// ---------------------------------------------------------------------------
// Audit
// ---------------------------------------------------------------------------

func buildAudit(ctx context.Context, s Sources, req Request, progress Progress) (Content, error) {
	l := req.Locale
	c := baseContent(req)

	if s.Audits == nil {
		return c, domain.NewValidationError("the audit module is not available on this deployment")
	}

	progress(30, "loading audits")
	var audits []domain.ComplianceAudit
	if req.AuditID != uuid.Nil {
		a, err := s.Audits.GetAudit(ctx, req.TenantID, req.AuditID)
		if err != nil {
			return c, err
		}
		if a == nil {
			return c, domain.NewNotFoundError("audit", req.AuditID)
		}
		audits = []domain.ComplianceAudit{*a}
		c.Subtitle = a.Title
	} else {
		all, err := s.Audits.ListAudits(ctx, req.TenantID)
		if err != nil {
			return c, err
		}
		audits = all
		c.Subtitle = Localise(l, "Tous les audits", "All audits")
	}

	progress(55, "loading remediation plans")
	remediations, err := s.Audits.ListRemediations(ctx, req.TenantID)
	if err != nil {
		remediations = nil // a missing remediation list degrades its section, not the report
	}

	auditRows := make([][]string, 0, len(audits))
	completed := 0
	for _, a := range audits {
		if strings.EqualFold(string(a.Status), "completed") {
			completed++
		}
		auditRows = append(auditRows, []string{
			a.Title,
			string(a.Type),
			string(a.Status),
			a.Auditor,
			formatOptionalDate(l, a.CompletedAt),
		})
	}

	openPlans := 0
	planRows := make([][]string, 0, len(remediations))
	for _, r := range remediations {
		status := strings.ToLower(string(r.Status))
		if status != "completed" && status != "cancelled" {
			openPlans++
		}
		planRows = append(planRows, []string{
			r.ControlCode,
			r.Title,
			strings.ToLower(string(r.Priority)),
			status,
			formatOptionalDate(l, r.DueDate),
		})
	}

	c.Summary = []KeyValue{
		{Label: Localise(l, "Audits", "Audits"), Value: fmt.Sprintf("%d", len(audits))},
		{Label: Localise(l, "Terminés", "Completed"), Value: fmt.Sprintf("%d", completed)},
		{Label: Localise(l, "Remédiations ouvertes", "Open remediations"),
			Value: fmt.Sprintf("%d", openPlans), Status: openStatus(openPlans)},
	}
	c.Sections = []Section{
		{
			Heading: Localise(l, "Audits", "Audits"),
			Table: &Table{
				Columns: []string{
					Localise(l, "Intitulé", "Title"),
					Localise(l, "Type", "Type"),
					Localise(l, "Statut", "Status"),
					Localise(l, "Auditeur", "Auditor"),
					Localise(l, "Terminé le", "Completed"),
				},
				Rows: auditRows, StatusColumn: 2,
			},
			Empty: Localise(l, "Aucun audit planifié.", "No audit planned."),
		},
		{
			Heading: Localise(l, "Plans de remédiation", "Remediation plans"),
			Table: &Table{
				Columns: []string{
					Localise(l, "Contrôle", "Control"),
					Localise(l, "Plan", "Plan"),
					Localise(l, "Priorité", "Priority"),
					Localise(l, "Statut", "Status"),
					Localise(l, "Échéance", "Due"),
				},
				Rows: planRows, StatusColumn: 2,
			},
			Empty: Localise(l, "Aucun plan de remédiation ouvert.", "No open remediation plan."),
		},
	}
	progress(80, "rendering")
	return c, nil
}

// ---------------------------------------------------------------------------
// Executive summary and board pack
// ---------------------------------------------------------------------------

func buildExecutive(ctx context.Context, s Sources, req Request, progress Progress) (Content, error) {
	l := req.Locale
	c := baseContent(req)
	c.Subtitle = Localise(l, "Posture de sécurité et de conformité", "Security and compliance posture")

	progress(25, "compliance")
	frameworks, controls, implemented, applicable := complianceTotals(ctx, s, req.TenantID)

	progress(50, "risks")
	riskCount, criticalRisks, totalALE := riskTotals(ctx, s, req.TenantID)

	coverage := 0.0
	if applicable > 0 {
		coverage = float64(implemented) / float64(applicable) * 100
	}

	c.Summary = []KeyValue{
		{Label: Localise(l, "Conformité globale", "Overall compliance"), Value: Percent(coverage),
			Status: coverageStatus(coverage)},
		{Label: Localise(l, "Risques critiques", "Critical risks"), Value: fmt.Sprintf("%d", criticalRisks),
			Status: openStatus(criticalRisks)},
		{Label: Localise(l, "Exposition annuelle", "Annual exposure"), Value: formatXAF(totalALE)},
	}

	c.Sections = []Section{{
		Heading: Localise(l, "Lecture d'ensemble", "At a glance"),
		Body: []string{
			Localise(l,
				fmt.Sprintf("%d référentiel(s) suivis, %d contrôles au total, dont %d implémentés sur %d applicables.",
					frameworks, controls, implemented, applicable),
				fmt.Sprintf("%d framework(s) tracked, %d controls in total, %d implemented of %d applicable.",
					frameworks, controls, implemented, applicable)),
			Localise(l,
				fmt.Sprintf("%d risque(s) au registre, dont %d critiques ; exposition annuelle attendue %s.",
					riskCount, criticalRisks, formatXAF(totalALE)),
				fmt.Sprintf("%d risk(s) on the register, %d of them critical; expected annual exposure %s.",
					riskCount, criticalRisks, formatXAF(totalALE))),
		},
	}}
	progress(80, "rendering")
	return c, nil
}

func buildBoard(ctx context.Context, s Sources, req Request, progress Progress) (Content, error) {
	l := req.Locale
	c, err := buildExecutive(ctx, s, req, progress)
	if err != nil {
		return c, err
	}
	c.Title = req.Template.Title(l)
	c.Subtitle = Localise(l, "Document destiné au conseil d'administration",
		"Document for the board of directors")

	// The board pack is the executive summary with the technical register left
	// out and the decision framing added. Same figures, different reading: a
	// board is asked to decide, not to review controls.
	c.Sections = append(c.Sections, Section{
		Heading: Localise(l, "Ce qui est demandé au conseil", "What the board is asked"),
		Body: []string{
			Localise(l,
				"Prendre acte de la posture ci-dessus, et arbitrer les moyens à allouer aux risques critiques restants.",
				"Note the posture above, and decide the resources to allocate to the remaining critical risks."),
		},
	})
	return c, nil
}

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

func complianceTotals(ctx context.Context, s Sources, tenantID uuid.UUID) (frameworks, controls, implemented, applicable int) {
	if s.Compliance == nil {
		return
	}
	fws, err := s.Compliance.ListFrameworks(ctx, tenantID)
	if err != nil {
		return
	}
	frameworks = len(fws)
	for _, fw := range fws {
		cs, err := s.Compliance.ListControlsByFramework(ctx, tenantID, fw.ID)
		if err != nil {
			continue
		}
		for _, c := range cs {
			controls++
			if c.Status == domain.ControlStatusNotApplicable {
				continue
			}
			applicable++
			if c.Status == domain.ControlStatusImplemented {
				implemented++
			}
		}
	}
	return
}

func riskTotals(ctx context.Context, s Sources, tenantID uuid.UUID) (count, critical int, ale float64) {
	if s.Risks == nil {
		return
	}
	risks, err := s.Risks.ListRisksForFinancial(ctx, tenantID)
	if err != nil {
		return
	}
	count = len(risks)
	for _, r := range risks {
		if strings.EqualFold(string(r.Criticality), "critical") {
			critical++
		}
		ale += r.ALEXAF
	}
	return
}

func statusLabel(l domain.ReportLocale, status string) string {
	if l != domain.ReportLocaleEN {
		switch status {
		case "implemented":
			return "implemented"
		case "in_progress":
			return "in_progress"
		case "not_applicable":
			return "not_applicable"
		default:
			return "not_implemented"
		}
	}
	return status
}

// coverageStatus turns a percentage into a status word the print palette can
// tint. The thresholds match the ones the product already uses on screen, so a
// report does not contradict the dashboard it was generated from.
func coverageStatus(pct float64) string {
	switch {
	case pct >= 80:
		return "low"
	case pct >= 50:
		return "medium"
	default:
		return "critical"
	}
}

// openStatus tints a count of open work: none is good, some is worth noting.
func openStatus(n int) string {
	switch {
	case n == 0:
		return "low"
	case n <= 3:
		return "medium"
	default:
		return "critical"
	}
}

func formatXAF(v float64) string {
	if v <= 0 {
		return "0 FCFA"
	}
	switch {
	case v >= 1_000_000_000:
		return fmt.Sprintf("%.2f Md FCFA", v/1_000_000_000)
	case v >= 1_000_000:
		return fmt.Sprintf("%.1f M FCFA", v/1_000_000)
	default:
		return fmt.Sprintf("%.0f FCFA", v)
	}
}

func formatOptionalDate(l domain.ReportLocale, t *time.Time) string {
	if t == nil || t.IsZero() {
		return "—"
	}
	return FormatDate(l, *t)
}
