// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package compliance

// COBAC (CEMAC) — Contrôle interne des établissements de crédit.
//
// Contrôles dérivés du Règlement COBAC R-2016/04 relatif au contrôle interne dans les
// établissements de crédit et les holdings financières, adopté le 8 mars 2016 par la
// Commission Bancaire de l'Afrique Centrale et entré en vigueur le 1er janvier 2017.
//
// Ce règlement structure le dispositif de contrôle interne (contrôle permanent, audit
// interne, conformité, gestion des risques), l'organisation comptable, les systèmes de
// mesure et de surveillance des risques et le reporting — il s'adosse donc naturellement à
// un produit GRC. Les descriptions sont des reformulations synthétiques rédigées pour ce
// produit ; la SourceReference cite l'article précis. À valider par un professionnel avant
// usage en audit réel : les codes d'article sont fiables (relevés dans le texte fourni).

func init() {
	register(Catalog{
		Key:         "cobac",
		Name:        "COBAC (CEMAC) — Contrôle interne",
		Version:     "R-2016/04",
		Description: "Exigences minimales de contrôle interne, de gestion des risques, de conformité et de reporting pour les établissements de crédit et holdings financières de la CEMAC (Règlement COBAC R-2016/04).",
		Available:   true,
		Controls:    cobacControls,
	})
}

const cobac = "Règlement COBAC R-2016/04, art. "

