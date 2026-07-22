// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial
// This file is part of the OpenRisk Enterprise Edition and is NOT covered by the
// AGPL; it is licensed under the OpenRisk Commercial License (see LICENSE.commercial).

package ai

import (
	"context"
	"fmt"
	"strings"
)

// TemplateAssistant is the deterministic, no-network implementation of Assistant.
// It produces correct, useful (if less nuanced) results purely from the assembled
// context — no API key required. It is both the zero-config default and the
// fallback the application layer drops to whenever the Claude call fails, so the
// GRC assistant features always work. Every method is deterministic and never
// returns an error.
type TemplateAssistant struct{}

// NewTemplateAssistant returns a ready-to-use deterministic assistant.
func NewTemplateAssistant() *TemplateAssistant { return &TemplateAssistant{} }

func (a *TemplateAssistant) Name() string { return "template" }

func isEN(l Locale) bool { return l.Normalize() == LocaleEN }

func tr(l Locale, fr, en string) string {
	if isEN(l) {
		return en
	}
	return fr
}

// -----------------------------------------------------------------------------
// 1. Treatment plan
// -----------------------------------------------------------------------------

func (a *TemplateAssistant) SuggestTreatmentPlan(_ context.Context, in RiskContext) (TreatmentPlan, error) {
	crit := strings.ToLower(in.Criticality)
	strategy := "mitigate"
	if crit == "low" {
		strategy = "accept"
	}

	name := in.Name
	if name == "" {
		name = tr(in.Locale, "ce risque", "this risk")
	}
	summary := fmt.Sprintf(
		tr(in.Locale,
			"Le risque « %s » est de criticité %s (probabilité %.0f %%, impact %.1f/10, score %.2f). ",
			"Risk \"%s\" is of %s criticality (probability %.0f%%, impact %.1f/10, score %.2f). "),
		name, crit, in.Probability*100, in.Impact, in.Score)
	if in.AssetName != "" {
		summary += fmt.Sprintf(tr(in.Locale,
			"Il concerne l'actif « %s » (%s, criticité %s). ",
			"It affects asset \"%s\" (%s, %s criticality). "),
			in.AssetName, in.AssetType, in.AssetCriticality)
	}
	if in.ALEXAF > 0 {
		summary += fmt.Sprintf(tr(in.Locale,
			"L'exposition annuelle estimée est de %s. ",
			"Estimated annual loss exposure is %s. "),
			FormatFCFA(in.ALEXAF))
	}

	actions := []TreatmentPlanAction{}
	switch strategy {
	case "accept":
		actions = append(actions, TreatmentPlanAction{
			Title:       tr(in.Locale, "Documenter l'acceptation du risque", "Document risk acceptance"),
			Description: tr(in.Locale, "Faire valider l'acceptation par le propriétaire du risque et fixer une date de revue.", "Have the risk owner sign off on acceptance and set a review date."),
			Priority:    "low",
		})
	default:
		actions = append(actions,
			TreatmentPlanAction{
				Title:       tr(in.Locale, "Réduire la probabilité", "Reduce likelihood"),
				Description: tr(in.Locale, "Renforcer les contrôles préventifs (durcissement, MFA, segmentation, gestion des correctifs) sur l'actif concerné.", "Strengthen preventive controls (hardening, MFA, segmentation, patch management) on the affected asset."),
				Priority:    ternary(crit == "critical" || crit == "high", "high", "medium"),
			},
			TreatmentPlanAction{
				Title:       tr(in.Locale, "Réduire l'impact", "Reduce impact"),
				Description: tr(in.Locale, "Mettre en place des mesures de détection et de résilience (journalisation, sauvegardes, plan de réponse).", "Put detection and resilience measures in place (logging, backups, response plan)."),
				Priority:    "medium",
			},
			TreatmentPlanAction{
				Title:       tr(in.Locale, "Assigner et suivre", "Assign and track"),
				Description: tr(in.Locale, "Nommer un responsable, fixer une échéance et suivre le risque résiduel jusqu'à sa clôture.", "Assign an owner, set a deadline, and track residual risk to closure."),
				Priority:    "medium",
			},
		)
	}

	rationale := tr(in.Locale,
		"Stratégie déduite de la criticité et du score du risque, en cohérence avec le moteur de score P×I×criticité.",
		"Strategy derived from the risk's criticality and score, consistent with the P×I×criticality score engine.")

	return TreatmentPlan{
		Summary:             strings.TrimSpace(summary),
		RecommendedStrategy: strategy,
		Actions:             actions,
		Rationale:           rationale,
	}, nil
}

