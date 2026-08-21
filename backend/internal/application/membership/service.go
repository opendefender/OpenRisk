// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package membership holds the use cases for administering an organization's
// members: listing them, inviting people, accepting and revoking invitations,
// changing roles, deactivating and revoking access, and reading the membership
// audit trail.
//
// TENANT SCOPE. Every use case takes the tenant id as its first argument, and
// that id always comes from the authenticated session — never from a body, a
// query string or a path segment. AcceptInvitation is the single exception, and
// it takes no tenant at all: the organization is read from the invitation the
// token resolves to.
//
// DATA EXPOSURE. Nothing in this package returns a password hash, an MFA
// secret, a session token or an invitation token. The views below are
// allowlists — a field reaches the API because it was written here, so a column
// added to the model tomorrow cannot leak by default.
package membership

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// ---------------------------------------------------------------------------
// Ports
// ---------------------------------------------------------------------------

// AuditSink records a membership event. Best-effort and nil-safe: journalling
// must never be the reason a legitimate administrative action fails.
type AuditSink interface {
	Record(ctx context.Context, ev domain.AuditEvent)
}

// AuditReader reads back the membership slice of the audit trail.
type AuditReader interface {
	List(ctx context.Context, tenantID uuid.UUID, f domain.AuditEventFilter) ([]domain.AuditEvent, int64, error)
}

// UserDirectory is the narrow slice of the user store this package needs:
// resolve people by id or email, and create the account an invitation was
// accepted by. Satisfied by GormUserRepository.
type UserDirectory interface {
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	GetByUsername(ctx context.Context, username string) (*domain.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
	Create(ctx context.Context, user *domain.User) error
	EmailsByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error)
}

// InvitationMailer delivers an invitation. It reports honestly whether the
// message went out: a transport that is not configured must return an error,
// never a silent success, or the product would tell an administrator that an
// invitation was emailed when nothing left the building.
type InvitationMailer interface {
	SendInvitation(ctx context.Context, m InvitationMail) error
}

// InvitationMail is everything the mailer needs. AcceptURL carries the one-time
// token and is the only place in the system, besides the creating admin's
// response, where the plaintext exists.
type InvitationMail struct {
	To           string
	OrgName      string
	InviterName  string
	RoleLabel    string
	AcceptURL    string
	ExpiresAt    time.Time
	Locale       string
	SendersEmail string
}

// SessionRevoker ends a member's sessions when their access is withdrawn.
// Satisfied by the auth TokenManager's RevokeAllUserTokens.
type SessionRevoker interface {
	RevokeAllUserTokens(ctx context.Context, userID uuid.UUID) error
}

// PasswordHasher hashes the password an invitee chooses on acceptance.
type PasswordHasher interface {
	Hash(password string) (string, error)
}

// OnboardingCompleter marks a newly joined member as past the signup wizard.
//
// The wizard's first screen asks the user to set up an organization. That is
// the right question for someone creating a tenant and the wrong one for
// someone joining one that already exists — and the route guard blocks the
// whole application until onboarding is complete, so an invitee would be held
// in front of a form about an organization they did not create and cannot
// edit. This is the same reasoning as BackfillExistingMembers, applied to the
// members who arrive after the upgrade rather than before it.
//
// Optional and nil-safe, and never fatal: failing to tick a wizard flag must
// not undo a membership that was just created.
type OnboardingCompleter interface {
	MarkComplete(ctx context.Context, tenantID, userID uuid.UUID) error
}

// OrganizationReader resolves the tenant's own profile.
type OrganizationReader interface {
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Organization, error)
}

// ---------------------------------------------------------------------------
// Views — the allowlisted shapes that cross the API boundary
// ---------------------------------------------------------------------------

