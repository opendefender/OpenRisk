// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package compliance

// ANTIC (Cameroun) — Cybersécurité et cybercriminalité.
//
// Contrôles dérivés de la Loi n°2010/012 du 21 décembre 2010 relative à la cybersécurité
// et la cybercriminalité au Cameroun, dont la régulation des activités de sécurité
// électronique est confiée à l'Agence Nationale des Technologies de l'Information et de la
// Communication (ANTIC), qui est également l'Autorité de Certification Racine.
//
// Ce catalogue retient les obligations à la charge des opérateurs de réseaux, exploitants
// de systèmes d'information, fournisseurs d'accès/de contenus et autorités de
// certification. Les descriptions sont des reformulations synthétiques rédigées pour ce
// produit ; la SourceReference cite l'article précis. À valider par un juriste avant usage
// en audit réel : les codes d'article sont fiables (relevés dans le texte fourni), la
// formulation reste à confirmer.

func init() {
	register(Catalog{
		Key:         "antic-cm",
		Name:        "ANTIC (Cameroun) — Cybersécurité",
		Version:     "2010",
		Description: "Obligations de cybersécurité et de protection des réseaux, systèmes d'information et données personnelles au titre de la Loi n°2010/012 du 21 décembre 2010 (Cameroun).",
		Available:   true,
		Controls:    anticCMControls,
	})
}

const anticCM = "Loi n°2010/012 (Cameroun), art. "

