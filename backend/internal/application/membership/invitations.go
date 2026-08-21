// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package membership

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// DeliveryStatus reports what actually happened to the invitation email. It is
// returned to the inviting administrator so the UI can say "sent" only when
// something was sent.
type DeliveryStatus string

const (
	// DeliverySent means the mail transport accepted the message.
	DeliverySent DeliveryStatus = "sent"
	// DeliveryUnavailable means no mail transport is configured. The invitation
	// is real and the link works; it just has to be delivered by hand.
	DeliveryUnavailable DeliveryStatus = "unavailable"
	// DeliveryFailed means a transport exists and rejected the message.
	DeliveryFailed DeliveryStatus = "failed"
)

// InviteResult is the answer to creating or resending an invitation.
type InviteResult struct {
	Invitation InvitationView `json:"invitation"`
	Delivery   DeliveryStatus `json:"delivery"`
	// DeliveryDetail explains a non-sent delivery in words the admin can act on.
	DeliveryDetail string `json:"delivery_detail,omitempty"`
	// AcceptURL carries the one-time token and is returned ONLY when the email
	// did not go out. When mail works, the invitee is the only one who receives
	// the link, and an administrator has no reason to hold a credential that
	// authenticates as somebody else. When mail does not work, withholding it
	// would leave a real invitation that nobody can reach — so the admin gets it
	// once, to deliver by hand, and it is never returned again by any listing.
	AcceptURL string `json:"accept_url,omitempty"`
}

// InviteInput is an administrator inviting somebody.
type InviteInput struct {
	ActorID      uuid.UUID
	Email        string
	Role         domain.MemberRole
	BusinessRole domain.BusinessRoleKey
	Locale       string
}

// Invite creates an invitation and sends it.
//
// The order matters. The invitation is persisted BEFORE the mail is attempted:
// a mail failure must leave a revocable, resendable invitation, not a message
// that went out referring to a row that was never written.
func (s *Service) Invite(ctx context.Context, tenantID uuid.UUID, in InviteInput) (*InviteResult, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewUnauthorizedError("no organization in context")
	}
	email, err := validateEmail(in.Email)
	if err != nil {
		return nil, err
	}
	role, business, err := validateRoles(in.Role, in.BusinessRole)
	if err != nil {
		return nil, err
	}

	// Already a member of THIS organization? Being a member elsewhere is normal
	// and must not block the invitation, so the check is scoped to the tenant.
	if existing, err := s.users.GetByEmail(ctx, email); err != nil {
		return nil, err
	} else if existing != nil {
		m, err := s.repo.GetMember(ctx, tenantID, existing.ID)
		if err != nil {
			return nil, err
		}
		switch {
		case m == nil, m.EffectiveStatus() == domain.MembershipRevoked:
			// Not a member, or a revoked one: inviting is the right verb. A
			// revoked membership is reinstated on acceptance rather than
			// duplicated, since one membership per (organization, user) is a
			// database invariant.
		case m.EffectiveStatus() == domain.MembershipDeactivated:
			// A suspended member is still in the member list, where Reactivate is
			// one click away. A second route to the same seat would only make it
			// unclear which one restored their access.
			return nil, &domain.AppError{
				Err:     domain.ErrConflict,
				Message: "this person is already a member here, currently deactivated — reactivate them instead",
				Code:    http.StatusConflict,
			}
		default:
			return nil, &domain.AppError{
				Err:     domain.ErrConflict,
				Message: "this person is already a member of this organization",
				Code:    http.StatusConflict,
			}
		}
	}

	if pending, err := s.repo.FindPendingInvitation(ctx, tenantID, email); err != nil {
		return nil, err
	} else if pending != nil && pending.IsUsable(s.clock()) {
		// A live invitation already exists. Minting a second one would leave two
		// working tokens for one seat; the admin wants Resend.
		return nil, domain.NewConflictError("invitation", "email")
	}

	now := s.clock()
	inv, token, err := domain.NewInvitation(tenantID, in.ActorID, email, role, business, now)
	if err != nil {
		return nil, domain.NewInternalError("could not mint an invitation token")
	}
	if err := s.repo.CreateInvitation(ctx, inv); err != nil {
		return nil, err
	}

	// The audit trail records that an invitation was created, for whom and at
	// what role — never the token, and never anything derived from it.
	s.record(ctx, tenantID, in.ActorID, domain.AuditActionCreate, "invitation", inv.ID.String(),
		fmt.Sprintf("Invited %s as %s", inv.Email, inv.Role), nil,
		domain.JSONMap{"email": inv.Email, "role": string(inv.Role), "expires_at": inv.ExpiresAt})

	return s.deliver(ctx, tenantID, in.ActorID, inv, token, in.Locale, false), nil
}

