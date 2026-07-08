// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package compliance

// BCEAO / UEMOA — Systèmes et moyens de paiement.
//
// Contrôles dérivés du Règlement n°15/2002/CM/UEMOA du 19 septembre 2002 relatif aux
// systèmes de paiement dans les États membres de l'UEMOA, complété par ses instructions
// d'application (surveillance, monnaie électronique, centralisation des incidents de
// paiement) et l'Avis n°001-09-2012 sur les relevés de compte électroniques.
//
// Les descriptions ci-dessous sont des reformulations synthétiques rédigées pour ce
// produit ; elles résument l'obligation portée par chaque article et ne reproduisent pas
// le texte réglementaire mot pour mot. La SourceReference cite l'article précis dont le
// contrôle découle. Ce catalogue mérite encore une passe de revue par un juriste bancaire
// UEMOA avant usage dans un audit réel — les codes d'article sont fiables (relevés dans le
// texte fourni), la formulation reste à valider.

func init() {
	register(Catalog{
		Key:         "bceao",
		Name:        "BCEAO / UEMOA — Systèmes et moyens de paiement",
		Version:     "2002",
		Description: "Obligations des banques et établissements assujettis au titre du Règlement n°15/2002/CM/UEMOA et de ses instructions (chèque, carte, virement électronique, monnaie électronique, incidents de paiement).",
		Available:   true,
		Controls:    bceaoControls,
	})
}

const (
	bceaoReg  = "Règlement n°15/2002/CM/UEMOA, art. "
	bceaoME   = "Instruction n°008-05-2015 (monnaie électronique), art. "
	bceaoSurv = "Instruction n°127-07-08 (surveillance des systèmes de paiement), art. "
	bceaoCIP  = "Instruction n°009/07/RSP/2010 (centralisation des incidents de paiement), art. "
	bceaoErel = "Avis n°001-09-2012 (transmission électronique des relevés de compte)"
)

