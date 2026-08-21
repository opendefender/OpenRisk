// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package membership

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// tokenFromURL pulls the one-time token back out of the accept link, which is
// the only place a test can legitimately get it — exactly like the invitee.
func tokenFromURL(t *testing.T, raw string) string {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse accept url: %v", err)
	}
	tok := u.Query().Get("token")
	if tok == "" {
		t.Fatalf("no token in %q", raw)
	}
	return tok
}

// inviteAndCaptureToken invites somebody and returns the token, taking it from
// the mail the service sent rather than from any API response — the response
// only carries the link when delivery failed.
func (h *harness) inviteAndCaptureToken(t *testing.T, email string, role domain.MemberRole) (*InviteResult, string) {
	t.Helper()
	res, err := h.svc.Invite(context.Background(), h.tenantA, InviteInput{
		ActorID: h.adminA.UserID, Email: email, Role: role,
	})
	if err != nil {
		t.Fatalf("invite %s: %v", email, err)
	}
	h.mailer.mu.Lock()
	defer h.mailer.mu.Unlock()
	if len(h.mailer.sent) == 0 {
		t.Fatal("no invitation mail was sent")
	}
	return res, tokenFromURL(t, h.mailer.sent[len(h.mailer.sent)-1].AcceptURL)
}

func TestInvite_CreatesAuditsAndSendsWithoutLeakingTheToken(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	res, err := h.svc.Invite(ctx, h.tenantA, InviteInput{
		ActorID: h.adminA.UserID, Email: " New.Person@Example.COM ", Role: domain.RoleUser,
		BusinessRole: domain.BusinessRoleKey("auditor"),
	})
	if err != nil {
		t.Fatalf("invite: %v", err)
	}
	if res.Invitation.Email != "new.person@example.com" {
		t.Errorf("the address must be normalised, got %q", res.Invitation.Email)
	}
	if res.Delivery != DeliverySent {
		t.Errorf("a working transport must report sent, got %q", res.Delivery)
	}
	// When the email went out, the administrator has no reason to hold a
	// credential that authenticates as somebody else.
	if res.AcceptURL != "" {
		t.Error("the accept link must not be returned when the email was delivered")
	}

	ev := h.audit.find("invitation", "create")
	if ev == nil {
		t.Fatal("creating an invitation must be audited")
	}
	// The audit trail is read widely. It records who was invited at what role —
	// never the credential.
	for k, v := range ev.After {
		if s, ok := v.(string); ok && (strings.Contains(strings.ToLower(k), "token") || len(s) == 43) {
			t.Errorf("audit field %q looks like token material: %v", k, v)
		}
	}

	page, err := h.svc.ListInvitations(ctx, h.tenantA, domain.InvitationQuery{})
	if err != nil || page.Total != 1 {
		t.Fatalf("the invitation must be listed: %v %d", err, page.Total)
	}
	if page.Items[0].Status != domain.InvitationPending {
		t.Errorf("a fresh invitation must be pending, got %s", page.Items[0].Status)
	}
	if page.Items[0].InvitedByEmail != "admin@a.io" {
		t.Errorf("the inviter must be resolved for display, got %q", page.Items[0].InvitedByEmail)
	}
}

func TestInvite_Validation(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	for _, bad := range []string{"", "not-an-email", "no@domain", strings.Repeat("a", 320) + "@x.io"} {
		if _, err := h.svc.Invite(ctx, h.tenantA, InviteInput{ActorID: h.adminA.UserID, Email: bad}); statusOf(err) != http.StatusBadRequest {
			t.Errorf("email %q must be refused, got %v", bad, err)
		}
	}
	if _, err := h.svc.Invite(ctx, h.tenantA, InviteInput{
		ActorID: h.adminA.UserID, Email: "x@y.io", Role: domain.RoleRoot,
	}); statusOf(err) != http.StatusBadRequest {
		t.Errorf("root must not be invitable, got %v", err)
	}
	if _, err := h.svc.Invite(ctx, h.tenantA, InviteInput{
		ActorID: h.adminA.UserID, Email: "x@y.io", BusinessRole: "wizard",
	}); statusOf(err) != http.StatusBadRequest {
		t.Errorf("an unknown business role must be refused, got %v", err)
	}
	// An existing member of THIS organization cannot be invited again.
	if _, err := h.svc.Invite(ctx, h.tenantA, InviteInput{
		ActorID: h.adminA.UserID, Email: "admin@a.io",
	}); statusOf(err) != http.StatusConflict {
		t.Errorf("inviting an existing member must conflict, got %v", err)
	}
	// A live invitation must not be duplicated — that would leave two working
	// tokens for one seat. The admin wants Resend.
	h.inviteAndCaptureToken(t, "dup@x.io", domain.RoleUser)
	if _, err := h.svc.Invite(ctx, h.tenantA, InviteInput{ActorID: h.adminA.UserID, Email: "dup@x.io"}); statusOf(err) != http.StatusConflict {
		t.Errorf("a second live invitation must conflict, got %v", err)
	}
}