// ResendInput re-sends an outstanding invitation.
type ResendInput struct {
	ActorID      uuid.UUID
	InvitationID uuid.UUID
	Locale       string
}

// Resend rotates the invitation's token and sends it again.
//
// Rotation is the policy: exactly one token can open the door at a time. That
// keeps an unbounded number of valid credentials from accumulating behind a
// button anyone can click, and it makes "resend" genuinely invalidate a link
// that was forwarded to the wrong person.
func (s *Service) Resend(ctx context.Context, tenantID uuid.UUID, in ResendInput) (*InviteResult, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewUnauthorizedError("no organization in context")
	}
	inv, err := s.repo.GetInvitation(ctx, tenantID, in.InvitationID)
	if err != nil {
		return nil, err
	}
	if inv == nil {
		return nil, domain.NewNotFoundError("invitation", in.InvitationID)
	}

	now := s.clock()
	// Cooldown, send cap and terminal states, all decided by the invitation
	// itself so the API and the UI agree on why the button is disabled.
	if err := inv.CanResend(now); err != nil {
		return nil, err
	}
	token, err := inv.Rotate(now)
	if err != nil {
		return nil, domain.NewInternalError("could not mint an invitation token")
	}
	if err := s.repo.SaveInvitation(ctx, inv); err != nil {
		return nil, err
	}

	s.record(ctx, tenantID, in.ActorID, domain.AuditActionUpdate, "invitation", inv.ID.String(),
		fmt.Sprintf("Re-sent the invitation to %s (send %d)", inv.Email, inv.SendCount), nil,
		domain.JSONMap{"email": inv.Email, "send_count": inv.SendCount, "expires_at": inv.ExpiresAt})

	return s.deliver(ctx, tenantID, in.ActorID, inv, token, in.Locale, true), nil
}

// RevokeInvitation withdraws an outstanding invitation, invalidating its token.
func (s *Service) RevokeInvitation(ctx context.Context, tenantID, actorID, invitationID uuid.UUID) (*InvitationView, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewUnauthorizedError("no organization in context")
	}
	inv, err := s.repo.GetInvitation(ctx, tenantID, invitationID)
	if err != nil {
		return nil, err
	}
	if inv == nil {
		return nil, domain.NewNotFoundError("invitation", invitationID)
	}

	now := s.clock()
	switch inv.Status {
	case domain.InvitationAccepted:
		// The membership already exists. Revoking the invitation now would look
		// like it withdrew access, which it does not — that is Deactivate.
		return nil, domain.NewConflictError("invitation", "status")
	case domain.InvitationRevoked:
		return nil, domain.NewGoneError("this invitation was already revoked")
	}

	before := domain.JSONMap{"status": string(inv.State(now))}
	inv.Status = domain.InvitationRevoked
	inv.RevokedAt = &now
	a := actorID
	inv.RevokedByID = &a
	// Rotating on revoke makes the withdrawal effective even if the stored
	// status were somehow read stale: the token that was mailed no longer
	// hashes to anything in the table, so the link is dead on lookup and not
	// merely on a status check.
	if _, hash, err := domain.NewInvitationToken(); err == nil {
		inv.TokenHash = hash
	}
	inv.UpdatedAt = now
	if err := s.repo.SaveInvitation(ctx, inv); err != nil {
		return nil, err
	}

	s.record(ctx, tenantID, actorID, domain.AuditActionRevoke, "invitation", inv.ID.String(),
		fmt.Sprintf("Revoked the invitation to %s", inv.Email), before,
		domain.JSONMap{"status": string(domain.InvitationRevoked)})

	v := toInvitationView(inv, now)
	return &v, nil
}

// ListInvitations returns one page of the tenant's invitations.
func (s *Service) ListInvitations(ctx context.Context, tenantID uuid.UUID, q domain.InvitationQuery) (*Page[InvitationView], error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewUnauthorizedError("no organization in context")
	}
	rows, total, err := s.repo.ListInvitations(ctx, tenantID, q)
	if err != nil {
		return nil, err
	}
	s.resolveEmails(ctx, rows)
	now := s.clock()
	items := make([]InvitationView, 0, len(rows))
	for i := range rows {
		items = append(items, toInvitationView(&rows[i], now))
	}
	return &Page[InvitationView]{Items: items, Total: total, Limit: q.Limit, Offset: q.Offset}, nil
}

// ---------------------------------------------------------------------------
// Acceptance
// ---------------------------------------------------------------------------

