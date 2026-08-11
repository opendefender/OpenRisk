// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package isolation holds the tenant-isolation coverage gate.
//
// Audit finding F-07. Tenant isolation is the existential risk of a multi-tenant
// GRC platform, and the project has already shipped six real cross-tenant leaks
// (fixed 2026-07-23). Every one of them was found by a human reading code, not
// by CI. The gate here changes that growth pattern: the route surface is derived
// from the router by AST, and any parameterised route with no recorded isolation
// decision fails the build.
//
// The gate deliberately does NOT assert that isolation works — it asserts that
// somebody has decided, in writing, how each route is isolated. That is a weaker
// claim than an end-to-end probe and it is stated as such, but it is the part
// that closes automatically over new code.
//
// Known blind spot: the gate tracks routes with an ID in the *path*, because
// that is the surface where a caller names someone else's resource. A route that
// takes the target id (or worse, the tenant) in the request *body* is an equally
// good IDOR surface and is invisible here. That is not hypothetical —
// POST /scanner/mitigations/auto-complete read its tenant from the payload and
// let any authenticated user complete another tenant's sub-action; it is
// unparameterised, so this gate never asked about it. It is now covered by
// handler/mitigation_autocomplete_isolation_test. Detecting the shape statically
// would mean parsing each handler's body struct, which is a larger change than
// the gate warrants; until then, body-addressed writes need a probe by hand.
package isolation

import "strings"

// Status records how a route's tenant isolation is accounted for.
type Status string

const (
	// Covered: an automated test asserts cross-tenant access is refused.
	Covered Status = "covered"

	// PublicByDesign: no tenant scoping applies. Unauthenticated or
	// pre-authentication surface.
	PublicByDesign Status = "public-by-design"

	// MachineAuthenticated: authenticated by a non-user credential (scanner agent
	// token, webhook secret) whose scope is the credential itself, not a tenant
	// session.
	MachineAuthenticated Status = "machine-authenticated"

	// SelfScoped: operates only on the caller's own identity, taken from the
	// signed token rather than from a path parameter.
	SelfScoped Status = "self-scoped"

	// Unreachable: route exists but cannot execute — its tables are deliberately
	// absent from AutoMigrate, so every call fails before reaching a query.
	Unreachable Status = "unreachable"

	// Pending: a known gap. Visible debt, not a silent hole.
	Pending Status = "pending"
)

// Decision is the recorded reasoning for one route pattern.
type Decision struct {
	// Pattern matches route paths with ":param" segments normalised to "{id}".
	// A trailing "/*" makes it a prefix match covering a module's subtree.
	Pattern string
	Status  Status
	// Evidence names the test or states the justification. Required — a decision
	// without a reason is indistinguishable from a guess.
	Evidence string
}