// Being a member of another organization is normal and must never block an
// invitation — otherwise a consultant could only ever work for one customer.
func TestInvite_MemberOfAnotherTenantIsNotABlocker(t *testing.T) {
	h := newHarness(t)
	h.addMember(t, h.tenantB, "consultant@x.io", domain.RoleUser, domain.MembershipActive)
	if _, err := h.svc.Invite(context.Background(), h.tenantA, InviteInput{
		ActorID: h.adminA.UserID, Email: "consultant@x.io",
	}); err != nil {
		t.Fatalf("a member of another tenant must be invitable here: %v", err)
	}
}

// The invitation must survive a mail failure, and the failure must be reported
// as a failure — never as a green "sent".
func TestInvite_DeliveryIsReportedHonestly(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	h.mailer.err = errors.New("smtp: connection refused")

	res, err := h.svc.Invite(ctx, h.tenantA, InviteInput{ActorID: h.adminA.UserID, Email: "a@x.io"})
	if err != nil {
		t.Fatalf("a mail failure must not fail the invitation: %v", err)
	}
	if res.Delivery != DeliveryFailed {
		t.Errorf("delivery must be reported as failed, got %q", res.Delivery)
	}
	if res.AcceptURL == "" {
		t.Error("when the mail did not go out the admin needs the link to deliver by hand")
	}
	// And it is a real, revocable invitation, not a phantom.
	page, _ := h.svc.ListInvitations(ctx, h.tenantA, domain.InvitationQuery{})
	if page.Total != 1 {
		t.Fatalf("the invitation must be persisted despite the mail failure, got %d", page.Total)
	}

	// No transport at all is a different, equally honest answer.
	h.svc.mailer = nil
	res, err = h.svc.Invite(ctx, h.tenantA, InviteInput{ActorID: h.adminA.UserID, Email: "b@x.io"})
	if err != nil || res.Delivery != DeliveryUnavailable || res.AcceptURL == "" {
		t.Fatalf("no transport must report unavailable with a link: %v %+v", err, res)
	}
}

func TestResend_RotatesTokenAndRespectsCooldown(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	res, first := h.inviteAndCaptureToken(t, "resend@x.io", domain.RoleUser)

	if _, err := h.svc.Resend(ctx, h.tenantA, ResendInput{ActorID: h.adminA.UserID, InvitationID: res.Invitation.ID}); statusOf(err) != http.StatusTooManyRequests {
		t.Fatalf("resending inside the cooldown must be 429, got %v", err)
	}

	h.now = h.now.Add(domain.InvitationResendCooldown + time.Second)
	out, err := h.svc.Resend(ctx, h.tenantA, ResendInput{ActorID: h.adminA.UserID, InvitationID: res.Invitation.ID})
	if err != nil {
		t.Fatalf("resend: %v", err)
	}
	if out.Invitation.SendCount != 2 {
		t.Errorf("the send must be counted, got %d", out.Invitation.SendCount)
	}
	h.mailer.mu.Lock()
	second := tokenFromURL(t, h.mailer.sent[len(h.mailer.sent)-1].AcceptURL)
	h.mailer.mu.Unlock()
	if second == first {
		t.Fatal("a resend must mint a new token")
	}
	// Exactly one token opens the door at a time — that is what makes "resend"
	// genuinely invalidate a link forwarded to the wrong person.
	if _, err := h.svc.PreviewInvitation(ctx, first); statusOf(err) != http.StatusNotFound {
		t.Errorf("the superseded token must stop working, got %v", err)
	}
	if _, err := h.svc.PreviewInvitation(ctx, second); err != nil {
		t.Errorf("the new token must work: %v", err)
	}
	if ev := h.audit.find("invitation", "update"); ev == nil {
		t.Error("a resend must be audited")
	}
}

