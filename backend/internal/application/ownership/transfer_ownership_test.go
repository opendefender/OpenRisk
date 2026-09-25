// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

package ownership

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

// fakeOwnerStore keeps one owner slot per (tenant, id), so a lookup from the
// wrong tenant misses exactly like the GORM store's WHERE clause does.
type fakeOwnerStore struct {
	rows   map[uuid.UUID]map[string]*ownedRow
	writes int
}

type ownedRow struct {
	owner *uuid.UUID
	title string
}

func newFakeOwnerStore() *fakeOwnerStore {
	return &fakeOwnerStore{rows: map[uuid.UUID]map[string]*ownedRow{}}
}

func (f *fakeOwnerStore) put(tenant uuid.UUID, id, title string, owner *uuid.UUID) {
	if f.rows[tenant] == nil {
		f.rows[tenant] = map[string]*ownedRow{}
	}
	f.rows[tenant][id] = &ownedRow{owner: owner, title: title}
}

func (f *fakeOwnerStore) CurrentOwner(_ context.Context, tenantID uuid.UUID, id string) (*uuid.UUID, string, error) {
	row, ok := f.rows[tenantID][id]
	if !ok {
		return nil, "", domain.NewNotFoundError("risk", id)
	}
	return row.owner, row.title, nil
}

func (f *fakeOwnerStore) SetOwner(_ context.Context, tenantID uuid.UUID, id string, owner uuid.UUID) error {
	row, ok := f.rows[tenantID][id]
	if !ok {
		return domain.NewNotFoundError("risk", id)
	}
	o := owner
	row.owner = &o
	f.writes++
	return nil
}

type recordingAudit struct{ events []domain.AuditEvent }

func (r *recordingAudit) Record(_ context.Context, ev domain.AuditEvent) {
	r.events = append(r.events, ev)
}

type resourceNotification struct {
	userID       uuid.UUID
	resourceID   *uuid.UUID
	resourceType string
	notifType    domain.NotificationType
}

type resourceNotifier struct{ sent []resourceNotification }

func (f *resourceNotifier) NotifyInApp(userID, _ uuid.UUID, notifType domain.NotificationType, _, _ string, resourceID *uuid.UUID, resourceType string) error {
	f.sent = append(f.sent, resourceNotification{userID, resourceID, resourceType, notifType})
	return nil
}

type transferFixture struct {
	uc                          *TransferOwnershipUseCase
	store                       *fakeOwnerStore
	audit                       *recordingAudit
	notifier                    *resourceNotifier
	tenantA, tenantB            uuid.UUID
	actor, alice, bob, stranger uuid.UUID
	riskID                      string
}

// newTransferFixture: alice owns a risk of tenant A; bob is an active member of
// A; carol is a deactivated member of A; stranger belongs to tenant B only.
func newTransferFixture(t *testing.T) *transferFixture {
	t.Helper()
	f := &transferFixture{
		store:    newFakeOwnerStore(),
		audit:    &recordingAudit{},
		notifier: &resourceNotifier{},
		tenantA:  uuid.New(), tenantB: uuid.New(),
		actor: uuid.New(), alice: uuid.New(), bob: uuid.New(), stranger: uuid.New(),
		riskID: uuid.New().String(),
	}
	carol := uuid.New()
	members := &fakeMembers{rows: []domain.OrganizationMember{
		member(f.tenantA, f.actor, domain.RoleAdmin, "", true, "actor@a.io", "Actor"),
		member(f.tenantA, f.alice, domain.RoleUser, "", true, "alice@a.io", "Alice"),
		member(f.tenantA, f.bob, domain.RoleUser, "", true, "bob@a.io", "Bob"),
		member(f.tenantA, carol, domain.RoleUser, "", false, "carol@a.io", "Carol"),
		member(f.tenantB, f.stranger, domain.RoleUser, "", true, "stranger@b.io", "Stranger"),
	}}
	lookup := &fakeLookup{emails: map[uuid.UUID]string{f.alice: "alice@a.io", f.bob: "bob@a.io"}}
	svc := NewService().WithMembers(members).WithUsers(lookup).WithNotifier(f.notifier)

	alice := f.alice
	f.store.put(f.tenantA, f.riskID, "Ransomware on the ERP", &alice)
	f.uc = NewTransferOwnershipUseCase(svc, map[TransferEntity]OwnerStore{
		TransferRisk: f.store,
	}).WithAudit(f.audit)
	return f
}

func (f *transferFixture) input(newOwner uuid.UUID) TransferOwnershipInput {
	return TransferOwnershipInput{Entity: TransferRisk, ID: f.riskID, NewOwnerID: newOwner, Actor: f.actor, Locale: "en"}
}

