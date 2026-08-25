// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"time"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// OR26-03 — deferrable MFA enrolment.
//
// THE PROBLEM THIS FILE FIXES: login answered `mfa_enrollment_required: true`
// the moment a privileged account had no authenticator, and that answer carried
// no session. A brand-new evaluator therefore met a QR code before they had seen
// a single screen of the product — a wall in front of the value, not a gate in
// front of a risk.
//
// The fix is a POLICY, not a bypass. Three inputs decide, in one place:
//
//	MFA state (enrolled?) + privilege (does this role mandate MFA?) + elapsed
//	time (has the grace period run out?)  =>  is enrolment mandatory right now?
//
// Everything else — the login use case, the request-time guard, the session
// contract, the banner — reads DecideMFA and renders what it says. Scattering
// `if isAdmin` across layers is exactly how the frontend and the backend end up
// disagreeing about who is allowed in, so there is deliberately one function.
// ---------------------------------------------------------------------------

// MFAGraceDaysDefault is the shipped default: a privileged account has a week to
// enrol before the product stops letting it in.
//
// Seven days is long enough to cover a holiday weekend and short enough that a
// privileged account cannot quietly live without a second factor.
const MFAGraceDaysDefault = 7

// MFAGraceDaysMin / MFAGraceDaysMax bound what an administrator may configure.
//
//	0  — enrolment is mandatory immediately (the pre-OR26-03 behaviour, still
//	     available to deployments that want it).
//	90 — the ceiling. An unbounded value would let "configure the policy" become
//	     "switch the policy off", which is the one thing the configuration must
//	     not be able to express.
const (
	MFAGraceDaysMin = 0
	MFAGraceDaysMax = 90
)

