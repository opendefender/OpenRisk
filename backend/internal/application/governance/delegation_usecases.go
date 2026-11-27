// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package governance

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// -------------------- Create --------------------

// CreateDelegationInput is the payload to grant a user's rights to a colleague
// for a bounded window (e.g. while on leave).
type CreateDelegationInput struct {
	DelegatorID uuid.UUID // whose rights are lent (defaults to the actor)
	DelegateID  uuid.UUID // who receives them
	Reason      string
	Permissions []string // subset of the delegator's rights, or ["*"]
	StartsAt    *time.Time
	EndsAt      *time.Time
	// ActorIsAdmin says whether the caller administers the organisation. Only an
	// admin may record a delegation on somebody ELSE's behalf; see Execute.
	ActorIsAdmin bool
}

// MemberLookup is the narrow port that proves a user belongs to the tenant.
// Satisfied structurally by the membership repository (cmd/server memberDirectory).
type MemberLookup interface {
	GetMember(ctx context.Context, tenantID, userID uuid.UUID) (*domain.OrganizationMember, error)
}

type CreateDelegationUseCase struct {
	repo     domain.DelegationRepository
	recorder *AuditRecorder
	lookup   UserLookup
	members  MemberLookup
}

// WithMembers makes Execute refuse a delegate who is not a member of the
// tenant (#1319). Without it any user id, from any organisation, was accepted,
// and listing the delegation then resolved that user's e-mail address.
func (uc *CreateDelegationUseCase) WithMembers(m MemberLookup) *CreateDelegationUseCase {
	uc.members = m
	return uc
}

func NewCreateDelegationUseCase(repo domain.DelegationRepository) *CreateDelegationUseCase {
	return &CreateDelegationUseCase{repo: repo}
}

func (uc *CreateDelegationUseCase) WithRecorder(r *AuditRecorder) *CreateDelegationUseCase {
	uc.recorder = r
	return uc
}
func (uc *CreateDelegationUseCase) WithUserLookup(l UserLookup) *CreateDelegationUseCase {
	uc.lookup = l
	return uc
}

func (uc *CreateDelegationUseCase) Execute(ctx context.Context, tenantID, actorID uuid.UUID, in CreateDelegationInput) (*domain.Delegation, error) {
	delegator := in.DelegatorID
	if delegator == uuid.Nil {
		delegator = actorID
	}
	if delegator == uuid.Nil {
		return nil, domain.NewValidationError("delegator is required")
	}
	// #529 — a delegation is a grant of the DELEGATOR's authority, and
	// DecideApproval layers the delegator's roles onto whoever holds it
	// (resolveApprover → domain.CanSign). Letting a caller name an arbitrary
	// delegator therefore lets any member mint themselves an admin's signature
	// on a maker-checker request. You lend your own rights; an org admin may
	// record a delegation for someone else (the "they left, reassign it" case).
	if delegator != actorID && !in.ActorIsAdmin {
		return nil, domain.NewForbiddenError("only an organization admin may create a delegation on another member's behalf")
	}
	if in.DelegateID == uuid.Nil {
		return nil, domain.NewValidationError("delegate is required")
	}
	if in.DelegateID == delegator {
		return nil, domain.NewValidationError("cannot delegate to yourself")
	}
	// #1319: both parties must belong to THIS tenant. The delegator check matters
	// as much as the delegate's: an admin may name another delegator, and the
	// listing resolves both e-mails. A foreign or unknown id answers not-found,
	// the same for both, so the answer says nothing about whether the user
	// exists elsewhere. Fail closed when no membership lookup is wired.
	if uc.members == nil {
		return nil, domain.NewForbiddenError("delegations are unavailable: no membership lookup is configured")
	}
	if delegator != actorID {
		if m, err := uc.members.GetMember(ctx, tenantID, delegator); err != nil {
			return nil, err
		} else if m == nil {
			return nil, domain.NewNotFoundError("user", delegator.String())
		}
	}
	member, err := uc.members.GetMember(ctx, tenantID, in.DelegateID)
	if err != nil {
		return nil, err
	}
	if member == nil {
		return nil, domain.NewNotFoundError("user", in.DelegateID.String())
	}
	if !member.IsActive {
		return nil, domain.NewValidationError("the delegate is deactivated and cannot receive a delegation")
	}
	perms := normalisePermissions(in.Permissions)
	if len(perms) == 0 {
		return nil, domain.NewValidationError("at least one permission (or \"*\") is required")
	}

	start := time.Now().UTC()
	if in.StartsAt != nil {
		start = in.StartsAt.UTC()
	}
	if in.EndsAt == nil {
		return nil, domain.NewValidationError("end date is required")
	}
	end := in.EndsAt.UTC()
	if !end.After(start) {
		return nil, domain.NewValidationError("end date must be after start date")
	}

	d := &domain.Delegation{
		TenantID:    tenantID,
		DelegatorID: delegator,
		DelegateID:  in.DelegateID,
		Reason:      strings.TrimSpace(in.Reason),
		Permissions: domain.StringList(perms),
		Status:      domain.DelegationActive,
		StartsAt:    start,
		EndsAt:      end,
		CreatedBy:   actorID,
	}
	if err := uc.repo.Create(ctx, d); err != nil {
		return nil, err
	}
	uc.enrich(ctx, tenantID, []domain.Delegation{*d})
	if uc.recorder != nil {
		actor := actorID
		uc.recorder.Record(ctx, domain.AuditEvent{
			TenantID:   tenantID,
			ActorID:    &actor,
			Action:     domain.AuditActionDelegate,
			EntityType: "delegation",
			EntityID:   d.ID.String(),
			Summary:    "delegated rights to " + d.DelegateID.String(),
			After:      domain.JSONMap{"delegate_id": d.DelegateID.String(), "permissions": perms, "ends_at": end},
		})
	}
	// Re-read to project resolved emails onto the returned struct.
	if list, err := uc.repo.List(ctx, tenantID, domain.DelegationFilter{}); err == nil {
		for i := range list {
			if list[i].ID == d.ID {
				return &list[i], nil
			}
		}
	}
	return d, nil
}

