// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package actioncenter

// Deep links. Every path here was checked against the real route table in
// frontend/src/shared/routeModel.ts and frontend/src/App.tsx. An Action Center
// whose links 404 is the "inert control presented as live" the constitution
// forbids (CLAUDE.md rule 12) — and it would be worse than no Action Center,
// because the list looks authoritative.
//
// TWO SHAPES, because the frontend has two ways of opening a thing:
//
//  1. A real route with its own page — mitigations, incidents, remediation
//     plans. These take the id in the path.
//
//  2. The universal entity drawer — risks and evidence. There is NO
//     `/risks/:riskId` page: the risk register opens a risk in a drawer whose
//     state lives entirely in the query string
//     (frontend/src/features/entity-drawer/drawerState.ts: `drawer` = entity
//     type, `entity` = id). The link therefore targets the list route and asks
//     it to open the drawer, which is the only shareable URL for a risk that
//     the product actually has.
//
// The #429 spec suggested `/risks/{id}`; that route does not exist and was not
// implemented. See the PR for the correction.

const (
	// routeMitigationDetail — /risks/mitigations/:mitigationId (App.tsx).
	routeMitigationDetail = "/risks/mitigations/"
	// routeRiskRegister — /risks, opened with the drawer params.
	routeRiskRegister = "/risks"
	// routeGovernance — /governance. Approvals have no per-request route; the
	// governance screen is where a pending request is signed.
	routeGovernance = "/governance"
	// routeIncidentWarRoom — /incidents/:id/war-room (App.tsx).
	routeIncidentWarRoom = "/incidents/"
	// routeEvidenceLibrary — /compliance/evidence, opened with the drawer params.
	routeEvidenceLibrary = "/compliance/evidence"
	// routeRemediationDetail — /compliance/remediation/:planId (routeModel.ts).
	routeRemediationDetail = "/compliance/remediation/"
)

// drawerQuery builds the entity-drawer query string. Kept in one function so
// the two drawer-backed categories cannot drift apart.
func drawerQuery(entityType, id string) string {
	return "?drawer=" + entityType + "&entity=" + id
}

func MitigationLink(id string) string  { return routeMitigationDetail + id }
func RiskLink(id string) string        { return routeRiskRegister + drawerQuery("risk", id) }
func ApprovalLink() string             { return routeGovernance }
func IncidentLink(id string) string    { return routeIncidentWarRoom + id + "/war-room" }
func EvidenceLink(id string) string    { return routeEvidenceLibrary + drawerQuery("evidence", id) }
func RemediationLink(id string) string { return routeRemediationDetail + id }