// MFAPolicy is the tenant-scoped configuration behind the decision.
//
// One row per tenant. Absent row = defaults, so a tenant that has never opened
// the setting behaves exactly like one that saved the defaults.
type MFAPolicy struct {
	// No `default:gen_random_uuid()`: it is Postgres-only and makes AutoMigrate
	// unusable against the sqlite the repository tests run on. Writers assign it.
	ID       uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	TenantID uuid.UUID `gorm:"type:uuid;not null;uniqueIndex" json:"tenant_id"`
	// GraceDays is how long a privileged member may hold a session before an
	// authenticator becomes mandatory. Bounded by MFAGraceDaysMin/Max.
	//
	// NO `default:` tag, deliberately. GORM omits a zero-valued field on INSERT
	// when the column declares a default, so `default:7` made it impossible to
	// save 0 — the strictest setting, "require MFA immediately", silently became
	// the most permissive one. The DB-level default still exists in migration
	// 0060 for raw inserts; every write from Go carries an explicit value, and
	// callers that have none use DefaultMFAPolicy.
	GraceDays int `gorm:"not null" json:"grace_days"`
	// UpdatedByID is the administrator who last saved it. The full who/when/
	// before/after record lives in the governance audit trail; this is the
	// convenience field the settings screen shows.
	UpdatedByID *uuid.UUID `gorm:"type:uuid" json:"updated_by_id,omitempty"`
	CreatedAt   time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName pins the table name.
func (MFAPolicy) TableName() string { return "mfa_policies" }

// The Auditable opt-in lives in governance_auditable.go with every other
// entity's, so one file answers "what is journalled?" — see the plugin in
// internal/infrastructure/audittrail.

// DefaultMFAPolicy is what a tenant with no saved row behaves as.
func DefaultMFAPolicy(tenantID uuid.UUID) MFAPolicy {
	return MFAPolicy{TenantID: tenantID, GraceDays: MFAGraceDaysDefault}
}

// Validate enforces the bounds. Returning a typed ValidationError keeps the
// handler free of policy knowledge.
func (p MFAPolicy) Validate() error {
	if p.GraceDays < MFAGraceDaysMin || p.GraceDays > MFAGraceDaysMax {
		return NewValidationError("grace_days must be between 0 and 90 days")
	}
	return nil
}

// EffectiveGraceDays clamps a stored value into the legal range.
//
// A row written before the bounds existed — or by a direct SQL edit — must not
// be able to express "never require MFA". Clamping here means the decision
// function cannot be handed a value the policy would refuse to accept.
func (p MFAPolicy) EffectiveGraceDays() int {
	switch {
	case p.GraceDays < MFAGraceDaysMin:
		return MFAGraceDaysMin
	case p.GraceDays > MFAGraceDaysMax:
		return MFAGraceDaysMax
	default:
		return p.GraceDays
	}
}

// ---------------------------------------------------------------------------
// Privilege
// ---------------------------------------------------------------------------

// MFAPrivilegeSet names the roles for which MFA is not negotiable.
//
// It is split in two because OpenRisk has two role vocabularies that both grant
// real power (see business_roles.go): the ORG role (root/admin — the wildcard
// "*" permission, tenant administration, API token minting) and the BUSINESS
// role preset (rssi — the security officer, who reads every register and drives
// the security posture). The issue names "Admin/RSSI"; this is where that
// sentence becomes code.
type MFAPrivilegeSet struct {
	OrgRoles      map[string]bool
	BusinessRoles map[string]bool
}

// DefaultMFAPrivilegeRoles is the shipped privilege set: root + admin org roles,
// rssi business role. A deployment can widen or narrow it (MFA_REQUIRED_ROLES /
// MFA_REQUIRED_BUSINESS_ROLES) — that is a deployment decision, not a tenant one,
// because it decides who is privileged, not how long they get.
func DefaultMFAPrivilegeRoles() ([]string, []string) {
	return []string{string(RoleRoot), string(RoleAdmin)}, []string{string(BusinessRoleRSSI)}
}

// NewMFAPrivilegeSet builds a set from role name lists, normalising case.
func NewMFAPrivilegeSet(orgRoles, businessRoles []string) MFAPrivilegeSet {
	s := MFAPrivilegeSet{
		OrgRoles:      make(map[string]bool, len(orgRoles)),
		BusinessRoles: make(map[string]bool, len(businessRoles)),
	}
	for _, r := range orgRoles {
		if r = normaliseRoleKey(r); r != "" {
			s.OrgRoles[r] = true
		}
	}
	for _, r := range businessRoles {
		if r = normaliseRoleKey(r); r != "" {
			s.BusinessRoles[r] = true
		}
	}
	return s
}

// Empty reports whether the set names nobody — i.e. mandatory MFA is switched
// off for this deployment entirely.
func (s MFAPrivilegeSet) Empty() bool {
	return len(s.OrgRoles) == 0 && len(s.BusinessRoles) == 0
}

// Includes reports whether a member holding these roles is privileged.
func (s MFAPrivilegeSet) Includes(orgRole MemberRole, businessRole BusinessRoleKey) bool {
	if s.OrgRoles[normaliseRoleKey(string(orgRole))] {
		return true
	}
	return s.BusinessRoles[normaliseRoleKey(string(businessRole))]
}

func normaliseRoleKey(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z':
			out = append(out, r+('a'-'A'))
		case r == ' ' || r == '\t' || r == '\n' || r == '\r':
			// trimmed
		default:
			out = append(out, r)
		}
	}
	return string(out)
}

// ---------------------------------------------------------------------------
// The decision
// ---------------------------------------------------------------------------

// MFARequirementState is the resolved state, as a closed vocabulary.
//
// A single boolean was not enough: "not configured" covers both a member who is
// merely being encouraged and a privileged account three days from lockout, and
// the UI has to be able to tell those apart without re-deriving the rule.
type MFARequirementState string

const (
	// MFAStateConfigured — an authenticator is enrolled and verified. Nothing to
	// prompt, nothing to enforce.
	MFAStateConfigured MFARequirementState = "configured"
	// MFAStateRecommended — not enrolled, not privileged. Strongly encouraged,
	// never enforced. This is the state that unblocks the evaluation journey.
	MFAStateRecommended MFARequirementState = "recommended"
	// MFAStateGraceActive — privileged, not enrolled, still inside the window.
	// Access is allowed and the deadline is shown.
	MFAStateGraceActive MFARequirementState = "grace_active"
	// MFAStateGraceExpiring — same as above, inside the final stretch. A separate
	// state so the banner can escalate without the client inventing a threshold.
	MFAStateGraceExpiring MFARequirementState = "grace_expiring"
	// MFAStateRequired — enrolment is mandatory now. The server refuses protected
	// access until an authenticator exists.
	MFAStateRequired MFARequirementState = "required"
)

