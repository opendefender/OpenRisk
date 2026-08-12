// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import "github.com/google/uuid"

// ---------------------------------------------------------------------------
// The shipped attribute table, one schema per asset category.
//
// PROVENANCE — read this before treating the table below as authoritative.
//
// The task specified "[paste here the attribute table from §3-W9 of the
// roadmap]". ROADMAP.md has no §3-W9 and no attribute table; the placeholder was
// never filled in. Rather than ship eight empty schemas, the table below is a
// proposal derived from what the rest of OpenRisk already needs from an asset:
//
//   - the fingerprint signals the vulnerability↔asset correlator matches on
//     (hostname, IP, CPE, cloud resource id) — see pkg/assetmatch;
//   - the exposure and criticality inputs the vuln→risk rule reads
//     (internet_exposed, environment, network zone) — see VulnRiskRule;
//   - the compliance evidence the GRC modules ask for on vendors and processing
//     activities (DPA, certifications, legal basis, transfers, DPIA).
//
// Every schema is tenant-editable through PUT /attack-surface/schemas/:category,
// so a tenant that disagrees with a choice here changes it without a deploy, and
// Customized/Version record that they did. If the intended table turns up, it
// replaces this file's contents; nothing else changes.
// ---------------------------------------------------------------------------

func f(v float64) *float64 { return &v }

// environmentDef is shared by every category that runs somewhere, so "prod"
// means the same thing on a server, an application and a cloud resource.
func environmentDef() AttributeDef {
	return AttributeDef{
		Key: "environment", Label: "Environnement", LabelEN: "Environment",
		Type: AttrEnum, Required: true, Group: "Exploitation",
		Enum: []string{"production", "pre-production", "recette", "developpement", "test"},
		Help: "Un actif de production pèse plus lourd dans le score de risque.",
	}
}

func internetExposedDef() AttributeDef {
	return AttributeDef{
		Key: "internet_exposed", Label: "Exposé sur Internet", LabelEN: "Internet exposed",
		Type: AttrBoolean, Group: "Exposition",
		Help: "Accessible depuis un réseau non maîtrisé. Utilisé par la règle vulnérabilité → risque.",
	}
}

func cpesDef() AttributeDef {
	return AttributeDef{
		Key: "cpes", Label: "CPE (logiciels identifiés)", LabelEN: "CPE",
		Type: AttrStringList, Group: "Identité", Fingerprint: FingerprintCPE,
		Help: "cpe:2.3:a:apache:log4j:2.14.1 — sert à corréler les CVE à cet actif.",
	}
}

