// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package compliance

// CIS Critical Security Controls v8 — the 18 top-level Controls. Control numbers
// and titles are CIS's own public structure and are reliable; descriptions here
// are original summaries of each Control's intent, not CIS's own text (CIS
// Controls are © the Center for Internet Security). A tenant can add ad-hoc
// Safeguard-level controls to the imported framework.

func init() {
	register(Catalog{
		Key:         "cis-v8",
		Name:        "CIS Critical Security Controls",
		Version:     "8",
		Description: "The 18 CIS Critical Security Controls (v8) — prioritized, prescriptive cyber-defense best practices.",
		Available:   true,
		Controls:    cisV8Controls,
	})
}

const cisV8Source = "CIS Controls v8, Control "

var cisV8Controls = []CatalogControl{
	{"CIS-1", "Inventory and Control of Enterprise Assets", "Actively manage (inventory, track and correct) all enterprise assets connected to the infrastructure, so only authorized devices are given access and unauthorized/unmanaged devices are found and remediated.", cisV8Source + "1"},
	{"CIS-2", "Inventory and Control of Software Assets", "Actively manage all software (operating systems and applications) on the network so only authorized software is installed and can execute, and unauthorized software is found and prevented from installing or executing.", cisV8Source + "2"},
	{"CIS-3", "Data Protection", "Develop processes and technical controls to identify, classify, securely handle, retain and dispose of data.", cisV8Source + "3"},
	{"CIS-4", "Secure Configuration of Enterprise Assets and Software", "Establish and maintain the secure configuration of enterprise assets (end-user devices, servers, network devices) and software (operating systems and applications).", cisV8Source + "4"},
	{"CIS-5", "Account Management", "Use processes and tools to assign and manage authorization to credentials for user, administrator and service accounts across the enterprise.", cisV8Source + "5"},
	{"CIS-6", "Access Control Management", "Use processes and tools to create, assign, manage and revoke access credentials and privileges for user, administrator and service accounts.", cisV8Source + "6"},
	{"CIS-7", "Continuous Vulnerability Management", "Develop a plan to continuously assess and track vulnerabilities on all enterprise assets, and to remediate and minimize the window of opportunity for attackers.", cisV8Source + "7"},
	{"CIS-8", "Audit Log Management", "Collect, alert on, review and retain audit logs of events that could help detect, understand or recover from an attack.", cisV8Source + "8"},
	{"CIS-9", "Email and Web Browser Protections", "Improve protections and detections of threats from email and web vectors, which attackers use to manipulate human behavior through direct engagement.", cisV8Source + "9"},
	{"CIS-10", "Malware Defenses", "Prevent or control the installation, spread and execution of malicious applications, code or scripts on enterprise assets.", cisV8Source + "10"},
	{"CIS-11", "Data Recovery", "Establish and maintain data-recovery practices sufficient to restore in-scope enterprise assets to a pre-incident, trusted state.", cisV8Source + "11"},
	{"CIS-12", "Network Infrastructure Management", "Establish, implement and actively manage network devices to prevent attackers from exploiting vulnerable network services and access points.", cisV8Source + "12"},
	{"CIS-13", "Network Monitoring and Defense", "Operate processes and tooling to establish and maintain comprehensive network monitoring and defense against security threats across the enterprise's network infrastructure and user base.", cisV8Source + "13"},
	{"CIS-14", "Security Awareness and Skills Training", "Establish and maintain a security-awareness program to influence behavior among the workforce to be security-conscious and properly skilled to reduce cybersecurity risks.", cisV8Source + "14"},
	{"CIS-15", "Service Provider Management", "Develop a process to evaluate service providers who hold sensitive data or are responsible for critical IT platforms or processes, to ensure they protect those assets appropriately.", cisV8Source + "15"},
	{"CIS-16", "Application Software Security", "Manage the security life cycle of in-house developed, hosted or acquired software to prevent, detect and remediate security weaknesses before they can affect the enterprise.", cisV8Source + "16"},
	{"CIS-17", "Incident Response Management", "Establish a program to develop and maintain incident-response capability (policies, plans, procedures, roles, training and communications) to prepare, detect and quickly respond to an attack.", cisV8Source + "17"},
	{"CIS-18", "Penetration Testing", "Test the effectiveness and resiliency of enterprise assets by identifying and exploiting weaknesses in controls (people, processes and technology), and simulating the objectives and actions of an attacker.", cisV8Source + "18"},
}