// decisions is the authoritative record.
//
// Adding a parameterised route without adding an entry here fails
// TestEveryParameterisedRouteHasAnIsolationDecision. That is the point: it
// converts "somebody must remember to test the new endpoint" into a build error.
var decisions = []Decision{
	// --- Public / pre-authentication -------------------------------------
	{"/api/v1/auth/oauth2/login/{id}", PublicByDesign,
		"provider name in path, no tenant resource touched; runs before a session exists"},
	{"/api/v1/auth/oauth2/callback/{id}", PublicByDesign,
		"provider name in path; issues the session rather than reading tenant data"},

	// --- Machine identities ----------------------------------------------
	{"/api/v1/vulnerabilities/webhook/{id}", MachineAuthenticated,
		"opaque per-integration webhook token resolves the tenant; the token IS the credential"},
	{"/api/v1/scanner/*", MachineAuthenticated,
		"agent-scoped token plus HMAC on push; tenant derives from the enrolled agent, not the caller"},

	// --- Caller's own identity -------------------------------------------
	{"/api/v1/auth/pat/{id}", SelfScoped,
		"PAT CRUD is scoped to the authenticated owner via claims, not the path"},
	{"/api/v1/auth/sessions/{id}", SelfScoped,
		"session revocation is scoped to the authenticated owner: GormSessionRepository.Revoke " +
			"filters id AND user_id, so a foreign session id affects 0 rows and reads back as " +
			"not-found rather than forbidden (covered by TestRevoke_CrossUser_IsNotFound)"},

	// --- Covered by automated isolation tests -----------------------------
	{"/api/v1/assets/{id}", Covered,
		"application/asset get/update/delete cross-tenant tests + gorm_asset_repository_test"},
	{"/api/v1/assets/{id}/history", Covered,
		"application/asset list_asset_snapshots_test asserts tenant scoping"},
	{"/api/v1/asset-dependencies/{id}", Covered,
		"gorm_asset_dependency_repository_test: cross-tenant GetByID returns nil"},
	{"/api/v1/compliance/*", Covered,
		"application/compliance controls+evidences tests, gorm_compliance_repository_test, gorm_compliance_audit_repository_test, gorm_control_mapping_repository_test"},
	{"/api/v1/reports/jobs/{id}", Covered,
		"gorm_report_job_repository_test: cross-tenant GetByID is ErrNotFound, List is tenant-scoped, Update cannot cross tenants; application/reportjob TestGet_CrossTenant_IsNotFound"},
	{"/api/v1/reports/jobs/{id}/download", Covered,
		"same repository path as the job read — Download resolves the job through GetByID before serving any bytes, so a foreign id 404s before the artifact is touched"},
	{"/api/v1/incidents/*", Covered,
		"handler/incident_handler_test TestIncident_CrossTenant_TimelineAndActions (regression for the July IDOR)"},
	{"/api/v1/risks/{id}/timeline", Covered,
		"service/risk_timeline_service_test TestRiskTimeline_CrossTenant"},
	{"/api/v1/risks/{id}/timeline/*", Covered,
		"same ownsRisk gate as the parent timeline route; TestRiskTimeline_CrossTenant and _RecentChanges_ScopedToTenant cover the subpaths"},
	{"/api/v1/custom-fields/*", Covered,
		"service/custom_field_service_test TestCustomField_TenantIsolation"},
	{"/api/v1/teams/{id}", Covered,
		"handler/team_isolation_test TestTeam_TenantIsolationPredicate"},
	{"/api/v1/teams/*", Covered,
		"handler/team_isolation_test covers member add/remove paths"},
	{"/api/v1/users/{id}", Covered,
		"handler/user_isolation_test TestUser_TenantScoping"},
	{"/api/v1/users/*", Covered,
		"handler/user_isolation_test covers status/role/delete by id"},
	{"/api/v1/notifications/{id}", Covered,
		"repository/notification_repository_test scopes by user and tenant"},
	{"/api/v1/notifications/{id}/read", Covered,
		"repository/notification_repository_test scopes by user and tenant"},
	{"/api/v1/vulnerabilities/{id}", Covered,
		"application/vulnerability integrations_test: cross-tenant lookup returns 404"},
	{"/api/v1/vulnerabilities/*", Covered,
		"application/vulnerability integrations_test covers status/ticket subpaths"},
	{"/api/v1/rbac/members/{id}/business-role", Covered,
		"application/rbac business_role_usecases_test: update scoped to id+org"},
	{"/api/v1/onboarding/steps/{id}", Covered,
		"the :step param is a wizard step NAME from a closed vocabulary (domain.ParseOnboardingStep rejects anything else), never an entity id — there is no id in this path to forge. The row it writes is keyed by the caller's own (tenant, user) from the request context: gorm_activation_repository_test TestOnboardingProgress_MissingAndIsolated asserts a cross-tenant Get reads back nil, and application/activation TestWizard_OrganizationWriteRequiresPermission asserts a non-admin cannot write the organization through it"},

	// --- Unreachable ------------------------------------------------------
	{"/api/v1/marketplace/*", Unreachable,
		"Connector/MarketplaceApp carry no gorm tags and are excluded from AutoMigrate (main.go), so the tables do not exist and every call fails before querying. Documented latent IDOR — must be resolved before the module is revived"},

	// --- Known gaps -------------------------------------------------------
	// These execute against real tenant data and rely on repository-level
	// scoping that no test currently pins end to end.
	{"/api/v1/risks/{id}", Covered,
		"handler/risk_isolation_test drives read/update/transition/review/delete from tenant A at tenant B's risk over the real handler+repository chain; validated by sabotage"},
	{"/api/v1/risks/{id}/transition", Covered,
		"handler/risk_isolation_test TestRiskIsolation_CrossTenantAccessIsRefused/transition_phase"},
	{"/api/v1/risks/{id}/review", Covered,
		"handler/risk_isolation_test TestRiskIsolation_CrossTenantAccessIsRefused/mark_reviewed"},
	{"/api/v1/risks/{id}/transitions", Covered,
		"application/risk transition_state_test TestAvailableTransitions_NotFound: an id outside the tenant reads back as not-found through the same GetByID(tenant) path"},
	// The controlled category vocabulary. Both write routes go through
	// GormRiskCategoryRepository, which puts tenant_id in the WHERE clause of the
	// UPDATE and of the DELETE (and of the risk detach inside the same
	// transaction), so a forged id from another organisation affects 0 rows and
	// surfaces as not-found.
	{"/api/v1/risk-categories/{id}", Covered,
		"repository/gorm_risk_taxonomy_repository_test TestRiskCategoryRepo_CrossTenantUpdateAndDeleteRefused"},
	{"/api/v1/risks/*", Pending,
		"financial/smart-score/simulate/mitigations subpaths lack cross-tenant assertions"},
	{"/api/v1/mitigations/*", Pending,
		"sub-action paths verify ownership through the parent plan; not pinned by a test"},
	{"/api/v1/governance/*", Pending,
		"approvals/workflows/delegations are tenant-scoped in use cases; no cross-tenant test"},
	{"/api/v1/automation/rules/*", Pending,
		"ListEnabledByTrigger is tenant-scoped; per-rule routes lack cross-tenant assertions"},
	{"/api/v1/reports/board/{id}", Pending,
		"GormBoardReportRepository is tenant-scoped; not pinned by a test"},
	{"/api/v1/reports/board/*", Pending,
		"approve/pdf subpaths lack cross-tenant assertions"},
	{"/api/v1/ai/*", Pending,
		"AI use cases read through tenant-scoped repos; no cross-tenant assertion on the id paths"},
	{"/api/v1/audit-logs/*", Pending,
		"admin-only; tenant scoping not pinned by a test"},
	{"/api/v1/rbac/*", Pending,
		"admin-only tenant/user/role admin paths; GetTenantUsers gates on level, not pinned"},
	{"/api/v1/tokens/*", Pending,
		"API token management; ownership checks not pinned by a test"},
	{"/api/v1/bulk-operations/{id}", Pending,
		"service filters by tenant (see bulk_operation_service); not pinned by a test"},
	{"/api/v1/integrations/{id}/test", Pending,
		"integration config is tenant-scoped; not pinned by a test"},
	{"/api/v1/cti/vulnerabilities/{id}", PublicByDesign,
		"CTI feed data (NVD/CISA) is global threat intelligence, not tenant-owned"},
	{"/api/v1/score-engine/*", Covered,
		"handler test TestScoreEngine_AssetLoadTenantScoped (regression for the July fail-open)"},
}

// Normalise rewrites a concrete route path into registry pattern form, so
// ":id", ":riskId" and ":userId" all collapse onto "{id}".
func Normalise(path string) string {
	segments := strings.Split(path, "/")
	for i, segment := range segments {
		if strings.HasPrefix(segment, ":") {
			segments[i] = "{id}"
		}
	}
	return strings.Join(segments, "/")
}

// Lookup finds the decision for a route path, preferring the most specific
// match so a precise entry always beats a module-wide prefix.
func Lookup(path string) (Decision, bool) {
	normalised := Normalise(path)

	var best Decision
	var found bool
	for _, d := range decisions {
		if !matches(d.Pattern, normalised) {
			continue
		}
		if !found || len(d.Pattern) > len(best.Pattern) {
			best, found = d, true
		}
	}
	return best, found
}

func matches(pattern, path string) bool {
	if prefix, isWildcard := strings.CutSuffix(pattern, "/*"); isWildcard {
		return path == prefix || strings.HasPrefix(path, prefix+"/")
	}
	return pattern == path
}

// Decisions exposes the registry for reporting.
func Decisions() []Decision {
	out := make([]Decision, len(decisions))
	copy(out, decisions)
	return out
}
