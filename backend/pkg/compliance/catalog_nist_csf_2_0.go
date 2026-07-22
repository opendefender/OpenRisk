// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package compliance

// NIST Cybersecurity Framework (CSF) 2.0 — the 22 Categories across the 6
// Functions (GOVERN, IDENTIFY, PROTECT, DETECT, RESPOND, RECOVER). The Function
// and Category identifiers (GV.OC, ID.AM, PR.AA, …) are NIST's own public
// structure and are reliable; the descriptions here are original summaries of
// each Category's intent written for this product, not NIST's own text. Verify
// against the published CSF 2.0 Core (NIST, Feb 2024) before an audit.
//
// Modeled at the Category level (not the 106 Subcategories) so the framework is
// complete and accurate at a granularity a tenant can meaningfully assess; a
// tenant can add ad-hoc Subcategory controls to any imported framework.

func init() {
	register(Catalog{
		Key:         "nist-csf-2.0",
		Name:        "NIST Cybersecurity Framework",
		Version:     "2.0",
		Description: "The NIST CSF 2.0 Core — 22 Categories across GOVERN, IDENTIFY, PROTECT, DETECT, RESPOND and RECOVER.",
		Available:   true,
		Controls:    nistCSF20Controls,
	})
}

const nistCSFSource = "NIST CSF 2.0 Core, Category "

var nistCSF20Controls = []CatalogControl{
	// --- GOVERN (GV) ---
	{"GV.OC", "Organizational Context", "Understand the circumstances — mission, stakeholder expectations, dependencies, and legal, regulatory and contractual requirements — surrounding the organization's cybersecurity risk management decisions.", nistCSFSource + "GV.OC"},
	{"GV.RM", "Risk Management Strategy", "Establish, communicate and use the organization's priorities, constraints, risk tolerance and appetite statements, and assumptions to support operational risk decisions.", nistCSFSource + "GV.RM"},
	{"GV.RR", "Roles, Responsibilities, and Authorities", "Establish and communicate cybersecurity roles, responsibilities and authorities to foster accountability, performance assessment and continuous improvement.", nistCSFSource + "GV.RR"},
	{"GV.PO", "Policy", "Establish, communicate and enforce organizational cybersecurity policy.", nistCSFSource + "GV.PO"},
	{"GV.OV", "Oversight", "Use the results of organization-wide cybersecurity risk management activities and performance to inform, improve and adjust the risk management strategy.", nistCSFSource + "GV.OV"},
	{"GV.SC", "Cybersecurity Supply Chain Risk Management", "Identify, establish, manage, monitor and improve cyber supply chain risk management processes agreed to by organizational stakeholders.", nistCSFSource + "GV.SC"},

	// --- IDENTIFY (ID) ---
	{"ID.AM", "Asset Management", "Identify and manage the assets — data, hardware, software, systems, facilities, services and people — that enable the organization to achieve its purposes, consistent with their relative importance and risk.", nistCSFSource + "ID.AM"},
	{"ID.RA", "Risk Assessment", "Understand the cybersecurity risk to the organization, its assets and individuals, including threats, vulnerabilities, likelihoods and impacts.", nistCSFSource + "ID.RA"},
	{"ID.IM", "Improvement", "Identify improvements to organizational cybersecurity risk management processes, procedures and activities across all CSF Functions.", nistCSFSource + "ID.IM"},

	// --- PROTECT (PR) ---
	{"PR.AA", "Identity Management, Authentication, and Access Control", "Limit access to physical and logical assets to authorized users, services and hardware, commensurate with the assessed risk of unauthorized access.", nistCSFSource + "PR.AA"},
	{"PR.AT", "Awareness and Training", "Provide the organization's personnel with cybersecurity awareness and training so they can perform their cybersecurity-related tasks.", nistCSFSource + "PR.AT"},
	{"PR.DS", "Data Security", "Manage data consistent with the organization's risk strategy to protect the confidentiality, integrity and availability of information.", nistCSFSource + "PR.DS"},
	{"PR.PS", "Platform Security", "Manage the hardware, software (e.g. firmware, operating systems, applications) and services of physical and virtual platforms consistent with the organization's risk strategy.", nistCSFSource + "PR.PS"},
	{"PR.IR", "Technology Infrastructure Resilience", "Manage security architectures with the organization's risk strategy to protect asset confidentiality, integrity and availability, and organizational resilience.", nistCSFSource + "PR.IR"},

	// --- DETECT (DE) ---
	{"DE.CM", "Continuous Monitoring", "Monitor assets to find anomalies, indicators of compromise and other potentially adverse events.", nistCSFSource + "DE.CM"},
	{"DE.AE", "Adverse Event Analysis", "Analyze anomalies, indicators of compromise and other potentially adverse events to characterize the events and detect cybersecurity incidents.", nistCSFSource + "DE.AE"},

	// --- RESPOND (RS) ---
	{"RS.MA", "Incident Management", "Manage responses to detected cybersecurity incidents.", nistCSFSource + "RS.MA"},
	{"RS.AN", "Incident Analysis", "Investigate incidents to ensure effective response and support forensics and recovery activities.", nistCSFSource + "RS.AN"},
	{"RS.CO", "Incident Response Reporting and Communication", "Coordinate response activities with internal and external stakeholders as required by laws, regulations or policies.", nistCSFSource + "RS.CO"},
	{"RS.MI", "Incident Mitigation", "Perform activities to prevent expansion of an event and mitigate its effects.", nistCSFSource + "RS.MI"},

	// --- RECOVER (RC) ---
	{"RC.RP", "Incident Recovery Plan Execution", "Perform restoration activities to ensure operational availability of systems and services affected by cybersecurity incidents.", nistCSFSource + "RC.RP"},
	{"RC.CO", "Incident Recovery Communication", "Coordinate restoration activities with internal and external parties.", nistCSFSource + "RC.CO"},
}
