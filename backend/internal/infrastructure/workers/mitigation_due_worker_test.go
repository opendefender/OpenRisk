// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package workers

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
)

type dueStore struct {
	mu      sync.Mutex
	plans   []domain.Mitigation
	stamped []uuid.UUID
}

func (s *dueStore) ListDueForReminder(context.Context, time.Time) ([]domain.Mitigation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []domain.Mitigation
	for _, p := range s.plans {
		if p.ReminderD7SentAt == nil {
			out = append(out, p)
		}
	}
	return out, nil
}

func (s *dueStore) MarkReminderSent(_ context.Context, tenantID, id uuid.UUID, offset int, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.plans {
		if s.plans[i].ID == id && s.plans[i].TenantID == tenantID {
			s.plans[i].MarkReminderSent(offset, at)
			s.stamped = append(s.stamped, id)
		}
	}
	return nil
}

func duePlans(n int) []domain.Mitigation {
	due := time.Now().AddDate(0, 0, 5)
	out := make([]domain.Mitigation, n)
	for i := range out {
		assignee := uuid.New()
		out[i] = domain.Mitigation{
			ID: uuid.New(), TenantID: uuid.New(), Title: "Rotate keys",
			Status: domain.MitigationInProgress, DueDate: &due,
			Ownership: domain.Ownership{AssigneeID: &assignee},
		}
	}
	return out
}

// The sweep's own stamps deduplicate its reminders, so its notices skip the
// dispatcher's e-mail throttle (#997).
func TestMitigationDueSweep_Success(t *testing.T) {
	store := &dueStore{plans: duePlans(2)}
	var got []domain.Notice
	w := NewMitigationDueWorker(store, func(_ context.Context, n domain.Notice) { got = append(got, n) }, zerolog.Nop())

	w.Sweep(context.Background())
	require.Len(t, got, 2)
	for i, n := range got {
		require.True(t, n.Deduplicated)
		require.Equal(t, store.plans[i].TenantID, n.TenantID)
	}
}

// #997 criterion 6 — a relay that hangs spends the sweep's budget. The plans the
// sweep did not reach stay unstamped, and the next tick sends them.
func TestMitigationDueSweep_BudgetLeavesUnreachedRowsForNextTick(t *testing.T) {
	store := &dueStore{plans: duePlans(3)}
	var notified []uuid.UUID
	hang := true
	w := NewMitigationDueWorker(store, func(ctx context.Context, n domain.Notice) {
		notified = append(notified, *n.ResourceID)
		if hang {
			<-ctx.Done() // a blackholed relay, bounded by the sweep's deadline
		}
	}, zerolog.Nop()).WithBudget(50 * time.Millisecond)

	start := time.Now()
	w.Sweep(context.Background())
	require.Less(t, time.Since(start), time.Second, "the budget bounds the sweep")
	require.Equal(t, []uuid.UUID{store.plans[0].ID}, store.stamped, "only the plan it reached is stamped")
	require.Len(t, notified, 1)

	hang = false
	w.Sweep(context.Background())
	require.ElementsMatch(t, []uuid.UUID{store.plans[0].ID, store.plans[1].ID, store.plans[2].ID}, store.stamped)
	require.Len(t, notified, 3, "the next tick picks up the two it left")
}

// A sweep already cancelled stamps nothing.
func TestMitigationDueSweep_CancelledStampsNothing(t *testing.T) {
	store := &dueStore{plans: duePlans(2)}
	w := NewMitigationDueWorker(store, func(context.Context, domain.Notice) {
		t.Fatal("no notice from a cancelled sweep")
	}, zerolog.Nop())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	w.Sweep(ctx)
	require.Empty(t, store.stamped)
}