// InvitationPreview is what an unauthenticated visitor holding a link may see
// before deciding to accept: which organization, at what role, for which
// address. Nothing about the organization's contents, its size, or its other
// members — the link is a bearer credential and it buys exactly this.
type InvitationPreview struct {
	OrganizationName string            `json:"organization_name"`
	Email            string            `json:"email"`
	Role             domain.MemberRole `json:"role"`
	ExpiresAt        time.Time         `json:"expires_at"`
	// RequiresSignup tells the UI whether to ask for a password.
	RequiresSignup bool `json:"requires_signup"`
}

// PreviewInvitation resolves a token to what its holder may see.
func (s *Service) PreviewInvitation(ctx context.Context, token string) (*InvitationPreview, error) {
	inv, err := s.resolveUsableToken(ctx, token)
	if err != nil {
		return nil, err
	}
	out := &InvitationPreview{Email: inv.Email, Role: inv.Role, ExpiresAt: inv.ExpiresAt}
	if s.orgs != nil {
		if org, err := s.orgs.GetByID(ctx, inv.OrganizationID); err == nil && org != nil {
			out.OrganizationName = org.Name
		}
	}
	if u, err := s.users.GetByEmail(ctx, inv.Email); err == nil {
		out.RequiresSignup = u == nil
	}
	return out, nil
}

// AcceptInput redeems an invitation.
type AcceptInput struct {
	Token string
	// UserID is set when an authenticated user accepts. It must match the
	// invited address; otherwise the link would let whoever holds it join under
	// their own account, which is exactly the misuse the email binding prevents.
	UserID uuid.UUID
	// FullName / Password are used only when the invitee has no account yet.
	FullName string
	Password string
}

// AcceptResult reports which membership was created.
type AcceptResult struct {
	OrganizationID   uuid.UUID         `json:"organization_id"`
	OrganizationName string            `json:"organization_name,omitempty"`
	UserID           uuid.UUID         `json:"user_id"`
	Email            string            `json:"email"`
	Role             domain.MemberRole `json:"role"`
	// CreatedAccount tells the caller whether it must sign in for the first time.
	CreatedAccount bool `json:"created_account"`
}

