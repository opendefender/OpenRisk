// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package compliance

// ISO 22301:2019 — Security and resilience — Business continuity management
// systems — Requirements.
//
// Modelled at the level of the standard's numbered requirements (Clauses 4 to
// 10), which is the level an auditor works at and the level the tenant can
// evidence: a BIA report answers 8.2.2, a test report answers 8.6. Clause
// numbers and titles are the standard's own published structure and are
// reliable; the descriptions are original summaries of what each clause asks
// for, not ISO's text. Verify against ISO 22301:2019 before an audit.
//
// Clauses 1 to 3 (scope, normative references, terms) carry no requirements and
// are deliberately absent — listing them would put rows in a register that can
// never be evidenced or closed.

func init() {
	register(Catalog{
		Key:         "iso22301-2019",
		Name:        "ISO 22301",
		Version:     "2019",
		Description: "Systèmes de management de la continuité d'activité — Exigences. Contexte, leadership, planification, support, fonctionnement (BIA, stratégie, plans, exercices), évaluation des performances et amélioration.",
		Available:   true,
		Controls:    iso223012019Controls,
	})
}

const iso22301Source = "ISO 22301:2019, Article "

var iso223012019Controls = []CatalogControl{
	// Clause 4 — Context of the organization
	{"4.1", "Compréhension de l'organisation et de son contexte",
		"Déterminer les enjeux internes et externes pertinents pour la finalité de l'organisme et qui influent sur sa capacité à atteindre les résultats attendus de son système de management de la continuité d'activité.",
		iso22301Source + "4.1"},
	{"4.2", "Besoins et attentes des parties intéressées",
		"Identifier les parties intéressées pertinentes pour le SMCA, leurs exigences, et les exigences légales et réglementaires applicables à la continuité d'activité.",
		iso22301Source + "4.2"},
	{"4.3", "Détermination du domaine d'application du SMCA",
		"Définir les limites et l'applicabilité du système de management, en précisant les produits et services, activités et sites couverts, ainsi que les exclusions et leur justification.",
		iso22301Source + "4.3"},
	{"4.4", "Système de management de la continuité d'activité",
		"Établir, mettre en œuvre, tenir à jour et améliorer en continu le SMCA, y compris les processus nécessaires et leurs interactions.",
		iso22301Source + "4.4"},

	// Clause 5 — Leadership
	{"5.1", "Leadership et engagement",
		"La direction démontre son leadership et son engagement : intégration des exigences de continuité aux processus métier, mise à disposition des ressources, communication de l'importance du SMCA.",
		iso22301Source + "5.1"},
	{"5.2", "Politique de continuité d'activité",
		"Établir une politique de continuité d'activité appropriée à la finalité de l'organisme, fournissant un cadre pour les objectifs, documentée, communiquée et disponible aux parties intéressées.",
		iso22301Source + "5.2"},
	{"5.3", "Rôles, responsabilités et autorités",
		"Attribuer et communiquer les responsabilités et autorités pour les rôles pertinents du SMCA, y compris le compte rendu de performance à la direction.",
		iso22301Source + "5.3"},

	// Clause 6 — Planning
	{"6.1", "Actions face aux risques et opportunités",
		"Déterminer les risques et opportunités à traiter pour que le SMCA atteigne ses résultats, prévenir les effets indésirables et réaliser l'amélioration continue ; planifier les actions et évaluer leur efficacité.",
		iso22301Source + "6.1"},
	{"6.2", "Objectifs de continuité d'activité et planification",
		"Établir des objectifs de continuité mesurables, cohérents avec la politique, en tenant compte des exigences applicables, et planifier ce qui sera fait, avec quelles ressources, par qui et selon quelles échéances.",
		iso22301Source + "6.2"},
	{"6.3", "Planification des modifications du SMCA",
		"Réaliser toute modification du système de management de manière planifiée, en considérant sa finalité, ses conséquences potentielles, la disponibilité des ressources et l'attribution des responsabilités.",
		iso22301Source + "6.3"},

	// Clause 7 — Support
	{"7.1", "Ressources",
		"Déterminer et fournir les ressources nécessaires à l'établissement, la mise en œuvre, la tenue à jour et l'amélioration continue du SMCA.",
		iso22301Source + "7.1"},
	{"7.2", "Compétences",
		"Déterminer les compétences nécessaires des personnes agissant sous le contrôle de l'organisme, s'assurer qu'elles sont compétentes, et conserver les preuves de ces compétences.",
		iso22301Source + "7.2"},
	{"7.3", "Sensibilisation",
		"S'assurer que les personnes concernées ont connaissance de la politique de continuité, de leur contribution à l'efficacité du SMCA et des conséquences d'un non-respect des exigences.",
		iso22301Source + "7.3"},
	{"7.4", "Communication",
		"Déterminer les besoins de communication interne et externe pertinents pour le SMCA : sur quels sujets, à quels moments, avec qui, et par quels moyens.",
		iso22301Source + "7.4"},
	{"7.5", "Informations documentées",
		"Créer, mettre à jour et maîtriser les informations documentées exigées par la norme et jugées nécessaires à l'efficacité du SMCA, y compris leur disponibilité, protection et conservation.",
		iso22301Source + "7.5"},

	// Clause 8 — Operation (the heart of the standard)
	{"8.1", "Planification et maîtrise opérationnelles",
		"Planifier, mettre en œuvre et maîtriser les processus nécessaires pour satisfaire aux exigences et appliquer les actions déterminées à l'article 6, y compris la maîtrise des processus externalisés.",
		iso22301Source + "8.1"},
	{"8.2", "Analyse d'impact sur l'activité et appréciation des risques",
		"Mettre en œuvre un processus formel et documenté d'analyse d'impact sur l'activité (BIA) et d'appréciation des risques liés à l'interruption des activités.",
		iso22301Source + "8.2"},
	{"8.2.2", "Analyse d'impact sur l'activité (BIA)",
		"Déterminer les activités prioritaires, les impacts d'une interruption dans le temps, les délais maximaux d'interruption admissibles (MTPD), les objectifs de temps de reprise (RTO) et les ressources nécessaires.",
		iso22301Source + "8.2.2"},
	{"8.2.3", "Appréciation des risques d'interruption",
		"Identifier, analyser et évaluer les risques d'interruption des activités prioritaires et de leurs ressources, et déterminer ceux qui exigent un traitement.",
		iso22301Source + "8.2.3"},
	{"8.3", "Stratégies et solutions de continuité d'activité",
		"Identifier et sélectionner des stratégies de continuité fondées sur les résultats de la BIA et de l'appréciation des risques, en tenant compte des besoins en ressources et des coûts.",
		iso22301Source + "8.3"},
	{"8.4", "Plans et procédures de continuité d'activité",
		"Établir et documenter des plans et procédures de continuité qui définissent la structure de réponse, les rôles, les critères d'activation, les procédures d'alerte et de communication et les modalités de reprise.",
		iso22301Source + "8.4"},
	{"8.4.2", "Structure de réponse aux incidents",
		"Mettre en place une structure identifiant les équipes et les personnes ayant l'autorité et la compétence pour répondre à un incident perturbateur, avec des critères d'activation documentés.",
		iso22301Source + "8.4.2"},
	{"8.4.3", "Alerte et communication",
		"Documenter et maintenir les procédures de détection, de surveillance, d'alerte interne et de communication avec les parties intéressées, y compris les autorités et les médias.",
		iso22301Source + "8.4.3"},
	{"8.4.4", "Plans de continuité d'activité",
		"Documenter des plans opérationnels permettant de poursuivre ou reprendre les activités prioritaires dans les délais fixés, avec les ressources, dépendances et interfaces identifiées.",
		iso22301Source + "8.4.4"},
	{"8.4.5", "Rétablissement",
		"Disposer de procédures documentées pour rétablir les activités à partir des mesures temporaires adoptées pendant et après la perturbation.",
		iso22301Source + "8.4.5"},
	{"8.5", "Programme d'exercices et de tests",
		"Mettre en œuvre et tenir à jour un programme d'exercices et de tests cohérent avec le domaine d'application, validant les stratégies et les plans, et produisant des comptes rendus formalisés avec actions correctives.",
		iso22301Source + "8.5"},
	{"8.6", "Évaluation de la documentation et des capacités",
		"Évaluer, à intervalles planifiés, l'adéquation et l'efficacité de l'analyse d'impact, de l'appréciation des risques, des stratégies, des plans et des procédures.",
		iso22301Source + "8.6"},

	// Clause 9 — Performance evaluation
	{"9.1", "Surveillance, mesure, analyse et évaluation",
		"Déterminer ce qui doit être surveillé et mesuré, les méthodes, les échéances et qui analyse les résultats ; conserver les informations documentées comme preuves.",
		iso22301Source + "9.1"},
	{"9.2", "Audit interne",
		"Réaliser des audits internes à intervalles planifiés pour vérifier la conformité du SMCA aux exigences propres de l'organisme et à la norme, et son efficacité de mise en œuvre.",
		iso22301Source + "9.2"},
	{"9.3", "Revue de direction",
		"Procéder, à intervalles planifiés, à la revue du SMCA par la direction afin de s'assurer qu'il est toujours approprié, adapté et efficace, et en conserver les comptes rendus.",
		iso22301Source + "9.3"},

	// Clause 10 — Improvement
	{"10.1", "Non-conformité et action corrective",
		"Réagir aux non-conformités, en évaluer les causes, mettre en œuvre les actions correctives nécessaires et en examiner l'efficacité ; conserver les preuves de la nature des non-conformités et des suites données.",
		iso22301Source + "10.1"},
	{"10.2", "Amélioration continue",
		"Améliorer en continu la pertinence, l'adéquation et l'efficacité du système de management de la continuité d'activité.",
		iso22301Source + "10.2"},
}