// DefaultAttributes returns the shipped schema for a category. An unknown
// category returns nil.
func DefaultAttributes(cat AssetCategory) []AttributeDef {
	switch cat {

	case CategoryServer:
		return []AttributeDef{
			{Key: "hostname", Label: "Nom d'hôte", LabelEN: "Hostname", Type: AttrHostname, Required: true,
				Group: "Identité", Fingerprint: FingerprintHostname},
			{Key: "ip_addresses", Label: "Adresses IP", LabelEN: "IP addresses", Type: AttrIPList,
				Group: "Identité", Fingerprint: FingerprintIP},
			{Key: "operating_system", Label: "Système d'exploitation", LabelEN: "Operating system", Type: AttrString, Required: true, Group: "Identité"},
			{Key: "os_version", Label: "Version de l'OS", LabelEN: "OS version", Type: AttrString, Group: "Identité"},
			environmentDef(),
			internetExposedDef(),
			{Key: "network_zone", Label: "Zone réseau", LabelEN: "Network zone", Type: AttrEnum, Group: "Exposition",
				Enum: []string{"dmz", "interne", "administration", "industriel", "invite"},
				Help: "Sert au regroupement par zone dans la vue topologie."},
			{Key: "physical_location", Label: "Localisation", LabelEN: "Location", Type: AttrString, Group: "Exploitation"},
			{Key: "last_patched", Label: "Dernier correctif appliqué", LabelEN: "Last patched", Type: AttrDate, Group: "Exploitation"},
			{Key: "backup_enabled", Label: "Sauvegardé", LabelEN: "Backed up", Type: AttrBoolean, Group: "Exploitation"},
			cpesDef(),
		}

	case CategoryWorkstation:
		return []AttributeDef{
			{Key: "hostname", Label: "Nom d'hôte", LabelEN: "Hostname", Type: AttrHostname, Required: true,
				Group: "Identité", Fingerprint: FingerprintHostname},
			{Key: "ip_addresses", Label: "Adresses IP", LabelEN: "IP addresses", Type: AttrIPList,
				Group: "Identité", Fingerprint: FingerprintIP},
			{Key: "operating_system", Label: "Système d'exploitation", LabelEN: "Operating system", Type: AttrString, Required: true, Group: "Identité"},
			{Key: "os_version", Label: "Version de l'OS", LabelEN: "OS version", Type: AttrString, Group: "Identité"},
			{Key: "assigned_user", Label: "Utilisateur affecté", LabelEN: "Assigned user", Type: AttrString, Group: "Rattachement"},
			{Key: "department", Label: "Service", LabelEN: "Department", Type: AttrString, Group: "Rattachement"},
			{Key: "disk_encrypted", Label: "Disque chiffré", LabelEN: "Disk encrypted", Type: AttrBoolean, Group: "Sécurité"},
			{Key: "edr_installed", Label: "EDR installé", LabelEN: "EDR installed", Type: AttrBoolean, Group: "Sécurité"},
			{Key: "mobile_device", Label: "Poste nomade", LabelEN: "Mobile device", Type: AttrBoolean, Group: "Sécurité"},
			{Key: "last_seen", Label: "Dernière connexion", LabelEN: "Last seen", Type: AttrDate, Group: "Exploitation"},
			cpesDef(),
		}

	case CategoryApplication:
		return []AttributeDef{
			{Key: "application_name", Label: "Nom applicatif", LabelEN: "Application name", Type: AttrString, Required: true, Group: "Identité"},
			{Key: "version", Label: "Version", LabelEN: "Version", Type: AttrString, Group: "Identité"},
			{Key: "url", Label: "URL principale", LabelEN: "Primary URL", Type: AttrURL, Group: "Identité",
				Fingerprint: FingerprintHostname, Help: "Le nom d'hôte de cette URL sert aussi d'empreinte de corrélation."},
			{Key: "technology_stack", Label: "Technologies", LabelEN: "Technology stack", Type: AttrStringList, Group: "Identité"},
			environmentDef(),
			internetExposedDef(),
			{Key: "authentication", Label: "Authentification", LabelEN: "Authentication", Type: AttrEnum, Group: "Sécurité",
				Enum: []string{"aucune", "mot-de-passe", "sso", "mfa", "certificat"}},
			{Key: "data_classification", Label: "Classification des données", LabelEN: "Data classification", Type: AttrEnum, Group: "Sécurité",
				Enum: []string{"public", "interne", "confidentiel", "secret"}},
			{Key: "business_owner", Label: "Responsable métier", LabelEN: "Business owner", Type: AttrString, Group: "Rattachement"},
			{Key: "source_repository", Label: "Dépôt de code", LabelEN: "Source repository", Type: AttrURL, Group: "Exploitation"},
			cpesDef(),
		}

	case CategoryDatabase:
		return []AttributeDef{
			{Key: "engine", Label: "Moteur", LabelEN: "Engine", Type: AttrEnum, Required: true, Group: "Identité",
				Enum: []string{"postgresql", "mysql", "mariadb", "oracle", "sql-server", "mongodb", "redis", "elasticsearch", "autre"}},
			{Key: "version", Label: "Version", LabelEN: "Version", Type: AttrString, Group: "Identité"},
			{Key: "hostname", Label: "Nom d'hôte", LabelEN: "Hostname", Type: AttrHostname, Group: "Identité", Fingerprint: FingerprintHostname},
			{Key: "port", Label: "Port", LabelEN: "Port", Type: AttrInteger, Group: "Identité", Min: f(1), Max: f(65535)},
			environmentDef(),
			{Key: "encrypted_at_rest", Label: "Chiffrée au repos", LabelEN: "Encrypted at rest", Type: AttrBoolean, Group: "Sécurité"},
			{Key: "contains_personal_data", Label: "Contient des données personnelles", LabelEN: "Contains personal data", Type: AttrBoolean, Group: "Conformité"},
			{Key: "backup_frequency", Label: "Fréquence de sauvegarde", LabelEN: "Backup frequency", Type: AttrEnum, Group: "Exploitation",
				Enum: []string{"aucune", "quotidienne", "hebdomadaire", "mensuelle", "continue"}},
			{Key: "retention_days", Label: "Rétention (jours)", LabelEN: "Retention (days)", Type: AttrInteger, Group: "Conformité", Min: f(0)},
			{Key: "record_count", Label: "Volume (enregistrements)", LabelEN: "Record count", Type: AttrInteger, Group: "Conformité", Min: f(0)},
			cpesDef(),
		}

	case CategoryNetwork:
		return []AttributeDef{
			{Key: "device_type", Label: "Type d'équipement", LabelEN: "Device type", Type: AttrEnum, Required: true, Group: "Identité",
				Enum: []string{"pare-feu", "routeur", "commutateur", "vpn", "repartiteur-de-charge", "point-acces-wifi", "proxy"}},
			{Key: "management_ip", Label: "IP d'administration", LabelEN: "Management IP", Type: AttrIP, Required: true,
				Group: "Identité", Fingerprint: FingerprintIP},
			{Key: "hostname", Label: "Nom d'hôte", LabelEN: "Hostname", Type: AttrHostname, Group: "Identité", Fingerprint: FingerprintHostname},
			{Key: "vendor", Label: "Constructeur", LabelEN: "Vendor", Type: AttrString, Group: "Identité"},
			{Key: "model", Label: "Modèle", LabelEN: "Model", Type: AttrString, Group: "Identité"},
			{Key: "firmware_version", Label: "Version du firmware", LabelEN: "Firmware version", Type: AttrString, Group: "Exploitation"},
			{Key: "network_zone", Label: "Zone réseau", LabelEN: "Network zone", Type: AttrEnum, Required: true, Group: "Exposition",
				Enum: []string{"dmz", "interne", "administration", "industriel", "invite", "perimetre"}},
			{Key: "managed_subnets", Label: "Sous-réseaux gérés", LabelEN: "Managed subnets", Type: AttrStringList, Group: "Exposition"},
			internetExposedDef(),
			{Key: "end_of_support", Label: "Fin de support", LabelEN: "End of support", Type: AttrDate, Group: "Exploitation"},
			cpesDef(),
		}

	case CategoryCloud:
		return []AttributeDef{
			{Key: "provider", Label: "Fournisseur", LabelEN: "Provider", Type: AttrEnum, Required: true, Group: "Identité",
				Enum: []string{"aws", "azure", "gcp", "ovh", "scaleway", "oracle-cloud", "autre"}},
			{Key: "resource_id", Label: "Identifiant de ressource", LabelEN: "Resource id", Type: AttrString, Required: true,
				Group: "Identité", Fingerprint: FingerprintCloudID,
				Help: "ARN, resource id Azure ou self-link GCP. C'est l'empreinte la plus fiable pour corréler les vulnérabilités cloud."},
			{Key: "account_id", Label: "Compte / abonnement", LabelEN: "Account / subscription", Type: AttrString, Group: "Identité"},
			{Key: "region", Label: "Région", LabelEN: "Region", Type: AttrString, Group: "Identité"},
			{Key: "service", Label: "Service", LabelEN: "Service", Type: AttrString, Group: "Identité", Help: "EC2, S3, AKS, Cloud SQL…"},
			environmentDef(),
			{Key: "publicly_accessible", Label: "Accessible publiquement", LabelEN: "Publicly accessible", Type: AttrBoolean, Group: "Exposition"},
			internetExposedDef(),
			{Key: "encryption_enabled", Label: "Chiffrement activé", LabelEN: "Encryption enabled", Type: AttrBoolean, Group: "Sécurité"},
			{Key: "cloud_tags", Label: "Étiquettes cloud", LabelEN: "Cloud tags", Type: AttrStringList, Group: "Exploitation"},
			cpesDef(),
		}

	case CategoryVendor:
		return []AttributeDef{
			{Key: "legal_name", Label: "Raison sociale", LabelEN: "Legal name", Type: AttrString, Required: true, Group: "Identité"},
			{Key: "country", Label: "Pays", LabelEN: "Country", Type: AttrString, Required: true, Group: "Identité"},
			{Key: "service_provided", Label: "Service fourni", LabelEN: "Service provided", Type: AttrText, Required: true, Group: "Contrat"},
			{Key: "contract_reference", Label: "Référence du contrat", LabelEN: "Contract reference", Type: AttrString, Group: "Contrat"},
			{Key: "contract_end", Label: "Fin de contrat", LabelEN: "Contract end", Type: AttrDate, Group: "Contrat"},
			{Key: "contact_email", Label: "Contact", LabelEN: "Contact email", Type: AttrEmail, Group: "Contrat"},
			{Key: "data_shared", Label: "Données partagées", LabelEN: "Data shared", Type: AttrEnum, Required: true, Group: "Conformité",
				Enum: []string{"aucune", "personnelles", "sensibles", "financieres", "sante"}},
			{Key: "dpa_signed", Label: "Accord de sous-traitance signé", LabelEN: "DPA signed", Type: AttrBoolean, Group: "Conformité",
				Help: "Article 28 RGPD."},
			{Key: "certifications", Label: "Certifications", LabelEN: "Certifications", Type: AttrMultiEnum, Group: "Conformité",
				Enum: []string{"iso-27001", "soc-2", "pci-dss", "hds", "iso-22301", "aucune"}},
			{Key: "service_criticality", Label: "Criticité du service", LabelEN: "Service criticality", Type: AttrEnum, Group: "Conformité",
				Enum: []string{"faible", "moyenne", "haute", "critique"}},
			{Key: "last_assessment", Label: "Dernière évaluation", LabelEN: "Last assessment", Type: AttrDate, Group: "Conformité"},
		}

	case CategoryData:
		return []AttributeDef{
			{Key: "processing_name", Label: "Nom du traitement", LabelEN: "Processing name", Type: AttrString, Required: true, Group: "Identité"},
			{Key: "purpose", Label: "Finalité", LabelEN: "Purpose", Type: AttrText, Required: true, Group: "Identité"},
			{Key: "data_categories", Label: "Catégories de données", LabelEN: "Data categories", Type: AttrMultiEnum, Required: true, Group: "Conformité",
				Enum: []string{"identification", "contact", "financieres", "sante", "biometriques", "localisation", "connexion", "professionnelles"}},
			{Key: "data_subjects", Label: "Personnes concernées", LabelEN: "Data subjects", Type: AttrMultiEnum, Group: "Conformité",
				Enum: []string{"clients", "prospects", "salaries", "candidats", "fournisseurs", "mineurs"}},
			{Key: "legal_basis", Label: "Base légale", LabelEN: "Legal basis", Type: AttrEnum, Required: true, Group: "Conformité",
				Enum: []string{"consentement", "contrat", "obligation-legale", "interet-vital", "mission-interet-public", "interet-legitime"}},
			{Key: "role", Label: "Rôle", LabelEN: "Role", Type: AttrEnum, Group: "Conformité",
				Enum: []string{"responsable-de-traitement", "sous-traitant", "responsable-conjoint"}},
			{Key: "retention_period", Label: "Durée de conservation", LabelEN: "Retention period", Type: AttrString, Group: "Conformité"},
			{Key: "transfers_outside_eu", Label: "Transfert hors UE", LabelEN: "Transfers outside EU", Type: AttrBoolean, Group: "Conformité"},
			{Key: "dpia_required", Label: "AIPD requise", LabelEN: "DPIA required", Type: AttrBoolean, Group: "Conformité"},
			{Key: "dpia_completed", Label: "AIPD réalisée", LabelEN: "DPIA completed", Type: AttrBoolean, Group: "Conformité"},
			{Key: "record_volume", Label: "Volume (personnes)", LabelEN: "Volume (data subjects)", Type: AttrInteger, Group: "Conformité", Min: f(0)},
		}
	}
	return nil
}