func statusOf(err error) int {
	var appErr *domain.AppError
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return 0
}

func TestTransferOwnership_Success(t *testing.T) {
	f := newTransferFixture(t)

	res, err := f.uc.Execute(context.Background(), f.tenantA, f.input(f.bob))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if res.PreviousOwnerID == nil || *res.PreviousOwnerID != f.alice || res.OwnerID != f.bob {
		t.Fatalf("result: previous=%v owner=%v, want %v → %v", res.PreviousOwnerID, res.OwnerID, f.alice, f.bob)
	}
	if got := f.store.rows[f.tenantA][f.riskID].owner; got == nil || *got != f.bob {
		t.Fatalf("stored owner = %v, want %v", got, f.bob)
	}

	// Audit: one transfer entry, old owner → new owner, attributed to the actor.
	if len(f.audit.events) != 1 {
		t.Fatalf("audit events = %d, want 1", len(f.audit.events))
	}
	ev := f.audit.events[0]
	if ev.Action != domain.AuditActionTransfer || ev.EntityType != "risk" || ev.EntityID != f.riskID {
		t.Fatalf("audit event: action=%q entity=%s/%s", ev.Action, ev.EntityType, ev.EntityID)
	}
	if ev.TenantID != f.tenantA || ev.ActorID == nil || *ev.ActorID != f.actor {
		t.Fatalf("audit tenant/actor: %v / %v", ev.TenantID, ev.ActorID)
	}
	if ev.Before["owner_id"] != f.alice.String() || ev.After["owner_id"] != f.bob.String() {
		t.Fatalf("audit before/after: %v → %v", ev.Before, ev.After)
	}
	if !strings.Contains(ev.Summary, "alice@a.io") || !strings.Contains(ev.Summary, "bob@a.io") ||
		!strings.Contains(ev.Summary, "Ransomware on the ERP") {
		t.Fatalf("audit summary does not name both owners and the risk: %q", ev.Summary)
	}

	// Notification: the new owner, and only them, linked to the risk.
	if len(f.notifier.sent) != 1 {
		t.Fatalf("notifications = %d, want 1", len(f.notifier.sent))
	}
	n := f.notifier.sent[0]
	if n.userID != f.bob || n.resourceType != "risk" || n.resourceID == nil || n.resourceID.String() != f.riskID {
		t.Fatalf("notification: %+v", n)
	}
	if n.notifType != domain.NotificationTypeActionAssigned {
		t.Fatalf("notification type = %q", n.notifType)
	}
}

func TestTransferOwnership_Success_FromNoOwner(t *testing.T) {
	f := newTransferFixture(t)
	f.store.rows[f.tenantA][f.riskID].owner = nil

	res, err := f.uc.Execute(context.Background(), f.tenantA, f.input(f.bob))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.PreviousOwnerID != nil {
		t.Fatalf("previous owner = %v, want nil", res.PreviousOwnerID)
	}
	ev := f.audit.events[0]
	if ev.Before["owner_id"] != nil || !strings.Contains(ev.Summary, "from nobody") {
		t.Fatalf("an unowned entity must journal as 'from nobody': %q, before=%v", ev.Summary, ev.Before)
	}
}

func TestTransferOwnership_NotFound(t *testing.T) {
	f := newTransferFixture(t)
	in := f.input(f.bob)
	in.ID = uuid.New().String()

	_, err := f.uc.Execute(context.Background(), f.tenantA, in)
	if statusOf(err) != http.StatusNotFound {
		t.Fatalf("want 404, got %v", err)
	}
	if f.store.writes != 0 || len(f.audit.events) != 0 || len(f.notifier.sent) != 0 {
		t.Fatal("a missing entity must not write, journal or notify")
	}
}

func TestTransferOwnership_CrossTenant(t *testing.T) {
	f := newTransferFixture(t)

	// Tenant B names tenant A's risk by id: it does not exist for them.
	_, err := f.uc.Execute(context.Background(), f.tenantB, TransferOwnershipInput{
		Entity: TransferRisk, ID: f.riskID, NewOwnerID: f.stranger, Actor: f.stranger,
	})
	if statusOf(err) != http.StatusNotFound {
		t.Fatalf("want 404 for another tenant's risk, got %v", err)
	}
	if got := f.store.rows[f.tenantA][f.riskID].owner; got == nil || *got != f.alice {
		t.Fatalf("tenant A's risk was modified: owner=%v", got)
	}
}

