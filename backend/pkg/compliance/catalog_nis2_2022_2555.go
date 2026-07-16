// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package compliance

// NIS2 — Directive (UE) 2022/2555 concernant des mesures pour un niveau élevé
// commun de cybersécurité dans l'ensemble de l'Union. Le cœur opérationnel est
// l'article 21 (mesures de gestion des risques de cybersécurité) qui énumère dix
// mesures minimales (a→j), encadré par la gouvernance (art. 20) et les obligations
// de notification (art. 23). Les numéros d'articles sont la structure publique de
// la directive (fiables) ; les descriptions sont des résumés originaux, pas le
// texte officiel. Vérifier contre la Directive (UE) 2022/2555 avant un audit.

func init() {
	register(Catalog{
		Key:         "nis2-2022-2555",
		Name:        "NIS2 (UE 2022/2555)",
		Version:     "2022",
		Description: "Directive NIS2 — cybersécurité des entités essentielles et importantes : gouvernance, les dix mesures de gestion des risques (art. 21) et les obligations de notification d'incidents.",
		Available:   true,
		Controls:    nis220222555Controls,
	})
}

const nis2Source = "NIS2 (UE) 2022/2555, art. "

var nis220222555Controls = []CatalogControl{
	// Gouvernance
	{"Art.20", "Gouvernance", "Les organes de direction approuvent les mesures de gestion des risques de cybersécurité, en supervisent la mise en œuvre et suivent des formations dédiées.", nis2Source + "20"},

	// Article 21 — les dix mesures minimales de gestion des risques (a → j)
	{"Art.21(a)", "Politiques d'analyse des risques et de sécurité des SI", "Adopter des politiques relatives à l'analyse des risques et à la sécurité des systèmes d'information.", nis2Source + "21.2(a)"},
	{"Art.21(b)", "Gestion des incidents", "Mettre en place une capacité de gestion des incidents (prévention, détection et réponse).", nis2Source + "21.2(b)"},
	{"Art.21(c)", "Continuité des activités", "Assurer la continuité des activités : gestion des sauvegardes, reprise après sinistre et gestion de crise.", nis2Source + "21.2(c)"},
	{"Art.21(d)", "Sécurité de la chaîne d'approvisionnement", "Sécuriser la chaîne d'approvisionnement, y compris les relations avec les fournisseurs et prestataires de services directs.", nis2Source + "21.2(d)"},
	{"Art.21(e)", "Sécurité de l'acquisition, du développement et de la maintenance", "Intégrer la sécurité dans l'acquisition, le développement et la maintenance des réseaux et systèmes, y compris la gestion et la divulgation des vulnérabilités.", nis2Source + "21.2(e)"},
	{"Art.21(f)", "Évaluation de l'efficacité des mesures", "Définir des politiques et procédures pour évaluer l'efficacité des mesures de gestion des risques de cybersécurité.", nis2Source + "21.2(f)"},
	{"Art.21(g)", "Cyberhygiène et formation", "Mettre en œuvre des pratiques d'hygiène informatique de base et une formation à la cybersécurité.", nis2Source + "21.2(g)"},
	{"Art.21(h)", "Cryptographie et chiffrement", "Définir des politiques et procédures relatives à l'usage de la cryptographie et, le cas échéant, du chiffrement.", nis2Source + "21.2(h)"},
	{"Art.21(i)", "Sécurité des ressources humaines et contrôle d'accès", "Gérer la sécurité des ressources humaines, les politiques de contrôle d'accès et la gestion des actifs.", nis2Source + "21.2(i)"},
	{"Art.21(j)", "Authentification multifacteur et communications sécurisées", "Recourir à l'authentification multifacteur ou continue, et à des communications vocales, vidéo et textuelles sécurisées, ainsi qu'à des communications d'urgence sécurisées.", nis2Source + "21.2(j)"},

	// Obligations de notification
	{"Art.23", "Obligations de notification d'incidents", "Notifier sans retard injustifié tout incident important : alerte précoce (24 h), notification (72 h) et rapport final (1 mois).", nis2Source + "23"},
}
