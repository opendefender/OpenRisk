// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package compliance

import "sort"

// Coverage says how much of the target control the source control answers.
//
// Two values, not five. A compliance officer's decision is binary in practice —
// "can I reuse this proof as it stands, or do I need something more?" — and a
// scale with an "almost equivalent" rung only produces arguments about which
// rung applies. Full means the same evidence satisfies both as written; Partial
// means it contributes but leaves something to demonstrate.
type Coverage string

const (
	CoverageFull    Coverage = "full"
	CoveragePartial Coverage = "partial"
)

// Valid reports whether c is a known coverage level.
func (c Coverage) Valid() bool { return c == CoverageFull || c == CoveragePartial }

// CrosswalkEntry is one curated correspondence between a control in one catalog
// and a control in another.
//
// Rationale is required and is not decoration. A crosswalk is an editorial
// judgement, and the number it feeds ("47% already covered") is one an auditor
// will push back on. The tenant needs to see WHY the product thinks their SOC 2
// access-control evidence answers ISO A.5.15, so they can accept it, argue with
// it, or delete the link.
type CrosswalkEntry struct {
	SourceCatalog string
	SourceCode    string
	TargetCatalog string
	TargetCode    string
	Coverage      Coverage
	Rationale     string
}

// crosswalks is the curated registry. Undirected in meaning — see Between,
// which matches a pair in either order — but stored one way round to keep the
// data readable.
//
// SCOPE, stated honestly: these are structural correspondences at the level the
// catalogs are modelled (ISO Annex A control, SOC 2 criterion, CSF category).
// They are a starting point that saves a compliance officer the first pass, NOT
// an accredited mapping. Every materialised link is marked origin=curated so a
// tenant can see what the product asserted, and edit or remove it. Where a
// correspondence is arguable it is recorded as partial rather than full: over-
// claiming coverage is the failure mode that matters, because it tells someone
// they can stop working.
var crosswalks = []CrosswalkEntry{
	// -- ISO/IEC 27001:2022 Annex A  <->  SOC 2 / AICPA TSC 2017 ---------------
	{"iso27001-2022", "A.5.1", "soc2-tsc", "CC5.3", CoverageFull,
		"Les deux exigent des politiques de sécurité approuvées par la direction et diffusées : la même politique signée répond aux deux."},
	{"iso27001-2022", "A.5.2", "soc2-tsc", "CC1.3", CoverageFull,
		"Rôles et responsabilités de sécurité définis et attribués — même organigramme, mêmes lettres de mission."},
	{"iso27001-2022", "A.5.3", "soc2-tsc", "CC1.5", CoveragePartial,
		"La séparation des tâches contribue à la redevabilité exigée par CC1.5, mais CC1.5 couvre aussi l'évaluation de la performance."},
	{"iso27001-2022", "A.5.15", "soc2-tsc", "CC6.1", CoverageFull,
		"Politique de contrôle d'accès et sa mise en œuvre logique : la même matrice d'accès et ses preuves de revue servent aux deux."},
	{"iso27001-2022", "A.5.16", "soc2-tsc", "CC6.2", CoverageFull,
		"Gestion des identités sur tout le cycle de vie — création, modification, retrait — attestée par les mêmes registres."},
	{"iso27001-2022", "A.5.18", "soc2-tsc", "CC6.3", CoverageFull,
		"Attribution, revue et révocation des droits d'accès : une revue trimestrielle des habilitations couvre les deux."},
	{"iso27001-2022", "A.5.19", "soc2-tsc", "CC9.2", CoverageFull,
		"Sécurité dans les relations fournisseurs : mêmes clauses contractuelles et même processus d'évaluation des tiers."},
	{"iso27001-2022", "A.5.24", "soc2-tsc", "CC7.3", CoveragePartial,
		"La planification de la gestion des incidents alimente CC7.3, qui exige en plus l'évaluation des événements de sécurité détectés."},
	{"iso27001-2022", "A.5.26", "soc2-tsc", "CC7.4", CoverageFull,
		"Réponse aux incidents : la même procédure et les mêmes comptes rendus d'incident répondent aux deux."},
	{"iso27001-2022", "A.5.27", "soc2-tsc", "CC7.5", CoveragePartial,
		"Le retour d'expérience après incident contribue au rétablissement exigé par CC7.5, qui porte aussi sur la remise en service."},
	{"iso27001-2022", "A.5.30", "soc2-tsc", "A1.2", CoverageFull,
		"Préparation TIC à la continuité d'activité : plan de continuité, sauvegardes et tests communs aux deux référentiels."},
	{"iso27001-2022", "A.8.2", "soc2-tsc", "CC6.1", CoveragePartial,
		"Les droits d'accès privilégiés sont un sous-ensemble du contrôle d'accès logique de CC6.1 : la preuve compte, mais CC6.1 demande plus large."},

	// -- ISO/IEC 27001:2022 Annex A  <->  NIST CSF 2.0 ------------------------
	{"iso27001-2022", "A.5.1", "nist-csf-2.0", "GV.PO", CoverageFull,
		"Politique de sécurité établie, approuvée et communiquée : c'est exactement l'objet de la catégorie GOVERN/Policy."},
	{"iso27001-2022", "A.5.2", "nist-csf-2.0", "GV.RR", CoverageFull,
		"Rôles, responsabilités et autorités de cybersécurité — même documentation d'organisation."},
	{"iso27001-2022", "A.5.7", "nist-csf-2.0", "ID.RA", CoveragePartial,
		"Le renseignement sur les menaces alimente l'appréciation des risques, qui couvre aussi vulnérabilités et impacts."},
	{"iso27001-2022", "A.5.9", "nist-csf-2.0", "ID.AM", CoverageFull,
		"Inventaire des informations et des actifs associés : même inventaire, mêmes propriétaires."},
	{"iso27001-2022", "A.5.15", "nist-csf-2.0", "PR.AA", CoverageFull,
		"Contrôle d'accès et gestion des identités/authentification — les mêmes preuves d'habilitation valent pour PR.AA."},
	{"iso27001-2022", "A.5.19", "nist-csf-2.0", "GV.SC", CoveragePartial,
		"La sécurité des relations fournisseurs contribue à la gestion des risques de la chaîne d'approvisionnement, plus large dans le CSF 2.0."},
	{"iso27001-2022", "A.5.24", "nist-csf-2.0", "RS.MA", CoverageFull,
		"Planification et préparation de la gestion des incidents : même plan de réponse."},
	{"iso27001-2022", "A.5.25", "nist-csf-2.0", "DE.AE", CoverageFull,
		"Appréciation et qualification des événements de sécurité — même procédure de triage."},
	{"iso27001-2022", "A.5.26", "nist-csf-2.0", "RS.MI", CoveragePartial,
		"La réponse aux incidents inclut l'endiguement visé par RS.MI, qui se concentre sur l'atténuation."},
	{"iso27001-2022", "A.5.29", "nist-csf-2.0", "RC.RP", CoverageFull,
		"Sécurité de l'information pendant une perturbation et exécution du plan de rétablissement."},

	// -- ISO/IEC 27001:2022 Annex A  <->  PCI DSS 4.0 -------------------------
	{"iso27001-2022", "A.5.15", "pci-dss-4.0", "PCI-7", CoverageFull,
		"Restreindre l'accès aux données selon le besoin d'en connaître : même politique d'accès et mêmes revues."},
	{"iso27001-2022", "A.5.16", "pci-dss-4.0", "PCI-8", CoverageFull,
		"Identification des utilisateurs et authentification des accès aux composants du système."},
	{"iso27001-2022", "A.5.24", "pci-dss-4.0", "PCI-12", CoveragePartial,
		"L'exigence PCI-12 couvre la politique globale de sécurité, dont le plan de réponse aux incidents n'est qu'une partie."},

	// -- SOC 2 / TSC  <->  NIST CSF 2.0 ---------------------------------------
	{"soc2-tsc", "CC6.1", "nist-csf-2.0", "PR.AA", CoverageFull,
		"Sécurité des accès logiques et gestion des identités/authentification : mêmes preuves d'habilitation."},
	{"soc2-tsc", "CC7.2", "nist-csf-2.0", "DE.CM", CoverageFull,
		"Surveillance continue de l'infrastructure pour détecter les anomalies — même dispositif de supervision."},
	{"soc2-tsc", "CC7.4", "nist-csf-2.0", "RS.MA", CoverageFull,
		"Réponse aux incidents identifiés : même procédure, mêmes comptes rendus."},
	{"soc2-tsc", "CC3.2", "nist-csf-2.0", "ID.RA", CoverageFull,
		"Identification et analyse des risques pesant sur les objectifs — même appréciation des risques."},
}