// -----------------------------------------------------------------------------
// 2. Emerging risk detection
// -----------------------------------------------------------------------------

func (a *TemplateAssistant) DetectEmergingRisks(_ context.Context, in IntelInput) (EmergingRisksResult, error) {
	// Deterministic keyword scan: surface a candidate risk per matched theme. This
	// is intentionally simple — the Claude path does the nuanced extraction; this
	// keeps the feature usable and honest with no key.
	type theme struct {
		keys     []string
		titleFR  string
		titleEN  string
		category string
		severity string
		prob     float64
		impact   float64
	}
	themes := []theme{
		{[]string{"ransomware", "rançongiciel", "extorsion"}, "Exposition au rançongiciel", "Ransomware exposure", "malware", "critical", 0.6, 9},
		{[]string{"phishing", "hameçonnage", "spear"}, "Campagne d'hameçonnage ciblée", "Targeted phishing campaign", "social-engineering", "high", 0.7, 7},
		{[]string{"cve-", "zero-day", "0-day", "vulnérab", "vulnerab", "exploit"}, "Vulnérabilité exploitée activement", "Actively exploited vulnerability", "vulnerability", "high", 0.6, 8},
		{[]string{"ddos", "déni de service", "denial of service"}, "Attaque par déni de service", "Denial-of-service attack", "availability", "medium", 0.5, 6},
		{[]string{"supply chain", "chaîne d'approvision", "third party", "fournisseur", "tiers"}, "Risque sur la chaîne d'approvisionnement", "Supply-chain risk", "third-party", "high", 0.5, 8},
		{[]string{"data breach", "fuite de données", "exfiltration", "leak"}, "Fuite / exfiltration de données", "Data breach / exfiltration", "data", "high", 0.5, 8},
		{[]string{"credential", "identifiant", "password", "mot de passe", "brute"}, "Compromission d'identifiants", "Credential compromise", "access", "high", 0.6, 7},
		{[]string{"insider", "interne malveillant", "menace interne"}, "Menace interne", "Insider threat", "insider", "medium", 0.3, 7},
	}
	lower := strings.ToLower(in.Text + " " + in.Context)
	known := map[string]bool{}
	for _, k := range in.KnownRisks {
		known[strings.ToLower(strings.TrimSpace(k))] = true
	}

	var risks []EmergingRisk
	for _, t := range themes {
		matched := false
		for _, k := range t.keys {
			if strings.Contains(lower, k) {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		title := tr(in.Locale, t.titleFR, t.titleEN)
		if known[strings.ToLower(title)] {
			continue
		}
		risks = append(risks, EmergingRisk{
			Title:                title,
			Description:          tr(in.Locale, "Risque identifié dans le texte fourni ("+in.Source+").", "Risk identified in the provided text ("+in.Source+")."),
			Category:             t.category,
			Severity:             t.severity,
			Rationale:            tr(in.Locale, "Correspondance de mots-clés dans la source analysée.", "Keyword match in the analysed source."),
			SuggestedProbability: t.prob,
			SuggestedImpact:      t.impact,
		})
	}

	summary := tr(in.Locale,
		fmt.Sprintf("%d risque(s) émergent(s) détecté(s) par analyse de mots-clés.", len(risks)),
		fmt.Sprintf("%d emerging risk(s) detected by keyword analysis.", len(risks)))
	if len(risks) == 0 {
		summary = tr(in.Locale, "Aucun risque émergent évident détecté dans le texte fourni.", "No obvious emerging risk detected in the provided text.")
	}
	return EmergingRisksResult{Summary: summary, Risks: risks}, nil
}

// -----------------------------------------------------------------------------
// 3. Natural-language assistant (RAG Q&A)
// -----------------------------------------------------------------------------

func (a *TemplateAssistant) Answer(_ context.Context, in AssistantQuery) (AssistantAnswer, error) {
	if len(in.Snippets) == 0 {
		return AssistantAnswer{
			Answer: tr(in.Locale,
				"Je n'ai trouvé aucun élément pertinent dans votre base GRC pour répondre à cette question. Précisez un risque, un contrôle ou une CVE, ou ajoutez les données correspondantes dans OpenRisk.",
				"I found no relevant item in your GRC knowledge base for this question. Try naming a specific risk, control or CVE, or add the corresponding data in OpenRisk."),
			Sources: nil,
		}, nil
	}

	var b strings.Builder
	b.WriteString(tr(in.Locale,
		"D'après votre base de connaissances GRC, voici les éléments pertinents :\n",
		"Based on your GRC knowledge base, here are the relevant items:\n"))
	var sources []string
	for _, s := range in.Snippets {
		b.WriteString(fmt.Sprintf("• [%s] %s — %s: %s\n", s.Kind, s.Ref, s.Title, s.Detail))
		ref := s.Ref
		if ref == "" {
			ref = s.Title
		}
		sources = append(sources, ref)
	}
	b.WriteString(tr(in.Locale,
		"\n(Réponse générée sans LLM — configurez ANTHROPIC_API_KEY pour une synthèse en langage naturel.)",
		"\n(Answer generated without an LLM — set ANTHROPIC_API_KEY for a natural-language synthesis.)"))

	return AssistantAnswer{Answer: b.String(), Sources: sources}, nil
}

// -----------------------------------------------------------------------------
// 4. Audit report generation
// -----------------------------------------------------------------------------

func (a *TemplateAssistant) SummarizeAudit(_ context.Context, in AuditContext) (AuditNarrative, error) {
	fw := in.FrameworkName
	if fw == "" {
		fw = tr(in.Locale, "l'ensemble du programme de conformité", "the whole compliance program")
	}
	exec := fmt.Sprintf(tr(in.Locale,
		"L'audit « %s » (%s) porte sur %s. Sur %d contrôle(s) évalué(s), %d sont mis en œuvre et %d présentent un écart, soit un taux de conformité de %.0f %%. ",
		"Audit \"%s\" (%s) covers %s. Of %d control(s) assessed, %d are implemented and %d show a gap, for a %.0f%% compliance rate. "),
		in.Title, in.Type, fw, in.TotalControls, in.Implemented, in.Gaps, in.PercentComplete)
	if in.OpenRemediations > 0 {
		exec += fmt.Sprintf(tr(in.Locale, "%d plan(s) de remédiation sont ouverts. ", "%d remediation plan(s) are open. "), in.OpenRemediations)
	}

	var findings strings.Builder
	if len(in.TopGaps) == 0 {
		findings.WriteString(tr(in.Locale, "Aucun écart notable n'a été relevé.", "No notable gap was identified."))
	} else {
		findings.WriteString(tr(in.Locale, "Écarts notables relevés :\n", "Notable gaps identified:\n"))
		for _, g := range in.TopGaps {
			findings.WriteString(fmt.Sprintf("• %s — %s (%s)\n", g.Code, g.Name, g.Status))
		}
	}

	recs := []string{}
	if in.Gaps > 0 {
		recs = append(recs, tr(in.Locale,
			fmt.Sprintf("Ouvrir un plan de remédiation pour chacun des %d écart(s) et l'assigner à un responsable.", in.Gaps),
			fmt.Sprintf("Open a remediation plan for each of the %d gap(s) and assign an owner.", in.Gaps)))
	}
	if in.PercentComplete < 80 {
		recs = append(recs, tr(in.Locale,
			"Prioriser les contrôles à fort impact pour atteindre un socle d'au moins 80 %.",
			"Prioritise high-impact controls to reach a baseline of at least 80%."))
	}
	recs = append(recs, tr(in.Locale,
		"Planifier un audit de suivi pour vérifier la clôture des écarts.",
		"Schedule a follow-up audit to verify gap closure."))

	conclusion := tr(in.Locale,
		fmt.Sprintf("La posture de conformité auditée s'établit à %.0f %%. La clôture des écarts identifiés permettra d'atteindre le niveau attendu.", in.PercentComplete),
		fmt.Sprintf("The audited compliance posture stands at %.0f%%. Closing the identified gaps will bring it to the expected level.", in.PercentComplete))

	return AuditNarrative{
		ExecutiveSummary: strings.TrimSpace(exec),
		Findings:         strings.TrimSpace(findings.String()),
		Recommendations:  recs,
		Conclusion:       conclusion,
	}, nil
}

// -----------------------------------------------------------------------------
// 5. Evidence document analysis
// -----------------------------------------------------------------------------

func (a *TemplateAssistant) AnalyzeEvidence(_ context.Context, in EvidenceContext) (EvidenceAssessment, error) {
	// Deterministic heuristic: match control keywords against the evidence
	// filename/description/excerpt. Without extracted content, confidence is low and
	// the verdict is "insufficient" (never a false "satisfies").
	haystack := strings.ToLower(in.EvidenceFilename + " " + in.EvidenceDescription + " " + in.EvidenceExcerpt)
	needle := strings.ToLower(in.ControlName + " " + in.ControlDescription)

	overlap := keywordOverlap(needle, haystack)
	hasContent := strings.TrimSpace(in.EvidenceExcerpt) != ""

	verdict := "insufficient"
	confidence := 0.3
	switch {
	case hasContent && overlap >= 3:
		verdict, confidence = "satisfies", 0.7
	case hasContent && overlap >= 1:
		verdict, confidence = "partial", 0.5
	case !hasContent && overlap >= 2:
		verdict, confidence = "partial", 0.35
	case overlap == 0:
		verdict, confidence = "insufficient", 0.25
	}

	rationale := tr(in.Locale,
		fmt.Sprintf("Analyse heuristique de la preuve « %s » face au contrôle %s. %d terme(s) du contrôle retrouvé(s) dans la preuve%s.",
			in.EvidenceFilename, in.ControlCode, overlap,
			ternary(hasContent, "", tr(in.Locale, " (contenu non extrait — confiance abaissée)", " (no content extracted — confidence lowered)"))),
		fmt.Sprintf("Heuristic analysis of evidence \"%s\" against control %s. %d control term(s) found in the evidence%s.",
			in.EvidenceFilename, in.ControlCode, overlap,
			ternary(hasContent, "", " (no content extracted — confidence lowered)")))

	var gaps, suggestions []string
	if verdict != "satisfies" {
		gaps = append(gaps, tr(in.Locale,
			"La preuve ne couvre pas clairement toute l'exigence du contrôle.",
			"The evidence does not clearly cover the full control requirement."))
		suggestions = append(suggestions, tr(in.Locale,
			"Fournir un document explicitement lié à l'exigence (politique signée, capture de configuration, journal daté).",
			"Provide a document explicitly tied to the requirement (signed policy, config screenshot, dated log)."))
	}
	if !hasContent {
		suggestions = append(suggestions, tr(in.Locale,
			"Le contenu du fichier n'a pas pu être analysé automatiquement ; une revue humaine reste recommandée.",
			"The file content could not be analysed automatically; a human review is still recommended."))
	}

	return EvidenceAssessment{
		Verdict:     verdict,
		Confidence:  confidence,
		Rationale:   rationale,
		Gaps:        gaps,
		Suggestions: suggestions,
	}, nil
}

// keywordOverlap counts distinct meaningful words (>3 chars) from needle that
// appear in haystack.
func keywordOverlap(needle, haystack string) int {
	seen := map[string]bool{}
	count := 0
	for _, w := range strings.FieldsFunc(needle, func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9') && !(r >= 'à' && r <= 'ÿ')
	}) {
		if len(w) <= 3 || seen[w] {
			continue
		}
		seen[w] = true
		if strings.Contains(haystack, w) {
			count++
		}
	}
	return count
}

func ternary(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}