// MFAGraceExpiringWithin is how close to the deadline the state escalates from
// grace_active to grace_expiring.
const MFAGraceExpiringWithin = 48 * time.Hour

// MFADecisionInput is everything the decision needs. All of it is server-side
// fact: nothing here can be supplied by the client.
type MFADecisionInput struct {
	// Enrolled is true only for a VERIFIED authenticator. A half-finished
	// enrolment (secret generated, code never confirmed) is not protection.
	Enrolled bool
	// Privileged is the output of MFAPrivilegeSet.Includes for this member.
	Privileged bool
	// GraceStartedAt anchors the countdown: when this member became subject to
	// the requirement (membership start, or promotion into a privileged role).
	// Zero means "unknown" — see the decision rules.
	GraceStartedAt time.Time
	// GraceDays comes from the tenant policy, already clamped.
	GraceDays int
	Now       time.Time
}

// MFADecision is the resolved answer, and the shape the session contract ships.
type MFADecision struct {
	State MFARequirementState `json:"state"`
	// Configured mirrors State == configured, kept as a field because it is the
	// single most-read fact and a string compare in a template is worse.
	Configured bool `json:"configured"`
	// Required is the enforcement bit: when true, the server refuses protected
	// access until an authenticator is enrolled.
	Required bool `json:"required"`
	// Privileged says the requirement applies to this member at all.
	Privileged bool `json:"privileged"`
	// GraceActive is true while a privileged member may still defer.
	GraceActive bool `json:"grace_period_active"`
	// Deadline is when deferral ends. nil for a member the requirement does not
	// apply to, and for an already-enrolled one.
	Deadline *time.Time `json:"deadline,omitempty"`
	// GraceDays is echoed so the UI can say "your organization allows N days"
	// without a second call.
	GraceDays int `json:"grace_days"`
}

// DecideMFA is the single source of truth for "must this member enrol now?".
//
// The rules, in the order a reader would ask them:
//
//  1. Enrolled → done. Nothing to prompt, nothing to enforce.
//  2. Not privileged → recommended, never required. THIS is the OR26-03 change:
//     an ordinary member reaches the dashboard, the onboarding wizard and their
//     first risk with a banner, not a wall.
//  3. Privileged with a zero-day grace → required immediately.
//  4. Privileged with an unknown anchor → required. An unresolvable guard fails
//     CLOSED: a missing timestamp must not read as "infinite grace", because
//     that is precisely the state an attacker would try to produce.
//  5. Privileged inside the window → allowed, with a deadline. Escalates to
//     grace_expiring in the final MFAGraceExpiringWithin.
//  6. Privileged past the window → required.
func DecideMFA(in MFADecisionInput) MFADecision {
	if in.Enrolled {
		return MFADecision{
			State:      MFAStateConfigured,
			Configured: true,
			Privileged: in.Privileged,
			GraceDays:  in.GraceDays,
		}
	}

	if !in.Privileged {
		return MFADecision{
			State:     MFAStateRecommended,
			GraceDays: in.GraceDays,
		}
	}

	if in.GraceDays <= 0 {
		return MFADecision{
			State:      MFAStateRequired,
			Required:   true,
			Privileged: true,
			GraceDays:  in.GraceDays,
		}
	}

	if in.GraceStartedAt.IsZero() {
		// Fail closed. See rule 4.
		return MFADecision{
			State:      MFAStateRequired,
			Required:   true,
			Privileged: true,
			GraceDays:  in.GraceDays,
		}
	}

	deadline := in.GraceStartedAt.Add(time.Duration(in.GraceDays) * 24 * time.Hour)
	if !in.Now.Before(deadline) {
		return MFADecision{
			State:      MFAStateRequired,
			Required:   true,
			Privileged: true,
			Deadline:   &deadline,
			GraceDays:  in.GraceDays,
		}
	}

	state := MFAStateGraceActive
	if deadline.Sub(in.Now) <= MFAGraceExpiringWithin {
		state = MFAStateGraceExpiring
	}
	return MFADecision{
		State:       state,
		Privileged:  true,
		GraceActive: true,
		Deadline:    &deadline,
		GraceDays:   in.GraceDays,
	}
}