var cobacControls = []CatalogControl{
	// --- Dispositif d'ensemble ---
	{"COBAC-CI-1", "Système de contrôle interne à deux niveaux", "Organiser un système de contrôle interne comprenant un contrôle permanent (deux échelons) et un contrôle périodique (audit interne), couvrant la vérification des opérations, la maîtrise des risques et la fiabilité de l'information comptable et financière.", cobac + "3"},
	{"COBAC-CI-2", "Adéquation du dispositif à la taille et aux risques", "Adapter l'ensemble des dispositifs de contrôle interne à la nature, au volume et aux risques des activités, à la taille et aux implantations de l'établissement.", cobac + "6"},
	{"COBAC-CI-3", "Contrôle interne sur base consolidée ou combinée", "Mettre en œuvre les moyens de s'assurer du respect des dispositions de contrôle interne au sein des entités contrôlées et de la maîtrise des risques au niveau consolidé ou combiné.", cobac + "7"},
	{"COBAC-CI-4", "Séparation des tâches et indépendance des unités", "Établir une stricte indépendance entre les unités chargées de l'initiation, de l'exécution, de la validation, de la comptabilisation et du contrôle de chaque opération, et identifier les zones de conflits d'intérêts.", cobac + "8"},
	{"COBAC-CI-5", "Culture de contrôle interne", "Promouvoir, à tous les niveaux du personnel, une culture de contrôle interne à laquelle chaque agent comprend son rôle et est pleinement associé.", cobac + "9"},

	// --- Gouvernance ---
	{"COBAC-GOV-1", "Surveillance du contrôle interne par l'organe délibérant", "Faire assurer par l'organe délibérant la mise en place et le suivi du système de contrôle interne, l'approbation de la politique de gestion des risques et la fixation de limites, avec examen au moins annuel des résultats du contrôle interne.", cobac + "14"},
	{"COBAC-GOV-2", "Charte de contrôle interne", "Élaborer une charte de contrôle interne précisant les dispositifs, l'indépendance vis-à-vis des unités opérationnelles et les niveaux de responsabilité, approuvée par l'organe délibérant et revue au moins tous les trois ans.", cobac + "22"},
	{"COBAC-RISK-1", "Dispositif de gestion des risques par type de risque", "Mettre en œuvre, pour chaque risque, un système d'identification, d'analyse, de mesure, de surveillance et de maîtrise, incluant une cartographie des risques au regard des facteurs internes et externes.", cobac + "10"},
	{"COBAC-RISK-2", "Analyse prospective des nouveaux produits et opérations", "Analyser en amont et de façon prospective les risques encourus avant de lancer de nouveaux produits, de modifier significativement un produit existant ou de réaliser des opérations de croissance ou exceptionnelles.", cobac + "11"},

	// --- Comité d'audit ---
	{"COBAC-AUDIT-1", "Comité d'audit obligatoire", "Mettre en place un comité d'audit chargé d'assister l'organe délibérant dans la supervision du système de contrôle interne et l'appréciation de la fiabilité de l'information financière.", cobac + "25"},
	{"COBAC-AUDIT-2", "Composition et indépendance du comité d'audit", "Constituer le comité d'audit d'au moins trois membres, présidé par un membre de l'organe délibérant et comprenant un administrateur indépendant, à l'exclusion de toute personne exerçant des responsabilités exécutives.", cobac + "30"},
	{"COBAC-AUDIT-3", "Notification des membres du comité d'audit", "Notifier au Secrétaire Général de la COBAC la nomination des membres du comité d'audit avant sa prise d'effet, avec les pièces attestant de leurs compétences.", cobac + "31"},

	// --- Contrôle permanent ---
	{"COBAC-PERM-1", "Dispositif de contrôle permanent", "Doter l'établissement d'un contrôle permanent garantissant la régularité, la sécurité et la validation des opérations, l'adéquation des procédures de mesure et de limitation des risques et le suivi des risques.", cobac + "36"},
	{"COBAC-PERM-2", "Responsable du contrôle permanent", "Désigner un ou plusieurs responsables du contrôle permanent, n'effectuant au niveau le plus élevé aucune opération commerciale, financière ou comptable.", cobac + "40"},

	// --- Audit interne ---
	{"COBAC-AI-1", "Fonction d'audit interne indépendante", "Doter l'établissement d'un audit interne fonctionnant de manière indépendante, rattaché à l'organe délibérant et au comité d'audit, chargé d'évaluer périodiquement l'efficacité des processus de gestion des risques et de gouvernance.", cobac + "45"},
	{"COBAC-AI-2", "Charte d'audit interne", "Élaborer une charte d'audit interne définissant la position, les pouvoirs, les objectifs, les responsabilités de la fonction et les modalités de communication de ses résultats, communiquée au Secrétariat Général de la Commission Bancaire.", cobac + "44"},
	{"COBAC-AI-3", "Plan d'audit pluriannuel", "Préparer un plan d'audit pluriannuel approuvé par le comité d'audit, couvrant l'ensemble des activités, fonctions et implantations, y compris les filiales, dans un délai maximal de trois ans.", cobac + "48"},
	{"COBAC-AI-4", "Nomination du responsable de l'audit interne", "Faire nommer et révoquer le responsable de l'audit interne par l'organe délibérant, sur proposition de l'organe exécutif et après approbation du comité d'audit, et notifier cette nomination à la COBAC.", cobac + "49"},

	// --- Conformité ---
	{"COBAC-CONF-1", "Dispositif de contrôle de la conformité", "Doter l'établissement d'une structure de contrôle de la conformité indépendante des entités opérationnelles, chargée du suivi et de la maîtrise du risque de non-conformité.", cobac + "54"},
	{"COBAC-CONF-2", "Responsable de la conformité", "Désigner un responsable chargé de la cohérence et de l'efficacité du contrôle du risque de non-conformité, dont l'identité est communiquée au Secrétariat Général de la COBAC.", cobac + "55"},
	{"COBAC-CONF-3", "Examen de conformité des produits nouveaux", "Soumettre les produits nouveaux et les transformations significatives à une procédure d'approbation préalable incluant un avis écrit du responsable de la conformité.", cobac + "121"},

	// --- Gestion des risques ---
	{"COBAC-GR-1", "Comité et responsable de la gestion des risques", "Instituer un comité des risques et désigner un responsable de la gestion des risques, notifié à la Commission Bancaire, n'effectuant aucune opération commerciale, financière ou comptable.", cobac + "57"},
	{"COBAC-GR-2", "Simulation de crise sur les risques significatifs", "Faire procéder par le responsable de la gestion des risques, au moins une fois par an, à une simulation de crise sur les risques les plus significatifs, communiquée au Secrétariat Général de la COBAC.", cobac + "61"},

	// --- Externalisation ---
	{"COBAC-EXT-1", "Encadrement contractuel de l'externalisation", "Encadrer toute activité externalisée par un contrat écrit, une politique formalisée de contrôle des prestataires et le maintien de l'entière maîtrise de l'activité, sans réduction des responsabilités des fonctions de contrôle.", cobac + "65"},
	{"COBAC-EXT-2", "Localisation des données dans l'État du siège", "Conserver et rendre accessibles en permanence, sur le territoire de l'État du siège dans la CEMAC, les serveurs, dossiers physiques, procédures et archives de l'établissement.", cobac + "65"},
	{"COBAC-EXT-3", "Accord préalable de la COBAC pour l'externalisation", "Soumettre l'externalisation des opérations à l'accord préalable du Secrétaire Général de la COBAC, en fournissant tout élément relatif à la décision envisagée.", cobac + "69"},

	// --- Organisation comptable et systèmes d'information ---
	{"COBAC-COMPTA-1", "Contrôle comptable dédié", "Se doter d'un dispositif formel de contrôle de la comptabilité comprenant une fonction dédiée assurant la fiabilité et l'exhaustivité des données comptables et financières.", cobac + "75"},
	{"COBAC-COMPTA-2", "Piste d'audit comptable", "Prévoir une piste d'audit permettant de reconstituer les opérations dans l'ordre chronologique, de justifier toute information par une pièce d'origine et d'expliquer l'évolution des soldes d'un arrêté à l'autre.", cobac + "76"},
	{"COBAC-SI-1", "Sécurité et contrôle du système d'information", "Déterminer un niveau de sécurité informatique adapté aux métiers, disposer de procédures de secours informatique et préserver en toutes circonstances l'intégrité et la confidentialité des opérations.", cobac + "82"},
	{"COBAC-PCA-1", "Cohérence avec le plan de continuité d'activité", "S'assurer que le système de contrôle interne est cohérent avec le plan de continuité d'activité exigé par la réglementation.", cobac + "131"},

	// --- Mesure des risques : crédit ---
	{"COBAC-CRED-1", "Sélection et mesure du risque de crédit", "Disposer d'une procédure de sélection des risques de crédit et d'un système de mesure identifiant de manière centralisée les risques à l'égard des parties liées et appréhendant les niveaux de risque par notation interne.", cobac + "92"},
	{"COBAC-CRED-2", "Analyse trimestrielle de la qualité des engagements", "Procéder au moins trimestriellement à l'analyse de l'évolution de la qualité des engagements, aux reclassements nécessaires et à la détermination des niveaux appropriés de provisionnement.", cobac + "98"},
	{"COBAC-CRED-3", "Simulation de crise sur le risque de crédit", "Procéder au moins une fois par an à une simulation de crise sur le risque de crédit, dont les résultats sont communiqués au Secrétariat Général de la COBAC.", cobac + "99"},

	// --- Mesure des risques : liquidité ---
	{"COBAC-LIQ-1", "Dispositif d'évaluation du risque de liquidité", "Se doter d'un dispositif permettant à tout moment d'évaluer le risque de liquidité, de couvrir les exigibilités par les disponibilités et de suivre les échéanciers des engagements.", cobac + "100"},
	{"COBAC-LIQ-2", "Plan de financement d'urgence", "Mettre formellement en place un plan de financement d'urgence exposant les stratégies de résolution des pénuries de liquidité, régulièrement testé et mis à jour.", cobac + "103"},
	{"COBAC-LIQ-3", "Simulations de crise de liquidité", "Effectuer périodiquement des simulations de crise sur divers scénarios de tensions de liquidité et utiliser les résultats pour adapter les stratégies et le plan de financement d'urgence.", cobac + "104"},

	// --- Mesure des risques : opérationnel et marché ---
	{"COBAC-OP-1", "Gestion du risque opérationnel", "Identifier et évaluer le risque opérationnel inhérent aux produits, activités, processus et systèmes, mettre en œuvre un suivi régulier des profils de risque et des mesures d'atténuation.", cobac + "111"},
	{"COBAC-MKT-1", "Dispositif de gestion du risque de marché", "Se doter d'un dispositif d'identification, d'évaluation et de maîtrise du risque de marché (taux, portefeuille de négociation, change), avec enregistrement quotidien des opérations et mesure de l'exposition.", cobac + "116"},
	{"COBAC-MKT-2", "Simulation de crise sur le risque de marché", "Procéder au moins une fois par an à une simulation de crise sur le risque de marché, communiquée au Secrétariat Général de la COBAC.", cobac + "119"},

	// --- Limites et surveillance ---
	{"COBAC-LIM-1", "Système de limites globales et opérationnelles", "Mettre en place des systèmes de surveillance et de maîtrise des risques faisant apparaître des limites globales et opérationnelles, régulièrement revues, ainsi que les procédures d'alerte des organes exécutif et délibérant.", cobac + "132"},
	{"COBAC-LIM-2", "Contrôle du respect des limites", "Contrôler le respect des limites de façon régulière par le contrôle permanent et inopinée par l'audit interne, avec rapport aux organes exécutif et délibérant expliquant les dépassements.", cobac + "136"},

	// --- Reporting ---
	{"COBAC-REP-1", "Reporting interne trimestriel sur les limites", "Définir des procédures d'information au moins trimestrielle de l'organe exécutif sur le respect des limites de risque, notamment lorsque les limites globales sont susceptibles d'être atteintes.", cobac + "143"},
	{"COBAC-REP-2", "Rapport annuel de contrôle interne à la COBAC", "Élaborer au moins une fois par an un rapport sur les conditions du contrôle interne et la gestion des risques, adressé au Secrétariat Général de la COBAC avant le 30 avril suivant la fin de l'exercice.", cobac + "150"},
	{"COBAC-REP-3", "Rapport annuel de conformité à la COBAC", "Faire établir par le responsable de la conformité un rapport annuel sur ses activités, adressé au Secrétariat Général de la Commission Bancaire au plus tard le 31 mars suivant la fin de l'exercice.", cobac + "147"},
	{"COBAC-FP-1", "Évaluation de l'adéquation des fonds propres internes", "Mettre en place des systèmes et procédures pour évaluer en permanence l'adéquation des fonds propres internes à la nature et à l'étendue des risques et maintenir un niveau de fonds propres jugé approprié.", cobac + "152"},
}
