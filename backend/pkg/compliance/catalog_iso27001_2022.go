// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package compliance

// ISO/IEC 27001:2022 Annex A — 93 controls across 4 themes (Organizational, People,
// Physical, Technological). Reference codes and control titles follow the published
// structure of the 2022 revision. Descriptions below are original, written for this
// product to summarize each control's intent — they are not the standard's own control
// text, which is copyrighted and licensed separately by ISO.
//
// This content has not yet had a dedicated compliance-expert review pass; treat reference
// codes/titles as reliable (they're the standard's own public structure) but verify exact
// wording against a licensed copy of ISO/IEC 27001:2022 before using this in a real audit.

func init() {
	register(Catalog{
		Key:         "iso27001-2022",
		Name:        "ISO/IEC 27001",
		Version:     "2022",
		Description: "Information security management system (ISMS) — Annex A reference controls.",
		Available:   true,
		Controls:    iso27001_2022Controls,
	})
}

const iso27001Source = "ISO/IEC 27001:2022, Annexe A, "

var iso27001_2022Controls = []CatalogControl{
	// --- A.5 Organizational controls (37) ---
	{"A.5.1", "Policies for information security", "Define, approve, publish, and periodically review a set of information security policies covering the organization's overall approach and topic-specific rules.", iso27001Source + "A.5.1"},
	{"A.5.2", "Information security roles and responsibilities", "Define and allocate information security roles and responsibilities according to organizational needs.", iso27001Source + "A.5.2"},
	{"A.5.3", "Segregation of duties", "Separate conflicting duties and areas of responsibility to reduce opportunities for unauthorized or unintentional modification or misuse of assets.", iso27001Source + "A.5.3"},
	{"A.5.4", "Management responsibilities", "Require management to ensure all personnel apply information security in accordance with established policies and procedures.", iso27001Source + "A.5.4"},
	{"A.5.5", "Contact with authorities", "Establish and maintain appropriate contacts with relevant authorities (law enforcement, regulators, supervisory bodies).", iso27001Source + "A.5.5"},
	{"A.5.6", "Contact with special interest groups", "Maintain contact with special interest groups, security forums, and professional associations to stay current on threats and best practice.", iso27001Source + "A.5.6"},
	{"A.5.7", "Threat intelligence", "Collect and analyze information relating to information security threats to produce actionable threat intelligence.", iso27001Source + "A.5.7"},
	{"A.5.8", "Information security in project management", "Integrate information security requirements into project management, regardless of project type.", iso27001Source + "A.5.8"},
	{"A.5.9", "Inventory of information and other associated assets", "Develop and maintain an inventory of information and other associated assets, including owners.", iso27001Source + "A.5.9"},
	{"A.5.10", "Acceptable use of information and other associated assets", "Identify, document, and implement rules for the acceptable use and handling procedures of assets.", iso27001Source + "A.5.10"},
	{"A.5.11", "Return of assets", "Require personnel and other interested parties to return all organizational assets in their possession upon change or termination of employment, contract, or agreement.", iso27001Source + "A.5.11"},
	{"A.5.12", "Classification of information", "Classify information according to the confidentiality, integrity, availability, and relevant interested-party requirements.", iso27001Source + "A.5.12"},
	{"A.5.13", "Labelling of information", "Develop and implement procedures for information labelling consistent with the adopted classification scheme.", iso27001Source + "A.5.13"},
	{"A.5.14", "Information transfer", "Put in place rules, procedures, or agreements to protect information transferred internally and with external parties.", iso27001Source + "A.5.14"},
	{"A.5.15", "Access control", "Establish and implement rules to control physical and logical access to information and other associated assets based on business and security requirements.", iso27001Source + "A.5.15"},
	{"A.5.16", "Identity management", "Manage the full lifecycle of identities to enable unique identification of individuals and systems and appropriate assignment of access rights.", iso27001Source + "A.5.16"},
	{"A.5.17", "Authentication information", "Control the allocation and management of authentication information through a proper management process, including advice to personnel.", iso27001Source + "A.5.17"},
	{"A.5.18", "Access rights", "Provision, review, modify, and remove access rights in accordance with the organization's access control policy and rules.", iso27001Source + "A.5.18"},
	{"A.5.19", "Information security in supplier relationships", "Define and implement processes and procedures to manage the information security risks associated with the use of supplier products or services.", iso27001Source + "A.5.19"},
	{"A.5.20", "Addressing information security within supplier agreements", "Establish and agree relevant information security requirements with each supplier based on the type of supplier relationship.", iso27001Source + "A.5.20"},
	{"A.5.21", "Managing information security in the ICT supply chain", "Define and implement processes and procedures to manage the information security risks associated with the ICT products and services supply chain.", iso27001Source + "A.5.21"},
	{"A.5.22", "Monitoring, review and change management of supplier services", "Regularly monitor, review, evaluate, and manage change in supplier information security practices and service delivery.", iso27001Source + "A.5.22"},
	{"A.5.23", "Information security for use of cloud services", "Establish processes for the acquisition, use, management, and exit from cloud services in accordance with the organization's information security requirements.", iso27001Source + "A.5.23"},
	{"A.5.24", "Information security incident management planning and preparation", "Plan and prepare for managing information security incidents by defining, establishing, and communicating processes, roles, and responsibilities.", iso27001Source + "A.5.24"},
	{"A.5.25", "Assessment and decision on information security events", "Assess information security events and decide whether they are to be categorized as information security incidents.", iso27001Source + "A.5.25"},
	{"A.5.26", "Response to information security incidents", "Respond to information security incidents in accordance with documented procedures.", iso27001Source + "A.5.26"},
	{"A.5.27", "Learning from information security incidents", "Use knowledge gained from information security incidents to strengthen and improve controls.", iso27001Source + "A.5.27"},
	{"A.5.28", "Collection of evidence", "Establish and implement procedures for the identification, collection, acquisition, and preservation of evidence related to information security events.", iso27001Source + "A.5.28"},
	{"A.5.29", "Information security during disruption", "Plan how to maintain information security at an appropriate level during disruption.", iso27001Source + "A.5.29"},
	{"A.5.30", "ICT readiness for business continuity", "Plan, implement, maintain, and test ICT readiness based on business continuity objectives and requirements.", iso27001Source + "A.5.30"},
	{"A.5.31", "Legal, statutory, regulatory and contractual requirements", "Identify, document, and keep up to date legal, statutory, regulatory, and contractual requirements relevant to information security.", iso27001Source + "A.5.31"},
	{"A.5.32", "Intellectual property rights", "Implement procedures to protect intellectual property rights while complying with legal, statutory, regulatory, and contractual requirements.", iso27001Source + "A.5.32"},
	{"A.5.33", "Protection of records", "Protect records from loss, destruction, falsification, unauthorized access, and unauthorized release.", iso27001Source + "A.5.33"},
	{"A.5.34", "Privacy and protection of PII", "Identify and meet requirements regarding the preservation of privacy and protection of personally identifiable information (PII).", iso27001Source + "A.5.34"},
	{"A.5.35", "Independent review of information security", "Review the organization's approach to managing information security and its implementation independently, at planned intervals or upon significant change.", iso27001Source + "A.5.35"},
	{"A.5.36", "Compliance with policies, rules and standards for information security", "Regularly review compliance with the organization's information security policy, topic-specific policies, rules, and standards.", iso27001Source + "A.5.36"},
	{"A.5.37", "Documented operating procedures", "Document and make available to personnel who need them the operating procedures for information processing facilities.", iso27001Source + "A.5.37"},

	// --- A.6 People controls (8) ---
	{"A.6.1", "Screening", "Carry out background verification checks on candidates prior to joining, and on an ongoing basis, proportional to business requirements, information classification, and perceived risk.", iso27001Source + "A.6.1"},
	{"A.6.2", "Terms and conditions of employment", "State personnel's and the organization's responsibilities for information security in employment contract terms.", iso27001Source + "A.6.2"},
	{"A.6.3", "Information security awareness, education and training", "Provide personnel and relevant interested parties with appropriate awareness, education, and training, and regular updates on organizational policies and procedures.", iso27001Source + "A.6.3"},
	{"A.6.4", "Disciplinary process", "Formalize and communicate a disciplinary process to take action against personnel and other relevant interested parties who have committed an information security policy violation.", iso27001Source + "A.6.4"},
	{"A.6.5", "Responsibilities after termination or change of employment", "Define and enforce information security responsibilities and duties that remain valid after termination or change of employment.", iso27001Source + "A.6.5"},
	{"A.6.6", "Confidentiality or non-disclosure agreements", "Identify, document, regularly review, and have personnel and relevant interested parties sign confidentiality or non-disclosure agreements.", iso27001Source + "A.6.6"},
	{"A.6.7", "Remote working", "Implement security measures when personnel are working remotely to protect information accessed, processed, or stored outside the organization's premises.", iso27001Source + "A.6.7"},
	{"A.6.8", "Information security event reporting", "Provide a mechanism for personnel to report observed or suspected information security events through appropriate channels in a timely manner.", iso27001Source + "A.6.8"},

	// --- A.7 Physical controls (14) ---
	{"A.7.1", "Physical security perimeters", "Define and use security perimeters to protect areas containing information and other associated assets.", iso27001Source + "A.7.1"},
	{"A.7.2", "Physical entry", "Protect secure areas by appropriate entry controls and access points.", iso27001Source + "A.7.2"},
	{"A.7.3", "Securing offices, rooms and facilities", "Design and implement physical security for offices, rooms, and facilities.", iso27001Source + "A.7.3"},
	{"A.7.4", "Physical security monitoring", "Continuously monitor premises for unauthorized physical access.", iso27001Source + "A.7.4"},
	{"A.7.5", "Protecting against physical and environmental threats", "Design and implement protection against physical and environmental threats such as natural disasters and other intentional or unintentional threats.", iso27001Source + "A.7.5"},
	{"A.7.6", "Working in secure areas", "Design and implement security measures for working in secure areas.", iso27001Source + "A.7.6"},
	{"A.7.7", "Clear desk and clear screen", "Define and appropriately enforce clear desk rules for papers and removable storage media, and clear screen rules for information processing facilities.", iso27001Source + "A.7.7"},
	{"A.7.8", "Equipment siting and protection", "Site equipment securely and protect it to reduce the risks from physical and environmental threats and unauthorized access.", iso27001Source + "A.7.8"},
	{"A.7.9", "Security of assets off-premises", "Protect off-site assets, taking into account the different risks of working outside the organization's premises.", iso27001Source + "A.7.9"},
	{"A.7.10", "Storage media", "Manage storage media through their lifecycle of acquisition, use, transportation, and disposal in accordance with the classification scheme and handling requirements.", iso27001Source + "A.7.10"},
	{"A.7.11", "Supporting utilities", "Protect information processing facilities from power failures and other disruptions caused by failures in supporting utilities.", iso27001Source + "A.7.11"},
	{"A.7.12", "Cabling security", "Protect power and telecommunications cabling carrying data or supporting information services from interception, interference, or damage.", iso27001Source + "A.7.12"},
	{"A.7.13", "Equipment maintenance", "Maintain equipment correctly to ensure the availability, integrity, and confidentiality of information.", iso27001Source + "A.7.13"},
	{"A.7.14", "Secure disposal or re-use of equipment", "Verify that all items of equipment containing storage media are checked to ensure sensitive data and licensed software have been removed or securely overwritten prior to disposal or re-use.", iso27001Source + "A.7.14"},

	// --- A.8 Technological controls (34) ---
	{"A.8.1", "User endpoint devices", "Protect information stored on, processed by, or accessible via user endpoint devices.", iso27001Source + "A.8.1"},
	{"A.8.2", "Privileged access rights", "Restrict and manage the allocation and use of privileged access rights.", iso27001Source + "A.8.2"},
	{"A.8.3", "Information access restriction", "Restrict access to information and other associated assets in accordance with the established access control policy.", iso27001Source + "A.8.3"},
	{"A.8.4", "Access to source code", "Appropriately manage read and write access to source code, development tools, and software libraries.", iso27001Source + "A.8.4"},
	{"A.8.5", "Secure authentication", "Implement secure authentication technologies and procedures based on information access restrictions and the access control policy.", iso27001Source + "A.8.5"},
	{"A.8.6", "Capacity management", "Monitor and adjust the use of resources in line with current and expected capacity requirements.", iso27001Source + "A.8.6"},
	{"A.8.7", "Protection against malware", "Implement protection against malware, supported by appropriate user awareness.", iso27001Source + "A.8.7"},
	{"A.8.8", "Management of technical vulnerabilities", "Obtain information about technical vulnerabilities of information systems in use, evaluate exposure, and take appropriate measures.", iso27001Source + "A.8.8"},
	{"A.8.9", "Configuration management", "Establish, document, implement, monitor, and review configurations, including security configurations, of hardware, software, services, and networks.", iso27001Source + "A.8.9"},
	{"A.8.10", "Information deletion", "Delete information stored in information systems, devices, or in any other storage media when no longer required.", iso27001Source + "A.8.10"},
	{"A.8.11", "Data masking", "Use data masking in accordance with the organization's access control policy, business requirements, and applicable regulations.", iso27001Source + "A.8.11"},
	{"A.8.12", "Data leakage prevention", "Apply data leakage prevention measures to systems, networks, and any other devices that process, store, or transmit sensitive information.", iso27001Source + "A.8.12"},
	{"A.8.13", "Information backup", "Maintain and regularly test backup copies of information, software, and systems in accordance with an agreed backup policy.", iso27001Source + "A.8.13"},
	{"A.8.14", "Redundancy of information processing facilities", "Implement information processing facilities with sufficient redundancy to meet availability requirements.", iso27001Source + "A.8.14"},
	{"A.8.15", "Logging", "Produce, store, protect, and analyze logs that record activities, exceptions, faults, and other relevant events.", iso27001Source + "A.8.15"},
	{"A.8.16", "Monitoring activities", "Monitor networks, systems, and applications for anomalous behavior and evaluate potential information security incidents.", iso27001Source + "A.8.16"},
	{"A.8.17", "Clock synchronization", "Synchronize the clocks of information processing systems used by the organization to approved time sources.", iso27001Source + "A.8.17"},
	{"A.8.18", "Use of privileged utility programs", "Restrict and tightly control the use of utility programs that might be capable of overriding system and application controls.", iso27001Source + "A.8.18"},
	{"A.8.19", "Installation of software on operational systems", "Implement procedures and measures to securely manage software installation on operational systems.", iso27001Source + "A.8.19"},
	{"A.8.20", "Networks security", "Secure, manage, and control networks and network devices to protect information in systems and applications.", iso27001Source + "A.8.20"},
	{"A.8.21", "Security of network services", "Identify, implement, and monitor security mechanisms, service levels, and service requirements of network services.", iso27001Source + "A.8.21"},
	{"A.8.22", "Segregation of networks", "Segregate groups of information services, users, and information systems in the organization's networks.", iso27001Source + "A.8.22"},
	{"A.8.23", "Web filtering", "Manage access to external websites to reduce exposure to malicious content.", iso27001Source + "A.8.23"},
	{"A.8.24", "Use of cryptography", "Define and implement rules for the effective use of cryptography, including cryptographic key management.", iso27001Source + "A.8.24"},
	{"A.8.25", "Secure development life cycle", "Establish and apply rules for the secure development of software and systems.", iso27001Source + "A.8.25"},
	{"A.8.26", "Application security requirements", "Identify, specify, and approve information security requirements when developing or acquiring applications.", iso27001Source + "A.8.26"},
	{"A.8.27", "Secure system architecture and engineering principles", "Establish, document, maintain, and apply principles for engineering secure systems across the development lifecycle.", iso27001Source + "A.8.27"},
	{"A.8.28", "Secure coding", "Apply secure coding principles to software development.", iso27001Source + "A.8.28"},
	{"A.8.29", "Security testing in development and acceptance", "Define and implement security testing processes in the development lifecycle.", iso27001Source + "A.8.29"},
	{"A.8.30", "Outsourced development", "Direct, monitor, and review the activities related to outsourced system development.", iso27001Source + "A.8.30"},
	{"A.8.31", "Separation of development, test and production environments", "Separate and secure development, testing, and production environments.", iso27001Source + "A.8.31"},
	{"A.8.32", "Change management", "Subject changes to information processing facilities and information systems to change management procedures.", iso27001Source + "A.8.32"},
	{"A.8.33", "Test information", "Appropriately select, protect, and manage test information.", iso27001Source + "A.8.33"},
	{"A.8.34", "Protection of information systems during audit testing", "Plan and agree audit tests and other assurance activities involving assessment of operational systems between the tester and appropriate management to minimize disruption.", iso27001Source + "A.8.34"},
}
