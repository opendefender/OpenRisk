// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package crosstenant is the live cross-tenant suite (#1319): two real tenants
// on a booted server over Postgres, and A's token pointed at B's ids.
//
// This file and its siblings are plain data and carry no build tag, so the
// ratchet in ratchet_test.go compiles them under the unit CI. The live suite
// that runs the probes is behind the `crosstenant` tag (suite_test.go) and
// skips unless OPENRISK_BASE_URL is set.
//
// What a probe counts for is decided by Evaluate (evaluator.go): a control run
// as B must succeed, A must get 404 or 403, the status must equal A's status for
// a random id, B's object must not change, and no A body may contain B's data.
package crosstenant

import (
	"strings"
)

// Kind names a resource a probe needs from tenant B. A Kind that starts with "="
// is a literal path segment (for example "=risk" in /entities/:type/:id) and
// needs no seed.
type Kind string

const (
	KindRisk        Kind = "risk"
	KindMitigation  Kind = "mitigation"
	KindSubAction   Kind = "subaction"
	KindAsset       Kind = "asset"
	KindIncident    Kind = "incident"
	KindCustomField Kind = "customfield"
	KindSavedView   Kind = "savedview"
	KindCategory    Kind = "riskcategory"
	KindTeam        Kind = "team"
	KindRule        Kind = "rule"
	KindReport      Kind = "report"
	KindBoardReport Kind = "boardreport"
	KindTemplate    Kind = "vqtemplate"
	KindEvidence    Kind = "evidence"
	KindFramework   Kind = "framework"
	KindAudit       Kind = "audit"
	KindRemediation Kind = "remediation"
	// KindUser is a second user inside the tenant, so that a transfer-owner or a
	// team-member call has a valid in-tenant target for B's control run.
	KindUser Kind = "user"

	KindAssetPeer Kind = "assetpeer" // a second asset, the other end of a dependency
	KindAssetDep  Kind = "assetdep"
	KindControl   Kind = "control" // a framework control; also yields the framework

	KindAssessment      Kind = "assessment"  // a sent vendor assessment
	KindScoreConfig     Kind = "scoreconfig" // a scoring configuration, id chosen by the caller
	KindVendor          Kind = "vendor"      // an asset of category vendor
	KindInternalControl Kind = "internalcontrol"
	KindTenant          Kind = "tenant" // the tenant's own id (virtual: nothing is created)
	KindWorkflow        Kind = "workflow"
	KindApproval        Kind = "approval"
	KindDelegation      Kind = "delegation"
	KindMember          Kind = "member" // organisation membership of a second user
	KindInvitation      Kind = "invitation"

	// Legacy RBAC (#1518). KindRBACRole is a custom role of the tenant,
	// KindRBACUser a second user that is a member of the tenant through
	// user_tenants (KindUser alone is not: POST /users writes no such row), and
	// KindPermission a permission id taken from a predefined role. Permissions
	// are deployment-wide, so a seed of it is never recorded as tenant data.
	KindRBACRole   Kind = "rbacrole"
	KindRBACUser   Kind = "rbacuser"
	KindPermission Kind = "permission"

	// Setup variants: the same object after the state a route requires.
	KindMitigationInReview Kind = "mitigationreview" // status REVIEW, for validate
	KindSubActionDone      Kind = "subactiondone"    // completed, for revert
	KindIncidentPM         Kind = "incidentpm"       // has a post-mortem, for publish
)

// KindSpec describes how the harness reads a seeded kind back.
type KindSpec struct {
	// ReadPath is B's own GET of the object, used for the unchanged-body check.
	// {kind} placeholders are replaced with the ids of the seed. Empty means the
	// kind has no read route, and the probe records Verified=false.
	ReadPath string
	// IntID marks sequential integer ids (incidents): the random comparison id is
	// then a large integer, not a UUID.
	IntID bool
}

