// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package activation

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
	ent "github.com/opendefender/openrisk/pkg/entitlements"
)

// fixedRisks answers the risk cap with fixed numbers (-1 = unlimited).
type fixedRisks struct{ limit, used int }

func (q fixedRisks) Capacity(context.Context, uuid.UUID, ent.LimitKey) (bool, int, int, ent.Plan, error) {
	return q.limit < 0 || q.used < q.limit, q.limit, q.used, ent.PlanFree, nil
}

// #783: the three starter risks are written at once, so they must ALL fit
// under Free's 30, or none is written.
func TestAdoptStarterRisksQuota_Success(t *testing.T) {
	for _, q := range []fixedRisks{{limit: 30, used: 27}, {limit: -1, used: 10000}} {
		repo, writer := newFakeRepo(), newFakeStarterWriter()
		tenant, user := uuid.New(), uuid.New()
		seedAnswers(repo, tenant, user, "banking", "CM")
		_, err := NewStarterRisksUseCase(repo, writer).WithQuota(q).Adopt(context.Background(), tenant, user, threeKeys(t), "fr")
		if err != nil {
			t.Fatalf("%+v: %v", q, err)
		}
	}
}

func TestAdoptStarterRisksQuota_Unauthorized(t *testing.T) {
	repo, writer := newFakeRepo(), newFakeStarterWriter()
	tenant, user := uuid.New(), uuid.New()
	seedAnswers(repo, tenant, user, "banking", "CM")
	_, err := NewStarterRisksUseCase(repo, writer).WithQuota(fixedRisks{limit: 30, used: 28}).
		Adopt(context.Background(), tenant, user, threeKeys(t), "fr")
	var lim *domain.LimitReachedError
	if !errors.As(err, &lim) || lim.Key != string(ent.LimitRisks) || lim.Limit != 30 || lim.Used != 28 {
		t.Fatalf("28 + 3 over 30: want LimitReachedError risks 30/28, got %v", err)
	}
	if writer.calls != 0 {
		t.Fatalf("a refused adoption writes nothing, wrote %d", writer.calls)
	}
}

func TestAdoptStarterRisksQuota_NotFound(t *testing.T) {
	// An unknown key is a validation error even at the cap: the caller learns
	// what is wrong with the request first.
	repo, writer := newFakeRepo(), newFakeStarterWriter()
	tenant, user := uuid.New(), uuid.New()
	_, err := NewStarterRisksUseCase(repo, writer).WithQuota(fixedRisks{limit: 30, used: 30}).
		Adopt(context.Background(), tenant, user, []string{"nope-1", "nope-2", "nope-3"}, "fr")
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("want ErrValidation, got %v", err)
	}
}
