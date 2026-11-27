// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package crosstenant

// Automation, reports, vendors' questionnaires, evidence and compliance.
var workflowProbes = []Probe{
	list("/api/v1/automation/rules", KindRule),
	get("/api/v1/automation/rules/:id", KindRule),
	put("/api/v1/automation/rules/:id", `{"name":"changed by attacker","trigger":"risk_created","actions":[{"type":"notify"}]}`, KindRule),
	del("/api/v1/automation/rules/:id", KindRule),
	post("/api/v1/automation/rules/:id/dry-run", `{}`, KindRule),
	post("/api/v1/automation/rules/:id/enable", `{}`, KindRule),
	get("/api/v1/automation/rules/:id/executions", KindRule),
	post("/api/v1/automation/rules/:id/run", `{"confirm":true}`, KindRule),
	post("/api/v1/automation/rules/:id/suspend", `{"reason":"attacker"}`, KindRule),

	list("/api/v1/reports", KindReport),
	get("/api/v1/reports/:reportId", KindReport),
	del("/api/v1/reports/:reportId", KindReport),
	post("/api/v1/reports/:reportId/comments", `{"body":"attacker comment"}`, KindReport),
	q(get("/api/v1/reports/:reportId/compare", KindReport), "with={{report}}"),
	get("/api/v1/reports/:reportId/progress", KindReport),
	post("/api/v1/reports/:reportId/transition", `{"to":"in_review"}`, KindReport),
	get("/api/v1/reports/:reportId/verify", KindReport),
	get("/api/v1/reports/:reportId/versions", KindReport),

	list("/api/v1/reports/board", KindBoardReport),
	get("/api/v1/reports/board/:reportId", KindBoardReport),
	patch("/api/v1/reports/board/:reportId", `{"title":"changed by attacker"}`, KindBoardReport),
	del("/api/v1/reports/board/:reportId", KindBoardReport),
	post("/api/v1/reports/board/:reportId/approve", `{}`, KindBoardReport),
	get("/api/v1/reports/board/:reportId/pdf", KindBoardReport),

	list("/api/v1/vendor-questionnaire-templates", KindTemplate),
	get("/api/v1/vendor-questionnaire-templates/:id", KindTemplate),
	put("/api/v1/vendor-questionnaire-templates/:id", `{"name":"changed by attacker","language":"en","questions":[{"text":"q","answer_type":"text"}]}`, KindTemplate),
	post("/api/v1/vendor-questionnaire-templates/:id/archive", `{}`, KindTemplate),

	list("/api/v1/evidence", KindEvidence),
	get("/api/v1/evidence/:evidenceId", KindEvidence),
	patch("/api/v1/evidence/:evidenceId", `{"description":"changed by attacker"}`, KindEvidence),
	del("/api/v1/evidence/:evidenceId", KindEvidence),
	post("/api/v1/evidence/:evidenceId/links", `{"control_ids":["{{control}}"]}`, KindEvidence, KindControl),
	post("/api/v1/evidence/:evidenceId/review", `{"decision":"accepted"}`, KindEvidence),

	list("/api/v1/compliance/frameworks", KindFramework),
	get("/api/v1/compliance/frameworks/:frameworkId", KindFramework),
	del("/api/v1/compliance/frameworks/:frameworkId", KindFramework),
	get("/api/v1/compliance/frameworks/:frameworkId/controls", KindFramework),
	post("/api/v1/compliance/frameworks/:frameworkId/controls", `{"name":"attacker control"}`, KindFramework),
	get("/api/v1/compliance/frameworks/:frameworkId/export", KindFramework),
	get("/api/v1/compliance/frameworks/:frameworkId/inherited-coverage", KindFramework),
	get("/api/v1/compliance/frameworks/:frameworkId/progress", KindFramework),
	get("/api/v1/compliance/frameworks/:frameworkId/report", KindFramework),
	get("/api/v1/compliance/controls/:controlId", KindControl),
	patch("/api/v1/compliance/controls/:controlId", `{"description":"changed by attacker"}`, KindControl),
	del("/api/v1/compliance/controls/:controlId", KindControl),
	get("/api/v1/compliance/controls/:controlId/evidences", KindControl),

	list("/api/v1/compliance/audits", KindAudit),
	get("/api/v1/compliance/audits/:id", KindAudit),
	patch("/api/v1/compliance/audits/:id", `{"summary":"changed by attacker"}`, KindAudit),
	del("/api/v1/compliance/audits/:id", KindAudit),
	post("/api/v1/compliance/audits/:id/generate-remediations", `{}`, KindAudit),

	list("/api/v1/compliance/remediations", KindRemediation),
	patch("/api/v1/compliance/remediations/:id", `{"description":"changed by attacker"}`, KindRemediation),
	del("/api/v1/compliance/remediations/:id", KindRemediation),

	get("/api/v1/compliance/internal-controls/:controlId", KindInternalControl),
	patch("/api/v1/compliance/internal-controls/:controlId", `{"description":"changed by attacker"}`, KindInternalControl),
	del("/api/v1/compliance/internal-controls/:controlId", KindInternalControl),
	get("/api/v1/compliance/internal-controls/:controlId/implementations", KindInternalControl),
	get("/api/v1/compliance/internal-controls/:controlId/requirements", KindInternalControl),
	get("/api/v1/compliance/internal-controls/:controlId/status", KindInternalControl),
	list("/api/v1/compliance/internal-controls", KindInternalControl),
	get("/api/v1/vendors/:id/chain", KindVendor),
	get("/api/v1/vendors/:id/assessments", KindVendor),
	list("/api/v1/vendors", KindVendor),
	post("/api/v1/vendors/:id/assets", `{"asset_id":"{{asset}}","verb":"depends_on"}`, KindVendor, KindAsset),
	post("/api/v1/vendors/:id/assessments", `{"template_id":"{{vqtemplate}}","due_at":"{{in30d}}","contact_email":"vendor@xt.example.com","contact_language":"en"}`, KindVendor, KindTemplate),
	get("/api/v1/vendor-assessments/:id", KindAssessment),
	post("/api/v1/vendor-assessments/:id/resend", `{}`, KindAssessment),
	post("/api/v1/vendor-assessments/:id/revoke", `{}`, KindAssessment),
}