// AcceptInvitation redeems a token into a membership.
//
// The tenant is NOT a parameter: it comes from the invitation the token
// resolves to. Nothing the caller sends chooses the organization or the role —
// both are read from the stored row, which is what stops a crafted request
// joining an arbitrary tenant as an administrator.
func (s *Service) AcceptInvitation(ctx context.Context, in AcceptInput) (*AcceptResult, error) {
	inv, err := s.resolveUsableToken(ctx, in.Token)
	if err != nil {
		return nil, err
	}
	now := s.clock()

	user, err := s.users.GetByEmail(ctx, inv.Email)
	if err != nil {
		return nil, err
	}
	createdAccount := false

	switch {
	case in.UserID != uuid.Nil:
		// Signed in already: the session's identity must be the invited address.
		signedIn, err := s.users.GetByID(ctx, in.UserID)
		if err != nil {
			return nil, err
		}
		if signedIn == nil || domain.NormalizeEmail(signedIn.Email) != inv.Email {
			// The reason goes in Message, not in Detail: handlers serialise
			// MessageFromError, and NewForbiddenError's Detail never leaves the
			// process. "access denied" tells someone holding a legitimate link
			// nothing they can act on, whereas naming the invited address tells
			// them to switch accounts. It reveals nothing they do not already
			// hold — the address is in the invitation they are presenting.
			return nil, &domain.AppError{
				Err:     domain.ErrForbidden,
				Message: "this invitation was issued to " + inv.Email + " — sign in as that address to accept it",
				Code:    http.StatusForbidden,
			}
		}
		user = signedIn

	case user == nil:
		// No account yet: the invitation is also the sign-up.
		if s.hasher == nil {
			return nil, domain.NewInternalError("account creation is unavailable")
		}
		if len(in.Password) < 12 {
			return nil, domain.NewValidationError("choose a password of at least 12 characters")
		}
		name := strings.TrimSpace(in.FullName)
		if name == "" {
			return nil, domain.NewValidationError("your full name is required")
		}
		hashed, err := s.hasher.Hash(in.Password)
		if err != nil {
			return nil, domain.NewInternalError("could not secure the password")
		}
		username, err := s.uniqueUsername(ctx, inv.Email)
		if err != nil {
			return nil, err
		}
		org := inv.OrganizationID
		user = &domain.User{
			ID: uuid.New(), Email: inv.Email, Username: username, Password: hashed,
			FullName: name, DefaultOrgID: &org, IsActive: true,
			CreatedAt: now, UpdatedAt: now,
		}
		if err := s.users.Create(ctx, user); err != nil {
			return nil, err
		}
		createdAccount = true

	default:
		// The address has an account but nobody is signed in. Creating the
		// membership here would let whoever holds the link attach a seat to
		// somebody else's account without them ever authenticating.
		return nil, domain.NewUnauthorizedError("sign in as " + inv.Email + " to accept this invitation")
	}

	// A revoked prior membership is reinstated rather than duplicated — the
	// unique index over (organization, user) would refuse a second row anyway.
	if existing, err := s.repo.GetMember(ctx, inv.OrganizationID, user.ID); err != nil {
		return nil, err
	} else if existing != nil {
		return s.reinstate(ctx, inv, existing, now)
	}

	inv.AcceptedAt = &now
	uid := user.ID
	inv.AcceptedByID = &uid
	member := &domain.OrganizationMember{
		ID:             uuid.New(),
		OrganizationID: inv.OrganizationID,
		UserID:         user.ID,
		Role:           inv.Role,
		BusinessRole:   inv.BusinessRole,
		Status:         domain.MembershipActive,
		IsActive:       true,
		JoinedAt:       now,
		InvitedByID:    &inv.InvitedByID,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := s.repo.AcceptInvitation(ctx, inv, member); err != nil {
		return nil, err
	}

	// The actor is the invitee: they performed this, and an audit trail that
	// attributed it to the inviter would misreport who joined.
	s.record(ctx, inv.OrganizationID, user.ID, domain.AuditActionCreate, "organization_member", member.ID.String(),
		fmt.Sprintf("%s accepted the invitation and joined as %s", inv.Email, inv.Role), nil,
		domain.JSONMap{"email": inv.Email, "role": string(inv.Role), "invitation_id": inv.ID.String()})

	s.completeOnboarding(ctx, inv.OrganizationID, user.ID)
	return s.acceptResult(ctx, inv, user, createdAccount), nil
}

// completeOnboarding takes a joined member past the signup wizard. Best-effort:
// the membership is already written, and a wizard flag is not worth undoing it
// for.
func (s *Service) completeOnboarding(ctx context.Context, tenantID, userID uuid.UUID) {
	if s.onboarding == nil {
		return
	}
	_ = s.onboarding.MarkComplete(ctx, tenantID, userID)
}

// reinstate re-activates a membership that already exists for the invitee,
// consuming the invitation in the same breath.
func (s *Service) reinstate(ctx context.Context, inv *domain.Invitation, m *domain.OrganizationMember, now time.Time) (*AcceptResult, error) {
	if m.EffectiveStatus().GrantsAccess() {
		// Already a member with access: consume the invitation so the link stops
		// working, and report success rather than an error the invitee cannot act
		// on — they are, after all, in the organization.
		inv.Status = domain.InvitationAccepted
		inv.AcceptedAt = &now
		uid := m.UserID
		inv.AcceptedByID = &uid
		inv.UpdatedAt = now
		if err := s.repo.SaveInvitation(ctx, inv); err != nil {
			return nil, err
		}
	} else {
		before := m.EffectiveStatus()
		if !m.SetStatus(domain.MembershipActive, now) {
			// A revoked membership is terminal by design; re-admitting somebody
			// whose access was revoked is a decision an administrator makes, not
			// something an old link performs.
			return nil, &domain.AppError{
				Err:     domain.ErrForbidden,
				Message: "access to this organization was revoked — ask an administrator to invite you again",
				Code:    http.StatusForbidden,
			}
		}
		m.Role = inv.Role
		m.BusinessRole = inv.BusinessRole
		if err := s.repo.SaveMember(ctx, m); err != nil {
			return nil, err
		}
		inv.Status = domain.InvitationAccepted
		inv.AcceptedAt = &now
		uid := m.UserID
		inv.AcceptedByID = &uid
		inv.UpdatedAt = now
		if err := s.repo.SaveInvitation(ctx, inv); err != nil {
			return nil, err
		}
		s.record(ctx, inv.OrganizationID, m.UserID, domain.AuditActionUpdate, "organization_member", m.ID.String(),
			fmt.Sprintf("%s rejoined via invitation (was %s)", inv.Email, before),
			domain.JSONMap{"status": string(before)}, domain.JSONMap{"status": string(domain.MembershipActive)})
	}

	user, _ := s.users.GetByID(ctx, m.UserID)
	if user == nil {
		user = &domain.User{ID: m.UserID, Email: inv.Email}
	}
	s.completeOnboarding(ctx, inv.OrganizationID, m.UserID)
	return s.acceptResult(ctx, inv, user, false), nil
}

func (s *Service) acceptResult(ctx context.Context, inv *domain.Invitation, user *domain.User, created bool) *AcceptResult {
	out := &AcceptResult{
		OrganizationID: inv.OrganizationID,
		UserID:         user.ID,
		Email:          inv.Email,
		Role:           inv.Role,
		CreatedAccount: created,
	}
	if s.orgs != nil {
		if org, err := s.orgs.GetByID(ctx, inv.OrganizationID); err == nil && org != nil {
			out.OrganizationName = org.Name
		}
	}
	return out
}

// resolveUsableToken turns a bearer token into a redeemable invitation, or into
// the reason it is not one.
//
// Every failure that is not "this specific invitation is finished" answers 404.
// An unknown token, a malformed token and a token for another tenant are
// indistinguishable from outside, so the endpoint cannot be used to test
// guesses. Revoked and expired answer 410 on purpose: those are told to someone
// who legitimately held the link, and "this is over" is actionable where "not
// found" would just look broken.
func (s *Service) resolveUsableToken(ctx context.Context, token string) (*domain.Invitation, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, domain.NewNotFoundError("invitation", "token")
	}
	inv, err := s.repo.FindInvitationByToken(ctx, token)
	if err != nil {
		return nil, err
	}
	if inv == nil {
		return nil, domain.NewNotFoundError("invitation", "token")
	}
	// Belt and braces over the hash lookup, in constant time.
	if !domain.InvitationTokenMatches(token, inv.TokenHash) {
		return nil, domain.NewNotFoundError("invitation", "token")
	}
	switch inv.State(s.clock()) {
	case domain.InvitationPending:
		return inv, nil
	case domain.InvitationAccepted:
		return nil, domain.NewGoneError("this invitation has already been used")
	case domain.InvitationRevoked:
		return nil, domain.NewGoneError("this invitation was revoked")
	default:
		return nil, domain.NewGoneError("this invitation has expired — ask for a new one")
	}
}