// -------------------- List --------------------

type ListDelegationsUseCase struct {
	repo    domain.DelegationRepository
	lookup  UserLookup
	members MemberLookup
}

// WithMembers limits the e-mails shown to members of the tenant, so delegations
// stored before the create-time check stop resolving foreign users (#1319).
func (uc *ListDelegationsUseCase) WithMembers(m MemberLookup) *ListDelegationsUseCase {
	uc.members = m
	return uc
}

func NewListDelegationsUseCase(repo domain.DelegationRepository) *ListDelegationsUseCase {
	return &ListDelegationsUseCase{repo: repo}
}
func (uc *ListDelegationsUseCase) WithUserLookup(l UserLookup) *ListDelegationsUseCase {
	uc.lookup = l
	return uc
}

func (uc *ListDelegationsUseCase) Execute(ctx context.Context, tenantID uuid.UUID, f domain.DelegationFilter) ([]domain.Delegation, error) {
	list, err := uc.repo.List(ctx, tenantID, f)
	if err != nil {
		return nil, err
	}
	// Expire lapsed-but-still-"active" rows in the projection so the UI is honest
	// (the row is only mutated on explicit revoke; this is a read-time view).
	now := time.Now().UTC()
	for i := range list {
		if list[i].Status == domain.DelegationActive && now.After(list[i].EndsAt) {
			list[i].Status = "expired"
		}
	}
	uc.enrich(ctx, tenantID, list)
	return list, nil
}

// -------------------- Revoke --------------------

type RevokeDelegationUseCase struct {
	repo     domain.DelegationRepository
	recorder *AuditRecorder
}

func NewRevokeDelegationUseCase(repo domain.DelegationRepository) *RevokeDelegationUseCase {
	return &RevokeDelegationUseCase{repo: repo}
}
func (uc *RevokeDelegationUseCase) WithRecorder(r *AuditRecorder) *RevokeDelegationUseCase {
	uc.recorder = r
	return uc
}

// RevokeDelegationInput names the delegation and carries the caller's standing,
// so the use case can decide whether they may end this grant.
type RevokeDelegationInput struct {
	ID           uuid.UUID
	ActorIsAdmin bool
}

