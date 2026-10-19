// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package actioncenter

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
)

// #811 — an expired acceptance lands in the Action Center of the people the
// acceptance names, whatever their job role, and nobody else's unless their
// role covers it.
func expiredAcceptance(owner, approver uuid.UUID) domain.Risk {
	r := domain.Risk{ID: uuid.New(), TenantID: tenantA, Title: "Accès VPN hérité"}
	r.OwnerID = &owner
	r.AcceptanceApprovedBy = &approver
	r.AcceptanceExpiresAt = ptime(testNow.AddDate(0, 0, -2))
	r.AcceptanceExpiredAt = ptime(testNow.AddDate(0, 0, -2))
	r.SetState(domain.StateAssessed)
	return r
}

func TestActionCenter_ExpiredAcceptance_ShownToOwnerAndApprover(t *testing.T) {
	for name, r := range map[string]domain.Risk{
		"owner":    expiredAcceptance(userA, otherU),
		"approver": expiredAcceptance(otherU, userA),
	} {
		t.Run(name, func(t *testing.T) {
			s := newStub()
			s.role = domain.BusinessRoleViewer // a role that sees no category on its own
			s.expired = []domain.Risk{r}

			res, err := newUC(s).GetActionCenter(caller(), 20, 0)
			require.NoError(t, err)
			require.Equal(t, []ItemType{ItemTypeExpiredAcceptance}, types(res.Data))
			item := res.Data[0]
			require.Equal(t, RiskLink(r.ID.String()), item.DeepLink)
			require.Equal(t, domain.AcceptanceExpiredReason, item.Meta["reason"])
			require.Equal(t, RankExpiredAcceptance, item.CategoryRank)
		})
	}
}

func TestActionCenter_ExpiredAcceptance_HiddenFromBystanders(t *testing.T) {
	s := newStub()
	s.role = domain.BusinessRoleViewer
	s.expired = []domain.Risk{expiredAcceptance(otherU, otherU)}

	res, err := newUC(s).GetActionCenter(caller(), 20, 0)
	require.NoError(t, err)
	require.Empty(t, res.Data, "a viewer named nowhere on the acceptance must not see it")
}

func TestActionCenter_ExpiredAcceptance_ShownToRiskManagers(t *testing.T) {
	s := newStub()
	s.role = domain.BusinessRoleRiskManager
	s.expired = []domain.Risk{expiredAcceptance(otherU, otherU)}

	res, err := newUC(s).GetActionCenter(caller(), 20, 0)
	require.NoError(t, err)
	require.Equal(t, []ItemType{ItemTypeExpiredAcceptance}, types(res.Data))
}