func (s *Service) uniqueUsername(ctx context.Context, email string) (string, error) {
	base := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
			return r
		default:
			return -1
		}
	}, strings.Split(email, "@")[0])
	if len(base) < 3 {
		base += "usr"
	}
	candidate := base
	for i := 0; i < 20; i++ {
		existing, err := s.users.GetByUsername(ctx, candidate)
		if err != nil {
			return "", err
		}
		if existing == nil {
			return candidate, nil
		}
		candidate = fmt.Sprintf("%s%d", base, i+1)
	}
	return "", domain.NewConflictError("user", "username")
}

// ---------------------------------------------------------------------------
// Delivery
// ---------------------------------------------------------------------------

// deliver attempts to email the invitation and reports honestly what happened.
func (s *Service) deliver(ctx context.Context, tenantID, actorID uuid.UUID, inv *domain.Invitation, token, locale string, resend bool) *InviteResult {
	now := s.clock()
	acceptURL := s.acceptURL(token)
	out := &InviteResult{Invitation: toInvitationView(inv, now)}

	if s.mailer == nil {
		out.Delivery = DeliveryUnavailable
		out.DeliveryDetail = "No email transport is configured on this deployment — share the link below yourself."
		out.AcceptURL = acceptURL
		return out
	}

	mail := InvitationMail{
		To:        inv.Email,
		RoleLabel: string(inv.Role),
		AcceptURL: acceptURL,
		ExpiresAt: inv.ExpiresAt,
		Locale:    locale,
	}
	if s.orgs != nil {
		if org, err := s.orgs.GetByID(ctx, tenantID); err == nil && org != nil {
			mail.OrgName = org.Name
		}
	}
	if actorID != uuid.Nil {
		if u, err := s.users.GetByID(ctx, actorID); err == nil && u != nil {
			mail.InviterName = u.FullName
			mail.SendersEmail = u.Email
		}
	}

	if err := s.mailer.SendInvitation(ctx, mail); err != nil {
		// A failed send is reported as failed and the link is handed back, so the
		// administrator is never told an email went out that did not.
		out.Delivery = DeliveryFailed
		out.DeliveryDetail = "The invitation was created but the email could not be sent — share the link below yourself."
		out.AcceptURL = acceptURL
		return out
	}
	out.Delivery = DeliverySent
	return out
}

// acceptURL builds the link the invitee follows.
func (s *Service) acceptURL(token string) string {
	base := s.baseURL
	if base == "" {
		base = "http://localhost:5173"
	}
	return fmt.Sprintf("%s/invitations/accept?token=%s", base, url.QueryEscape(token))
}
