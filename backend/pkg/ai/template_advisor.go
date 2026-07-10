// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package ai

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// TemplateAdvisor writes a correct, board-appropriate narrative purely from the
// numbers, with no network call and no API key. It is deterministic: the same
// posture always yields the same prose. It is both the zero-config default and
// the fallback the use case drops to when the Claude call is unavailable.
type TemplateAdvisor struct{}

// NewTemplateAdvisor returns a ready-to-use deterministic advisor.
func NewTemplateAdvisor() *TemplateAdvisor { return &TemplateAdvisor{} }

func (a *TemplateAdvisor) Name() string { return "template" }

// GenerateBoardNarrative never returns an error — it is the safety net.
func (a *TemplateAdvisor) GenerateBoardNarrative(_ context.Context, p BoardPosture) (BoardNarrative, error) {
	if p.Locale.Normalize() == LocaleEN {
		return a.english(p), nil
	}
	return a.french(p), nil
}

// grade classifies the overall posture into a short qualitative label used to
// colour the tone of the summary. Thresholds mirror the compliance report's
// headline colour bands so the documents agree.
func (a *TemplateAdvisor) grade(p BoardPosture) (fr, en string) {
	switch {
	case p.RisksCritical == 0 && p.OverallCompliancePercent >= 80:
		return "maîtrisée", "well under control"
	case p.RisksCritical <= 2 && p.OverallCompliancePercent >= 50:
		return "globalement satisfaisante mais perfectible", "broadly acceptable but improvable"
	default:
		return "à renforcer en priorité", "requiring priority attention"
	}
}

func (a *TemplateAdvisor) french(p BoardPosture) BoardNarrative {
	org := p.OrganizationName
	if org == "" {
		org = "l'organisation"
	}
	gradeFR, _ := a.grade(p)

	exec := fmt.Sprintf(
		"Ce rapport présente au conseil d'administration la posture de risque et de conformité de %s pour la période de %s. "+
			"La situation d'ensemble est %s. À ce jour, %d risque(s) sont suivis, dont %d de niveau critique et %d de niveau élevé. "+
			"Le niveau de conformité réglementaire consolidé atteint %.0f %%, et l'exposition financière annuelle estimée s'établit à %s.",
		org, p.PeriodLabel, gradeFR, p.RisksTotal, p.RisksCritical, p.RisksHigh, p.OverallCompliancePercent, FormatFCFA(p.FinancialExposureFCFA))

	risk := fmt.Sprintf(
		"Le registre des risques comprend %d risque(s) actif(s) : %d critique(s), %d élevé(s), %d moyen(s) et %d faible(s). ",
		p.RisksTotal, p.RisksCritical, p.RisksHigh, p.RisksMedium, p.RisksLow)
	if p.RisksCritical > 0 {
		risk += fmt.Sprintf("Les %d risque(s) critique(s) appellent une attention et un plan de traitement immédiats, car ils concentrent l'essentiel de l'exposition. ", p.RisksCritical)
	} else {
		risk += "Aucun risque de niveau critique n'est ouvert, ce qui traduit une bonne maîtrise des menaces les plus graves. "
	}

	comp := fmt.Sprintf(
		"La conformité réglementaire consolidée se situe à %.0f %% des contrôles applicables mis en œuvre. ",
		p.OverallCompliancePercent)
	comp += a.frameworkSentenceFR(p.Frameworks)

	fin := fmt.Sprintf(
		"L'exposition financière annuelle estimée liée aux risques ouverts est de %s. "+
			"Ce montant est une estimation d'ordre de grandeur, obtenue en pondérant chaque risque par une valeur de référence selon sa criticité ; "+
			"il vise à donner au conseil une échelle de l'enjeu, non un chiffre comptable.",
		FormatFCFA(p.FinancialExposureFCFA))

	return BoardNarrative{
		ExecutiveSummary:     exec,
		RiskCommentary:       risk,
		ComplianceCommentary: comp,
		FinancialCommentary:  fin,
		Recommendations:      a.recommendationsFR(p),
	}
}