var anticCMControls = []CatalogControl{
	// --- Audit de sécurité ---
	{"ANTIC-AUDIT-1", "Audit de sécurité obligatoire", "Soumettre les réseaux de communications électroniques et les systèmes d'information des opérateurs, autorités de certification et fournisseurs de services à un audit de sécurité obligatoire.", anticCM + "13"},
	{"ANTIC-AUDIT-2", "Audit de sécurité périodique", "Faire réaliser par l'Agence un audit de sécurité et une mesure d'impact de gravité au moins une fois par an ou lorsque les circonstances l'exigent.", anticCM + "32"},

	// --- Protection des réseaux de communications électroniques ---
	{"ANTIC-RES-1", "Information des usagers sur les risques du réseau", "Prendre les mesures techniques et administratives de sécurité et informer les usagers des dangers, des risques de violation de sécurité et des moyens de sécuriser leurs communications.", anticCM + "24"},
	{"ANTIC-RES-2", "Conservation des données de connexion et de trafic (10 ans)", "Conserver les données de connexion et de trafic pendant une durée de dix ans, accessibles lors des investigations judiciaires.", anticCM + "25"},
	{"ANTIC-RES-3", "Surveillance du trafic réseau", "Installer des mécanismes de surveillance du trafic des données du réseau, sans porter atteinte aux libertés individuelles des usagers.", anticCM + "25"},

	// --- Protection des systèmes d'information ---
	{"ANTIC-SI-1", "Mesures techniques de sécurité des systèmes d'information", "Se doter de systèmes normalisés permettant d'identifier, évaluer, traiter et gérer les risques, et mettre en place des mécanismes garantissant disponibilité, intégrité, authentification, non-répudiation, confidentialité et sécurité physique.", anticCM + "26"},
	{"ANTIC-SI-2", "Protection des plateformes contre les intrusions", "Protéger les plateformes des systèmes d'information contre les rayonnements et les intrusions, notamment au moyen d'un système de détection d'intrusions, et faire viser les mécanismes par l'Agence.", anticCM + "26"},
	{"ANTIC-SI-3", "Information des usagers sur la sécurisation", "Informer les usagers des dangers des systèmes non sécurisés et leur proposer des moyens techniques de protection (contrôle parental, antivirus, anti-logiciels espions, pare-feu, détection d'intrusions, mises à jour).", anticCM + "27"},
	{"ANTIC-SI-4", "Interdiction des contenus illicites et logiciels malveillants", "Informer les utilisateurs de l'interdiction de diffuser des contenus illicites et de concevoir des logiciels trompeurs, espions, potentiellement indésirables ou frauduleux.", anticCM + "28"},
	{"ANTIC-SI-5", "Conservation des journaux des systèmes d'information (10 ans)", "Conserver les données de connexion et de trafic des systèmes d'information pendant dix ans et installer des mécanismes de surveillance du contrôle d'accès aux données.", anticCM + "29"},
	{"ANTIC-SI-6", "Révision périodique du dispositif de sécurité", "Évaluer et réviser les systèmes de sécurité et introduire les modifications appropriées aux pratiques, mesures et techniques en fonction de l'évolution des technologies.", anticCM + "30"},

	// --- Fournisseurs d'accès, de services et de contenus ---
	{"ANTIC-CONT-1", "Disponibilité des contenus et filtres de protection", "Assurer la disponibilité des contenus et des données stockées et mettre en place des filtres contre les atteintes aux données personnelles et à la vie privée des utilisateurs.", anticCM + "31"},
	{"ANTIC-ACC-1", "Information des abonnés sur la restriction d'accès", "Informer les abonnés de l'existence de moyens techniques permettant de restreindre ou de sélectionner l'accès à certains services et leur proposer au moins l'un de ces moyens.", anticCM + "33"},
	{"ANTIC-ACC-2", "Retrait prompt des contenus illicites hébergés", "En tant qu'hébergeur, agir promptement pour retirer un contenu manifestement illicite ou en rendre l'accès impossible dès la connaissance des faits.", anticCM + "34"},
	{"ANTIC-ACC-3", "Conservation des données d'identification des créateurs de contenu (10 ans)", "Conserver pendant dix ans les données permettant l'identification de toute personne ayant contribué à la création du contenu des services fournis.", anticCM + "35"},
	{"ANTIC-ACC-4", "Identification de l'éditeur de service (mentions légales)", "Mettre à la disposition du public les informations d'identification de l'éditeur du service de communications électroniques (identité ou raison sociale, adresse, directeur de publication, prestataire d'hébergement).", anticCM + "37"},

	// --- Protection de la vie privée ---
	{"ANTIC-VP-1", "Confidentialité des communications", "Assurer, en tant qu'opérateur ou exploitant, la confidentialité des communications acheminées ainsi que des données relatives au trafic.", anticCM + "42"},
	{"ANTIC-VP-2", "Responsabilité sur les contenus véhiculés", "Assumer la responsabilité des contenus véhiculés par le système d'information, notamment lorsqu'ils portent atteinte à la dignité humaine, à l'honneur ou à la vie privée.", anticCM + "43"},
	{"ANTIC-VP-3", "Interdiction d'interception sans consentement", "S'interdire d'écouter, d'intercepter, de stocker les communications et les données de trafic ou de les soumettre à toute surveillance sans le consentement des utilisateurs, sauf autorisation légale.", anticCM + "44"},
	{"ANTIC-VP-4", "Consentement préalable au stockage/accès sur l'équipement terminal", "N'accéder aux informations stockées, ou n'en stocker, dans l'équipement terminal d'une personne qu'avec son consentement préalable.", anticCM + "47"},
	{"ANTIC-VP-5", "Interdiction du spam trompeur et de l'usurpation d'identité", "S'interdire l'émission de messages de prospection dissimulant l'identité de l'émetteur ou sans adresse de désinscription valide, ainsi que l'émission de messages usurpant l'identité d'autrui.", anticCM + "48"},

	// --- Certification et signature électroniques ---
	{"ANTIC-CERT-1", "Autorisation préalable de l'activité de certification", "N'exercer l'activité de certification électronique qu'après autorisation préalable, en tant qu'autorité de certification accréditée.", anticCM + "10"},
	{"ANTIC-CERT-2", "Responsabilité et garantie financière de l'autorité de certification", "Justifier d'une garantie financière suffisante ou d'une assurance couvrant la responsabilité civile professionnelle pour le préjudice causé aux personnes s'étant fiées aux certificats qualifiés.", anticCM + "16"},
	{"ANTIC-SIG-1", "Conditions de la signature électronique avancée", "N'assimiler à la signature manuscrite qu'une signature électronique avancée liée exclusivement au signataire, sous son contrôle exclusif, dont toute modification est décelable et reposant sur un certificat qualifié.", anticCM + "18"},

	// --- Cryptographie ---
	{"ANTIC-CRYPTO-1", "Remise des conventions de déchiffrement sur réquisition", "Remettre aux officiers de police judiciaire ou agents habilités de l'Agence, sur leur demande, les conventions permettant le déchiffrement des données transformées au moyen des prestations de cryptographie fournies.", anticCM + "58"},
}
