// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// The timeline asks the trail for several entity types at once: a control is
// recorded as "compliance_control" by one writer and "control" by the other,
// and the Activity page filters by domain (#905). The chain repository — the
// one production reads — ignored EntityTypes, so a filtered feed came back
// unfiltered, and an incident's history (sequential ids like "5") could pick
// up another type's row with the same id.
func TestAuditChainList_FiltersByEntityTypes(t *testing.T) {
	db := newGovernanceDB(t)
	repo := NewGormAuditChainRepository(db)
	ctx := context.Background()
	tenant := uuid.New()
	for _, et := range []string{"incident", "mitigation", "control", "compliance_control", "risk"} {
		e := &domain.AuditEvent{TenantID: tenant, Action: domain.AuditActionCreate, EntityType: et, EntityID: "5", Summary: "create " + et}
		if err := repo.Append(ctx, e); err != nil {
			t.Fatalf("append %s: %v", et, err)
		}
	}

	got, _, err := repo.List(ctx, tenant, domain.AuditEventFilter{EntityTypes: []string{"control", "compliance_control"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d rows for two control types, want 2", len(got))
	}
	for _, e := range got {
		if e.EntityType != "control" && e.EntityType != "compliance_control" {
			t.Fatalf("a %s row came through the control filter", e.EntityType)
		}
	}

	one, _, err := repo.List(ctx, tenant, domain.AuditEventFilter{EntityTypes: []string{"incident"}, EntityID: "5"})
	if err != nil {
		t.Fatal(err)
	}
	if len(one) != 1 || one[0].EntityType != "incident" {
		t.Fatalf("incident 5's history picked up other types: %d rows", len(one))
	}
}