func (a *TemplateAdvisor) english(p BoardPosture) BoardNarrative {
	org := p.OrganizationName
	if org == "" {
		org = "the organization"
	}
	_, gradeEN := a.grade(p)

	exec := fmt.Sprintf(
		"This report presents to the board of directors the risk and compliance posture of %s for the %s period. "+
			"The overall situation is %s. As of today, %d risk(s) are tracked, including %d critical and %d high. "+
			"Consolidated regulatory compliance stands at %.0f %%, and the estimated annual financial exposure is %s.",
		org, p.PeriodLabel, gradeEN, p.RisksTotal, p.RisksCritical, p.RisksHigh, p.OverallCompliancePercent, FormatFCFA(p.FinancialExposureFCFA))

	risk := fmt.Sprintf(
		"The risk register holds %d active risk(s): %d critical, %d high, %d medium and %d low. ",
		p.RisksTotal, p.RisksCritical, p.RisksHigh, p.RisksMedium, p.RisksLow)
	if p.RisksCritical > 0 {
		risk += fmt.Sprintf("The %d critical risk(s) call for immediate attention and a treatment plan, as they concentrate most of the exposure. ", p.RisksCritical)
	} else {
		risk += "No critical risk is open, reflecting good control over the most severe threats. "
	}

	comp := fmt.Sprintf(
		"Consolidated regulatory compliance stands at %.0f %% of applicable controls implemented. ",
		p.OverallCompliancePercent)
	comp += a.frameworkSentenceEN(p.Frameworks)

	fin := fmt.Sprintf(
		"The estimated annual financial exposure from open risks is %s. "+
			"This is an order-of-magnitude estimate, obtained by weighting each risk by a reference value based on its criticality; "+
			"it is meant to give the board a sense of scale, not an accounting figure.",
		FormatFCFA(p.FinancialExposureFCFA))

	return BoardNarrative{
		ExecutiveSummary:     exec,
		RiskCommentary:       risk,
		ComplianceCommentary: comp,
		FinancialCommentary:  fin,
		Recommendations:      a.recommendationsEN(p),
	}
}

// weakestFrameworks returns up to n frameworks with the lowest completion,
// deterministically ordered (by percent asc, then name).
func weakestFrameworks(fw []FrameworkPosture, n int) []FrameworkPosture {
	sorted := make([]FrameworkPosture, len(fw))
	copy(sorted, fw)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].PercentComplete != sorted[j].PercentComplete {
			return sorted[i].PercentComplete < sorted[j].PercentComplete
		}
		return sorted[i].Name < sorted[j].Name
	})
	if len(sorted) > n {
		sorted = sorted[:n]
	}
	return sorted
}

func (a *TemplateAdvisor) frameworkSentenceFR(fw []FrameworkPosture) string {
	if len(fw) == 0 {
		return "Aucun référentiel de conformité n'est encore suivi ; l'ajout d'un référentiel (ISO 27001, BCEAO, COBAC…) est recommandé pour structurer la démarche."
	}
	var parts []string
	for _, f := range weakestFrameworks(fw, 3) {
		parts = append(parts, fmt.Sprintf("%s (%.0f %%)", f.Name, f.PercentComplete))
	}
	return "Les référentiels les moins avancés sont : " + strings.Join(parts, ", ") + "."
}

func (a *TemplateAdvisor) frameworkSentenceEN(fw []FrameworkPosture) string {
	if len(fw) == 0 {
		return "No compliance framework is tracked yet; adding one (ISO 27001, BCEAO, COBAC…) is recommended to structure the effort."
	}
	var parts []string
	for _, f := range weakestFrameworks(fw, 3) {
		parts = append(parts, fmt.Sprintf("%s (%.0f%%)", f.Name, f.PercentComplete))
	}
	return "The least advanced frameworks are: " + strings.Join(parts, ", ") + "."
}

func (a *TemplateAdvisor) recommendationsFR(p BoardPosture) []string {
	var recs []string
	if p.RisksCritical > 0 {
		recs = append(recs, fmt.Sprintf("Traiter en priorité les %d risque(s) critique(s) sous 30 jours (plan d'action, propriétaire, échéance).", p.RisksCritical))
	}
	if p.OverallCompliancePercent < 80 {
		recs = append(recs, "Renforcer la mise en œuvre des contrôles de conformité pour atteindre un socle d'au moins 80 %.")
	}
	if weak := weakestFrameworks(p.Frameworks, 1); len(weak) > 0 && weak[0].PercentComplete < 60 {
		recs = append(recs, fmt.Sprintf("Concentrer l'effort sur le référentiel « %s » (%.0f %%), le moins avancé.", weak[0].Name, weak[0].PercentComplete))
	}
	recs = append(recs, "Réviser ce rapport au prochain conseil et suivre l'évolution de l'exposition financière trimestre après trimestre.")
	return recs
}

func (a *TemplateAdvisor) recommendationsEN(p BoardPosture) []string {
	var recs []string
	if p.RisksCritical > 0 {
		recs = append(recs, fmt.Sprintf("Treat the %d critical risk(s) as top priority within 30 days (action plan, owner, deadline).", p.RisksCritical))
	}
	if p.OverallCompliancePercent < 80 {
		recs = append(recs, "Strengthen compliance control implementation to reach a baseline of at least 80%.")
	}
	if weak := weakestFrameworks(p.Frameworks, 1); len(weak) > 0 && weak[0].PercentComplete < 60 {
		recs = append(recs, fmt.Sprintf("Focus effort on the \"%s\" framework (%.0f%%), the least advanced.", weak[0].Name, weak[0].PercentComplete))
	}
	recs = append(recs, "Review this report at the next board meeting and track financial exposure quarter over quarter.")
	return recs
}
