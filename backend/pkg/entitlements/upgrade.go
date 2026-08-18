// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package entitlements

import "fmt"

// planLabel is the human name of a plan.
func planLabel(p Plan) string {
	switch p {
	case PlanFree:
		return "Free"
	case PlanPro:
		return "Pro"
	case PlanBusiness:
		return "Business"
	case PlanEnterprise:
		return "Enterprise"
	default:
		return string(p)
	}
}

// featureBenefit is the value proposition shown on the wall — a wall that explains
// converts; a wall that hides frustrates (task §2). French, since the target market
// is FR/Maghreb/Sub-Saharan Africa; the frontend localises further.
var featureBenefit = map[Feature]string{
	FeatFinancialQuant:     "La quantification financière Monte-Carlo chiffre votre exposition en XAF et justifie vos budgets sécurité auprès de la direction.",
	FeatAIAdvisor:          "L'assistant IA GRC suggère des plans de traitement, détecte les risques émergents et rédige vos rapports d'audit.",
	FeatAutomation:         "L'automatisation (SOAR) enchaîne alertes, tickets et escalades SLA sans intervention manuelle.",
	FeatSmartScore:         "Le score de risque intelligent combine 8 facteurs (exposition, vulnérabilités, menaces, valeur financière) au lieu du seul P×I.",
	FeatExecutiveDashboard: "Le tableau de bord exécutif consolide toute votre posture (cyber score, exposition financière, KRI) en un écran pour le COMEX.",
	FeatScanner:            "Le scanner d'infrastructure découvre automatiquement vos actifs et leurs vulnérabilités (cloud, réseau, conteneurs).",
	FeatCTI:                "Le renseignement sur les menaces (CTI) enrichit vos vulnérabilités avec NVD, CISA-KEV et MITRE ATT&CK.",
	FeatGovernance:         "La gouvernance ajoute les workflows d'approbation Maker-Checker et la piste d'audit infalsifiable.",
	FeatSSO:                "Le SSO/SAML connecte OpenRisk à votre annuaire d'entreprise pour une authentification centralisée.",
	FeatMultiTenant:        "Le multi-organisation gère plusieurs entités sous un même compte.",
	FeatOnPremise:          "Le déploiement on-premise vous donne le contrôle total de vos données.",
}

// UpgradeMessage builds the paywall sentence for a feature: what it does + the
// plan that unlocks it. Never hides — always explains.
func UpgradeMessage(f Feature) string {
	req := planLabel(MinPlanFor(f))
	if b, ok := featureBenefit[f]; ok {
		return fmt.Sprintf("%s Disponible à partir du plan %s.", b, req)
	}
	return fmt.Sprintf("Cette fonctionnalité est disponible à partir du plan %s.", req)
}

// LimitMessage builds the copy for a reached resource cap.
func LimitMessage(k LimitKey, limit int) string {
	label := map[LimitKey]string{
		LimitUsers:        "utilisateurs",
		LimitRisks:        "risques",
		LimitAssets:       "actifs",
		LimitIntegrations: "intégrations",
	}[k]
	return fmt.Sprintf("Vous avez atteint la limite de %d %s de votre plan. Passez à un plan supérieur pour en ajouter davantage.", limit, label)
}
