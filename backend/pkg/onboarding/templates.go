// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package onboarding holds the static, PURE reference data behind the signup
// wizard: sectors, goals, the three risk suggestions offered per sector, and the
// frameworks suggested for a (sector, country) pair.
//
// Pure on purpose — stdlib only, no domain/GORM imports — so the product content
// is trivially testable and cannot drift into carrying request state. The wizard
// use cases read it; nothing here writes anything.
//
// A note on §5 of the spec, because it is the whole philosophy of this package:
// we NEVER create a risk automatically. These suggestions are pre-filled drafts
// the newcomer opens, adjusts and validates. The difference between "the product
// made a risk" and "I made my first risk" is the difference between a demo and
// ownership.
package onboarding

import "sort"

// Sector is one industry option in the wizard. Labels ship in both languages so
// the client stays a renderer.
type Sector struct {
	Key       string            `json:"key"`
	LabelI18n map[string]string `json:"label_i18n"`
}

// Goal is one answer to "what brings you here?" — it selects the template that
// gets loaded (which frameworks are suggested first, which landing page).
type Goal struct {
	Key       string            `json:"key"`
	LabelI18n map[string]string `json:"label_i18n"`
	// Frameworks are catalog keys (pkg/compliance) pushed to the top of the
	// suggestion list when this goal is chosen.
	Frameworks []string `json:"frameworks,omitempty"`
	// Landing is where the user is dropped after the wizard.
	Landing string `json:"landing"`
}

// RiskSuggestion is a pre-filled first-risk draft. Probability is on the domain's
// [0,1] scale and Impact on [0,10] — the same scales the Score Engine consumes,
// so what the user sees in the form is exactly what gets scored.
type RiskSuggestion struct {
	Key            string   `json:"key"`
	Title          string   `json:"title"`
	Description    string   `json:"description"`
	Probability    float64  `json:"probability"`
	Impact         float64  `json:"impact"`
	Category       string   `json:"category"`
	SuggestedAsset string   `json:"suggested_asset,omitempty"`
	SuggestedTags  []string `json:"suggested_tags,omitempty"`
}

// ---------------------------------------------------------------------------
// Sectors
// ---------------------------------------------------------------------------

var sectors = []Sector{
	{Key: "banking", LabelI18n: map[string]string{"fr": "Banque & finance", "en": "Banking & finance"}},
	{Key: "insurance", LabelI18n: map[string]string{"fr": "Assurance", "en": "Insurance"}},
	{Key: "telecom", LabelI18n: map[string]string{"fr": "Télécommunications", "en": "Telecommunications"}},
	{Key: "health", LabelI18n: map[string]string{"fr": "Santé", "en": "Healthcare"}},
	{Key: "public", LabelI18n: map[string]string{"fr": "Secteur public", "en": "Public sector"}},
	{Key: "energy", LabelI18n: map[string]string{"fr": "Énergie & utilities", "en": "Energy & utilities"}},
	{Key: "retail", LabelI18n: map[string]string{"fr": "Commerce & e-commerce", "en": "Retail & e-commerce"}},
	{Key: "tech", LabelI18n: map[string]string{"fr": "Technologie & SaaS", "en": "Technology & SaaS"}},
	{Key: "industry", LabelI18n: map[string]string{"fr": "Industrie & logistique", "en": "Industry & logistics"}},
	{Key: "other", LabelI18n: map[string]string{"fr": "Autre", "en": "Other"}},
}

// Sectors returns every selectable sector, in display order.
func Sectors() []Sector {
	out := make([]Sector, len(sectors))
	copy(out, sectors)
	return out
}

// ---------------------------------------------------------------------------
// Goals (spec §4, /onboarding/goal)
// ---------------------------------------------------------------------------

var goals = []Goal{
	{
		Key:        "pass_audit",
		LabelI18n:  map[string]string{"fr": "Passer un audit", "en": "Pass an audit"},
		Frameworks: []string{"iso27001-2022", "soc2-tsc"},
		Landing:    "/compliance/audits",
	},
	{
		Key:        "map_risks",
		LabelI18n:  map[string]string{"fr": "Cartographier mes risques", "en": "Map my risks"},
		Frameworks: []string{"iso31000-2018", "iso27005-2022"},
		Landing:    "/risks",
	},
	{
		Key:        "cobac_compliance",
		LabelI18n:  map[string]string{"fr": "Suivre la conformité COBAC", "en": "Track COBAC compliance"},
		Frameworks: []string{"cobac", "bceao"},
		Landing:    "/compliance",
	},
	{
		Key:       "other",
		LabelI18n: map[string]string{"fr": "Autre", "en": "Other"},
		Landing:   "/",
	},
}

