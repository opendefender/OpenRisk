// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

package orgdeletion

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

type memStore struct {
	active    *domain.OrgDeletionRequest
	created   *domain.OrgDeletionRequest
	completed []uuid.UUID
	canceled  bool
}

func (m *memStore) GetActive(context.Context, uuid.UUID) (*domain.OrgDeletionRequest, error) {
	return m.active, nil
}
func (m *memStore) Create(_ context.Context, r *domain.OrgDeletionRequest) error {
	m.created = r
	m.active = r
	return nil
}
func (m *memStore) Cancel(context.Context, uuid.UUID, uuid.UUID, time.Time) error {
	m.canceled = true
	m.active = nil
	return nil
}
func (m *memStore) ListDue(context.Context, time.Time) ([]domain.OrgDeletionRequest, error) {
	if m.active != nil {
		return []domain.OrgDeletionRequest{*m.active}, nil
	}
	return nil, nil
}
func (m *memStore) MarkCompleted(_ context.Context, id uuid.UUID, _ time.Time) error {
	m.completed = append(m.completed, id)
	return nil
}

type fakeOrg struct{ name string }

func (f fakeOrg) Name(context.Context, uuid.UUID) (string, error) { return f.name, nil }

type fakeExporter struct{ called bool }

func (f *fakeExporter) Export(context.Context, uuid.UUID) (string, error) {
	f.called = true
	return "/exports/x.json", nil
}

type fakeMFA struct{ err error }

func (f fakeMFA) VerifyRequired(context.Context, uuid.UUID, uuid.UUID, string) error { return f.err }

type fakePurger struct{ purged []uuid.UUID }

func (f *fakePurger) PurgeTenant(_ context.Context, t uuid.UUID) error {
	f.purged = append(f.purged, t)
	return nil
}

func deps() (*memStore, *fakeExporter, *fakePurger) {
	return &memStore{}, &fakeExporter{}, &fakePurger{}
}

func TestRequest_HappyPath_ExportsThenSchedules(t *testing.T) {
	store, exp, purger := deps()
	svc := NewService(store, fakeOrg{name: "Acme"}, exp, fakeMFA{}, purger)
	req, err := svc.Request(context.Background(), uuid.New(), uuid.New(), "Acme", "123456", "shutting down")
	if err != nil {
		t.Fatal(err)
	}
	if !exp.called {
		t.Fatal("export must run before scheduling")
	}
	if req.ExportPath == "" || req.Status != domain.DeletionPending {
		t.Fatalf("bad request %+v", req)
	}
	if req.ScheduledPurgeAt.Before(time.Now().Add(29 * 24 * time.Hour)) {
		t.Fatal("grace window should be ~30 days")
	}
}

func TestRequest_NameMismatch(t *testing.T) {
	store, exp, purger := deps()
	svc := NewService(store, fakeOrg{name: "Acme"}, exp, fakeMFA{}, purger)
	if _, err := svc.Request(context.Background(), uuid.New(), uuid.New(), "acme inc", "1", ""); err != ErrNameMismatch {
		t.Fatalf("expected name mismatch, got %v", err)
	}
	if exp.called {
		t.Fatal("must NOT export when name mismatches")
	}
}

func TestRequest_MFARejected(t *testing.T) {
	store, exp, purger := deps()
	svc := NewService(store, fakeOrg{name: "Acme"}, exp, fakeMFA{err: errors.New("bad code")}, purger)
	if _, err := svc.Request(context.Background(), uuid.New(), uuid.New(), "Acme", "000000", ""); err != ErrMFARequired {
		t.Fatalf("expected MFA required, got %v", err)
	}
	if exp.called {
		t.Fatal("must NOT export when MFA fails")
	}
}

func TestRequest_AlreadyPending(t *testing.T) {
	store, exp, purger := deps()
	store.active = &domain.OrgDeletionRequest{Status: domain.DeletionPending}
	svc := NewService(store, fakeOrg{name: "Acme"}, exp, fakeMFA{}, purger)
	if _, err := svc.Request(context.Background(), uuid.New(), uuid.New(), "Acme", "1", ""); err != ErrAlreadyPending {
		t.Fatalf("expected already pending, got %v", err)
	}
}

func TestRunDuePurges(t *testing.T) {
	store, exp, purger := deps()
	svc := NewService(store, fakeOrg{name: "Acme"}, exp, fakeMFA{}, purger)
	tenant := uuid.New()
	store.active = &domain.OrgDeletionRequest{ID: uuid.New(), OrganizationID: tenant, Status: domain.DeletionPending}
	n, err := svc.RunDuePurges(context.Background())
	if err != nil || n != 1 {
		t.Fatalf("expected 1 purge, got %d err=%v", n, err)
	}
	if len(purger.purged) != 1 || purger.purged[0] != tenant {
		t.Fatal("tenant should have been purged")
	}
	if len(store.completed) != 1 {
		t.Fatal("request should be marked completed")
	}
}

func TestCancel(t *testing.T) {
	store, exp, purger := deps()
	svc := NewService(store, fakeOrg{name: "Acme"}, exp, fakeMFA{}, purger)
	if err := svc.Cancel(context.Background(), uuid.New(), uuid.New()); err != ErrNoActiveRequest {
		t.Fatalf("cancel with nothing pending should error, got %v", err)
	}
	store.active = &domain.OrgDeletionRequest{Status: domain.DeletionPending}
	if err := svc.Cancel(context.Background(), uuid.New(), uuid.New()); err != nil {
		t.Fatal(err)
	}
	if !store.canceled {
		t.Fatal("should have canceled")
	}
}
