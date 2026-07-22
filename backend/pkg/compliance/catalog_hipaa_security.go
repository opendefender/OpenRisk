// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package compliance

// HIPAA Security Rule — the Standards of the Administrative, Physical and
// Technical Safeguards plus the Organizational and Documentation requirements,
// codified at 45 CFR Part 164, Subpart C (§§ 164.308–164.316). The CFR section
// citations are public U.S. federal regulation and are reliable. Descriptions
// here are original summaries of each Standard's intent; the required and
// addressable implementation specifications under each Standard can be added as
// ad-hoc controls on the imported framework.

func init() {
	register(Catalog{
		Key:         "hipaa-security",
		Name:        "HIPAA Security Rule",
		Version:     "45 CFR Part 164 Subpart C",
		Description: "Standards for the protection of electronic protected health information (ePHI) — Administrative, Physical and Technical Safeguards.",
		Available:   true,
		Controls:    hipaaSecurityControls,
	})
}

const hipaaSource = "45 CFR § "

var hipaaSecurityControls = []CatalogControl{
	// --- Administrative Safeguards (§ 164.308) ---
	{"164.308(a)(1)", "Security Management Process", "Implement policies and procedures to prevent, detect, contain and correct security violations — including risk analysis, risk management, a sanction policy and information system activity review.", hipaaSource + "164.308(a)(1)"},
	{"164.308(a)(2)", "Assigned Security Responsibility", "Identify the security official who is responsible for the development and implementation of the policies and procedures required by the Security Rule.", hipaaSource + "164.308(a)(2)"},
	{"164.308(a)(3)", "Workforce Security", "Implement policies and procedures to ensure that all workforce members have appropriate access to ePHI, and to prevent those who should not have access from obtaining it.", hipaaSource + "164.308(a)(3)"},
	{"164.308(a)(4)", "Information Access Management", "Implement policies and procedures for authorizing access to ePHI that are consistent with the applicable requirements of the Privacy Rule.", hipaaSource + "164.308(a)(4)"},
	{"164.308(a)(5)", "Security Awareness and Training", "Implement a security awareness and training program for all workforce members, including management (security reminders, malware protection, log-in monitoring and password management).", hipaaSource + "164.308(a)(5)"},
	{"164.308(a)(6)", "Security Incident Procedures", "Implement policies and procedures to address security incidents, including identifying and responding to suspected or known incidents and mitigating their harmful effects.", hipaaSource + "164.308(a)(6)"},
	{"164.308(a)(7)", "Contingency Plan", "Establish policies and procedures for responding to an emergency or other occurrence that damages systems containing ePHI (data backup, disaster recovery, emergency-mode operation, testing and criticality analysis).", hipaaSource + "164.308(a)(7)"},
	{"164.308(a)(8)", "Evaluation", "Perform periodic technical and non-technical evaluations to establish the extent to which security policies and procedures meet the Security Rule's requirements.", hipaaSource + "164.308(a)(8)"},
	{"164.308(b)(1)", "Business Associate Contracts and Other Arrangements", "Obtain satisfactory assurances, documented through a written contract, that a business associate will appropriately safeguard the ePHI it creates, receives, maintains or transmits.", hipaaSource + "164.308(b)(1)"},

	// --- Physical Safeguards (§ 164.310) ---
	{"164.310(a)(1)", "Facility Access Controls", "Implement policies and procedures to limit physical access to electronic information systems and the facilities in which they are housed, while ensuring properly authorized access is allowed.", hipaaSource + "164.310(a)(1)"},
	{"164.310(b)", "Workstation Use", "Implement policies and procedures that specify the proper functions to be performed, the manner in which they are performed, and the physical attributes of the surroundings of workstations that access ePHI.", hipaaSource + "164.310(b)"},
	{"164.310(c)", "Workstation Security", "Implement physical safeguards for all workstations that access ePHI to restrict access to authorized users.", hipaaSource + "164.310(c)"},
	{"164.310(d)(1)", "Device and Media Controls", "Implement policies and procedures governing the receipt and removal of hardware and electronic media that contain ePHI into and out of a facility, and their movement within it (disposal, media re-use, accountability, data backup and storage).", hipaaSource + "164.310(d)(1)"},

	// --- Technical Safeguards (§ 164.312) ---
	{"164.312(a)(1)", "Access Control", "Implement technical policies and procedures for electronic information systems that maintain ePHI to allow access only to authorized persons or software (unique user IDs, emergency access, automatic logoff and encryption/decryption).", hipaaSource + "164.312(a)(1)"},
	{"164.312(b)", "Audit Controls", "Implement hardware, software and procedural mechanisms that record and examine activity in information systems that contain or use ePHI.", hipaaSource + "164.312(b)"},
	{"164.312(c)(1)", "Integrity", "Implement policies and procedures to protect ePHI from improper alteration or destruction, including mechanisms to authenticate that ePHI has not been altered.", hipaaSource + "164.312(c)(1)"},
	{"164.312(d)", "Person or Entity Authentication", "Implement procedures to verify that a person or entity seeking access to ePHI is the one claimed.", hipaaSource + "164.312(d)"},
	{"164.312(e)(1)", "Transmission Security", "Implement technical security measures to guard against unauthorized access to ePHI transmitted over an electronic communications network (integrity controls and encryption).", hipaaSource + "164.312(e)(1)"},

	// --- Organizational Requirements (§ 164.314) ---
	{"164.314(a)(1)", "Business Associate Contracts or Other Arrangements", "Ensure that the contract or other arrangement between a covered entity and a business associate meets the applicable requirements for safeguarding ePHI.", hipaaSource + "164.314(a)(1)"},
	{"164.314(b)(1)", "Requirements for Group Health Plans", "Ensure a group health plan's documents require the plan sponsor to reasonably and appropriately safeguard ePHI created, received, maintained or transmitted on its behalf.", hipaaSource + "164.314(b)(1)"},

	// --- Policies, Procedures and Documentation (§ 164.316) ---
	{"164.316(a)", "Policies and Procedures", "Implement reasonable and appropriate policies and procedures to comply with the standards, implementation specifications and other requirements of the Security Rule.", hipaaSource + "164.316(a)"},
	{"164.316(b)(1)", "Documentation", "Maintain the required policies and procedures in written (which may be electronic) form, and retain documentation of any action, activity or assessment required by the Security Rule.", hipaaSource + "164.316(b)(1)"},
}