// Goals returns every selectable goal, in display order.
func Goals() []Goal {
	out := make([]Goal, len(goals))
	copy(out, goals)
	return out
}

// GoalByKey looks a goal up; ok is false for an unknown key.
func GoalByKey(key string) (Goal, bool) {
	for _, g := range goals {
		if g.Key == key {
			return g, true
		}
	}
	return Goal{}, false
}

// LandingForGoal is where the wizard drops the user. Unknown goals land on the
// dashboard, which is never wrong.
func LandingForGoal(key string) string {
	if g, ok := GoalByKey(key); ok && g.Landing != "" {
		return g.Landing
	}
	return "/"
}

// ---------------------------------------------------------------------------
// First-risk suggestions per sector (spec §5)
//
// Three per sector, always: enough to recognise oneself in the list, few enough
// to decide in seconds. The generic trio named in the spec is the fallback and
// also seeds the sectors that share those exposures.
// ---------------------------------------------------------------------------

var genericSuggestions = []RiskSuggestion{
	{
		Key:           "admin_credentials",
		Title:         "Compromission d'identifiants d'administrateur",
		Description:   "Un compte à privilèges élevés est compromis (hameçonnage, mot de passe réutilisé, absence de second facteur), donnant à un attaquant un accès étendu au système d'information.",
		Probability:   0.35,
		Impact:        8.5,
		Category:      "access_control",
		SuggestedTags: []string{"identité", "privilèges"},
	},
	{
		Key:           "payment_outage",
		Title:         "Indisponibilité du système de paiement",
		Description:   "Une panne ou une attaque rend le système de paiement indisponible : les transactions s'arrêtent, avec une perte de revenu par heure d'interruption et un impact direct sur la relation client.",
		Probability:   0.25,
		Impact:        9.0,
		Category:      "availability",
		SuggestedTags: []string{"disponibilité", "paiement"},
	},
	{
		Key:           "customer_data_leak",
		Title:         "Fuite de données clients",
		Description:   "Des données personnelles de clients sont exposées ou exfiltrées : obligation de notification au régulateur, sanction possible et atteinte durable à la réputation.",
		Probability:   0.30,
		Impact:        9.5,
		Category:      "data_protection",
		SuggestedTags: []string{"données personnelles", "confidentialité"},
	},
}

