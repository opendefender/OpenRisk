// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package membership

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
	ent "github.com/opendefender/openrisk/pkg/entitlements"
)

// fixedSeats answers the seat cap with fixed numbers.
type fixedSeats struct{ limit, used int }

func (q fixedSeats) Capacity(context.Context, uuid.UUID, ent.LimitKey) (bool, int, int, ent.Plan, error) {
	return q.used < q.limit, q.limit, q.used, ent.PlanFree, nil
}

func assertSeatRefused(t *testing.T, what string, err error) {
	t.Helper()
	var lim *domain.LimitReachedError
	if !errors.As(err, &lim) || lim.Key != string(ent.LimitUsers) || lim.Limit != 2 || lim.Used != 2 {
		t.Fatalf("%s at 2/2: want LimitReachedError users 2/2, got %v", what, err)
	}
}

// TestSeatQuota_Success: below the cap, a reactivation and the resend of an
// expired invitation both go through; withdrawing access never needs a seat.
func TestSeatQuota_Success(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	h.svc.WithSeatQuota(fixedSeats{limit: 2, used: 1})
	m := h.addMember(t, h.tenantA, "b@a.io", domain.RoleUser, domain.MembershipDeactivated)

	if _, err := h.svc.SetStatus(ctx, h.tenantA, SetStatusInput{
		ActorID: h.adminA.UserID, MemberID: m.ID, Status: domain.MembershipActive,
	}); err != nil {
		t.Fatalf("reactivation at 1/2: %v", err)
	}

	h.svc.WithSeatQuota(fixedSeats{limit: 2, used: 2})
	if _, err := h.svc.SetStatus(ctx, h.tenantA, SetStatusInput{
		ActorID: h.adminA.UserID, MemberID: m.ID, Status: domain.MembershipDeactivated,
	}); err != nil {
		t.Fatalf("deactivating at the cap must pass: %v", err)
	}

	h.svc.WithSeatQuota(fixedSeats{limit: 2, used: 1})
	res, _ := h.inviteAndCaptureToken(t, "late@x.io", domain.RoleUser)
	h.now = h.now.Add(domain.InvitationTTL + time.Hour)
	if _, err := h.svc.Resend(ctx, h.tenantA, ResendInput{ActorID: h.adminA.UserID, InvitationID: res.Invitation.ID}); err != nil {
		t.Fatalf("resending an expired invitation at 1/2: %v", err)
	}
}

// TestSeatQuota_Unauthorized: at the cap, the two moves that take a seat
// without creating anything are refused with the numbers (#783 criterion 1).
func TestSeatQuota_Unauthorized(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	m := h.addMember(t, h.tenantA, "b@a.io", domain.RoleUser, domain.MembershipDeactivated)
	res, _ := h.inviteAndCaptureToken(t, "late@x.io", domain.RoleUser)
	h.svc.WithSeatQuota(fixedSeats{limit: 2, used: 2})

	_, err := h.svc.SetStatus(ctx, h.tenantA, SetStatusInput{
		ActorID: h.adminA.UserID, MemberID: m.ID, Status: domain.MembershipActive,
	})
	assertSeatRefused(t, "reactivation", err)
	if got, _ := h.repo.GetMemberByID(ctx, h.tenantA, m.ID); got == nil || got.EffectiveStatus() != domain.MembershipDeactivated {
		t.Fatal("a refused reactivation must leave the member deactivated")
	}

	h.now = h.now.Add(domain.InvitationTTL + time.Hour)
	_, err = h.svc.Resend(ctx, h.tenantA, ResendInput{ActorID: h.adminA.UserID, InvitationID: res.Invitation.ID})
	assertSeatRefused(t, "resend of an expired invitation", err)
}

// TestSeatQuota_NotFound: an unknown member is NotFound, not a cap refusal.
func TestSeatQuota_NotFound(t *testing.T) {
	h := newHarness(t)
	h.svc.WithSeatQuota(fixedSeats{limit: 2, used: 2})
	_, err := h.svc.SetStatus(context.Background(), h.tenantA, SetStatusInput{
		ActorID: h.adminA.UserID, MemberID: uuid.New(), Status: domain.MembershipActive,
	})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}
