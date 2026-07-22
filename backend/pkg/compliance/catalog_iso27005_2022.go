// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package compliance

// ISO/IEC 27005:2022 — Guidance on managing information security risks. Unlike
// ISO 27001, this is a process standard, not a control set: it describes the
// activities of the information security risk management process. We model it at
// the clause/activity level so a tenant can assess whether each activity of the
// process is in place. Clause numbers are ISO's own published structure and are
// reliable; the descriptions are original summaries of each activity's intent,
// not ISO's own text. Verify against ISO/IEC 27005:2022 before an audit.

func init() {
	register(Catalog{
		Key:         "iso27005-2022",
		Name:        "ISO/IEC 27005",
		Version:     "2022",
		Description: "Information security risk management — the activities of the ISO 27005 risk management process (context, assessment, treatment, operation, ISMS integration).",
		Available:   true,
		Controls:    iso270052022Controls,
	})
}

const iso27005Source = "ISO/IEC 27005:2022, Clause "

var iso270052022Controls = []CatalogControl{
	// Clause 5 — Information security risk management process
	{"5", "Information Security Risk Management Process", "Establish and run an iterative information security risk management process aligned with the organization's ISMS, covering context establishment, assessment, treatment, acceptance, communication and monitoring.", iso27005Source + "5"},

	// Clause 6 — Context establishment
	{"6.1", "Organizational Considerations", "Establish the internal and external context, purpose and organizational considerations that frame information security risk management.", iso27005Source + "6.1"},
	{"6.2", "Identifying Basic Requirements of Interested Parties", "Identify the information security requirements and expectations of interested parties that risk management must satisfy.", iso27005Source + "6.2"},
	{"6.3", "Applying Risk Assessment", "Define and document the risk criteria — risk acceptance criteria and criteria for performing risk assessments — before assessment begins.", iso27005Source + "6.3"},

	// Clause 7 — Information security risk assessment process
	{"7.1", "General Risk Assessment Approach", "Define the overall approach to identifying, analysing and evaluating information security risks consistently across the organization.", iso27005Source + "7.1"},
	{"7.2", "Identifying Risks", "Identify the risks associated with the loss of confidentiality, integrity and availability of information, using an event-based and/or asset-based approach.", iso27005Source + "7.2"},
	{"7.3", "Analysing Risks", "Assess the potential consequences and the likelihood of identified risks and determine the resulting level of risk.", iso27005Source + "7.3"},
	{"7.4", "Evaluating Risks", "Compare the results of risk analysis against the risk criteria to prioritize risks and decide which require treatment.", iso27005Source + "7.4"},

	// Clause 8 — Information security risk treatment process
	{"8.1", "General Risk Treatment Approach", "Define how risk treatment options are selected and how the treatment process is documented and approved.", iso27005Source + "8.1"},
	{"8.2", "Selecting Risk Treatment Options", "Select appropriate treatment options (modify, retain, avoid or share the risk) based on the risk assessment results.", iso27005Source + "8.2"},
	{"8.3", "Determining Controls", "Determine the controls necessary to implement the chosen treatment options and compare them with a reference control set such as ISO 27001 Annex A.", iso27005Source + "8.3"},
	{"8.4", "Producing a Statement of Applicability", "Produce a Statement of Applicability justifying the inclusion or exclusion of controls and their implementation status.", iso27005Source + "8.4"},
	{"8.5", "Risk Treatment Plan", "Formulate a risk treatment plan assigning owners, resources, priorities and timelines to the selected controls.", iso27005Source + "8.5"},
	{"8.6", "Residual Risk Acceptance", "Obtain the risk owners' documented approval of the risk treatment plan and their acceptance of the residual risks.", iso27005Source + "8.6"},

	// Clause 9 — Operation
	{"9.1", "Performing Information Security Risk Assessment", "Perform information security risk assessments at planned intervals and when significant changes occur, retaining documented results.", iso27005Source + "9.1"},
	{"9.2", "Performing Information Security Risk Treatment", "Implement the risk treatment plan and retain documented information on the results of risk treatment.", iso27005Source + "9.2"},

	// Clause 10 — Leveraging related ISMS processes
	{"10.1", "Context of the Organization", "Integrate risk management with the ISMS's understanding of the organization and its context.", iso27005Source + "10.1"},
	{"10.2", "Monitoring and Review", "Continually monitor and review risks and the risk factors (value of assets, threats, vulnerabilities, likelihood, consequences) to keep the risk picture current.", iso27005Source + "10.2"},
	{"10.3", "Communication and Consultation", "Communicate and consult with internal and external interested parties about information security risks throughout the process.", iso27005Source + "10.3"},
}