// CrosswalksBetween returns the curated correspondences between two catalogs, in
// the direction asked for (source belongs to catalogA, target to catalogB),
// regardless of which way round the entry was written.
//
// Order is stable so a materialisation run produces the same links every time.
func CrosswalksBetween(catalogA, catalogB string) []CrosswalkEntry {
	if catalogA == catalogB {
		// A catalog does not crosswalk to itself; two controls of the same
		// framework being related is a different (and much weaker) claim.
		return nil
	}

	out := make([]CrosswalkEntry, 0, 8)
	for _, e := range crosswalks {
		switch {
		case e.SourceCatalog == catalogA && e.TargetCatalog == catalogB:
			out = append(out, e)
		case e.SourceCatalog == catalogB && e.TargetCatalog == catalogA:
			// Flip it: coverage and rationale describe the correspondence, which is
			// symmetric in the sense that matters here (the same evidence speaks to
			// both). Direction only decides which framework we are reporting on.
			out = append(out, CrosswalkEntry{
				SourceCatalog: catalogA, SourceCode: e.TargetCode,
				TargetCatalog: catalogB, TargetCode: e.SourceCode,
				Coverage: e.Coverage, Rationale: e.Rationale,
			})
		}
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].SourceCode != out[j].SourceCode {
			return out[i].SourceCode < out[j].SourceCode
		}
		return out[i].TargetCode < out[j].TargetCode
	})
	return out
}

// CrosswalkCatalogPairs lists every pair of catalogs that has curated content,
// so the UI can say up front which imports will inherit coverage instead of
// leaving the user to discover it by trying.
func CrosswalkCatalogPairs() [][2]string {
	seen := map[[2]string]bool{}
	out := make([][2]string, 0, 8)
	for _, e := range crosswalks {
		pair := [2]string{e.SourceCatalog, e.TargetCatalog}
		if pair[0] > pair[1] {
			pair[0], pair[1] = pair[1], pair[0]
		}
		if !seen[pair] {
			seen[pair] = true
			out = append(out, pair)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i][0] != out[j][0] {
			return out[i][0] < out[j][0]
		}
		return out[i][1] < out[j][1]
	})
	return out
}

// AllCrosswalks returns the whole curated registry (for tests and for the
// documentation endpoint).
func AllCrosswalks() []CrosswalkEntry {
	out := make([]CrosswalkEntry, len(crosswalks))
	copy(out, crosswalks)
	return out
}