func TestResend_SpamIsBounded(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	res, _ := h.inviteAndCaptureToken(t, "spam@x.io", domain.RoleUser)

	// Hammer it the way a script would: always past the cooldown, forever.
	sends := 1
	for i := 0; i < 50; i++ {
		h.now = h.now.Add(domain.InvitationResendCooldown + time.Second)
		if _, err := h.svc.Resend(ctx, h.tenantA, ResendInput{ActorID: h.adminA.UserID, InvitationID: res.Invitation.ID}); err != nil {
			if statusOf(err) != http.StatusBadRequest {
				t.Fatalf("the cap must refuse with a validation error, got %v", err)
			}
			break
		}
		sends++
	}
	if sends != domain.InvitationMaxResends {
		t.Fatalf("sends must stop at the cap: got %d, cap %d", sends, domain.InvitationMaxResends)
	}
}

func TestRevokeInvitation_KillsTheLink(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	res, token := h.inviteAndCaptureToken(t, "revoke@x.io", domain.RoleUser)

	view, err := h.svc.RevokeInvitation(ctx, h.tenantA, h.adminA.UserID, res.Invitation.ID)
	if err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if view.Status != domain.InvitationRevoked {
		t.Fatalf("status: %s", view.Status)
	}
	// Revocation rotates the hash, so the mailed link is dead on lookup rather
	// than merely on a status check.
	if _, err := h.svc.PreviewInvitation(ctx, token); statusOf(err) != http.StatusNotFound {
		t.Errorf("the revoked link must no longer resolve, got %v", err)
	}
	if _, err := h.svc.AcceptInvitation(ctx, AcceptInput{Token: token, FullName: "X", Password: "correct-horse-battery"}); statusOf(err) != http.StatusNotFound {
		t.Errorf("a revoked invitation must not be acceptable, got %v", err)
	}
	if _, err := h.svc.RevokeInvitation(ctx, h.tenantA, h.adminA.UserID, res.Invitation.ID); statusOf(err) != http.StatusGone {
		t.Errorf("revoking twice must be 410, got %v", err)
	}
	if _, err := h.svc.Resend(ctx, h.tenantA, ResendInput{ActorID: h.adminA.UserID, InvitationID: res.Invitation.ID}); statusOf(err) != http.StatusGone {
		t.Errorf("resending a revoked invitation must be 410, got %v", err)
	}
	if ev := h.audit.find("invitation", "revoke"); ev == nil {
		t.Error("a revocation must be audited")
	}
}

// Tenant A must not be able to read, revoke or resend tenant B's invitation,
// even holding its real id.
func TestInvitations_CrossTenantIsolation(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	adminB := h.addMember(t, h.tenantB, "admin@b.io", domain.RoleAdmin, domain.MembershipActive)
	resB, err := h.svc.Invite(ctx, h.tenantB, InviteInput{ActorID: adminB.UserID, Email: "target@b.io"})
	if err != nil {
		t.Fatalf("invite in B: %v", err)
	}

	page, _ := h.svc.ListInvitations(ctx, h.tenantA, domain.InvitationQuery{})
	if page.Total != 0 {
		t.Fatalf("tenant A must not see tenant B's invitations, got %d", page.Total)
	}
	if _, err := h.svc.RevokeInvitation(ctx, h.tenantA, h.adminA.UserID, resB.Invitation.ID); statusOf(err) != http.StatusNotFound {
		t.Errorf("cross-tenant revoke must be 404, got %v", err)
	}
	if _, err := h.svc.Resend(ctx, h.tenantA, ResendInput{ActorID: h.adminA.UserID, InvitationID: resB.Invitation.ID}); statusOf(err) != http.StatusNotFound {
		t.Errorf("cross-tenant resend must be 404, got %v", err)
	}
	// And tenant B's invitation is untouched.
	pageB, _ := h.svc.ListInvitations(ctx, h.tenantB, domain.InvitationQuery{})
	if pageB.Total != 1 || pageB.Items[0].Status != domain.InvitationPending {
		t.Fatalf("tenant B's invitation was disturbed: %+v", pageB.Items)
	}
}