func (uc *RevokeDelegationUseCase) Execute(ctx context.Context, tenantID, actorID uuid.UUID, in RevokeDelegationInput) (*domain.Delegation, error) {
	id := in.ID
	d, err := uc.repo.GetByID(ctx, id, tenantID)
	if err != nil {
		return nil, err
	}
	if d == nil {
		return nil, domain.NewNotFoundError("delegation", id)
	}
	// #529 — revocation ends somebody's standing authority. The two parties to
	// the grant may end it, and so may an org admin; an unrelated member may
	// not, or the approval inbox becomes deniable by anyone who can log in.
	if d.DelegatorID != actorID && d.DelegateID != actorID && !in.ActorIsAdmin {
		return nil, domain.NewForbiddenError("only the delegator, the delegate or an organization admin may revoke this delegation")
	}
	if d.Status == domain.DelegationRevoked {
		return nil, domain.NewValidationError("delegation is already revoked")
	}
	now := time.Now().UTC()
	d.Status = domain.DelegationRevoked
	d.RevokedAt = &now
	if err := uc.repo.Update(ctx, d); err != nil {
		return nil, err
	}
	if uc.recorder != nil {
		actor := actorID
		uc.recorder.Record(ctx, domain.AuditEvent{
			TenantID:   tenantID,
			ActorID:    &actor,
			Action:     domain.AuditActionRevoke,
			EntityType: "delegation",
			EntityID:   d.ID.String(),
			Summary:    "revoked delegation to " + d.DelegateID.String(),
		})
	}
	return d, nil
}

// -------------------- Resolve effective (delegated) permissions --------------------

// ResolveEffectivePermissionsUseCase returns the permission set a user currently
// holds by delegation (union of all delegations to them active right now). This
// is what an authorization layer would OR into the user's own permissions.
type ResolveEffectivePermissionsUseCase struct {
	repo domain.DelegationRepository
}

func NewResolveEffectivePermissionsUseCase(repo domain.DelegationRepository) *ResolveEffectivePermissionsUseCase {
	return &ResolveEffectivePermissionsUseCase{repo: repo}
}

func (uc *ResolveEffectivePermissionsUseCase) Execute(ctx context.Context, tenantID, delegateID uuid.UUID) ([]string, error) {
	if delegateID == uuid.Nil {
		return nil, domain.NewValidationError("delegate is required")
	}
	list, err := uc.repo.List(ctx, tenantID, domain.DelegationFilter{DelegateID: &delegateID, ActiveOnly: true})
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	seen := map[string]struct{}{}
	out := []string{}
	for i := range list {
		if !list[i].IsActiveAt(now) {
			continue
		}
		for _, p := range list[i].Permissions {
			if _, ok := seen[p]; ok {
				continue
			}
			seen[p] = struct{}{}
			out = append(out, p)
		}
	}
	return out, nil
}

// -------------------- shared helpers --------------------

func normalisePermissions(in []string) []string {
	seen := map[string]struct{}{}
	out := []string{}
	for _, p := range in {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	return out
}

// enrich fills delegator/delegate emails on a slice of delegations (best-effort).
func (uc *CreateDelegationUseCase) enrich(ctx context.Context, tenantID uuid.UUID, list []domain.Delegation) {
	enrichDelegationEmails(ctx, uc.lookup, uc.members, tenantID, list)
}
func (uc *ListDelegationsUseCase) enrich(ctx context.Context, tenantID uuid.UUID, list []domain.Delegation) {
	enrichDelegationEmails(ctx, uc.lookup, uc.members, tenantID, list)
}

// enrichDelegationEmails resolves e-mails only for users who are members of the
// tenant. EmailsByIDs is global, so without this filter a delegation naming a
// foreign user id showed that user's address (#1319). No membership lookup means
// no e-mail at all.
func enrichDelegationEmails(ctx context.Context, lookup UserLookup, members MemberLookup, tenantID uuid.UUID, list []domain.Delegation) {
	if lookup == nil || members == nil || len(list) == 0 {
		return
	}
	idset := map[uuid.UUID]struct{}{}
	for i := range list {
		idset[list[i].DelegatorID] = struct{}{}
		idset[list[i].DelegateID] = struct{}{}
	}
	ids := make([]uuid.UUID, 0, len(idset))
	for id := range idset {
		if id == uuid.Nil {
			continue
		}
		if m, err := members.GetMember(ctx, tenantID, id); err == nil && m != nil {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return
	}
	emails, err := lookup.EmailsByIDs(ctx, ids)
	if err != nil {
		return
	}
	for i := range list {
		list[i].DelegatorEmail = emails[list[i].DelegatorID]
		list[i].DelegateEmail = emails[list[i].DelegateID]
	}
}