var sectorSuggestions = map[string][]RiskSuggestion{
	"banking": {
		genericSuggestions[0],
		{
			Key:           "core_banking_outage",
			Title:         "Indisponibilité du système bancaire central",
			Description:   "Le core banking devient indisponible : opérations clients bloquées, obligation de déclaration au superviseur et exposition à des pénalités réglementaires.",
			Probability:   0.20,
			Impact:        9.5,
			Category:      "availability",
			SuggestedTags: []string{"core banking", "continuité"},
		},
		{
			Key:           "fraud_transfer",
			Title:         "Fraude sur les virements",
			Description:   "Un ordre de virement frauduleux est exécuté (usurpation d'identité du donneur d'ordre ou compromission d'un poste), entraînant une perte financière directe.",
			Probability:   0.30,
			Impact:        8.0,
			Category:      "fraud",
			SuggestedTags: []string{"fraude", "paiement"},
		},
	},
	"insurance": {
		genericSuggestions[2],
		genericSuggestions[0],
		{
			Key:           "claims_platform_outage",
			Title:         "Indisponibilité de la plateforme de sinistres",
			Description:   "La plateforme de déclaration et de gestion des sinistres est interrompue : engagements contractuels de délai non tenus et afflux de réclamations.",
			Probability:   0.25,
			Impact:        8.0,
			Category:      "availability",
			SuggestedTags: []string{"sinistres", "continuité"},
		},
	},
	"telecom": {
		{
			Key:           "network_core_outage",
			Title:         "Panne du cœur de réseau",
			Description:   "Une défaillance du cœur de réseau interrompt le service pour une part importante des abonnés, avec pénalités contractuelles et obligation de rapport au régulateur.",
			Probability:   0.25,
			Impact:        9.0,
			Category:      "availability",
			SuggestedTags: []string{"réseau", "continuité"},
		},
		genericSuggestions[2],
		genericSuggestions[0],
	},
	"health": {
		{
			Key:           "patient_data_leak",
			Title:         "Fuite de données de santé",
			Description:   "Des données de santé, particulièrement sensibles, sont exposées : notification obligatoire, sanction réglementaire et perte de confiance des patients.",
			Probability:   0.30,
			Impact:        9.5,
			Category:      "data_protection",
			SuggestedTags: []string{"données de santé", "confidentialité"},
		},
		{
			Key:           "ransomware_care",
			Title:         "Rançongiciel bloquant les activités de soin",
			Description:   "Un rançongiciel chiffre les systèmes cliniques : report des actes, bascule en mode dégradé et risque direct pour la prise en charge des patients.",
			Probability:   0.30,
			Impact:        9.5,
			Category:      "availability",
			SuggestedTags: []string{"rançongiciel", "continuité"},
		},
		genericSuggestions[0],
	},
	"public": {
		{
			Key:           "citizen_service_outage",
			Title:         "Indisponibilité d'un téléservice aux usagers",
			Description:   "Un téléservice destiné aux usagers devient indisponible : démarches bloquées, files d'attente en guichet et exposition médiatique.",
			Probability:   0.25,
			Impact:        7.5,
			Category:      "availability",
			SuggestedTags: []string{"téléservice", "continuité"},
		},
		genericSuggestions[2],
		genericSuggestions[0],
	},
	"energy": {
		{
			Key:           "ot_intrusion",
			Title:         "Intrusion sur le système industriel (OT)",
			Description:   "Un attaquant atteint le réseau industriel depuis la bureautique : risque d'arrêt de production, de dégât matériel et d'atteinte à la sécurité des personnes.",
			Probability:   0.20,
			Impact:        9.5,
			Category:      "industrial",
			SuggestedTags: []string{"OT", "SCADA"},
		},
		genericSuggestions[0],
		{
			Key:           "supply_interruption",
			Title:         "Interruption de la fourniture de service",
			Description:   "Un incident technique ou cyber interrompt la fourniture : pénalités contractuelles, déclaration au régulateur et impact sur les clients raccordés.",
			Probability:   0.25,
			Impact:        9.0,
			Category:      "availability",
			SuggestedTags: []string{"continuité"},
		},
	},
	"retail": {
		genericSuggestions[1],
		genericSuggestions[2],
		{
			Key:           "card_data_compromise",
			Title:         "Compromission de données de cartes bancaires",
			Description:   "Des données de cartes sont capturées sur le tunnel de paiement : obligations PCI DSS, coûts de remédiation et perte de confiance des acheteurs.",
			Probability:   0.25,
			Impact:        9.0,
			Category:      "data_protection",
			SuggestedTags: []string{"PCI DSS", "paiement"},
		},
	},
	"tech": {
		genericSuggestions[0],
		{
			Key:           "supply_chain_dependency",
			Title:         "Compromission d'une dépendance logicielle",
			Description:   "Une bibliothèque tierce embarquée dans le produit est compromise : le code malveillant se propage à la base installée avant d'être détecté.",
			Probability:   0.30,
			Impact:        8.5,
			Category:      "supply_chain",
			SuggestedTags: []string{"chaîne d'approvisionnement", "dépendances"},
		},
		{
			Key:           "saas_outage",
			Title:         "Indisponibilité de la plateforme SaaS",
			Description:   "La plateforme est indisponible pour l'ensemble des clients : engagements de niveau de service rompus, avoirs à émettre et résiliations.",
			Probability:   0.25,
			Impact:        8.5,
			Category:      "availability",
			SuggestedTags: []string{"SLA", "continuité"},
		},
	},
	"industry": {
		{
			Key:           "production_stop",
			Title:         "Arrêt de la ligne de production",
			Description:   "Un incident cyber ou technique arrête la production : coût par heure d'arrêt, retards de livraison et pénalités clients.",
			Probability:   0.25,
			Impact:        8.5,
			Category:      "availability",
			SuggestedTags: []string{"production", "continuité"},
		},
		genericSuggestions[0],
		{
			Key:           "supplier_failure",
			Title:         "Défaillance d'un fournisseur critique",
			Description:   "Un fournisseur unique sur un composant critique fait défaut : rupture d'approvisionnement sans solution de repli immédiate.",
			Probability:   0.30,
			Impact:        7.5,
			Category:      "third_party",
			SuggestedTags: []string{"fournisseur", "tiers"},
		},
	},
}

// RiskSuggestionsFor returns the three pre-filled first-risk drafts offered for a
// sector. An unknown or empty sector gets the generic trio — a newcomer is never
// shown an empty list.
func RiskSuggestionsFor(sector string) []RiskSuggestion {
	s, ok := sectorSuggestions[sector]
	if !ok {
		s = genericSuggestions
	}
	out := make([]RiskSuggestion, len(s))
	copy(out, s)
	return out
}

// ---------------------------------------------------------------------------
// Suggested frameworks per (sector, country) (spec §4, /onboarding/framework)
// ---------------------------------------------------------------------------

