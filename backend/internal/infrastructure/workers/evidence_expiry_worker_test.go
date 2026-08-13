// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package workers

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/opendefender/openrisk/internal/domain"
)

type fakeExpiryStore struct {
	rows       []domain.Evidence
	stamped    []uuid.UUID
	stampFails bool
}

func (s *fakeExpiryStore) ListExpiring(context.Context, time.Time, time.Duration, int) ([]domain.Evidence, error) {
	return s.rows, nil
}

func (s *fakeExpiryStore) MarkReminded(_ context.Context, id uuid.UUID, _ time.Time) error {
	if s.stampFails {
		return errors.New("db down")
	}
	s.stamped = append(s.stamped, id)
	return nil
}

type sentReminder struct {
	tenant, user, evidence uuid.UUID
	subject, message       string
}

func silentLogger() zerolog.Logger { return zerolog.New(io.Discard) }

func TestEvidenceExpiry_NotifiesTheAssigneeFirst(t *testing.T) {
	assignee, owner, collector := uuid.New(), uuid.New(), uuid.New()
	tenant := uuid.New()
	soon := time.Now().Add(5 * 24 * time.Hour)

	ev := domain.Evidence{
		ID: uuid.New(), TenantID: tenant, Title: "ISO certificate",
		ValidUntil: &soon, CollectedBy: &collector,
	}
	ev.Ownership.OwnerID = &owner
	ev.Ownership.AssigneeID = &assignee

	store := &fakeExpiryStore{rows: []domain.Evidence{ev}}
	var sent []sentReminder
	w := NewEvidenceExpiryWorker(store, func(_ context.Context, tid, uid, eid uuid.UUID, sub, msg string) {
		sent = append(sent, sentReminder{tid, uid, eid, sub, msg})
	}, silentLogger())

	w.Sweep(context.Background())

	if len(sent) != 1 {
		t.Fatalf("expected one reminder, got %d", len(sent))
	}
	// The person who must refresh the proof, not the person who filed it.
	if sent[0].user != assignee {
		t.Fatalf("reminder should go to the assignee")
	}
	if sent[0].tenant != tenant {
		t.Fatalf("reminder must be addressed with the artifact's own tenant")
	}
	if len(store.stamped) != 1 {
		t.Fatalf("the reminder must be stamped so it does not fire again")
	}
}

func TestEvidenceExpiry_FallsBackThroughOwnerToCollector(t *testing.T) {
	soon := time.Now().Add(3 * 24 * time.Hour)
	owner, collector := uuid.New(), uuid.New()

	withOwner := domain.Evidence{ID: uuid.New(), TenantID: uuid.New(), Title: "a", ValidUntil: &soon, CollectedBy: &collector}
	withOwner.Ownership.OwnerID = &owner
	onlyCollector := domain.Evidence{ID: uuid.New(), TenantID: uuid.New(), Title: "b", ValidUntil: &soon, CollectedBy: &collector}

	store := &fakeExpiryStore{rows: []domain.Evidence{withOwner, onlyCollector}}
	var sent []sentReminder
	w := NewEvidenceExpiryWorker(store, func(_ context.Context, tid, uid, eid uuid.UUID, sub, msg string) {
		sent = append(sent, sentReminder{tid, uid, eid, sub, msg})
	}, silentLogger())

	w.Sweep(context.Background())

	if len(sent) != 2 {
		t.Fatalf("expected two reminders, got %d", len(sent))
	}
	if sent[0].user != owner {
		t.Fatal("with no assignee, the owner answers for the proof")
	}
	if sent[1].user != collector {
		t.Fatal("with neither, the person who collected it is the last resort")
	}
}

// Nobody attached to the proof is exactly the proof that lapses. It must not
// crash the sweep, and it must not silently consume the other rows.
func TestEvidenceExpiry_UnownedArtifactIsSkippedNotFatal(t *testing.T) {
	soon := time.Now().Add(2 * 24 * time.Hour)
	owner := uuid.New()

	orphan := domain.Evidence{ID: uuid.New(), TenantID: uuid.New(), Title: "nobody's", ValidUntil: &soon}
	owned := domain.Evidence{ID: uuid.New(), TenantID: uuid.New(), Title: "someone's", ValidUntil: &soon}
	owned.Ownership.OwnerID = &owner

	store := &fakeExpiryStore{rows: []domain.Evidence{orphan, owned}}
	var sent []sentReminder
	w := NewEvidenceExpiryWorker(store, func(_ context.Context, tid, uid, eid uuid.UUID, sub, msg string) {
		sent = append(sent, sentReminder{tid, uid, eid, sub, msg})
	}, silentLogger())

	w.Sweep(context.Background())

	if len(sent) != 1 || sent[0].user != owner {
		t.Fatalf("the owned artifact must still be reminded, got %d reminders", len(sent))
	}
}

// Stamp before send: if the stamp fails, the notification must NOT go out, or the
// owner gets the same nudge every hour until they renew.
func TestEvidenceExpiry_StampFailureSuppressesTheSend(t *testing.T) {
	soon := time.Now().Add(4 * 24 * time.Hour)
	owner := uuid.New()
	ev := domain.Evidence{ID: uuid.New(), TenantID: uuid.New(), Title: "x", ValidUntil: &soon}
	ev.Ownership.OwnerID = &owner

	store := &fakeExpiryStore{rows: []domain.Evidence{ev}, stampFails: true}
	sent := 0
	w := NewEvidenceExpiryWorker(store, func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, string, string) {
		sent++
	}, silentLogger())

	w.Sweep(context.Background())

	if sent != 0 {
		t.Fatal("a reminder that cannot be stamped must not be sent — it would repeat every tick")
	}
}

// The copy has to change register once the date has passed: "renew this" and
// "this stopped covering your controls" are different messages.
func TestExpiryReminderCopy_ChangesToneOnceExpired(t *testing.T) {
	now := time.Now()

	future := now.Add(10 * 24 * time.Hour)
	subject, message := expiryReminderCopy(&domain.Evidence{Title: "Pentest", ValidUntil: &future}, now)
	if subject == "" || message == "" {
		t.Fatal("a reminder needs a subject and a message")
	}
	if !contains(message, "10") {
		t.Fatalf("the upcoming reminder should say how long is left: %q", message)
	}

	past := now.Add(-3 * 24 * time.Hour)
	subjectPast, messagePast := expiryReminderCopy(&domain.Evidence{Title: "Pentest", ValidUntil: &past}, now)
	if subjectPast == subject {
		t.Fatal("an expired artifact must not read like one that is merely due")
	}
	if !contains(messagePast, "plus") {
		t.Fatalf("the expired message should say the controls are no longer covered: %q", messagePast)
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (func() bool {
		for i := 0; i+len(needle) <= len(haystack); i++ {
			if haystack[i:i+len(needle)] == needle {
				return true
			}
		}
		return false
	})()
}