// Kinds is the read side of every seed kind.
var Kinds = map[Kind]KindSpec{
	KindRisk:            {ReadPath: "/api/v1/risks/{risk}"},
	KindMitigation:      {ReadPath: "/api/v1/mitigations/{mitigation}"},
	KindSubAction:       {ReadPath: "/api/v1/mitigations/{mitigation}"},
	KindAsset:           {ReadPath: "/api/v1/assets/{asset}"},
	KindIncident:        {ReadPath: "/api/v1/incidents/{incident}", IntID: true},
	KindCustomField:     {ReadPath: "/api/v1/custom-fields/{customfield}"},
	KindSavedView:       {},
	KindCategory:        {},
	KindTeam:            {ReadPath: "/api/v1/teams/{team}"},
	KindRule:            {ReadPath: "/api/v1/automation/rules/{rule}"},
	KindReport:          {ReadPath: "/api/v1/reports/{report}"},
	KindBoardReport:     {ReadPath: "/api/v1/reports/board/{boardreport}"},
	KindTemplate:        {ReadPath: "/api/v1/vendor-questionnaire-templates/{vqtemplate}"},
	KindEvidence:        {ReadPath: "/api/v1/evidence/{evidence}"},
	KindFramework:       {ReadPath: "/api/v1/compliance/frameworks/{framework}"},
	KindAudit:           {ReadPath: "/api/v1/compliance/audits/{audit}"},
	KindRemediation:     {},
	KindUser:            {},
	KindAssessment:      {ReadPath: "/api/v1/vendor-assessments/{assessment}"},
	KindScoreConfig:     {ReadPath: "/api/v1/score-engine/configs/{scoreconfig}"},
	KindVendor:          {ReadPath: "/api/v1/vendors/{vendor}/chain"},
	KindInternalControl: {ReadPath: "/api/v1/compliance/internal-controls/{internalcontrol}"},
	KindTenant:          {},
	KindWorkflow:        {},
	KindApproval:        {ReadPath: "/api/v1/governance/approvals/{approval}"},
	KindDelegation:      {},
	KindMember:          {ReadPath: "/api/v1/organization/members/{member}"},
	KindInvitation:      {},
	KindRBACRole:        {ReadPath: "/api/v1/rbac/roles/{rbacrole}"},
	KindRBACUser:        {ReadPath: "/api/v1/rbac/users/{rbacuser}"},
	// B's own read of the role the permission is attached to, so a changed
	// role_permissions set shows in the unchanged-body check.
	KindPermission: {ReadPath: "/api/v1/rbac/roles/{rbacrole}/permissions"},
	KindAssetPeer:  {ReadPath: "/api/v1/assets/{assetpeer}"},
	KindAssetDep:   {},
	KindControl:    {ReadPath: "/api/v1/compliance/controls/{control}"},

	KindMitigationInReview: {ReadPath: "/api/v1/mitigations/{mitigationreview}"},
	KindSubActionDone:      {ReadPath: "/api/v1/mitigations/{mitigation}"},
	KindIncidentPM:         {ReadPath: "/api/v1/incidents/{incidentpm}", IntID: true},
}

// SQLSeeds names every kind seeded by direct SQL rather than through the API,
// and why. The brief allows SQL only for rows that no user-facing route writes.
// Empty today: every kind in Kinds is created through its own API route.
var SQLSeeds = map[Kind]string{}

// Probe is one route under test.
type Probe struct {
	Method, Path string // exactly as routes.Extract prints it
	// Kinds are matched, in order, to the ":param" segments of Path; any left
	// over are the kinds the Body refers to as {{kind}}.
	Kinds []Kind
	// Body is a JSON template, the same for A and B; {{risk}} is replaced with
	// B's id (or a random id for the oracle run).
	Body string
	// Query is appended to the path as given, for routes that need a parameter
	// to answer at all (for example ?since=).
	Query string
	// Collection marks a GET list or aggregate with no id in its path.
	Collection bool
	// Aggregate marks a collection whose body is numbers or settings.
	Aggregate bool
	// Allow are extra pass statuses for the attack (for example 422); each needs
	// a Reason and a RegisterID.
	Allow      []int
	Reason     string
	RegisterID string
	// Own marks the "own object, foreign id in the body" shape (#1521): the path
	// params are filled with A's OWN objects of these kinds, in order, and Kinds
	// are the body kinds only, filled with B's ids. The attacker owns the target
	// and names a foreign id in the body. Such probes live in ownBodyProbes, not
	// in allProbes: the route is already probed with B's id in its path.
	Own []Kind
	// DropsForeign marks an own-object probe whose route ignores an id it cannot
	// resolve in the caller's tenant and still answers 200 (a list of links,
	// such as a risk's asset_ids). Such a probe passes on 200 only when the
	// control proves the field is applied and A's object holds nothing of B's.
	DropsForeign bool
	// LinkSQL counts the stored links from A's object to B's id, for a
	// DropsForeign probe: A's own read cannot show such a link, since that read
	// is itself filtered by tenant. Parameters: $1 A's tenant, $2 A's object id
	// (the path), $3 B's id (the body). A join table with no tenant_id is gated
	// through its parent's tenant column.
	LinkSQL string
	// BlockedBy names the issue of a functional defect that makes the route
	// fail for every caller (its own-ids control answers 5xx). While the control
	// fails the probe is skipped with that issue named; once the defect is fixed
	// the probe runs and is judged with no change here.
	BlockedBy string
}

