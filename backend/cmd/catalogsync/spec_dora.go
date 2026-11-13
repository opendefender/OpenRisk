// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

package main

// DORA — the rows a person chose, and the provisions deliberately left out.
//
// One row per paragraph that places an obligation on a financial entity, for
// Art. 5-30 (ICT risk management, incidents, testing, third-party risk). An
// article with no numbered paragraph is one row. Art. 45 (information sharing)
// stays as the one article-level row the earlier catalogue carried.
//
// Codes: "Art.6(4)" is Art. 6, paragraph 4. "Art.7", "Art.23" and "Art.45"
// keep the code the earlier, article-level catalogue used, so a tenant who
// imported it does not get them twice.
//
// The name is ours: a short label for the paragraph. The description is the
// paragraph itself, verbatim, extracted by the generator.
var doraRows = []euRow{
	// Chapitre II — Gestion du risque lié aux TIC
	{"Art.5(1)", "Cadre de gouvernance et de contrôle interne du risque TIC", 5, 1, ""},
	{"Art.5(2)", "Responsabilités de l'organe de direction", 5, 2, ""},
	{"Art.5(3)", "Rôle de suivi des accords avec les prestataires tiers TIC", 5, 3, ""},
	{"Art.5(4)", "Connaissances et compétences de l'organe de direction", 5, 4, ""},
	{"Art.6(1)", "Cadre de gestion du risque TIC solide et documenté", 6, 1, ""},
	{"Art.6(2)", "Contenu du cadre : stratégies, politiques, procédures, protocoles et outils", 6, 2, ""},
	{"Art.6(3)", "Réduction de l'incidence du risque TIC et information des autorités", 6, 3, ""},
	{"Art.6(4)", "Fonction de contrôle indépendante et trois lignes de défense", 6, 4, ""},
	{"Art.6(5)", "Documentation et réexamen annuel du cadre", 6, 5, ""},
	{"Art.6(6)", "Audit interne du cadre de gestion du risque TIC", 6, 6, ""},
	{"Art.6(7)", "Suivi formel des constatations d'audit TIC", 6, 7, ""},
	{"Art.6(8)", "Stratégie de résilience opérationnelle numérique", 6, 8, ""},
	{"Art.6(9)", "Stratégie multi-fournisseurs TIC (facultative)", 6, 9, ""},
	{"Art.6(10)", "Externalisation de la vérification de conformité", 6, 10, ""},
	{"Art.7", "Systèmes, protocoles et outils de TIC", 7, 0, ""},
	{"Art.8(1)", "Identification et classification des fonctions, actifs et rôles", 8, 1, ""},
	{"Art.8(2)", "Identification continue des sources de risque TIC", 8, 2, ""},
	{"Art.8(3)", "Évaluation des risques à chaque modification importante", 8, 3, ""},
	{"Art.8(4)", "Identification des actifs informationnels et TIC", 8, 4, ""},
	{"Art.8(5)", "Processus dépendant de prestataires tiers TIC", 8, 5, ""},
	{"Art.8(6)", "Inventaires tenus à jour", 8, 6, ""},
	{"Art.8(7)", "Évaluation annuelle du risque TIC des systèmes hérités", 8, 7, ""},
	{"Art.9(1)", "Suivi et contrôle permanents de la sécurité des systèmes TIC", 9, 1, ""},
	{"Art.9(2)", "Outils et politiques de sécurité TIC", 9, 2, ""},
	{"Art.9(3)", "Solutions et processus de TIC appropriés", 9, 3, ""},
	{"Art.9(4)", "Politiques de sécurité, d'accès, d'authentification et de changement", 9, 4, ""},
	{"Art.10(1)", "Détection rapide des activités anormales", 10, 1, ""},
	{"Art.10(2)", "Niveaux de contrôle, seuils d'alerte et réponse", 10, 2, ""},
	{"Art.10(3)", "Ressources de surveillance", 10, 3, ""},
	{"Art.10(4)", "Vérification des déclarations de transactions (PSCD)", 10, 4, ""},
	{"Art.11(1)", "Politique de continuité des activités de TIC", 11, 1, ""},
	{"Art.11(2)", "Mise en œuvre de la continuité des activités de TIC", 11, 2, ""},
	{"Art.11(3)", "Plans de réponse et de rétablissement des TIC", 11, 3, ""},
	{"Art.11(4)", "Plans de continuité testés périodiquement", 11, 4, ""},
	{"Art.11(5)", "Analyse des incidences sur les activités (BIA)", 11, 5, ""},
	{"Art.11(6)", "Tests annuels des plans de continuité et de communication de crise", 11, 6, ""},
	{"Art.11(7)", "Fonction de gestion de crise", 11, 7, ""},
	{"Art.11(8)", "Registre des activités pendant les perturbations", 11, 8, ""},
	{"Art.11(9)", "Résultats des tests de continuité (dépositaires centraux)", 11, 9, ""},
	{"Art.11(10)", "Estimation des coûts et pertes des incidents majeurs", 11, 10, ""},
	{"Art.12(1)", "Politiques de sauvegarde et méthodes de restauration", 12, 1, ""},
	{"Art.12(2)", "Systèmes de sauvegarde", 12, 2, ""},
	{"Art.12(3)", "Restauration sur des systèmes séparés", 12, 3, ""},
	{"Art.12(4)", "Capacités TIC redondantes", 12, 4, ""},
	{"Art.12(5)", "Site de traitement secondaire (dépositaires centraux)", 12, 5, ""},
	{"Art.12(6)", "Objectifs de délai et de point de rétablissement", 12, 6, ""},
	{"Art.12(7)", "Contrôles lors du rétablissement", 12, 7, ""},
	{"Art.13(1)", "Veille sur les vulnérabilités, cybermenaces et incidents", 13, 1, ""},
	{"Art.13(2)", "Examens post-incident", 13, 2, ""},
	{"Art.13(3)", "Intégration des enseignements tirés", 13, 3, ""},
	{"Art.13(4)", "Contrôle de l'efficacité de la stratégie de résilience", 13, 4, ""},
	{"Art.13(5)", "Compte rendu annuel à l'organe de direction", 13, 5, ""},
	{"Art.13(6)", "Sensibilisation et formation à la sécurité des TIC", 13, 6, ""},
	{"Art.13(7)", "Suivi des évolutions technologiques", 13, 7, ""},
	{"Art.14(1)", "Plans de communication de crise", 14, 1, ""},
	{"Art.14(2)", "Politiques de communication interne et externe", 14, 2, ""},
	{"Art.14(3)", "Responsable de la communication sur les incidents", 14, 3, ""},
	{"Art.16(1)", "Cadre simplifié de gestion du risque TIC", 16, 1, ""},
	{"Art.16(2)", "Documentation et réexamen du cadre simplifié", 16, 2, ""},

	// Chapitre III — Gestion, classification et notification des incidents
	{"Art.17(1)", "Processus de gestion des incidents liés aux TIC", 17, 1, ""},
	{"Art.17(2)", "Enregistrement des incidents et des cybermenaces importantes", 17, 2, ""},
	{"Art.17(3)", "Contenu du processus de gestion des incidents", 17, 3, ""},
	{"Art.18(1)", "Classification des incidents liés aux TIC", 18, 1, ""},
	{"Art.18(2)", "Classification des cybermenaces importantes", 18, 2, ""},
	{"Art.19(1)", "Déclaration des incidents majeurs à l'autorité compétente", 19, 1, ""},
	{"Art.19(2)", "Notification volontaire des cybermenaces importantes", 19, 2, ""},
	{"Art.19(3)", "Information des clients", 19, 3, ""},
	{"Art.19(4)", "Notification initiale, rapport intermédiaire et rapport final", 19, 4, ""},
	{"Art.19(5)", "Externalisation des obligations de déclaration", 19, 5, ""},
	{"Art.23", "Incidents opérationnels ou de sécurité liés au paiement", 23, 0, ""},

	// Chapitre IV — Tests de résilience opérationnelle numérique
	{"Art.24(1)", "Programme de tests de résilience opérationnelle numérique", 24, 1, ""},
	{"Art.24(2)", "Contenu du programme de tests", 24, 2, ""},
	{"Art.24(3)", "Approche fondée sur les risques", 24, 3, ""},
	{"Art.24(4)", "Indépendance des testeurs", 24, 4, ""},
	{"Art.24(5)", "Traitement des problèmes révélés par les tests", 24, 5, ""},
	{"Art.24(6)", "Tests annuels des systèmes soutenant des fonctions critiques", 24, 6, ""},
	{"Art.25(1)", "Tests des outils et systèmes de TIC", 25, 1, ""},
	{"Art.25(2)", "Évaluations de vulnérabilité avant déploiement (DCT et CCP)", 25, 2, ""},
	{"Art.25(3)", "Tests des microentreprises", 25, 3, ""},
	{"Art.26(1)", "Tests de pénétration fondés sur la menace (TLPT)", 26, 1, ""},
	{"Art.26(2)", "Périmètre des TLPT : fonctions critiques en production", 26, 2, ""},
	{"Art.26(3)", "Inclusion des prestataires tiers TIC dans les TLPT", 26, 3, ""},
	{"Art.26(4)", "Tests groupés impliquant un prestataire tiers TIC", 26, 4, ""},
	{"Art.26(5)", "Contrôles efficaces de gestion des risques pendant les TLPT", 26, 5, ""},
	{"Art.26(6)", "Synthèse, plans de mesures correctives et documentation", 26, 6, ""},
	{"Art.26(8)", "Recours aux testeurs et testeurs internes", 26, 8, ""},
	{"Art.27(1)", "Exigences applicables aux testeurs", 27, 1, ""},
	{"Art.27(2)", "Conditions du recours aux testeurs internes", 27, 2, ""},
	{"Art.27(3)", "Contrats avec les testeurs externes", 27, 3, ""},

	// Chapitre V — Gestion du risque lié aux prestataires tiers de services TIC
	{"Art.28(1)", "Gestion du risque lié aux prestataires tiers TIC", 28, 1, ""},
	{"Art.28(2)", "Stratégie relative au risque lié aux prestataires tiers TIC", 28, 2, ""},
	{"Art.28(3)", "Registre d'informations sur les accords contractuels", 28, 3, ""},
	{"Art.28(4)", "Évaluations préalables à la conclusion d'un accord", 28, 4, ""},
	{"Art.28(5)", "Normes de sécurité de l'information des prestataires", 28, 5, ""},
	{"Art.28(6)", "Fréquence des audits et inspections des prestataires", 28, 6, ""},
	{"Art.28(7)", "Motifs de résiliation des accords contractuels", 28, 7, ""},
	{"Art.28(8)", "Stratégies de sortie", 28, 8, ""},
	{"Art.29(1)", "Évaluation du risque de concentration", 29, 1, ""},
	{"Art.29(2)", "Sous-traitance des fonctions critiques ou importantes", 29, 2, ""},
	{"Art.30(1)", "Droits et obligations consignés par écrit", 30, 1, ""},
	{"Art.30(2)", "Clauses contractuelles minimales", 30, 2, ""},
	{"Art.30(3)", "Clauses pour les fonctions critiques ou importantes", 30, 3, ""},
	{"Art.30(4)", "Clauses contractuelles types", 30, 4, ""},

	// Chapitre VI — Partage d'informations
	{"Art.45", "Partage d'informations et de renseignements sur les cybermenaces", 45, 1, ""},
}

// doraExcluded are the provisions inside Art. 5-30 that address the European
// Supervisory Authorities, the competent authorities or the Member States,
// not the financial entity. A financial entity cannot evidence or close them,
// so they are not rows of its register.
var doraExcluded = []euExclusion{
	{11, []int{11}, "mandat aux AES (normes techniques de réglementation)"},
	{15, nil, "mandat aux AES (normes techniques de réglementation)"},
	{16, []int{3}, "mandat aux AES (normes techniques de réglementation)"},
	{18, []int{3, 4}, "mandat aux AES (normes techniques de réglementation)"},
	{19, []int{6, 7, 8}, "obligations de l'autorité compétente, des AES et de la BCE"},
	{20, nil, "mandat aux AES (contenu et modèles des notifications)"},
	{21, nil, "rapport des AES sur la centralisation des notifications"},
	{22, nil, "retour d'information des autorités et rapport des AES"},
	{26, []int{7, 9, 10, 11}, "attestation et désignation par les autorités ; mandat aux AES"},
	{28, []int{9, 10}, "mandat aux AES (normes techniques)"},
	{30, []int{5}, "mandat aux AES (normes techniques de réglementation)"},
}
