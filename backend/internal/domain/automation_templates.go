// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial
// This file is part of the OpenRisk Enterprise Edition and is NOT covered by the
// AGPL; it is licensed under the OpenRisk Commercial License (see LICENSE.commercial).

package domain

import (
	"fmt"
	"strings"
)

// ---------------------------------------------------------------------------
// Rule vocabulary and ready-made templates.
//
// A rule builder that speaks JSON teaches nobody. The vocabulary below is what
// lets the UI render a rule as a sentence — "Quand une vulnérabilité est
// détectée, Si sa criticité est au moins élevée, Alors ouvrir un risque puis
// alerter" — and lets a reviewer check a rule without reading its payload.
//
// Kept in the domain (pure, no GORM, no HTTP) so both the API and the tests
// read from the same source of truth, and the FR/EN wording cannot drift
// between the two.
// ---------------------------------------------------------------------------

// Locale selects the wording of a rendered sentence.
const (
	LocaleFR = "fr"
	LocaleEN = "en"
)

func normLocale(l string) string {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(l)), "en") {
		return LocaleEN
	}
	return LocaleFR
}

// TriggerLabel is the "Quand …" clause of a rule sentence.
func TriggerLabel(t AutomationTrigger, locale string) string {
	fr := map[AutomationTrigger]string{
		TriggerVulnerabilityDetected: "une vulnérabilité est détectée",
		TriggerRiskCreated:           "un risque est créé",
		TriggerRiskScoreUpdated:      "le score d'un risque change",
		TriggerIncidentCreated:       "un incident est déclaré",
		TriggerManual:                "je lance la règle manuellement",
	}
	en := map[AutomationTrigger]string{
		TriggerVulnerabilityDetected: "a vulnerability is detected",
		TriggerRiskCreated:           "a risk is created",
		TriggerRiskScoreUpdated:      "a risk score changes",
		TriggerIncidentCreated:       "an incident is declared",
		TriggerManual:                "I run the rule manually",
	}
	if normLocale(locale) == LocaleEN {
		if s, ok := en[t]; ok {
			return s
		}
		return string(t)
	}
	if s, ok := fr[t]; ok {
		return s
	}
	return string(t)
}

// ActionLabel is one clause of the "Alors …" list.
func ActionLabel(a AutomationAction, locale string) string {
	target := a.Target
	if target == "" {
		target = "—"
	}
	channels := strings.Join(a.Channels, ", ")
	if normLocale(locale) == LocaleEN {
		switch a.Type {
		case ActionScanAsset:
			return "re-scan the affected asset"
		case ActionCreateRisk:
			return "open a risk in the register"
		case ActionAssignOwner:
			return "assign it to " + target
		case ActionCreateTicket:
			if a.TicketProvider != "" {
				return "open a " + a.TicketProvider + " ticket"
			}
			return "open an ITSM ticket"
		case ActionNotify:
			if channels != "" {
				return "alert via " + channels
			}
			return "alert the configured channels"
		case ActionStartSLA:
			return "start the resolution countdown"
		case ActionResolveRisk:
			return "mark the risk resolved"
		case ActionCloseTicket:
			return "close the linked ticket"
		}
		return string(a.Type)
	}
	switch a.Type {
	case ActionScanAsset:
		return "relancer un scan de l'actif concerné"
	case ActionCreateRisk:
		return "ouvrir un risque dans le registre"
	case ActionAssignOwner:
		return "l'assigner à " + target
	case ActionCreateTicket:
		if a.TicketProvider != "" {
			return "ouvrir un ticket " + a.TicketProvider
		}
		return "ouvrir un ticket ITSM"
	case ActionNotify:
		if channels != "" {
			return "alerter via " + channels
		}
		return "alerter les canaux configurés"
	case ActionStartSLA:
		return "démarrer le compte à rebours de résolution"
	case ActionResolveRisk:
		return "marquer le risque comme résolu"
	case ActionCloseTicket:
		return "clôturer le ticket lié"
	}
	return string(a.Type)
}