// ---------------------------------------------------------------------------
// Acceptance
// ---------------------------------------------------------------------------

// An invitee joins an organization that already exists. The signup wizard's
// first screen asks the user to set one up, and the route guard holds the whole
// application shut until onboarding is complete — so without this a newly
// joined colleague's first screen is a form about a tenant they did not create
// and are not allowed to edit.
func TestAccept_TakesTheNewMemberPastTheSignupWizard(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	done := &recordingOnboarding{}
	h.svc.WithOnboarding(done)
	_, token := h.inviteAndCaptureToken(t, "wizard@x.io", domain.RoleUser)

	if _, err := h.svc.AcceptInvitation(ctx, AcceptInput{
		Token: token, FullName: "Wizard Skipper", Password: "correct-horse-battery-staple",
	}); err != nil {
		t.Fatalf("accept: %v", err)
	}
	if len(done.calls) != 1 || done.calls[0].tenant != h.tenantA {
		t.Fatalf("acceptance must mark onboarding complete for the tenant joined: %+v", done.calls)
	}

	// And a failure there must never undo a membership that was just created.
	done.err = errors.New("onboarding store down")
	_, token2 := h.inviteAndCaptureToken(t, "wizard2@x.io", domain.RoleUser)
	if _, err := h.svc.AcceptInvitation(ctx, AcceptInput{
		Token: token2, FullName: "Second Skipper", Password: "correct-horse-battery-staple",
	}); err != nil {
		t.Fatalf("an onboarding failure must not fail the acceptance: %v", err)
	}
	page, _ := h.svc.ListMembers(ctx, h.tenantA, domain.MemberQuery{Search: "wizard2@x.io"})
	if page.Total != 1 {
		t.Fatalf("the membership must exist despite the onboarding failure (got %d)", page.Total)
	}
}

type recordingOnboarding struct {
	calls []struct{ tenant, user uuid.UUID }
	err   error
}

func (r *recordingOnboarding) MarkComplete(_ context.Context, tenantID, userID uuid.UUID) error {
	r.calls = append(r.calls, struct{ tenant, user uuid.UUID }{tenantID, userID})
	return r.err
}

func TestAccept_NewAccountJoinsAtTheInvitedRole(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	_, token := h.inviteAndCaptureToken(t, "joiner@x.io", domain.RoleUser)

	preview, err := h.svc.PreviewInvitation(ctx, token)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if preview.OrganizationName != "Tenant A" || !preview.RequiresSignup || preview.Role != domain.RoleUser {
		t.Fatalf("preview: %+v", preview)
	}

	res, err := h.svc.AcceptInvitation(ctx, AcceptInput{
		Token: token, FullName: "New Joiner", Password: "correct-horse-battery-staple",
	})
	if err != nil {
		t.Fatalf("accept: %v", err)
	}
	if !res.CreatedAccount || res.OrganizationID != h.tenantA || res.Role != domain.RoleUser {
		t.Fatalf("accept result: %+v", res)
	}

	page, _ := h.svc.ListMembers(ctx, h.tenantA, domain.MemberQuery{Search: "joiner"})
	if page.Total != 1 || page.Items[0].Status != domain.MembershipActive {
		t.Fatalf("the invitee must now be an active member: %+v", page.Items)
	}
	// The acceptance is attributed to the person who joined, not to the inviter.
	ev := h.audit.find("organization_member", "create")
	if ev == nil || ev.ActorID == nil || *ev.ActorID == h.adminA.UserID {
		t.Errorf("acceptance must be audited as the invitee's action: %+v", ev)
	}

	// Replay: the token is consumed.
	if _, err := h.svc.AcceptInvitation(ctx, AcceptInput{Token: token, FullName: "X", Password: "correct-horse-battery"}); statusOf(err) != http.StatusGone {
		t.Errorf("a used token must be 410, got %v", err)
	}
}

