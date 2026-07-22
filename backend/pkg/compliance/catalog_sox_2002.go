// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package compliance

// SOX — the Sarbanes-Oxley Act of 2002. From a GRC/IT perspective, SOX compliance
// centers on internal control over financial reporting (ICFR) — the key statutory
// sections (302, 404, 409, 802, 806, 906) plus the IT General Controls (ITGC)
// domains auditors assess under the COSO framework. Section numbers are the Act's
// own public structure and the ITGC domains are the widely-recognized categories;
// descriptions are original summaries, not legal text. Verify against the Act and
// your auditor's control matrix before an audit.

func init() {
	register(Catalog{
		Key:         "sox-2002",
		Name:        "SOX (Sarbanes-Oxley)",
		Version:     "2002",
		Description: "Sarbanes-Oxley Act — internal control over financial reporting: the key statutory sections and the IT General Controls (access, change management, operations, program development) auditors assess.",
		Available:   true,
		Controls:    sox2002Controls,
	})
}

const (
	soxSecSource  = "Sarbanes-Oxley Act of 2002, Section "
	soxITGCSource = "SOX ITGC (COSO), Domain "
)

var sox2002Controls = []CatalogControl{
	// Key statutory sections
	{"SOX-302", "Corporate Responsibility for Financial Reports", "Principal officers certify each periodic report — its accuracy, the effectiveness of disclosure controls, and disclosure of deficiencies and fraud to auditors and the audit committee.", soxSecSource + "302"},
	{"SOX-404", "Management Assessment of Internal Controls", "Management establishes, documents, assesses and reports on the effectiveness of internal control over financial reporting, with independent auditor attestation.", soxSecSource + "404"},
	{"SOX-409", "Real Time Issuer Disclosures", "Disclose to the public, on a rapid and current basis, material changes in financial condition or operations.", soxSecSource + "409"},
	{"SOX-802", "Criminal Penalties for Altering Documents", "Retain records and audit work papers, and prohibit the alteration, destruction or falsification of records relevant to financial reporting.", soxSecSource + "802"},
	{"SOX-806", "Protection for Whistleblowers", "Protect employees who report fraud or violations of securities law from retaliation, and provide a confidential reporting mechanism.", soxSecSource + "806"},
	{"SOX-906", "Corporate Responsibility for Financial Reports (Certification)", "Principal executive and financial officers certify that periodic reports fully comply with securities law and fairly present the financial condition of the issuer.", soxSecSource + "906"},

	// IT General Controls (ITGC) domains
	{"ITGC-AC", "Access to Programs and Data", "Restrict logical access to financial applications, databases and infrastructure to authorized users through provisioning, periodic review, segregation of duties and privileged-access controls.", soxITGCSource + "Access to Programs and Data"},
	{"ITGC-CM", "Program Changes", "Authorize, test, approve and migrate changes to financially-relevant systems through a controlled change management process that separates development from production.", soxITGCSource + "Program Changes"},
	{"ITGC-PD", "Program Development", "Govern the acquisition and development of new financially-relevant systems with documented requirements, testing, approval and data-conversion controls.", soxITGCSource + "Program Development"},
	{"ITGC-OP", "Computer Operations", "Control the operation of financially-relevant systems: job scheduling, backup and recovery, incident/problem management and physical/environmental protection of infrastructure.", soxITGCSource + "Computer Operations"},
}
