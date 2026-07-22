// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package compliance

// NIST SP 800-53 Rev. 5 — Security and Privacy Controls for Information Systems
// and Organizations. The catalog has 1000+ base controls and enhancements across
// 20 control families. We model it at the family level (the 20 families), which is
// NIST's own public structure and is reliable; a tenant can add specific base
// controls (e.g. AC-2) as ad-hoc controls on the imported framework. Descriptions
// are original summaries of each family's purpose, not NIST's own text. Verify
// against NIST SP 800-53 Rev. 5 before an audit.

func init() {
	register(Catalog{
		Key:         "nist-800-53-r5",
		Name:        "NIST SP 800-53",
		Version:     "Rev. 5",
		Description: "Security and Privacy Controls for Information Systems and Organizations — the 20 control families (AC, AU, CM, IA, IR, RA, SC, SI, SR, PT, …).",
		Available:   true,
		Controls:    nist80053r5Controls,
	})
}

const nist80053Source = "NIST SP 800-53 Rev. 5, Family "

var nist80053r5Controls = []CatalogControl{
	{"AC", "Access Control", "Limit information system access to authorized users, processes and devices, and to the types of transactions and functions authorized users are permitted to exercise.", nist80053Source + "AC"},
	{"AT", "Awareness and Training", "Ensure that personnel are trained to carry out their assigned information security and privacy responsibilities and are aware of applicable policies, standards and procedures.", nist80053Source + "AT"},
	{"AU", "Audit and Accountability", "Create, protect and retain system audit records to enable monitoring, analysis, investigation and reporting of unlawful or unauthorized activity, and ensure actions can be traced to individuals.", nist80053Source + "AU"},
	{"CA", "Assessment, Authorization, and Monitoring", "Assess controls, authorize system operation, and continuously monitor security and privacy posture, including plans of action and milestones for deficiencies.", nist80053Source + "CA"},
	{"CM", "Configuration Management", "Establish and maintain baseline configurations and inventories of systems, and enforce configuration change control throughout the system development life cycle.", nist80053Source + "CM"},
	{"CP", "Contingency Planning", "Establish, maintain and test plans for emergency response, backup operations and post-disaster recovery to ensure the availability of critical information resources and continuity of operations.", nist80053Source + "CP"},
	{"IA", "Identification and Authentication", "Uniquely identify and authenticate organizational users, processes and devices before granting access to systems.", nist80053Source + "IA"},
	{"IR", "Incident Response", "Establish an operational incident-handling capability — preparation, detection, analysis, containment, eradication and recovery — and track, document and report incidents.", nist80053Source + "IR"},
	{"MA", "Maintenance", "Perform periodic and timely maintenance on systems and provide effective controls on the tools, techniques, mechanisms and personnel used to conduct maintenance.", nist80053Source + "MA"},
	{"MP", "Media Protection", "Protect system media (digital and non-digital), limit access to authorized users, and sanitize or destroy media before disposal or reuse.", nist80053Source + "MP"},
	{"PE", "Physical and Environmental Protection", "Limit physical access to systems, equipment and operating environments to authorized individuals, and protect against environmental hazards and supporting utilities failures.", nist80053Source + "PE"},
	{"PL", "Planning", "Develop, document and maintain security and privacy plans that describe the controls in place or planned and the rules of behaviour for individuals accessing systems.", nist80053Source + "PL"},
	{"PM", "Program Management", "Implement organization-wide information security and privacy program management controls, including a risk management strategy, resources and enterprise architecture.", nist80053Source + "PM"},
	{"PS", "Personnel Security", "Ensure individuals occupying positions of responsibility are trustworthy, apply screening, and protect systems during personnel actions such as transfers and terminations.", nist80053Source + "PS"},
	{"PT", "PII Processing and Transparency", "Process personally identifiable information in accordance with authority and privacy requirements, and provide transparency to individuals about that processing.", nist80053Source + "PT"},
	{"RA", "Risk Assessment", "Assess the risk to organizational operations, assets and individuals resulting from the operation of systems, including vulnerability scanning and the processing of PII.", nist80053Source + "RA"},
	{"SA", "System and Services Acquisition", "Allocate resources to protect systems, employ a system development life cycle with security and privacy considerations, and manage acquired services and supply.", nist80053Source + "SA"},
	{"SC", "System and Communications Protection", "Monitor, control and protect communications at system boundaries and employ architectural designs, software development techniques and systems engineering to promote security.", nist80053Source + "SC"},
	{"SI", "System and Information Integrity", "Identify, report and correct system flaws in a timely manner, protect against malicious code, and monitor systems for security alerts and advisories.", nist80053Source + "SI"},
	{"SR", "Supply Chain Risk Management", "Manage supply chain risks by developing a strategy, applying provenance and integrity controls, and assessing suppliers and the products and services they provide.", nist80053Source + "SR"},
}
