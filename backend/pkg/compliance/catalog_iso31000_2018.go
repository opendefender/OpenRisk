// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package compliance

// ISO 31000:2018 — Risk management — Guidelines. A generic (all-risk, not just
// information security) framework built on three parts: the Principles (Clause 4),
// the Framework (Clause 5) and the Process (Clause 6). We model each principle,
// framework component and process activity as an assessable item. Clause numbers
// and component names are ISO's own published structure and are reliable; the
// descriptions are original summaries, not ISO's own text. Verify against
// ISO 31000:2018 before an audit.

func init() {
	register(Catalog{
		Key:         "iso31000-2018",
		Name:        "ISO 31000",
		Version:     "2018",
		Description: "Risk management — Guidelines. The 8 principles, the risk management framework (leadership, integration, design, implementation, evaluation, improvement) and the risk management process.",
		Available:   true,
		Controls:    iso310002018Controls,
	})
}

const iso31000Source = "ISO 31000:2018, Clause "

var iso310002018Controls = []CatalogControl{
	// Clause 4 — Principles
	{"4-INT", "Principle: Integrated", "Risk management is an integral part of all organizational activities.", iso31000Source + "4 (Integrated)"},
	{"4-STR", "Principle: Structured and Comprehensive", "A structured and comprehensive approach to risk management contributes to consistent and comparable results.", iso31000Source + "4 (Structured and comprehensive)"},
	{"4-CUS", "Principle: Customized", "The risk management framework and process are customized and proportionate to the organization's external and internal context and objectives.", iso31000Source + "4 (Customized)"},
	{"4-INC", "Principle: Inclusive", "Appropriate and timely involvement of stakeholders enables their knowledge, views and perceptions to be considered.", iso31000Source + "4 (Inclusive)"},
	{"4-DYN", "Principle: Dynamic", "Risks can emerge, change or disappear as context changes; risk management anticipates, detects, acknowledges and responds to those changes in an appropriate and timely manner.", iso31000Source + "4 (Dynamic)"},
	{"4-INF", "Principle: Best Available Information", "The inputs to risk management are based on historical and current information, as well as on future expectations, with account taken of any limitations and uncertainties.", iso31000Source + "4 (Best available information)"},
	{"4-HUM", "Principle: Human and Cultural Factors", "Human behaviour and culture significantly influence all aspects of risk management at each level and stage.", iso31000Source + "4 (Human and cultural factors)"},
	{"4-IMP", "Principle: Continual Improvement", "Risk management is continually improved through learning and experience.", iso31000Source + "4 (Continual improvement)"},

	// Clause 5 — Framework
	{"5.2", "Leadership and Commitment", "Top management and oversight bodies ensure that risk management is integrated into all organizational activities and demonstrate leadership and commitment.", iso31000Source + "5.2"},
	{"5.3", "Integration", "Integrate risk management into the organization's structure, governance and all its activities, recognizing that it is dynamic and iterative.", iso31000Source + "5.3"},
	{"5.4", "Design", "Design the risk management framework by understanding the organization and its context, articulating commitment, assigning roles and authorities, and allocating resources.", iso31000Source + "5.4"},
	{"5.5", "Implementation", "Implement the framework by developing a plan, identifying decision-making, and ensuring arrangements for managing risk are understood and practised.", iso31000Source + "5.5"},
	{"5.6", "Evaluation", "Periodically measure the framework's performance against its purpose, implementation plans, indicators and expected behaviour.", iso31000Source + "5.6"},
	{"5.7", "Improvement", "Continually monitor, adapt and improve the risk management framework to address internal and external changes.", iso31000Source + "5.7"},

	// Clause 6 — Process
	{"6.2", "Communication and Consultation", "Assist relevant stakeholders in understanding risk, the basis for decisions, and the reasons particular actions are required, throughout the process.", iso31000Source + "6.2"},
	{"6.3", "Scope, Context and Criteria", "Define the scope of risk management activities and establish the external and internal context and the risk criteria.", iso31000Source + "6.3"},
	{"6.4.2", "Risk Identification", "Find, recognize and describe risks that might help or prevent the organization from achieving its objectives.", iso31000Source + "6.4.2"},
	{"6.4.3", "Risk Analysis", "Comprehend the nature of risk and its characteristics, including the level of risk, considering uncertainties, sources, consequences, likelihood, events, scenarios and controls.", iso31000Source + "6.4.3"},
	{"6.4.4", "Risk Evaluation", "Support decisions by comparing the results of risk analysis with the established risk criteria to determine where additional action is required.", iso31000Source + "6.4.4"},
	{"6.5", "Risk Treatment", "Select and implement options for addressing risk, formulate and implement risk treatment plans, and assess residual risk.", iso31000Source + "6.5"},
	{"6.6", "Monitoring and Review", "Assure and improve the quality and effectiveness of process design, implementation and outcomes through ongoing monitoring and periodic review.", iso31000Source + "6.6"},
	{"6.7", "Recording and Reporting", "Document and report the risk management process and its outcomes through appropriate mechanisms to support decisions and improve activities.", iso31000Source + "6.7"},
}