// ConditionLabels renders the "Si …" clauses. An empty result means the rule
// matches everything — which the UI should say out loud rather than leave blank.
func ConditionLabels(c AutomationConditions, locale string) []string {
	en := normLocale(locale) == LocaleEN
	var out []string
	if c.MinSeverity != "" {
		if en {
			out = append(out, "its severity is at least "+c.MinSeverity)
		} else {
			out = append(out, "sa criticité est au moins "+severityFR(c.MinSeverity))
		}
	}
	if c.MinCVSS > 0 {
		if en {
			out = append(out, fmt.Sprintf("its CVSS score is at least %.1f", c.MinCVSS))
		} else {
			out = append(out, fmt.Sprintf("son score CVSS est au moins %.1f", c.MinCVSS))
		}
	}
	if c.KEVOnly {
		if en {
			out = append(out, "it is on the CISA known-exploited list")
		} else {
			out = append(out, "elle est sur la liste CISA des vulnérabilités activement exploitées")
		}
	}
	if c.MinPriorityTier != "" {
		if en {
			out = append(out, "its priority is "+c.MinPriorityTier+" or stronger")
		} else {
			out = append(out, "sa priorité est "+c.MinPriorityTier+" ou plus forte")
		}
	}
	if len(c.AssetTags) > 0 {
		joined := strings.Join(c.AssetTags, ", ")
		if en {
			out = append(out, "the affected asset carries one of these tags: "+joined)
		} else {
			out = append(out, "l'actif concerné porte l'une de ces étiquettes : "+joined)
		}
	}
	return out
}

func severityFR(s string) string {
	switch strings.ToLower(s) {
	case "critical":
		return "critique"
	case "high":
		return "élevée"
	case "medium":
		return "moyenne"
	case "low":
		return "faible"
	}
	return s
}

// Describe renders the whole rule as one sentence in the requested locale. This
// is the string the rule list shows, so a reviewer reads intent, not JSON.
func (r *AutomationRule) Describe(locale string) string {
	en := normLocale(locale) == LocaleEN
	var b strings.Builder
	if en {
		b.WriteString("When " + TriggerLabel(r.Trigger, locale))
	} else {
		b.WriteString("Quand " + TriggerLabel(r.Trigger, locale))
	}

	conds := ConditionLabels(r.Conditions, locale)
	if len(conds) > 0 {
		if en {
			b.WriteString(", if " + joinWith(conds, " and "))
		} else {
			b.WriteString(", si " + joinWith(conds, " et "))
		}
	}

	acts := make([]string, 0, len(r.Actions))
	for _, a := range r.Actions {
		acts = append(acts, ActionLabel(a, locale))
	}
	if len(acts) == 0 {
		if en {
			b.WriteString(", then do nothing (this rule has no action)")
		} else {
			b.WriteString(", alors ne rien faire (cette règle n'a aucune action)")
		}
		return b.String()
	}
	if en {
		b.WriteString(", then " + joinWith(acts, ", then "))
	} else {
		b.WriteString(", alors " + joinWith(acts, ", puis "))
	}
	return b.String()
}

func joinWith(parts []string, sep string) string {
	return strings.Join(parts, sep)
}

// =============================================================================
// Ready-made templates
// =============================================================================

// AutomationTemplate is a rule someone can adopt in one click and then edit.
// Each one answers a question a security team actually has on day one, rather
// than demonstrating the feature.
type AutomationTemplate struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	NameEN      string `json:"name_en"`
	Description string `json:"description"`
	// UseCase says when to reach for this template — the sentence that stops a
	// user from adopting five templates and understanding none.
	UseCase    string               `json:"use_case"`
	UseCaseEN  string               `json:"use_case_en"`
	Trigger    AutomationTrigger    `json:"trigger"`
	Conditions AutomationConditions `json:"conditions"`
	Actions    AutomationActionList `json:"actions"`
	SLA        AutomationSLAConfig  `json:"sla"`
	Priority   int                  `json:"priority"`
	// RequiresChannels / RequiresTicketing warn up-front that a template needs a
	// capability the tenant may not have configured yet, instead of letting the
	// user discover it from a skipped step three days later.
	RequiresChannels  bool `json:"requires_channels"`
	RequiresTicketing bool `json:"requires_ticketing"`
}

