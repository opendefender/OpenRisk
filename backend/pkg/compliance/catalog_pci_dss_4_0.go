// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package compliance

// PCI DSS v4.0 — the 12 core Requirements, grouped into 6 goals. Requirement
// numbers and titles are the PCI Security Standards Council's own public
// structure and are reliable; descriptions here are original summaries, not the
// standard's own text (PCI DSS is © PCI SSC). A tenant can add the detailed
// sub-requirements (e.g. 1.2.1) as ad-hoc controls on the imported framework.

func init() {
	register(Catalog{
		Key:         "pci-dss-4.0",
		Name:        "PCI DSS",
		Version:     "4.0",
		Description: "Payment Card Industry Data Security Standard v4.0 — the 12 core requirements protecting cardholder data.",
		Available:   true,
		Controls:    pciDSS40Controls,
	})
}

const pciDSSSource = "PCI DSS v4.0, Requirement "

var pciDSS40Controls = []CatalogControl{
	// Goal: Build and Maintain a Secure Network and Systems
	{"PCI-1", "Install and Maintain Network Security Controls", "Install and maintain network security controls (NSCs, such as firewalls and other technologies) to protect the cardholder data environment from untrusted networks.", pciDSSSource + "1"},
	{"PCI-2", "Apply Secure Configurations to All System Components", "Apply secure configurations to all system components and remove or disable vendor default accounts, passwords and unnecessary services to reduce the attack surface.", pciDSSSource + "2"},
	// Goal: Protect Account Data
	{"PCI-3", "Protect Stored Account Data", "Protect stored account data through data retention/disposal policies, strong cryptography, and rendering the Primary Account Number (PAN) unreadable wherever it is stored.", pciDSSSource + "3"},
	{"PCI-4", "Protect Cardholder Data with Strong Cryptography During Transmission", "Protect cardholder data with strong cryptography during transmission over open, public networks, and never send unprotected PANs by end-user messaging technologies.", pciDSSSource + "4"},
	// Goal: Maintain a Vulnerability Management Program
	{"PCI-5", "Protect All Systems and Networks from Malicious Software", "Deploy and maintain anti-malware mechanisms and processes to protect all systems and networks from malicious software.", pciDSSSource + "5"},
	{"PCI-6", "Develop and Maintain Secure Systems and Software", "Develop and maintain secure systems and software through secure development practices, timely patching, and protection of public-facing web applications.", pciDSSSource + "6"},
	// Goal: Implement Strong Access Control Measures
	{"PCI-7", "Restrict Access to System Components and Cardholder Data by Business Need to Know", "Restrict access to system components and cardholder data to only those individuals whose job requires it, based on need to know and least privilege, deny by default.", pciDSSSource + "7"},
	{"PCI-8", "Identify Users and Authenticate Access to System Components", "Assign a unique ID to each user and strongly authenticate access to system components, including multi-factor authentication into the cardholder data environment.", pciDSSSource + "8"},
	{"PCI-9", "Restrict Physical Access to Cardholder Data", "Restrict physical access to cardholder data and to systems that store, process or transmit it, including facility entry controls and media handling/destruction.", pciDSSSource + "9"},
	// Goal: Regularly Monitor and Test Networks
	{"PCI-10", "Log and Monitor All Access to System Components and Cardholder Data", "Log all access to system components and cardholder data, protect and review those logs, and use time-synchronization and detection to identify anomalies.", pciDSSSource + "10"},
	{"PCI-11", "Test Security of Systems and Networks Regularly", "Regularly test the security of systems and networks through vulnerability scans, internal/external penetration testing, wireless detection and change-/tamper-detection.", pciDSSSource + "11"},
	// Goal: Maintain an Information Security Policy
	{"PCI-12", "Support Information Security with Organizational Policies and Programs", "Maintain an information security policy and program — risk assessment, awareness, personnel screening, third-party management and an incident response plan — that supports the protection of account data.", pciDSSSource + "12"},
}
