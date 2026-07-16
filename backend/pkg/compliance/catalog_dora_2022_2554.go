// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package compliance

// DORA — Règlement (UE) 2022/2554 sur la résilience opérationnelle numérique du
// secteur financier. Structuré autour de cinq piliers : gestion du risque TIC,
// gestion/notification des incidents, tests de résilience, risque lié aux tiers
// prestataires TIC, et partage d'informations. Modélisé au niveau des articles
// clés. Les numéros d'articles sont la structure publique du règlement (fiables) ;
// les descriptions sont des résumés originaux, pas le texte officiel. Vérifier
// contre le Règlement (UE) 2022/2554 avant un audit.

func init() {
	register(Catalog{
		Key:         "dora-2022-2554",
		Name:        "DORA (UE 2022/2554)",
		Version:     "2022",
		Description: "Digital Operational Resilience Act — résilience opérationnelle numérique du secteur financier : risque TIC, incidents, tests, tiers prestataires et partage d'informations.",
		Available:   true,
		Controls:    dora20222554Controls,
	})
}

const doraSource = "DORA (UE) 2022/2554, art. "

var dora20222554Controls = []CatalogControl{
	// Pilier 1 — Gestion du risque lié aux TIC (Chapitre II)
	{"Art.5", "Gouvernance et organisation", "L'organe de direction définit, approuve et supervise le cadre de gestion du risque lié aux TIC et en porte la responsabilité finale.", doraSource + "5"},
	{"Art.6", "Cadre de gestion du risque lié aux TIC", "Disposer d'un cadre solide, documenté et réexaminé, couvrant les stratégies, politiques, procédures et outils de protection des actifs informationnels et TIC.", doraSource + "6"},
	{"Art.7", "Systèmes, protocoles et outils TIC", "Utiliser et maintenir des systèmes, protocoles et outils TIC fiables, dotés de capacités suffisantes et technologiquement résilients.", doraSource + "7"},
	{"Art.8", "Identification", "Identifier, classer et documenter les fonctions métier, actifs informationnels et actifs TIC ainsi que leurs interdépendances.", doraSource + "8"},
	{"Art.9", "Protection et prévention", "Mettre en œuvre des politiques et mesures de sécurité (contrôle d'accès, chiffrement, gestion des changements) pour protéger les systèmes TIC.", doraSource + "9"},
	{"Art.10", "Détection", "Mettre en place des mécanismes de détection rapide des activités anormales et des incidents potentiels liés aux TIC.", doraSource + "10"},
	{"Art.11", "Réponse et rétablissement", "Disposer de politiques de continuité des activités et de plans de réponse et de rétablissement TIC testés et à jour.", doraSource + "11"},
	{"Art.12", "Sauvegarde, restauration et rétablissement", "Définir des politiques et procédures de sauvegarde et des méthodes de restauration/rétablissement préservant l'intégrité des données.", doraSource + "12"},
	{"Art.13", "Apprentissage et évolution", "Recueillir les enseignements des incidents et tests pour faire évoluer le cadre de gestion du risque TIC et la sensibilisation.", doraSource + "13"},

	// Pilier 2 — Gestion, classification et notification des incidents (Chapitre III)
	{"Art.17", "Processus de gestion des incidents TIC", "Définir et mettre en œuvre un processus de détection, gestion et notification des incidents liés aux TIC.", doraSource + "17"},
	{"Art.18", "Classification des incidents et cybermenaces", "Classer les incidents liés aux TIC et évaluer leur importance selon les critères prévus (clients affectés, durée, portée géographique, pertes de données…).", doraSource + "18"},
	{"Art.19", "Notification des incidents majeurs", "Notifier les incidents majeurs liés aux TIC à l'autorité compétente selon les délais et modèles prescrits.", doraSource + "19"},

	// Pilier 3 — Tests de résilience opérationnelle numérique (Chapitre IV)
	{"Art.24", "Programme de tests de résilience", "Établir un programme de tests de résilience opérationnelle numérique proportionné, sain et complet.", doraSource + "24"},
	{"Art.25", "Tests des outils et systèmes TIC", "Tester régulièrement les outils et systèmes TIC (analyses de vulnérabilités, tests de sécurité, tests de continuité) et remédier aux faiblesses.", doraSource + "25"},
	{"Art.26", "Tests avancés (TLPT)", "Réaliser des tests de pénétration fondés sur la menace (Threat-Led Penetration Testing) pour les entités désignées.", doraSource + "26"},

	// Pilier 4 — Gestion du risque lié aux tiers prestataires TIC (Chapitre V)
	{"Art.28", "Principes de gestion du risque tiers TIC", "Gérer le risque lié aux prestataires tiers de services TIC dans le cadre global de gestion du risque, avec une stratégie et un registre d'information.", doraSource + "28"},
	{"Art.29", "Concentration du risque", "Évaluer le risque de concentration lié au recours à des prestataires tiers de services TIC, y compris la sous-traitance en chaîne.", doraSource + "29"},
	{"Art.30", "Dispositions contractuelles clés", "Encadrer les prestations TIC par des contrats comportant les clauses obligatoires (accès, audit, résiliation, niveaux de service, sécurité).", doraSource + "30"},

	// Pilier 5 — Partage d'informations (Chapitre VI)
	{"Art.45", "Partage d'informations sur les cybermenaces", "Participer, le cas échéant, à des accords de partage d'informations et de renseignements sur les cybermenaces au sein de communautés de confiance.", doraSource + "45"},
}
