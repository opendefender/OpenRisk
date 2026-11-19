// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package risk

import (
	"context"

	"github.com/google/uuid"

	bulkapp "github.com/opendefender/openrisk/internal/application/bulk"
	"github.com/opendefender/openrisk/internal/domain"
)

// RiskOwnerLookup reads one risk, tenant-scoped. domain.RiskRepository
// satisfies it.
type RiskOwnerLookup interface {
	GetByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.Risk, error)
}

// BulkStateNotices tells each risk's owner when a governed bulk status change
// moved their risk (#757), as the legacy POST /risks/bulk does. It runs after
// the batch has committed (bulkapp.AfterApply), so a rolled-back batch has told
// nobody.
//
// The owner is read back per risk, tenant-scoped: the mutation carries the
// before and after state but not the owner, and the engine stays ignorant of
// what an owner is. A risk that cannot be read is skipped, never failed — the
// change has already happened, and a notice is best-effort.
func BulkStateNotices(risks RiskOwnerLookup, notices NoticeDeliverer) bulkapp.AfterApply {
	return func(ctx context.Context, caller bulkapp.Caller, change domain.BulkChange, mutations []domain.BulkMutation) {
		if notices == nil || risks == nil || change.Action != domain.BulkActionChangeStatus {
			return
		}
		for _, m := range mutations {
			from, _ := m.Before["lifecycle_state"].(string)
			to, _ := m.After["lifecycle_state"].(string)
			if from == to {
				continue
			}
			r, err := risks.GetByID(ctx, m.ID, caller.TenantID)
			if err != nil || r == nil {
				continue
			}
			notifyRiskStateChange(ctx, notices, caller.TenantID,
				riskStateChangeOf(r, domain.RiskState(from), domain.RiskState(to)), caller.UserID)
		}
	}
}
