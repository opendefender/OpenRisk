// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// "Reporter au prochain comité" (#903) appends a dated note to a pending
// request and saves it through UpdateRequest. The use-case tests run on a fake
// repository, so they never saw that the update wrote a fixed column list
// without deferrals: the API answered 200, the audit trail said one deferral,
// and the row stayed empty.
func TestUpdateRequest_PersistsDeferrals(t *testing.T) {
	ctx := context.Background()
	repo := NewGormApprovalRepository(newGovernanceDB(t))

	req := &domain.ApprovalRequest{
		ID: uuid.New(), TenantID: govA,
		EntityType: "risk_acceptance", Action: "accept", Title: "accept R-0115",
		Status: domain.ApprovalPending, RequestedBy: uuid.New(),
	}
	if err := repo.CreateRequest(ctx, req); err != nil {
		t.Fatalf("create request: %v", err)
	}

	at := time.Date(2026, 10, 9, 8, 0, 0, 0, time.UTC)
	req.Deferrals = append(req.Deferrals, domain.ApprovalDeferral{
		DeferredBy: uuid.NewString(), DeferredByEmail: "rssi@example.com",
		Comment: "budget review first", DeferredAt: at,
	})
	if err := repo.UpdateRequest(ctx, req); err != nil {
		t.Fatalf("update request: %v", err)
	}

	got, err := repo.GetRequestByID(ctx, req.ID, govA)
	if err != nil || got == nil {
		t.Fatalf("reload request: %v", err)
	}
	if len(got.Deferrals) != 1 {
		t.Fatalf("deferral not persisted: got %d deferrals", len(got.Deferrals))
	}
	d := got.Deferrals[0]
	if d.Comment != "budget review first" || d.DeferredByEmail != "rssi@example.com" || !d.DeferredAt.Equal(at) {
		t.Fatalf("deferral read back wrong: %+v", d)
	}
	if got.Status != domain.ApprovalPending {
		t.Fatalf("a deferral must leave the request pending, got %s", got.Status)
	}

	// Another tenant's id pair never matches the row.
	other := *req
	other.TenantID = govB
	if err := repo.UpdateRequest(ctx, &other); err == nil {
		t.Fatal("updating with another tenant must fail as not found")
	}
}