// countryFrameworks maps an ISO-3166 alpha-2 country to the frameworks that are
// locally binding or expected. Keys are pkg/compliance catalog keys.
var countryFrameworks = map[string][]string{
	// CEMAC / UEMOA — the regional banking supervisors.
	"CM": {"antic-cm", "cobac"},
	"GA": {"cobac"},
	"CG": {"cobac"},
	"TD": {"cobac"},
	"CF": {"cobac"},
	"GQ": {"cobac"},
	"SN": {"bceao"},
	"CI": {"bceao"},
	"BJ": {"bceao"},
	"BF": {"bceao"},
	"ML": {"bceao"},
	"NE": {"bceao"},
	"TG": {"bceao"},
	"GW": {"bceao"},
	// EU / EEA — GDPR always, plus the sectoral overlays handled below.
	"FR": {"gdpr-2016-679"},
	"BE": {"gdpr-2016-679"},
	"LU": {"gdpr-2016-679"},
	"DE": {"gdpr-2016-679"},
	"ES": {"gdpr-2016-679"},
	"IT": {"gdpr-2016-679"},
	"PT": {"gdpr-2016-679"},
	"NL": {"gdpr-2016-679"},
	// North America.
	"US": {"soc2-tsc"},
	"CA": {"soc2-tsc"},
	// Maghreb.
	"MA": {"iso27001-2022"},
	"TN": {"iso27001-2022"},
	"DZ": {"iso27001-2022"},
}

// sectorFrameworks maps a sector to the frameworks its peers run.
var sectorFrameworks = map[string][]string{
	"banking":   {"iso27001-2022", "pci-dss-4.0", "nist-csf-2.0"},
	"insurance": {"iso27001-2022", "nist-csf-2.0"},
	"telecom":   {"iso27001-2022", "nist-csf-2.0"},
	"health":    {"iso27001-2022", "hipaa-security"},
	"public":    {"iso27001-2022", "nist-800-53-r5"},
	"energy":    {"iso27001-2022", "nist-csf-2.0"},
	"retail":    {"pci-dss-4.0", "iso27001-2022"},
	"tech":      {"soc2-tsc", "iso27001-2022", "cis-v8"},
	"industry":  {"iso27001-2022", "cis-v8"},
	"other":     {"iso27001-2022"},
}

// euSectoralOverlay adds the EU sectoral regimes on top of GDPR.
var euSectoralOverlay = map[string][]string{
	"banking":   {"dora-2022-2554"},
	"insurance": {"dora-2022-2554"},
	"telecom":   {"nis2-2022-2555"},
	"energy":    {"nis2-2022-2555"},
	"health":    {"nis2-2022-2555"},
	"public":    {"nis2-2022-2555"},
}

var euCountries = map[string]bool{
	"FR": true, "BE": true, "LU": true, "DE": true,
	"ES": true, "IT": true, "PT": true, "NL": true,
}

// SuggestedFrameworks returns catalog keys ordered by relevance for a
// (sector, country, goal) triple, de-duplicated, most relevant first:
// the goal's own frameworks, then the country's binding regimes (plus the EU
// sectoral overlay), then the sector's peers. ISO 27001 closes the list as the
// universal default so the result is never empty.
func SuggestedFrameworks(sector, country, goal string) []string {
	var ordered []string
	seen := map[string]bool{}
	add := func(keys ...string) {
		for _, k := range keys {
			if k == "" || seen[k] {
				continue
			}
			seen[k] = true
			ordered = append(ordered, k)
		}
	}

	if g, ok := GoalByKey(goal); ok {
		add(g.Frameworks...)
	}
	add(countryFrameworks[normaliseCountry(country)]...)
	if euCountries[normaliseCountry(country)] {
		add(euSectoralOverlay[sector]...)
	}
	add(sectorFrameworks[sector]...)
	add("iso27001-2022")

	return ordered
}

// normaliseCountry upper-cases a 2-letter code and leaves anything else alone
// (an unknown value simply contributes no country-specific frameworks).
func normaliseCountry(country string) string {
	if len(country) != 2 {
		return country
	}
	b := []byte(country)
	for i := range b {
		if b[i] >= 'a' && b[i] <= 'z' {
			b[i] -= 'a' - 'A'
		}
	}
	return string(b)
}

// KnownSectorKeys returns the sector keys that carry a bespoke suggestion set,
// sorted — used by the tests and by nothing at runtime.
func KnownSectorKeys() []string {
	keys := make([]string, 0, len(sectorSuggestions))
	for k := range sectorSuggestions {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
