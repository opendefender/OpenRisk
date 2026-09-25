// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import "github.com/google/uuid"

// Auditable opt-ins (spec §15). A model gains automatic, immutable audit-trail
// coverage for every struct-form Create/Update/Delete simply by declaring its
// entity type here — the audittrail GORM plugin does the rest, so a developer
// can never forget to journal a mutation. To cover a new entity, add one line.
//
// Risk IS listed (#486), with a narrow snapshot. It used to be left out because
// the Score Engine worker writes it on a hot path; that worker issues targeted
// map updates the plugin skips, and it now journals its own score changes
// explicitly (workers.ScoreWorker). What the plugin adds is the before → after
// of every person's edit to the terms of the score, which is what ScoreWorking
// cites as each term's source record.

func (Asset) AuditEntityType() string { return "asset" }

func (ComplianceControl) AuditEntityType() string { return "compliance_control" }

// Operations module (incidents, automation, governance). These are the entities
// whose before → after an auditor actually asks about: who changed a rule's
// conditions, who reopened an incident, who edited an approval chain.
func (Incident) AuditEntityType() string { return "incident" }

func (AutomationRule) AuditEntityType() string { return "automation_rule" }

func (AutomationChannelConfig) AuditEntityType() string { return "automation_channel_config" }

func (ApprovalWorkflow) AuditEntityType() string { return "approval_workflow" }

func (ApprovalRequest) AuditEntityType() string { return "approval_request" }

func (Delegation) AuditEntityType() string { return "delegation" }

func (Mitigation) AuditEntityType() string { return "mitigation" }

func (ComplianceFramework) AuditEntityType() string { return "compliance_framework" }

func (Vulnerability) AuditEntityType() string { return "vulnerability" }

// Organization membership (W0-04). Who joined, whose role changed, who was
// deactivated and by whom is precisely the question an auditor opens the trail
// to answer. Invitation carries `json:"-"` on its token hash, so the plugin's
// json snapshot cannot capture the credential.
func (OrganizationMember) AuditEntityType() string { return "organization_member" }

func (Invitation) AuditEntityType() string { return "invitation" }

// The tenant's MFA grace policy. A change to it moves the deadline by which
// privileged accounts must hold a second factor, which is exactly the kind of
// security decision an auditor asks who made and when (OR26-03, invariant 7).
func (MFAPolicy) AuditEntityType() string { return "mfa_policy" }

func (Risk) AuditEntityType() string { return "risk" }

// AuditSnapshot is the part of a risk the trail captures: the terms of the
// frozen score formula, the result, and the fields a regulator asks who changed
// (status, ownership, classification). Deliberately not the whole row — the
// financial drivers, free text and associations would bury the change in noise.
func (r Risk) AuditSnapshot() map[string]interface{} {
	uuidOrNil := func(id *uuid.UUID) interface{} {
		if id == nil || *id == uuid.Nil {
			return nil
		}
		return id.String()
	}
	title := r.Title
	if title == "" {
		title = r.Name
	}
	tenant := r.TenantID
	if tenant == uuid.Nil {
		tenant = r.OrganizationID
	}
	return map[string]interface{}{
		"id":          r.ID.String(),
		"tenant_id":   tenant.String(),
		"title":       title,
		"probability": r.Probability,
		"impact":      r.Impact,
		"score":       r.Score,
		"criticality": string(r.Criticality),
		"status":      string(r.Status),
		"asset_id":    uuidOrNil(r.AssetID),
		"owner_id":    uuidOrNil(r.OwnerID),
		"assignee_id": uuidOrNil(r.AssigneeID),
		"reviewer_id": uuidOrNil(r.ReviewerID),
		"category_id": uuidOrNil(r.CategoryID),
	}
}
