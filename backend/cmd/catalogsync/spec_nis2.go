// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

package main

// NIS2 — the rows a person chose.
//
// Art. 20 (governance), every measure of Art. 21(2)(a)-(j) with the paragraphs
// around them, and the reporting obligations of Art. 23 that fall on the
// entity: paragraphs 1-4, the 24-hour derogation for trust service providers
// (the second subparagraph of paragraph 4), and paragraph 7, under which the
// authority may require the entity to inform the public. Paragraphs 5, 6 and
// 8 to 11 address CSIRTs, authorities, single points of contact and the
// Commission.
//
// Codes "Art.20" and "Art.21(a)" … "Art.21(j)" keep the codes the earlier
// catalogue used — "Art.21(a)" is Art. 21(2)(a) — so a tenant who imported it
// does not get them twice. The source reference of each row is exact.
//
// The annex of Implementing Regulation (EU) 2024/2690 is generated section by
// section ("1.1", "1.2", …): its own headings name the rows.
var nis2Rows = []euRow{
	{"Art.20", "Gouvernance : approbation, supervision et formation des organes de direction", 20, 0, ""},
	{"Art.21(1)", "Mesures de gestion des risques appropriées et proportionnées", 21, 1, ""},
	{"Art.21(a)", "Politiques d'analyse des risques et de sécurité des systèmes d'information", 21, 2, "a"},
	{"Art.21(b)", "Gestion des incidents", 21, 2, "b"},
	{"Art.21(c)", "Continuité des activités et gestion des crises", 21, 2, "c"},
	{"Art.21(d)", "Sécurité de la chaîne d'approvisionnement", 21, 2, "d"},
	{"Art.21(e)", "Sécurité de l'acquisition, du développement et de la maintenance", 21, 2, "e"},
	{"Art.21(f)", "Évaluation de l'efficacité des mesures", 21, 2, "f"},
	{"Art.21(g)", "Cyberhygiène et formation à la cybersécurité", 21, 2, "g"},
	{"Art.21(h)", "Cryptographie et, le cas échéant, chiffrement", 21, 2, "h"},
	{"Art.21(i)", "Sécurité des ressources humaines, contrôle d'accès et gestion des actifs", 21, 2, "i"},
	{"Art.21(j)", "Authentification multifacteur ou continue et communications sécurisées, selon les besoins", 21, 2, "j"},
	{"Art.21(3)", "Vulnérabilités propres aux fournisseurs et prestataires", 21, 3, ""},
	{"Art.21(4)", "Mesures correctives en cas de non-conformité", 21, 4, ""},
	{"Art.23(1)", "Notification des incidents importants", 23, 1, ""},
	{"Art.23(2)", "Communication aux destinataires des services en cas de cybermenace importante", 23, 2, ""},
	{"Art.23(3)", "Critères de l'incident important", 23, 3, ""},
	{"Art.23(4)(a)", "Alerte précoce sous 24 heures", 23, 4, "a"},
	{"Art.23(4)(b)", "Notification d'incident sous 72 heures", 23, 4, "b"},
	{"Art.23(4)(c)", "Rapport intermédiaire sur demande", 23, 4, "c"},
	{"Art.23(4)(d)", "Rapport final sous un mois", 23, 4, "d"},
	{"Art.23(4)(e)", "Rapport d'avancement pour un incident en cours", 23, 4, "e"},
	{"Art.23(4)(al.2)", "Notification sous 24 heures (prestataires de services de confiance)", 23, 4, "al.2"},
	{"Art.23(7)", "Information du public sur un incident important, à la demande de l'autorité", 23, 7, ""},
}