func TestTransferOwnership_RejectsMemberOfAnotherTenant(t *testing.T) {
	f := newTransferFixture(t)

	_, err := f.uc.Execute(context.Background(), f.tenantA, f.input(f.stranger))
	if statusOf(err) != http.StatusBadRequest {
		t.Fatalf("want 400 for a user outside the tenant, got %v", err)
	}
	if f.store.writes != 0 || len(f.audit.events) != 0 || len(f.notifier.sent) != 0 {
		t.Fatal("a rejected transfer must not write, journal or notify")
	}
}

func TestTransferOwnership_RejectsDeactivatedMember(t *testing.T) {
	f := newTransferFixture(t)
	var carol uuid.UUID
	for _, m := range f.uc.members.members.(*fakeMembers).rows {
		if !m.IsActive {
			carol = m.UserID
		}
	}

	_, err := f.uc.Execute(context.Background(), f.tenantA, f.input(carol))
	if statusOf(err) != http.StatusBadRequest {
		t.Fatalf("want 400 for a deactivated member, got %v", err)
	}
	if f.store.writes != 0 {
		t.Fatal("a deactivated member must not become owner")
	}
}

func TestTransferOwnership_ConflictWhenAlreadyOwner(t *testing.T) {
	f := newTransferFixture(t)

	_, err := f.uc.Execute(context.Background(), f.tenantA, f.input(f.alice))
	if statusOf(err) != http.StatusConflict {
		t.Fatalf("want 409 when the new owner already owns it, got %v", err)
	}
	if f.store.writes != 0 || len(f.audit.events) != 0 {
		t.Fatal("a no-op transfer must not write or journal")
	}
}

func TestTransferOwnership_Unauthorized(t *testing.T) {
	f := newTransferFixture(t)

	for name, tc := range map[string]struct {
		tenant uuid.UUID
		actor  uuid.UUID
	}{
		"no tenant": {uuid.Nil, f.actor},
		"no actor":  {f.tenantA, uuid.Nil},
	} {
		t.Run(name, func(t *testing.T) {
			in := f.input(f.bob)
			in.Actor = tc.actor
			_, err := f.uc.Execute(context.Background(), tc.tenant, in)
			if statusOf(err) != http.StatusUnauthorized {
				t.Fatalf("want 401, got %v", err)
			}
		})
	}
	if f.store.writes != 0 {
		t.Fatal("an unauthenticated call must not write")
	}
}

func TestTransferOwnership_Validation(t *testing.T) {
	f := newTransferFixture(t)

	unknown := f.input(f.bob)
	unknown.Entity = "asset"
	if _, err := f.uc.Execute(context.Background(), f.tenantA, unknown); statusOf(err) != http.StatusBadRequest {
		t.Fatalf("unknown entity: want 400, got %v", err)
	}

	noOwner := f.input(uuid.Nil)
	if _, err := f.uc.Execute(context.Background(), f.tenantA, noOwner); statusOf(err) != http.StatusBadRequest {
		t.Fatalf("missing new owner: want 400, got %v", err)
	}
	if f.store.writes != 0 {
		t.Fatal("an invalid request must not write")
	}
}

func TestTransferOwnership_SelfTransferIsJournalledButNotNotified(t *testing.T) {
	f := newTransferFixture(t)
	in := f.input(f.bob)
	in.Actor = f.bob // bob takes ownership himself

	if _, err := f.uc.Execute(context.Background(), f.tenantA, in); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(f.audit.events) != 1 {
		t.Fatalf("audit events = %d, want 1", len(f.audit.events))
	}
	if len(f.notifier.sent) != 0 {
		t.Fatalf("nobody is notified about taking ownership themselves, got %d", len(f.notifier.sent))
	}
}

func TestTransferOwnership_IntegerIDNotifiesWithoutResourceLink(t *testing.T) {
	f := newTransferFixture(t)
	incidents := newFakeOwnerStore()
	alice := f.alice
	incidents.put(f.tenantA, "42", "Phishing wave", &alice)
	f.uc.stores[TransferIncident] = incidents

	_, err := f.uc.Execute(context.Background(), f.tenantA, TransferOwnershipInput{
		Entity: TransferIncident, ID: "42", NewOwnerID: f.bob, Actor: f.actor,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(f.notifier.sent) != 1 || f.notifier.sent[0].resourceID != nil || f.notifier.sent[0].resourceType != "incident" {
		t.Fatalf("incident notification: %+v", f.notifier.sent)
	}
	if f.audit.events[0].EntityID != "42" || f.audit.events[0].EntityType != "incident" {
		t.Fatalf("incident audit entity: %s/%s", f.audit.events[0].EntityType, f.audit.events[0].EntityID)
	}
}