var bceaoControls = []CatalogControl{
	// --- Preuve et signature électroniques ---
	{"BCEAO-PREUVE-1", "Admissibilité et intégrité de l'écrit électronique", "Garantir que tout écrit sous forme électronique utilisé dans les systèmes de paiement permet d'identifier dûment son auteur et est établi et conservé dans des conditions qui en garantissent l'intégrité.", bceaoReg + "19"},
	{"BCEAO-PREUVE-2", "Conservation des messages de données (5 ans)", "Conserver les documents électroniques pendant cinq ans, sous une forme accessible et non altérable, avec les informations d'origine, de destination, de date et d'heure d'envoi ou de réception.", bceaoReg + "20"},
	{"BCEAO-SIG-1", "Signature électronique sécurisée", "N'accorder la présomption de fiabilité qu'aux signatures électroniques sécurisées établies au moyen d'un dispositif sécurisé de création et vérifiées par un certificat qualifié.", bceaoReg + "21"},
	{"BCEAO-SIG-2", "Dispositif sécurisé de création de signature", "N'employer que des dispositifs de création de signature certifiés conformes aux exigences réglementaires par un organisme agréé par la Banque Centrale.", bceaoReg + "23"},
	{"BCEAO-CERT-1", "Exigences des prestataires de services de certification", "S'assurer que tout prestataire de certification fournit un service d'annuaire et de révocation, un horodatage précis, un personnel qualifié et des procédures de sécurité garantissant la fiabilité des certificats.", bceaoReg + "27"},

	// --- Ouverture de compte et bancarisation ---
	{"BCEAO-KYC-1", "Vérification d'identité à l'ouverture de compte", "S'assurer, préalablement à l'ouverture d'un compte de dépôt, de l'identité et de l'adresse du demandeur sur présentation d'un document officiel en cours de validité ; pour les personnes morales, exiger l'extrait du Registre du Commerce et du Crédit Mobilier.", bceaoReg + "43"},
	{"BCEAO-COMPTE-1", "Service bancaire minimum", "Garantir au titulaire d'un compte de dépôt le service bancaire minimum : gestion du compte, mise à disposition d'un instrument de paiement sécurisé, virements, prélèvements et relevés.", bceaoReg + "10"},
	{"BCEAO-RELEVE-1", "Relevés de compte périodiques", "Adresser au client un relevé de compte au moins une fois par mois pour les comptes assortis d'un chéquier.", bceaoReg + "43"},

	// --- Chèque ---
	{"BCEAO-CHQ-1", "Consultation du fichier des incidents avant délivrance de chéquier", "Consulter le fichier des incidents de paiement (CIP) avant toute délivrance de formules de chèques afin de s'assurer que le demandeur n'est pas frappé d'une interdiction bancaire ou judiciaire.", bceaoReg + "45"},
	{"BCEAO-CHQ-2", "Traitement d'un chèque sans provision — avertissement", "En cas de rejet d'un chèque pour défaut ou insuffisance de provision, délivrer une attestation de rejet, enregistrer l'incident, adresser une lettre d'avertissement au titulaire et déclarer l'incident à la Banque Centrale.", bceaoReg + "114"},
	{"BCEAO-CHQ-3", "Interdiction bancaire et injonction de restitution", "À défaut de régularisation dans le délai réglementaire, signifier l'interdiction bancaire d'émettre des chèques (5 ans) et enjoindre au titulaire la restitution des formules de chèques en sa possession et celle de ses mandataires.", bceaoReg + "115"},
	{"BCEAO-CHQ-4", "Certificat de non-paiement", "Délivrer un certificat de non-paiement au porteur à défaut de paiement du chèque dans les trente jours de la première présentation.", bceaoReg + "123"},
	{"BCEAO-CHQ-5", "Déclaration et centralisation des incidents de paiement", "Déclarer à la Banque Centrale les refus de paiement, régularisations, interdictions, oppositions et comptes clôturés dans les conditions et délais fixés pour la Centrale des Incidents de Paiement.", bceaoCIP + "8"},

	// --- Carte bancaire et paiement électronique ---
	{"BCEAO-CARTE-1", "Vérification préalable à la délivrance d'une carte de paiement", "Avant de délivrer une carte de paiement, s'assurer que le demandeur n'a pas fait l'objet d'un retrait de carte, d'une interdiction bancaire ou judiciaire d'émettre des chèques, ou d'une condamnation pour fraude aux instruments de paiement.", bceaoReg + "139"},
	{"BCEAO-CARTE-2", "Retrait de carte en cas d'utilisation abusive", "En cas d'utilisation abusive, enjoindre au titulaire de restituer sa carte dans les quatre jours ouvrables et informer la Banque Centrale de la décision de retrait.", bceaoReg + "140"},
	{"BCEAO-CARTE-3", "Information des porteurs sur l'usage et les sanctions", "Informer toute personne qui en fait la demande des conditions d'utilisation des cartes et autres instruments électroniques de paiement, ainsi que des sanctions encourues en cas d'utilisation abusive.", bceaoReg + "137"},
	{"BCEAO-CARTE-4", "Confidentialité du code confidentiel", "Mettre en place, chez les commerçants acceptants, une installation permettant la composition du code confidentiel à l'abri des regards et l'occultation du numéro de carte sur les factures.", bceaoReg + "141"},
	{"BCEAO-CARTE-5", "Opposition au paiement par carte", "Traiter les oppositions au paiement pour perte, vol, utilisation frauduleuse ou procédure collective, y compris par appel téléphonique confirmé sous 24 heures, et en informer la Banque Centrale.", bceaoReg + "142"},

	// --- Virement électronique ---
	{"BCEAO-VIR-1", "Identification du destinataire du virement", "Veiller à la bonne identification du destinataire du virement avant la transmission de l'ordre de paiement par message de données.", bceaoReg + "133"},
	{"BCEAO-VIR-2", "Obligation générale de sécurité de l'expéditeur", "Prendre toutes les précautions techniques nécessaires à la sécurisation des données transmises au moment de l'émission de l'ordre de paiement.", bceaoReg + "134"},

	// --- Effets de commerce ---
	{"BCEAO-EFFET-1", "Déclaration des rejets d'effets de commerce", "Déclarer à la Banque Centrale, avec attestation de rejet au bénéficiaire et avis de non-paiement au débiteur, tout rejet de lettre de change acceptée ou de billet à ordre domicilié pour défaut ou insuffisance de provision.", bceaoReg + "239"},

	// --- Monnaie électronique (Instruction n°008-05-2015) ---
	{"BCEAO-ME-1", "Agrément ou autorisation préalable d'émission", "N'exercer une activité d'émission de monnaie électronique qu'après agrément (établissement de monnaie électronique) ou autorisation préalable de la Banque Centrale, ou information préalable pour les banques et établissements financiers de paiement.", bceaoME + "8"},
	{"BCEAO-ME-2", "Capital social minimum", "Justifier d'un capital social minimum de trois cents millions de FCFA intégralement libéré, ou de fonds propres et dépôts au moins équivalents pour un système financier décentralisé.", bceaoME + "11"},
	{"BCEAO-ME-3", "Cantonnement et protection des fonds reçus", "Domicilier sans délai les fonds représentant la contrepartie de la monnaie électronique émise dans un compte dédié, distinctement identifié, faisant l'objet d'une réconciliation quotidienne avec l'encours.", bceaoME + "32"},
	{"BCEAO-ME-4", "Couverture permanente de l'encours", "Maintenir en permanence des montants reçus supérieurs ou égaux à l'encours de la monnaie électronique en circulation.", bceaoME + "33"},
	{"BCEAO-ME-5", "Plafonnement des avoirs et rechargements", "Respecter les plafonds réglementaires : deux millions de FCFA d'avoirs par client identifié, dix millions de rechargements mensuels, et deux cent mille FCFA mensuels pour un détenteur non identifié, sauf autorisation expresse.", bceaoME + "31"},
	{"BCEAO-ME-6", "Lutte contre le blanchiment et le financement du terrorisme", "Mettre en place un système automatisé de surveillance des transactions, un dispositif spécifique LBC/FT et conserver les données pendant dix ans, avec déclaration des opérations suspectes à la CENTIF.", bceaoME + "26"},
	{"BCEAO-ME-7", "Sécurité technique et audit périodique", "Assurer disponibilité, intégrité, confidentialité, authenticité et non-répudiation des transactions, une piste d'audit sur dix ans, et faire attester ces exigences par un audit externe au moins tous les trois ans.", bceaoME + "7"},
	{"BCEAO-ME-8", "Remboursement à la valeur nominale", "Rembourser à tout moment, à la valeur nominale en FCFA, les unités de monnaie électronique non utilisées, dans un délai n'excédant pas trois jours ouvrés.", bceaoME + "35"},
	{"BCEAO-ME-9", "Identification des détenteurs", "Identifier les clients sur présentation d'un document officiel en cours de validité, préalablement à l'ouverture d'un compte de monnaie électronique, et en conserver copie.", bceaoME + "27"},
	{"BCEAO-ME-10", "Reporting périodique à la Banque Centrale", "Communiquer à la BCEAO le contrôle mensuel de l'encours (dans les quinze jours) et un rapport trimestriel de surveillance des activités de monnaie électronique.", bceaoME + "36"},
	{"BCEAO-ME-11", "Responsabilité à l'égard des distributeurs", "Demeurer responsable, vis-à-vis des clients et des tiers, de l'intégrité, de la fiabilité, de la sécurité, de la confidentialité et de la traçabilité des opérations réalisées par chaque distributeur.", bceaoME + "18"},

	// --- Surveillance des systèmes de paiement (Instruction n°127-07-08) ---
	{"BCEAO-SURV-1", "Conformité aux normes de sécurité des systèmes de paiement", "Maintenir la conformité des systèmes de paiement aux normes et standards internationaux en matière juridique, financière, technique, opérationnelle et d'efficacité.", bceaoSurv + "5"},
	{"BCEAO-SURV-2", "Notification des incidents à la Banque Centrale", "Communiquer à la BCEAO, dans les vingt-quatre heures suivant sa survenance, tout incident interrompant un système de paiement pour une durée supérieure à une heure.", bceaoSurv + "6"},

	// --- Relevés de compte électroniques (Avis n°001-09-2012) ---
	{"BCEAO-EREL-1", "Sécurisation des relevés de compte électroniques", "Obtenir le consentement écrit préalable du client et garantir, sur un espace sécurisé, l'identification de l'émetteur, la confidentialité, l'intégrité, la non-répudiation, l'authentification du client et l'archivage des relevés de compte électroniques.", bceaoErel},
}
