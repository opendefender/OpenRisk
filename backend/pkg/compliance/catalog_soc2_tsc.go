// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package compliance

// SOC 2 — the AICPA Trust Services Criteria (TSC, 2017 with 2022 revised points
// of focus): the Common Criteria (CC1–CC9, which every SOC 2 engagement covers)
// plus the criteria for the Availability, Confidentiality, Processing Integrity
// and Privacy categories. The criteria numbers (CC1.1, A1.1, …) are the AICPA's
// public structure and are reliable; descriptions here are original summaries of
// each criterion's intent, not the AICPA's own text (the TSC are © AICPA).

func init() {
	register(Catalog{
		Key:         "soc2-tsc",
		Name:        "SOC 2 (Trust Services Criteria)",
		Version:     "2017 (rev. 2022)",
		Description: "AICPA Trust Services Criteria — Common Criteria (CC1–CC9) plus Availability, Confidentiality, Processing Integrity and Privacy.",
		Available:   true,
		Controls:    soc2TSCControls,
	})
}

const soc2Source = "AICPA TSC 2017, "

var soc2TSCControls = []CatalogControl{
	// --- CC1 Control Environment ---
	{"CC1.1", "Commitment to Integrity and Ethical Values", "The entity demonstrates a commitment to integrity and ethical values.", soc2Source + "CC1.1"},
	{"CC1.2", "Board Independence and Oversight", "The board of directors demonstrates independence from management and exercises oversight of the development and performance of internal control.", soc2Source + "CC1.2"},
	{"CC1.3", "Structures, Reporting Lines, Authorities", "Management establishes, with board oversight, structures, reporting lines, and appropriate authorities and responsibilities in pursuit of objectives.", soc2Source + "CC1.3"},
	{"CC1.4", "Commitment to Competence", "The entity demonstrates a commitment to attract, develop, and retain competent individuals in alignment with objectives.", soc2Source + "CC1.4"},
	{"CC1.5", "Accountability", "The entity holds individuals accountable for their internal control responsibilities in the pursuit of objectives.", soc2Source + "CC1.5"},

	// --- CC2 Communication and Information ---
	{"CC2.1", "Quality Information", "The entity obtains or generates and uses relevant, quality information to support the functioning of internal control.", soc2Source + "CC2.1"},
	{"CC2.2", "Internal Communication", "The entity internally communicates information, including objectives and responsibilities for internal control, necessary to support its functioning.", soc2Source + "CC2.2"},
	{"CC2.3", "External Communication", "The entity communicates with external parties regarding matters affecting the functioning of internal control.", soc2Source + "CC2.3"},

	// --- CC3 Risk Assessment ---
	{"CC3.1", "Specify Objectives", "The entity specifies objectives with sufficient clarity to enable the identification and assessment of risks relating to objectives.", soc2Source + "CC3.1"},
	{"CC3.2", "Identify and Analyze Risk", "The entity identifies risks to the achievement of its objectives across the entity and analyzes them as a basis for determining how they should be managed.", soc2Source + "CC3.2"},
	{"CC3.3", "Consider Fraud", "The entity considers the potential for fraud in assessing risks to the achievement of objectives.", soc2Source + "CC3.3"},
	{"CC3.4", "Identify and Assess Change", "The entity identifies and assesses changes that could significantly impact the system of internal control.", soc2Source + "CC3.4"},

	// --- CC4 Monitoring Activities ---
	{"CC4.1", "Evaluate Controls", "The entity selects, develops, and performs ongoing and/or separate evaluations to ascertain whether the components of internal control are present and functioning.", soc2Source + "CC4.1"},
	{"CC4.2", "Communicate Deficiencies", "The entity evaluates and communicates internal control deficiencies in a timely manner to those responsible for taking corrective action.", soc2Source + "CC4.2"},

	// --- CC5 Control Activities ---
	{"CC5.1", "Select and Develop Controls", "The entity selects and develops control activities that contribute to the mitigation of risks to the achievement of objectives to acceptable levels.", soc2Source + "CC5.1"},
	{"CC5.2", "Technology Controls", "The entity selects and develops general control activities over technology to support the achievement of objectives.", soc2Source + "CC5.2"},
	{"CC5.3", "Deploy Through Policies", "The entity deploys control activities through policies that establish what is expected and procedures that put policies into action.", soc2Source + "CC5.3"},

	// --- CC6 Logical and Physical Access Controls ---
	{"CC6.1", "Logical Access Security", "The entity implements logical access security software, infrastructure, and architectures over protected information assets to protect them from security events.", soc2Source + "CC6.1"},
	{"CC6.2", "Registration and Authorization", "Prior to issuing system credentials, the entity registers and authorizes new internal and external users, and removes access when no longer required.", soc2Source + "CC6.2"},
	{"CC6.3", "Manage Access Rights", "The entity authorizes, modifies, or removes access to data, software, functions, and other protected assets based on roles, responsibilities, and least privilege.", soc2Source + "CC6.3"},
	{"CC6.4", "Restrict Physical Access", "The entity restricts physical access to facilities and protected information assets to authorized personnel.", soc2Source + "CC6.4"},
	{"CC6.5", "Dispose of Assets", "The entity discontinues logical and physical protections over physical assets only after the ability to read or recover data has been diminished and is no longer required.", soc2Source + "CC6.5"},
	{"CC6.6", "Protect Against External Threats", "The entity implements logical access security measures to protect against threats from sources outside its system boundaries.", soc2Source + "CC6.6"},
	{"CC6.7", "Restrict Transmission and Movement", "The entity restricts the transmission, movement, and removal of information to authorized users and processes, and protects it during transmission, movement, or removal.", soc2Source + "CC6.7"},
	{"CC6.8", "Prevent Unauthorized Software", "The entity implements controls to prevent or detect and act upon the introduction of unauthorized or malicious software.", soc2Source + "CC6.8"},

	// --- CC7 System Operations ---
	{"CC7.1", "Detect Configuration Changes and Vulnerabilities", "The entity uses detection and monitoring procedures to identify changes to configurations that introduce new vulnerabilities and susceptibilities to newly discovered vulnerabilities.", soc2Source + "CC7.1"},
	{"CC7.2", "Monitor for Anomalies", "The entity monitors system components and the operation of those components for anomalies indicative of malicious acts, natural disasters, and errors, and evaluates them.", soc2Source + "CC7.2"},
	{"CC7.3", "Evaluate Security Events", "The entity evaluates security events to determine whether they could or have resulted in a failure to meet objectives (a security incident), and responds accordingly.", soc2Source + "CC7.3"},
	{"CC7.4", "Respond to Incidents", "The entity responds to identified security incidents by executing a defined incident-response program to understand, contain, remediate, and communicate them.", soc2Source + "CC7.4"},
	{"CC7.5", "Recover from Incidents", "The entity identifies, develops, and implements activities to recover from identified security incidents.", soc2Source + "CC7.5"},

	// --- CC8 Change Management ---
	{"CC8.1", "Manage Changes", "The entity authorizes, designs, develops or acquires, configures, documents, tests, approves, and implements changes to infrastructure, data, software, and procedures to meet its objectives.", soc2Source + "CC8.1"},

	// --- CC9 Risk Mitigation ---
	{"CC9.1", "Mitigate Business Disruption Risk", "The entity identifies, selects, and develops risk mitigation activities for risks arising from potential business disruptions.", soc2Source + "CC9.1"},
	{"CC9.2", "Manage Vendor and Partner Risk", "The entity assesses and manages risks associated with vendors and business partners.", soc2Source + "CC9.2"},

	// --- Availability (A1) ---
	{"A1.1", "Capacity Management", "The entity maintains, monitors, and evaluates current processing capacity and use of system components to manage capacity demand and enable the implementation of additional capacity to meet its availability objectives.", soc2Source + "A1.1"},
	{"A1.2", "Environmental Protections and Backup", "The entity authorizes, designs, develops or acquires, implements, operates, approves, maintains, and monitors environmental protections, software, data backup processes, and recovery infrastructure to meet its availability objectives.", soc2Source + "A1.2"},
	{"A1.3", "Test Recovery", "The entity tests recovery plan procedures supporting system recovery to meet its availability objectives.", soc2Source + "A1.3"},

	// --- Confidentiality (C1) ---
	{"C1.1", "Identify and Maintain Confidential Information", "The entity identifies and maintains confidential information to meet its confidentiality objectives.", soc2Source + "C1.1"},
	{"C1.2", "Dispose of Confidential Information", "The entity disposes of confidential information to meet its confidentiality objectives.", soc2Source + "C1.2"},

	// --- Processing Integrity (PI1) ---
	{"PI1.1", "Quality Definitions and Communication", "The entity obtains or generates, uses, and communicates relevant, quality information regarding the definitions of data processed and product/service specifications to support the use of products and services.", soc2Source + "PI1.1"},
	{"PI1.2", "Complete and Accurate Inputs", "The entity implements policies and procedures over system inputs, including controls over completeness and accuracy, to meet its processing integrity objectives.", soc2Source + "PI1.2"},
	{"PI1.3", "Complete and Accurate Processing", "The entity implements policies and procedures over system processing to result in products, services, and reporting that meet the entity's processing integrity objectives.", soc2Source + "PI1.3"},
	{"PI1.4", "Accurate and Timely Outputs", "The entity implements policies and procedures to make available or deliver output completely, accurately, and timely in accordance with specifications to meet its processing integrity objectives.", soc2Source + "PI1.4"},
	{"PI1.5", "Store Inputs and Outputs", "The entity implements policies and procedures to store inputs, items in processing, and outputs completely, accurately, and timely in accordance with system specifications to meet its processing integrity objectives.", soc2Source + "PI1.5"},

	// --- Privacy (P1–P8) ---
	{"P1.1", "Notice of Privacy Practices", "The entity provides notice to data subjects about its privacy practices to meet its objectives related to privacy.", soc2Source + "P1.1"},
	{"P2.1", "Choice and Consent", "The entity communicates choices available regarding the collection, use, retention, disclosure, and disposal of personal information and obtains consent as needed to meet its privacy objectives.", soc2Source + "P2.1"},
	{"P3.1", "Collection Consistent with Objectives", "The entity collects personal information consistent with its objectives related to privacy.", soc2Source + "P3.1"},
	{"P4.1", "Use, Retention, and Disposal", "The entity limits the use, retention, and disposal of personal information to meet its objectives related to privacy.", soc2Source + "P4.1"},
	{"P5.1", "Access by Data Subjects", "The entity grants identified and authenticated data subjects the ability to access their stored personal information for review and, upon request, correction to meet its privacy objectives.", soc2Source + "P5.1"},
	{"P6.1", "Disclosure to Third Parties", "The entity discloses personal information to third parties with the explicit consent of data subjects, and such consent is obtained prior to disclosure, to meet its privacy objectives.", soc2Source + "P6.1"},
	{"P7.1", "Quality of Personal Information", "The entity collects and maintains accurate, up-to-date, complete, and relevant personal information to meet its objectives related to privacy.", soc2Source + "P7.1"},
	{"P8.1", "Monitoring and Enforcement", "The entity implements a process for receiving, addressing, resolving, and communicating the resolution of inquiries, complaints, and disputes from data subjects to meet its privacy objectives.", soc2Source + "P8.1"},
}