// DefaultCategoryLabel is the shipped display name of a category.
func DefaultCategoryLabel(cat AssetCategory) string {
	switch cat {
	case CategoryServer:
		return "Serveur"
	case CategoryWorkstation:
		return "Poste de travail"
	case CategoryApplication:
		return "Application"
	case CategoryDatabase:
		return "Base de données"
	case CategoryNetwork:
		return "Équipement réseau"
	case CategoryCloud:
		return "Ressource cloud"
	case CategoryVendor:
		return "Fournisseur / tiers"
	case CategoryData:
		return "Données / traitement"
	}
	return string(cat)
}

// DefaultSchemaFor builds the shipped (uncustomised) schema row for a tenant and
// category. Used to seed on first read and to serve "reset to default".
func DefaultSchemaFor(tenantID uuid.UUID, cat AssetCategory) *AssetTypeSchema {
	return &AssetTypeSchema{
		ID:         uuid.New(),
		TenantID:   tenantID,
		Category:   cat,
		Label:      DefaultCategoryLabel(cat),
		Attributes: DefaultAttributes(cat),
		Customized: false,
		Version:    1,
	}
}

// FingerprintSignals extracts the identity signals declared by a schema out of
// an asset's attribute bag. This is the bridge between §1 (typed attributes) and
// §3 (vulnerability↔asset correlation): the correlator does not hardcode which
// field holds a hostname — the schema says so.
type FingerprintSignals struct {
	Hostnames []string
	IPs       []string
	CloudIDs  []string
	CPEs      []string
}

// FingerprintSignalsFrom walks the schema, collects every attribute carrying a
// Fingerprint role and returns the values found in attrs.
func FingerprintSignalsFrom(defs []AttributeDef, attrs AssetAttributes) FingerprintSignals {
	var out FingerprintSignals
	add := func(role FingerprintRole, vals []string) {
		switch role {
		case FingerprintHostname:
			out.Hostnames = append(out.Hostnames, vals...)
		case FingerprintIP:
			out.IPs = append(out.IPs, vals...)
		case FingerprintCloudID:
			out.CloudIDs = append(out.CloudIDs, vals...)
		case FingerprintCPE:
			out.CPEs = append(out.CPEs, vals...)
		}
	}
	for _, d := range defs {
		if d.Fingerprint == FingerprintNone {
			continue
		}
		v, ok := attrs[d.Key]
		if !ok {
			continue
		}
		vals := attributeValuesAsStrings(v)
		if d.Type == AttrURL && d.Fingerprint == FingerprintHostname {
			vals = hostsFromURLs(vals)
		}
		add(d.Fingerprint, vals)
	}
	return out
}
