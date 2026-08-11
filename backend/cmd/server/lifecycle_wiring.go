// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package main

import (
	"context"
	"strings"

	"github.com/google/uuid"
	riskapp "github.com/opendefender/openrisk/internal/application/risk"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
)

// Adapters that let the risk lifecycle FSM ask its guards real questions
// without the risk module importing the mitigation or governance modules.
//
// Same shape as executive_wiring.go: the ports are declared by the consumer
// (application/risk), and the adapters live here in the composition root.

// ---------------------------------------------------------------------------
// Guard 1 & 2 — "IN_TREATMENT needs an active mitigation" and "MITIGATED needs
// every sub-action done".
// ---------------------------------------------------------------------------

type mitigationInspectorAdapter struct {
	plans      repository.MitigationRepository
	subActions repository.MitigationSubActionRepository
}

func newMitigationInspector(plans repository.MitigationRepository, subActions repository.MitigationSubActionRepository) riskapp.MitigationInspector {
	return &mitigationInspectorAdapter{plans: plans, subActions: subActions}
}

func (a *mitigationInspectorAdapter) SnapshotForRisk(_ context.Context, tenantID, riskID uuid.UUID) ([]riskapp.MitigationSnapshot, error) {
	rows, err := a.plans.ListByRiskID(tenantID.String(), riskID)
	if err != nil {
		return nil, err
	}
	out := make([]riskapp.MitigationSnapshot, 0, len(rows))
	for i := range rows {
		m := rows[i]
		snap := riskapp.MitigationSnapshot{
			ID:    m.ID,
			Ref:   mitigationRef(m),
			Title: m.Title,
			// Cancelled plans are dead weight; anything else is real work that
			// the lifecycle must account for.
			Active: m.Status != domain.MitigationCancelled,
		}
		subs, err := a.subActions.List(tenantID.String(), m.ID)
		if err != nil {
			return nil, err
		}
		for _, s := range subs {
			snap.TotalSubActions++
			if s.Completed {
				snap.CompletedSubActions++
			}
		}
		out = append(out, snap)
	}
	return out, nil
}

// mitigationRef builds the human reference quoted in a blocking reason
// ("2 sous-actions restantes sur la mitigation MIT-3F2A1B").
//
// Honest limitation: mitigations have no per-tenant sequence number, so this is
// derived from the uuid rather than being the "MIT-14" of the spec's example.
// Giving them a real sequence is a schema change of its own; a stable, quotable
// handle is what the message actually needs.
func mitigationRef(m domain.Mitigation) string {
	hex := strings.ReplaceAll(m.ID.String(), "-", "")
	if len(hex) >= 6 {
		return "MIT-" + strings.ToUpper(hex[:6])
	}
	return "MIT-" + strings.ToUpper(hex)
}

// ---------------------------------------------------------------------------
// Guard 3 — "RESIDUAL_ACCEPTED needs a validated Governance approval".
// ---------------------------------------------------------------------------

type approvalCheckerAdapter struct {
	requests domain.ApprovalRequestRepository
}

func newApprovalChecker(requests domain.ApprovalRequestRepository) riskapp.ApprovalChecker {
	return &approvalCheckerAdapter{requests: requests}
}

// RiskAcceptanceEntityType is the approval workflow entity that authorises
// accepting a risk's residual risk. It matches the entity_type an admin binds a
// workflow to in Governance.
const RiskAcceptanceEntityType = "risk_acceptance"

func (a *approvalCheckerAdapter) HasApprovedAcceptance(ctx context.Context, tenantID, riskID uuid.UUID) (bool, *uuid.UUID, error) {
	// Tenant-scoped by the repository (RULE #2); the entity filter narrows to
	// acceptance requests, and the entity id is matched here because the filter
	// has no field for it.
	rows, err := a.requests.ListRequests(ctx, tenantID, domain.ApprovalRequestFilter{
		EntityType: RiskAcceptanceEntityType,
	})
	if err != nil {
		return false, nil, err
	}
	var pending *uuid.UUID
	want := riskID.String()
	for i := range rows {
		if rows[i].EntityID != want {
			continue
		}
		switch rows[i].Status {
		case domain.ApprovalApproved:
			// One validated approval is enough, and it wins over any other
			// request in flight.
			return true, nil, nil
		case domain.ApprovalPending:
			if pending == nil {
				id := rows[i].ID
				pending = &id
			}
		}
	}
	return false, pending, nil
}