func TestAccept_RejectsBadTokensUniformly(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	// An unknown token, a malformed token and an empty token are
	// indistinguishable, so the endpoint cannot be used to test guesses.
	var messages []string
	for _, bad := range []string{"", "garbage", strings.Repeat("A", 43)} {
		_, err := h.svc.AcceptInvitation(ctx, AcceptInput{Token: bad, FullName: "X", Password: "correct-horse-battery"})
		if statusOf(err) != http.StatusNotFound {
			t.Fatalf("token %q must be 404, got %v", bad, err)
		}
		messages = append(messages, domain.MessageFromError(err))
	}
	for _, m := range messages[1:] {
		if m != messages[0] {
			t.Fatalf("bad tokens must answer identically, got %q vs %q", m, messages[0])
		}
	}

	// Expiry is a 410, told to somebody who legitimately held the link.
	_, token := h.inviteAndCaptureToken(t, "late@x.io", domain.RoleUser)
	h.now = h.now.Add(domain.InvitationTTL + time.Hour)
	if _, err := h.svc.AcceptInvitation(ctx, AcceptInput{Token: token, FullName: "X", Password: "correct-horse-battery"}); statusOf(err) != http.StatusGone {
		t.Errorf("an expired token must be 410, got %v", err)
	}
}

func TestAccept_RefusesEmailMismatchAndUnauthenticatedExistingAccount(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	_, token := h.inviteAndCaptureToken(t, "invited@x.io", domain.RoleUser)

	// Somebody else's session presenting the link must not be able to claim it:
	// otherwise a forwarded invitation becomes free membership for whoever
	// receives it.
	other := h.users.add(&domain.User{ID: uuid.New(), Email: "someone.else@x.io", Username: "else"})
	if _, err := h.svc.AcceptInvitation(ctx, AcceptInput{Token: token, UserID: other.ID}); statusOf(err) != http.StatusForbidden {
		t.Errorf("an email mismatch must be forbidden, got %v", err)
	}

	// The invited address already has an account but nobody is signed in:
	// attaching a seat to it without authentication would be an account takeover
	// primitive.
	h.users.add(&domain.User{ID: uuid.New(), Email: "invited@x.io", Username: "invited"})
	if _, err := h.svc.AcceptInvitation(ctx, AcceptInput{Token: token, FullName: "X", Password: "correct-horse-battery"}); statusOf(err) != http.StatusUnauthorized {
		t.Errorf("an existing account must require sign-in, got %v", err)
	}
}

func TestAccept_SignedInExistingUserJoins(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	existing := h.users.add(&domain.User{ID: uuid.New(), Email: "veteran@x.io", Username: "veteran", FullName: "Veteran"})
	_, token := h.inviteAndCaptureToken(t, "veteran@x.io", domain.RoleAdmin)

	res, err := h.svc.AcceptInvitation(ctx, AcceptInput{Token: token, UserID: existing.ID})
	if err != nil {
		t.Fatalf("accept: %v", err)
	}
	if res.CreatedAccount || res.UserID != existing.ID || res.Role != domain.RoleAdmin {
		t.Fatalf("accept result: %+v", res)
	}
}

func TestAccept_WeakPasswordAndMissingNameRefused(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	_, token := h.inviteAndCaptureToken(t, "weak@x.io", domain.RoleUser)

	if _, err := h.svc.AcceptInvitation(ctx, AcceptInput{Token: token, FullName: "X", Password: "short"}); statusOf(err) != http.StatusBadRequest {
		t.Errorf("a short password must be refused, got %v", err)
	}
	if _, err := h.svc.AcceptInvitation(ctx, AcceptInput{Token: token, Password: "correct-horse-battery-staple"}); statusOf(err) != http.StatusBadRequest {
		t.Errorf("a missing name must be refused, got %v", err)
	}
	// Neither failure consumed the invitation.
	if _, err := h.svc.PreviewInvitation(ctx, token); err != nil {
		t.Errorf("a refused acceptance must leave the invitation usable: %v", err)
	}
}

// A revoked membership is terminal. An old invitation link must not be a way
// back in — re-admission is an administrator's decision.
func TestAccept_RevokedMembershipIsNotReinstatedByALink(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	m := h.addMember(t, h.tenantA, "gone@x.io", domain.RoleUser, domain.MembershipRevoked)
	_, token := h.inviteAndCaptureToken(t, "gone@x.io", domain.RoleUser)

	if _, err := h.svc.AcceptInvitation(ctx, AcceptInput{Token: token, UserID: m.UserID}); statusOf(err) != http.StatusForbidden {
		t.Fatalf("a revoked membership must not be revived by a link, got %v", err)
	}
}