// AutomationTemplates returns the five ready-made playbooks.
func AutomationTemplates() []AutomationTemplate {
	return []AutomationTemplate{
		{
			Key:         "critical-vuln-response",
			Name:        "Réponse aux vulnérabilités critiques",
			NameEN:      "Critical vulnerability response",
			Description: "Ouvre un risque, l'assigne, alerte l'équipe et démarre un SLA de 24 h dès qu'une vulnérabilité critique est détectée.",
			UseCase:     "Le point de départ de la plupart des équipes : ne plus découvrir une vulnérabilité critique une semaine trop tard.",
			UseCaseEN:   "Where most teams start: stop finding out about a critical vulnerability a week late.",
			Trigger:     TriggerVulnerabilityDetected,
			Conditions:  AutomationConditions{MinSeverity: "critical"},
			Actions: AutomationActionList{
				{Type: ActionCreateRisk},
				{Type: ActionAssignOwner, Target: "admin"},
				{Type: ActionNotify, Channels: []string{ChannelInApp, ChannelEmail}},
				{Type: ActionStartSLA},
			},
			SLA: AutomationSLAConfig{
				CriticalMinutes: 1440, HighMinutes: 4320,
				EscalateAfterMinutes: 240, EscalateToRole: "admin",
				EscalateChannels: []string{ChannelInApp, ChannelEmail},
			},
			Priority:         10,
			RequiresChannels: true,
		},
		{
			Key:         "kev-emergency",
			Name:        "Urgence CISA KEV",
			NameEN:      "CISA KEV emergency",
			Description: "Pour une vulnérabilité activement exploitée : risque + ticket ITSM + alerte multi-canal + SLA de 4 h.",
			UseCase:     "Une vulnérabilité sur la liste KEV est exploitée en ce moment, pas en théorie — elle mérite un chemin plus court que le reste.",
			UseCaseEN:   "A KEV vulnerability is being exploited right now, not in theory — it deserves a shorter path than everything else.",
			Trigger:     TriggerVulnerabilityDetected,
			Conditions:  AutomationConditions{KEVOnly: true},
			Actions: AutomationActionList{
				{Type: ActionCreateRisk},
				{Type: ActionCreateTicket},
				{Type: ActionNotify, Channels: []string{ChannelInApp, ChannelEmail, ChannelSlack}},
				{Type: ActionStartSLA},
			},
			SLA: AutomationSLAConfig{
				CriticalMinutes: 240, HighMinutes: 480,
				EscalateAfterMinutes: 60, EscalateToRole: "admin",
				EscalateChannels: []string{ChannelInApp, ChannelEmail, ChannelSlack},
			},
			Priority:          5,
			RequiresChannels:  true,
			RequiresTicketing: true,
		},
		{
			Key:         "internet-facing-watch",
			Name:        "Surveillance des actifs exposés",
			NameEN:      "Internet-facing asset watch",
			Description: "Relance un scan ciblé et alerte dès qu'une vulnérabilité de criticité élevée touche un actif exposé sur Internet.",
			UseCase:     "Un actif exposé transforme une vulnérabilité moyenne en porte d'entrée : cette règle traite l'exposition comme un multiplicateur.",
			UseCaseEN:   "Exposure turns a medium vulnerability into a front door: this rule treats reachability as the multiplier it is.",
			Trigger:     TriggerVulnerabilityDetected,
			Conditions:  AutomationConditions{MinSeverity: "high", AssetTags: []string{"internet-facing"}},
			Actions: AutomationActionList{
				{Type: ActionScanAsset},
				{Type: ActionCreateRisk},
				{Type: ActionNotify, Channels: []string{ChannelInApp}},
			},
			Priority:         20,
			RequiresChannels: true,
		},
		{
			Key:         "critical-incident-escalation",
			Name:        "Escalade des incidents critiques",
			NameEN:      "Critical incident escalation",
			Description: "Alerte immédiatement la direction sécurité et démarre un SLA d'1 h à la déclaration d'un incident critique.",
			UseCase:     "Un incident critique déclaré à 23 h ne doit pas attendre la revue du matin.",
			UseCaseEN:   "A critical incident declared at 11pm should not wait for the morning review.",
			Trigger:     TriggerIncidentCreated,
			Conditions:  AutomationConditions{MinSeverity: "critical"},
			Actions: AutomationActionList{
				{Type: ActionNotify, Channels: []string{ChannelInApp, ChannelEmail, ChannelSMS}, Target: "admin"},
				{Type: ActionStartSLA},
			},
			SLA: AutomationSLAConfig{
				CriticalMinutes: 60, HighMinutes: 240,
				EscalateAfterMinutes: 30, EscalateToRole: "admin",
				EscalateChannels: []string{ChannelInApp, ChannelEmail, ChannelSMS},
			},
			Priority:         5,
			RequiresChannels: true,
		},
		{
			Key:         "risk-score-spike",
			Name:        "Hausse de score de risque",
			NameEN:      "Risk score spike",
			Description: "Réassigne et alerte quand le score d'un risque franchit le seuil critique après recalcul.",
			UseCase:     "Un risque devient critique sans que personne ne l'ait touché — parce que le contexte a changé. Cette règle le fait remonter.",
			UseCaseEN:   "A risk turns critical without anyone touching it, because the context changed. This rule surfaces that.",
			Trigger:     TriggerRiskScoreUpdated,
			Conditions:  AutomationConditions{MinSeverity: "critical"},
			Actions: AutomationActionList{
				{Type: ActionAssignOwner, Target: "admin"},
				{Type: ActionNotify, Channels: []string{ChannelInApp, ChannelEmail}},
			},
			Priority:         30,
			RequiresChannels: true,
		},
	}
}

// FindAutomationTemplate returns a template by key.
func FindAutomationTemplate(key string) (AutomationTemplate, bool) {
	for _, t := range AutomationTemplates() {
		if t.Key == key {
			return t, true
		}
	}
	return AutomationTemplate{}, false
}
