// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package compliance

// RGPD — Règlement (UE) 2016/679 relatif à la protection des personnes physiques
// à l'égard du traitement des données à caractère personnel. Modélisé au niveau des
// articles opérationnels que le responsable de traitement/sous-traitant doit
// satisfaire. Les numéros et intitulés d'articles sont la structure publique du
// règlement (fiables) ; les descriptions sont des résumés originaux, pas le texte
// officiel. Vérifier contre le texte du Règlement (UE) 2016/679 avant un audit.
// Descriptions en français : cadre européen, marché cible francophone.

func init() {
	register(Catalog{
		Key:         "gdpr-2016-679",
		Name:        "RGPD (UE 2016/679)",
		Version:     "2016",
		Description: "Règlement Général sur la Protection des Données — obligations du responsable de traitement et du sous-traitant : principes, bases légales, droits des personnes, sécurité, violations, DPIA, DPO et transferts.",
		Available:   true,
		Controls:    gdpr2016679Controls,
	})
}

const gdprSource = "RGPD (UE) 2016/679, art. "

var gdpr2016679Controls = []CatalogControl{
	// Chapitre II — Principes
	{"Art.5", "Principes relatifs au traitement", "Traiter les données de manière licite, loyale et transparente, pour des finalités déterminées, avec minimisation, exactitude, limitation de la conservation, intégrité/confidentialité et responsabilité (accountability).", gdprSource + "5"},
	{"Art.6", "Licéité du traitement", "Ne traiter des données que si au moins une base légale s'applique (consentement, contrat, obligation légale, intérêt vital, mission d'intérêt public, intérêt légitime).", gdprSource + "6"},
	{"Art.7", "Conditions applicables au consentement", "Pouvoir démontrer que la personne a consenti, présenter la demande de manière claire et distincte, et permettre un retrait aussi simple que le consentement.", gdprSource + "7"},
	{"Art.9", "Catégories particulières de données", "Interdire par principe le traitement des données sensibles (santé, biométrie, opinions…) sauf exception encadrée, et mettre en place des garanties renforcées.", gdprSource + "9"},

	// Chapitre III — Droits de la personne concernée
	{"Art.12-14", "Transparence et information", "Fournir une information concise, transparente et accessible sur le traitement, que les données soient collectées directement ou indirectement.", gdprSource + "12-14"},
	{"Art.15", "Droit d'accès", "Permettre à la personne d'obtenir la confirmation que ses données sont traitées, l'accès à ces données et les informations sur le traitement.", gdprSource + "15"},
	{"Art.16", "Droit de rectification", "Permettre la rectification et le complètement des données inexactes ou incomplètes dans les meilleurs délais.", gdprSource + "16"},
	{"Art.17", "Droit à l'effacement (« droit à l'oubli »)", "Effacer les données sur demande lorsque les conditions sont réunies et répercuter la demande aux destinataires et sous-traitants.", gdprSource + "17"},
	{"Art.18", "Droit à la limitation du traitement", "Restreindre le traitement dans les cas prévus (contestation d'exactitude, opposition, traitement illicite) plutôt que d'effacer.", gdprSource + "18"},
	{"Art.20", "Droit à la portabilité des données", "Restituer les données fournies par la personne dans un format structuré, couramment utilisé et lisible par machine, et les transmettre à un autre responsable si techniquement possible.", gdprSource + "20"},
	{"Art.21", "Droit d'opposition", "Permettre à la personne de s'opposer au traitement, notamment à la prospection commerciale et au profilage.", gdprSource + "21"},
	{"Art.22", "Décision individuelle automatisée", "Encadrer les décisions fondées exclusivement sur un traitement automatisé, y compris le profilage, produisant des effets juridiques.", gdprSource + "22"},

	// Chapitre IV — Responsable du traitement et sous-traitant
	{"Art.24", "Responsabilité du responsable de traitement", "Mettre en œuvre des mesures techniques et organisationnelles appropriées pour garantir et démontrer la conformité du traitement.", gdprSource + "24"},
	{"Art.25", "Protection des données dès la conception et par défaut", "Intégrer la protection des données dès la conception (privacy by design) et par défaut (privacy by default) dans les traitements.", gdprSource + "25"},
	{"Art.28", "Sous-traitant", "N'avoir recours qu'à des sous-traitants présentant des garanties suffisantes et encadrer la relation par un contrat conforme (article 28.3).", gdprSource + "28"},
	{"Art.30", "Registre des activités de traitement", "Tenir un registre des activités de traitement documentant finalités, catégories, destinataires, transferts et mesures de sécurité.", gdprSource + "30"},
	{"Art.32", "Sécurité du traitement", "Mettre en œuvre des mesures de sécurité adaptées au risque : pseudonymisation, chiffrement, confidentialité, intégrité, disponibilité, résilience et tests réguliers.", gdprSource + "32"},
	{"Art.33", "Notification d'une violation à l'autorité", "Notifier une violation de données à l'autorité de contrôle dans les 72 heures lorsqu'elle présente un risque pour les personnes.", gdprSource + "33"},
	{"Art.34", "Communication d'une violation à la personne", "Communiquer la violation aux personnes concernées dans les meilleurs délais lorsqu'elle engendre un risque élevé.", gdprSource + "34"},
	{"Art.35", "Analyse d'impact (AIPD/DPIA)", "Réaliser une analyse d'impact relative à la protection des données pour les traitements susceptibles d'engendrer un risque élevé.", gdprSource + "35"},
	{"Art.37-39", "Délégué à la protection des données (DPO)", "Désigner un DPO lorsque requis, garantir son indépendance et ses moyens, et lui confier l'information, le conseil et le contrôle de conformité.", gdprSource + "37-39"},
	{"Art.44-49", "Transferts vers des pays tiers", "N'effectuer de transferts hors UE que sous garanties appropriées (décision d'adéquation, clauses types, BCR) ou dérogation prévue.", gdprSource + "44-49"},
}