// ID is the probe name used in failure messages and in isolation evidence.
func (p Probe) ID() string { return p.Method + " " + p.Path }

// Exemption removes one route from the ratchet, with a reason and, for a
// residual risk, the register row that carries it.
type Exemption struct {
	Method, Path string
	Reason       string
	RegisterID   string
	// NotApplicable marks a route with nothing foreign to name: it takes no id
	// in its path or body, so there is no residual risk to register.
	NotApplicable bool
}

// GroupExemption removes a whole route group from the "at least one passing
// probe" rule. The suite does not use it for a group with any probe.
type GroupExemption struct {
	Group, Reason, RegisterID string
}

// ID is the exemption name used in failure messages.
func (e Exemption) ID() string { return e.Method + " " + e.Path }

// Group is the first path segment after /api/v1.
func Group(path string) string {
	rest := strings.TrimPrefix(path, "/api/v1/")
	if rest == path {
		return strings.TrimPrefix(path, "/")
	}
	if i := strings.IndexByte(rest, '/'); i >= 0 {
		return rest[:i]
	}
	return rest
}

// ParamCount is the number of ":param" segments in a path.
func ParamCount(path string) int {
	n := 0
	for _, s := range strings.Split(path, "/") {
		if strings.HasPrefix(s, ":") {
			n++
		}
	}
	return n
}

// IsLiteral reports whether k is a literal path segment rather than a seeded id.
func IsLiteral(k Kind) bool { return strings.HasPrefix(string(k), "=") }

// get, del, post and so on build the probe rows below, one line per route.
func get(path string, kinds ...Kind) Probe { return Probe{Method: "GET", Path: path, Kinds: kinds} }
func del(path string, kinds ...Kind) Probe { return Probe{Method: "DELETE", Path: path, Kinds: kinds} }
func post(path, body string, kinds ...Kind) Probe {
	return Probe{Method: "POST", Path: path, Body: body, Kinds: kinds}
}
func delWithBody(path, body string, kinds ...Kind) Probe {
	return Probe{Method: "DELETE", Path: path, Body: body, Kinds: kinds}
}
func put(path, body string, kinds ...Kind) Probe {
	return Probe{Method: "PUT", Path: path, Body: body, Kinds: kinds}
}
func patch(path, body string, kinds ...Kind) Probe {
	return Probe{Method: "PATCH", Path: path, Body: body, Kinds: kinds}
}

// list is a collection GET whose control must show one of kinds in B's body.
func list(path string, kinds ...Kind) Probe {
	return Probe{Method: "GET", Path: path, Kinds: kinds, Collection: true}
}

// agg is a collection GET that returns numbers or settings.
func agg(path string) Probe {
	return Probe{Method: "GET", Path: path, Collection: true, Aggregate: true}
}

// own turns a probe into the own-object shape: the path takes A's objects of
// kinds, the body takes B's.
func own(p Probe, kinds ...Kind) Probe { p.Own = kinds; return p }

// dropsForeign marks an own-object probe whose route drops a foreign id silently.
func dropsForeign(p Probe, linkSQL string) Probe {
	p.DropsForeign, p.LinkSQL = true, linkSQL
	return p
}

// blockedBy records the issue that keeps a probe's own-ids control failing.
func blockedBy(p Probe, issue string) Probe { p.BlockedBy = issue; return p }

func q(p Probe, query string) Probe { p.Query = query; return p }

func allow(p Probe, reason, rr string, statuses ...int) Probe {
	p.Allow, p.Reason, p.RegisterID = statuses, reason, rr
	return p
}

// allProbes is the whole table. It is built from the per-group files.
func allProbes() []Probe {
	var out []Probe
	out = append(out, riskProbes...)
	out = append(out, assetProbes...)
	out = append(out, workflowProbes...)
	out = append(out, collectionProbes...)
	out = append(out, orgProbes...)
	out = append(out, bodyProbes...)
	out = append(out, rbacProbes...)
	return out
}
