// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
)

// ---------------------------------------------------------------------------
// OR26-03 — resolving "must this member enrol now?" outside the login path.
//
// Login answers the question once, at the door. Everything AFTER the door needs
// the same answer: /auth/me (so the banner knows what to say) and the
// request-time guard (so a session minted on day one cannot still be working on
// day nine). One resolver serves both, which is what stops the two from ever
// disagreeing about who is allowed in.
// ---------------------------------------------------------------------------

// MFAMemberLookup resolves a membership. Narrow on purpose — the resolver reads,
// it never writes.
type MFAMemberLookup interface {
	GetOrganizationMember(ctx context.Context, userID, orgID uuid.UUID) (*domain.OrganizationMember, error)
}

// mfaStatusCacheTTL is how long a resolved decision is reused.
//
// The decision changes on three events — enrolment completes, the tenant policy
// changes, a role changes — and the first two invalidate this cache explicitly.
// The TTL is the backstop for the third and for anything that writes the tables
// directly. Sixty seconds keeps a guard on every authenticated request down to
// roughly one lookup per user per minute instead of three queries per request,
// while bounding how long a stale "still in grace" can survive a change nobody
// told the cache about.
const mfaStatusCacheTTL = 60 * time.Second

type mfaStatusCacheKey struct {
	user   uuid.UUID
	tenant uuid.UUID
}

type mfaStatusCacheEntry struct {
	decision  domain.MFADecision
	expiresAt time.Time
}

// MFAStatusResolver answers the requirement question for an authenticated
// caller, with a short-lived per-(user, tenant) cache.
//
// The cache key includes the tenant, so a user who belongs to two organizations
// gets two answers — a shared entry would let one tenant's policy decide another
// tenant's access.
type MFAStatusResolver struct {
	mfaRepo    repository.MFARepository
	members    MFAMemberLookup
	policies   MFAPolicyReader
	privileged domain.MFAPrivilegeSet
	now        func() time.Time

	mu    sync.Mutex
	cache map[mfaStatusCacheKey]mfaStatusCacheEntry
}

// NewMFAStatusResolver builds the resolver. mfaRepo and members are required;
// policies is optional and nil-safe (the tenant then behaves as defaults).
func NewMFAStatusResolver(mfaRepo repository.MFARepository, members MFAMemberLookup, orgRoles, businessRoles []string) *MFAStatusResolver {
	return &MFAStatusResolver{
		mfaRepo:    mfaRepo,
		members:    members,
		privileged: domain.NewMFAPrivilegeSet(orgRoles, businessRoles),
		cache:      make(map[mfaStatusCacheKey]mfaStatusCacheEntry),
	}
}

// WithPolicies wires the tenant grace-window reader.
func (r *MFAStatusResolver) WithPolicies(p MFAPolicyReader) *MFAStatusResolver {
	r.policies = p
	return r
}

// WithClock overrides the clock, for tests.
func (r *MFAStatusResolver) WithClock(f func() time.Time) *MFAStatusResolver {
	r.now = f
	return r
}

func (r *MFAStatusResolver) clock() time.Time {
	if r.now != nil {
		return r.now()
	}
	return time.Now()
}

// Invalidate drops the cached decision for one member.
//
// Called the moment enrolment is verified: without it a user who has just
// scanned the QR code would keep being told to enrol for up to a minute, which
// reads as "it didn't work" and produces a second, conflicting authenticator.
func (r *MFAStatusResolver) Invalidate(userID, tenantID uuid.UUID) {
	if r == nil {
		return
	}
	r.mu.Lock()
	delete(r.cache, mfaStatusCacheKey{user: userID, tenant: tenantID})
	r.mu.Unlock()
}

// InvalidateTenant drops every cached decision for one tenant.
//
// Called when the policy changes: shortening the window has to bite now, not a
// minute from now, or an administrator who just set "0 days" would watch their
// colleagues keep working and conclude the setting is decorative.
func (r *MFAStatusResolver) InvalidateTenant(tenantID uuid.UUID) {
	if r == nil {
		return
	}
	r.mu.Lock()
	for k := range r.cache {
		if k.tenant == tenantID {
			delete(r.cache, k)
		}
	}
	r.mu.Unlock()
}

// Resolve returns the MFA requirement for one authenticated caller.
//
// orgRoleHint is the org role carried by the caller's signed token. It is used
// ONLY to widen the privileged set, never to narrow it: a token cannot talk its
// way out of the requirement, but a membership row that has gone missing cannot
// silently exempt an administrator either.
//
// An error return means "could not determine". Callers must treat that as
// blocking rather than permissive — see MFAPolicyGuard.
func (r *MFAStatusResolver) Resolve(ctx context.Context, userID, tenantID uuid.UUID, orgRoleHint string) (domain.MFADecision, error) {
	if r == nil || r.mfaRepo == nil {
		// Nothing wired: nothing can be enrolled and nothing can be enforced.
		return domain.DecideMFA(domain.MFADecisionInput{GraceDays: domain.MFAGraceDaysDefault, Now: time.Now()}), nil
	}
	if userID == uuid.Nil || tenantID == uuid.Nil {
		return domain.MFADecision{}, domain.NewUnauthorizedError("authentication required")
	}

	key := mfaStatusCacheKey{user: userID, tenant: tenantID}
	now := r.clock()

	r.mu.Lock()
	if entry, ok := r.cache[key]; ok && now.Before(entry.expiresAt) {
		r.mu.Unlock()
		return entry.decision, nil
	}
	r.mu.Unlock()

	secret, err := r.mfaRepo.GetMFASecret(ctx, userID, tenantID)
	if err != nil {
		return domain.MFADecision{}, err
	}
	enrolled := secret != nil && secret.IsVerified

	var member *domain.OrganizationMember
	if r.members != nil {
		member, err = r.members.GetOrganizationMember(ctx, userID, tenantID)
		if err != nil {
			return domain.MFADecision{}, err
		}
	}

	policy := domain.DefaultMFAPolicy(tenantID)
	if r.policies != nil {
		p, pErr := r.policies.GetMFAPolicy(ctx, tenantID)
		if pErr != nil {
			// Same rule as login: a failed policy read falls back to the shipped
			// default window, never to "no requirement".
			p = nil
		}
		if p != nil {
			policy = *p
		}
	}

	in := domain.MFADecisionInput{
		Enrolled:  enrolled,
		GraceDays: policy.EffectiveGraceDays(),
		Now:       now,
	}
	if !r.privileged.Empty() {
		if member != nil {
			in.Privileged = r.privileged.Includes(member.Role, member.BusinessRole)
			in.GraceStartedAt = member.MFAGraceAnchor()
		}
		if !in.Privileged && orgRoleHint != "" {
			// Widen only. A missing membership row must not exempt an admin whose
			// signed token says they are one; the anchor stays zero, which
			// DecideMFA reads as fail-closed.
			in.Privileged = r.privileged.Includes(domain.MemberRole(orgRoleHint), "")
		}
	}

	decision := domain.DecideMFA(in)

	r.mu.Lock()
	r.cache[key] = mfaStatusCacheEntry{decision: decision, expiresAt: now.Add(mfaStatusCacheTTL)}
	r.mu.Unlock()

	return decision, nil
}