// MemberView is one row of the member list. It carries identity, access and
// timestamps, and nothing else: no password hash, no MFA state, no tokens.
type MemberView struct {
	MemberID uuid.UUID         `json:"member_id"`
	UserID   uuid.UUID         `json:"user_id"`
	Email    string            `json:"email"`
	FullName string            `json:"full_name"`
	OrgRole  domain.MemberRole `json:"org_role"`
	// Always emitted, never omitempty: a client has to be able to tell "this
	// member has no preset" from "the server did not say", and the members
	// screen renders one of those as an empty dropdown and the other as stale.
	BusinessRole  domain.BusinessRoleKey  `json:"business_role"`
	Status        domain.MembershipStatus `json:"status"`
	IsActive      bool                    `json:"is_active"`
	JoinedAt      time.Time               `json:"joined_at"`
	DeactivatedAt *time.Time              `json:"deactivated_at,omitempty"`
	RevokedAt     *time.Time              `json:"revoked_at,omitempty"`
	InvitedByID   *uuid.UUID              `json:"invited_by_id,omitempty"`
	LastLogin     *time.Time              `json:"last_login,omitempty"`
	// Permissions is the member's resolved effective access, so the UI shows
	// what someone can actually do rather than what their role is called.
	Permissions []string `json:"permissions"`
	// IsOwner marks the organization owner, whose role and membership are not
	// administered through this API.
	IsOwner bool `json:"is_owner"`
}

// InvitationView is one row of the invitation list. There is deliberately no
// token field of any kind.
type InvitationView struct {
	ID             uuid.UUID               `json:"id"`
	Email          string                  `json:"email"`
	Role           domain.MemberRole       `json:"role"`
	BusinessRole   domain.BusinessRoleKey  `json:"business_role,omitempty"`
	Status         domain.InvitationStatus `json:"status"`
	ExpiresAt      time.Time               `json:"expires_at"`
	InvitedByID    uuid.UUID               `json:"invited_by_id"`
	InvitedByEmail string                  `json:"invited_by_email,omitempty"`
	LastSentAt     time.Time               `json:"last_sent_at"`
	SendCount      int                     `json:"send_count"`
	AcceptedAt     *time.Time              `json:"accepted_at,omitempty"`
	RevokedAt      *time.Time              `json:"revoked_at,omitempty"`
	CreatedAt      time.Time               `json:"created_at"`
	// CanResend mirrors the server's own policy so the UI can disable the button
	// for the same reason the API would refuse it, instead of guessing.
	CanResend bool `json:"can_resend"`
}

// Page wraps a listing with its total, so a client can paginate over a number
// that does not change as it pages.
type Page[T any] struct {
	Items  []T   `json:"items"`
	Total  int64 `json:"total"`
	Limit  int   `json:"limit"`
	Offset int   `json:"offset"`
}

func toMemberView(m *domain.OrganizationMember) MemberView {
	v := MemberView{
		MemberID:      m.ID,
		UserID:        m.UserID,
		OrgRole:       m.Role,
		BusinessRole:  m.BusinessRole,
		Status:        m.EffectiveStatus(),
		IsActive:      m.EffectiveStatus().GrantsAccess(),
		JoinedAt:      m.JoinedAt,
		DeactivatedAt: m.DeactivatedAt,
		RevokedAt:     m.RevokedAt,
		InvitedByID:   m.InvitedByID,
		Permissions:   m.EffectivePermissions(),
		IsOwner:       m.IsRoot(),
	}
	if m.User != nil {
		v.Email = m.User.Email
		v.FullName = m.User.FullName
		v.LastLogin = m.User.LastLogin
	}
	return v
}

func toInvitationView(inv *domain.Invitation, now time.Time) InvitationView {
	return InvitationView{
		ID:             inv.ID,
		Email:          inv.Email,
		Role:           inv.Role,
		BusinessRole:   inv.BusinessRole,
		Status:         inv.State(now), // projected: expiry is time, not a stored flag
		ExpiresAt:      inv.ExpiresAt,
		InvitedByID:    inv.InvitedByID,
		InvitedByEmail: inv.InvitedByEmail,
		LastSentAt:     inv.LastSentAt,
		SendCount:      inv.SendCount,
		AcceptedAt:     inv.AcceptedAt,
		RevokedAt:      inv.RevokedAt,
		CreatedAt:      inv.CreatedAt,
		CanResend:      inv.CanResend(now) == nil,
	}
}

// ---------------------------------------------------------------------------
// Service
// ---------------------------------------------------------------------------

// Service is the member-administration use cases, sharing one repository. It is
// one struct rather than a struct per verb because the operations share the
// same guards and the same audit vocabulary, and splitting them would mean
// re-injecting the same six ports nine times.
//
// Optional collaborators are attached with With* and are nil-safe: a missing
// mailer degrades invitation DELIVERY, never invitation creation; a missing
// audit sink degrades the journal, never the action.
type Service struct {
	repo       domain.MembershipRepository
	users      UserDirectory
	orgs       OrganizationReader
	audit      AuditSink
	reader     AuditReader
	mailer     InvitationMailer
	revoker    SessionRevoker
	hasher     PasswordHasher
	onboarding OnboardingCompleter
	baseURL    string
	now        func() time.Time
}