// Inviting somebody who is already a member — suspended or not — is refused
// with an answer that names the action that WOULD work, rather than quietly
// opening a second route to the same seat.
func TestInvite_ExistingMemberIsRefusedWithTheRightVerb(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	h.addMember(t, h.tenantA, "suspended@x.io", domain.RoleUser, domain.MembershipDeactivated)

	_, err := h.svc.Invite(ctx, h.tenantA, InviteInput{ActorID: h.adminA.UserID, Email: "suspended@x.io"})
	if statusOf(err) != http.StatusConflict {
		t.Fatalf("inviting a deactivated member must conflict, got %v", err)
	}
	if !strings.Contains(domain.MessageFromError(err), "reactivate") {
		t.Errorf("the answer must point at Reactivate, got %q", domain.MessageFromError(err))
	}

	// A REVOKED membership is invitable again — re-admission is a deliberate act.
	h.addMember(t, h.tenantA, "returning@x.io", domain.RoleUser, domain.MembershipRevoked)
	if _, err := h.svc.Invite(ctx, h.tenantA, InviteInput{ActorID: h.adminA.UserID, Email: "returning@x.io"}); err != nil {
		t.Fatalf("a revoked membership must be re-invitable: %v", err)
	}
}

// A member deactivated AFTER their invitation went out still holds a live link.
// Accepting it is a legitimate re-admission: the administrator issued that
// invitation on purpose, and it is newer than the suspension is not — so the
// membership comes back at the role the invitation names.
func TestAccept_MembershipDeactivatedAfterInviteIsReinstated(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	_, token := h.inviteAndCaptureToken(t, "back@x.io", domain.RoleAdmin)
	// The invitee already had an account and a membership that was suspended
	// while the invitation was in flight.
	u := h.users.add(&domain.User{ID: uuid.New(), Email: "back@x.io", Username: "back"})
	m := &domain.OrganizationMember{
		ID: uuid.New(), OrganizationID: h.tenantA, UserID: u.ID, Role: domain.RoleUser,
		Status: domain.MembershipDeactivated, IsActive: false, User: u,
	}
	h.repo.mu.Lock()
	h.repo.members = append(h.repo.members, m)
	h.repo.mu.Unlock()

	res, err := h.svc.AcceptInvitation(ctx, AcceptInput{Token: token, UserID: u.ID})
	if err != nil {
		t.Fatalf("reinstate: %v", err)
	}
	if res.Role != domain.RoleAdmin {
		t.Errorf("the invitation's role must be applied, got %s", res.Role)
	}
	view, err := h.svc.GetMember(ctx, h.tenantA, m.ID)
	if err != nil || !view.IsActive || view.OrgRole != domain.RoleAdmin {
		t.Fatalf("the membership must be active again at the invited role: %v %+v", err, view)
	}
	// And exactly one membership exists — never a duplicate row.
	page, _ := h.svc.ListMembers(ctx, h.tenantA, domain.MemberQuery{Search: "back@"})
	if page.Total != 1 {
		t.Fatalf("exactly one membership must exist, got %d", page.Total)
	}
}

// Two acceptances of the same token must yield one membership. The repository
// enforces this transactionally; the fake mirrors that contract so the use case
// is exercised against it.
func TestAccept_ConcurrentReplayYieldsOneMembership(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	existing := h.users.add(&domain.User{ID: uuid.New(), Email: "race@x.io", Username: "race"})
	_, token := h.inviteAndCaptureToken(t, "race@x.io", domain.RoleUser)

	first, err1 := h.svc.AcceptInvitation(ctx, AcceptInput{Token: token, UserID: existing.ID})
	_, err2 := h.svc.AcceptInvitation(ctx, AcceptInput{Token: token, UserID: existing.ID})
	if err1 != nil || first == nil {
		t.Fatalf("the first acceptance must succeed: %v", err1)
	}
	if err2 == nil {
		t.Fatal("the second acceptance must not succeed")
	}
	page, _ := h.svc.ListMembers(ctx, h.tenantA, domain.MemberQuery{Search: "race"})
	if page.Total != 1 {
		t.Fatalf("exactly one membership must exist, got %d", page.Total)
	}
}