// NewService builds the service over its two required ports.
func NewService(repo domain.MembershipRepository, users UserDirectory) *Service {
	return &Service{repo: repo, users: users, now: time.Now}
}

func (s *Service) WithOrganizations(o OrganizationReader) *Service { s.orgs = o; return s }
func (s *Service) WithAudit(a AuditSink) *Service                  { s.audit = a; return s }
func (s *Service) WithAuditReader(r AuditReader) *Service          { s.reader = r; return s }
func (s *Service) WithMailer(m InvitationMailer) *Service          { s.mailer = m; return s }
func (s *Service) WithSessionRevoker(r SessionRevoker) *Service    { s.revoker = r; return s }
func (s *Service) WithPasswordHasher(h PasswordHasher) *Service    { s.hasher = h; return s }
func (s *Service) WithOnboarding(o OnboardingCompleter) *Service   { s.onboarding = o; return s }
func (s *Service) WithBaseURL(u string) *Service {
	s.baseURL = strings.TrimRight(strings.TrimSpace(u), "/")
	return s
}

// WithClock overrides the clock (tests only).
func (s *Service) WithClock(f func() time.Time) *Service { s.now = f; return s }

func (s *Service) clock() time.Time {
	if s.now == nil {
		return time.Now()
	}
	return s.now()
}

// record writes a membership audit event. Best-effort by construction.
func (s *Service) record(ctx context.Context, tenantID, actorID uuid.UUID, action domain.AuditAction, entityType, entityID, summary string, before, after domain.JSONMap) {
	if s.audit == nil || tenantID == uuid.Nil {
		return
	}
	var actor *uuid.UUID
	if actorID != uuid.Nil {
		a := actorID
		actor = &a
	}
	s.audit.Record(ctx, domain.AuditEvent{
		TenantID:   tenantID,
		ActorID:    actor,
		Action:     action,
		EntityType: entityType,
		EntityID:   entityID,
		Summary:    summary,
		Before:     before,
		After:      after,
	})
}

// resolveEmails fills InvitedByEmail on a batch of invitations in one query, so
// a list of fifty invitations does not become fifty user lookups.
func (s *Service) resolveEmails(ctx context.Context, rows []domain.Invitation) {
	if s.users == nil || len(rows) == 0 {
		return
	}
	seen := map[uuid.UUID]struct{}{}
	ids := make([]uuid.UUID, 0, len(rows))
	for i := range rows {
		if rows[i].InvitedByID == uuid.Nil {
			continue
		}
		if _, ok := seen[rows[i].InvitedByID]; ok {
			continue
		}
		seen[rows[i].InvitedByID] = struct{}{}
		ids = append(ids, rows[i].InvitedByID)
	}
	if len(ids) == 0 {
		return
	}
	// A failed lookup leaves the display name blank rather than failing the
	// listing: whose invitation it is matters less than seeing the invitations.
	byID, err := s.users.EmailsByIDs(ctx, ids)
	if err != nil {
		return
	}
	for i := range rows {
		rows[i].InvitedByEmail = byID[rows[i].InvitedByID]
	}
}

var emailPattern = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// validateEmail normalises and checks an address.
func validateEmail(raw string) (string, error) {
	e := domain.NormalizeEmail(raw)
	if e == "" || len(e) > 320 || !emailPattern.MatchString(e) {
		return "", domain.NewValidationError("a valid email address is required")
	}
	return e, nil
}

// validateRoles checks the org role and business role an invitation or a role
// change asks for. An empty org role defaults to "user" — the least privileged
// thing an invitation can mean.
func validateRoles(role domain.MemberRole, business domain.BusinessRoleKey) (domain.MemberRole, domain.BusinessRoleKey, error) {
	if role == "" {
		role = domain.RoleUser
	}
	if !domain.IsAssignableMemberRole(role) {
		return "", "", domain.NewValidationError("role must be one of: admin, user")
	}
	if business != "" && !domain.IsBusinessRole(business) {
		return "", "", domain.NewValidationError("unknown business role: " + string(business))
	}
	// An admin holds the wildcard, so a business role attached to one is a
	// least-privilege preset that grants nothing and would only mislead whoever
	// reads the member list next.
	if role == domain.RoleAdmin {
		business = ""
	}
	return role, business, nil
}
