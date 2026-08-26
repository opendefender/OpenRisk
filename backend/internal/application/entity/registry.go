// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package entity

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// Descriptor is the static contract of one entity type: what it is called, what
// permission gates it, which sections its drawer offers, and where its list
// lives.
//
// The descriptor table below is the single place that answers "may this user see
// this type?" — the HTTP layer, the relation filter and the tenant timeline all
// read it, so a type cannot be gated one way in the drawer and another way in a
// relation list.
type Descriptor struct {
	Type Type
	// Label is the English type name shown in the drawer header.
	Label string
	// ReadPermission is the permission string the caller must hold. It is the
	// SAME string the module's own routes require — deliberately, so the drawer
	// can never become a way around a module's gate.
	ReadPermission string
	// ListPath is the in-app route whose page hosts this type's drawer.
	ListPath string
	Sections []Section
}

// descriptors is the type table. Adding an entity type to the drawer is adding
// a row here plus a Resolver; nothing else in this package is type-aware.
var descriptors = map[Type]Descriptor{
	TypeAsset: {
		Type: TypeAsset, Label: "Asset",
		ReadPermission: "assets:read", ListPath: "/assets",
		Sections: []Section{SectionSummary, SectionRelations, SectionTimeline, SectionAudit},
	},
	TypeRisk: {
		Type: TypeRisk, Label: "Risk",
		ReadPermission: "risks:read", ListPath: "/risks",
		Sections: []Section{SectionSummary, SectionRelations, SectionTimeline, SectionAudit},
	},
	TypeVulnerability: {
		Type: TypeVulnerability, Label: "Vulnerability",
		ReadPermission: "vulnerabilities:read", ListPath: "/vulnerabilities",
		Sections: []Section{SectionSummary, SectionRelations, SectionTimeline, SectionAudit},
	},
	TypeFinding: {
		// A finding is a scanner-sourced vulnerability, so it is gated by the
		// vulnerability permission. Giving it a permission of its own would
		// invent an authorisation concept the product does not have.
		Type: TypeFinding, Label: "Finding",
		ReadPermission: "vulnerabilities:read", ListPath: "/vulnerabilities",
		Sections: []Section{SectionSummary, SectionRelations, SectionTimeline, SectionAudit},
	},
	TypeControl: {
		Type: TypeControl, Label: "Control",
		ReadPermission: "compliance:controls:read", ListPath: "/compliance",
		Sections: []Section{SectionSummary, SectionRelations, SectionTimeline, SectionAudit},
	},
	TypeIncident: {
		Type: TypeIncident, Label: "Incident",
		ReadPermission: "incidents:read", ListPath: "/incidents",
		Sections: []Section{SectionSummary, SectionRelations, SectionTimeline, SectionAudit},
	},
	TypeVendor: {
		// A vendor is an asset of category "vendor" — same table, same gate.
		Type: TypeVendor, Label: "Vendor",
		ReadPermission: "assets:read", ListPath: "/assets",
		Sections: []Section{SectionSummary, SectionRelations, SectionTimeline, SectionAudit},
	},
	TypeEvidence: {
		Type: TypeEvidence, Label: "Evidence",
		ReadPermission: "compliance:evidences:read", ListPath: "/evidence",
		Sections: []Section{SectionSummary, SectionRelations, SectionTimeline, SectionAudit},
	},
}

// DescriptorFor returns the static contract of a type.
func DescriptorFor(t Type) (Descriptor, bool) {
	d, ok := descriptors[t]
	return d, ok
}

// Descriptors returns every descriptor in declaration order — the catalogue the
// client uses to know which types it may address at all.
func Descriptors() []Descriptor {
	out := make([]Descriptor, 0, len(Types))
	for _, t := range Types {
		out = append(out, descriptors[t])
	}
	return out
}

// DeepLink is the canonical URL that opens one entity's drawer.
//
// It is built here, on the server, for one reason: the timeline and the relation
// lists both hand the client links to entities of types they know nothing about,
// and a link assembled independently in three places drifts. The query-parameter
// shape is the drawer state model documented in
// docs/W1-02_UNIVERSAL_ENTITY_DRAWER.md.
func DeepLink(t Type, id string) string {
	d, ok := descriptors[t]
	if !ok || id == "" {
		return ""
	}
	return fmt.Sprintf("%s?drawer=%s&entity=%s", d.ListPath, t, id)
}

// PermissionChecker answers "does the caller hold this permission?" with the
// product's wildcard semantics (`*`, `risks:*`). It is satisfied by the JWT
// claims the auth middleware already puts on the request, so the drawer's answer
// and the route guards' answer come from the same source.
type PermissionChecker interface {
	HasPermission(permission string) bool
}

// Caller is the authenticated identity a drawer request runs as. Every method in
// this package takes one; none of them take a tenant id from anywhere else.
type Caller struct {
	UserID   uuid.UUID
	TenantID uuid.UUID
	Perms    PermissionChecker
}

// Can reports whether the caller holds a permission. A caller with no permission
// source can do nothing — fail closed, never open.
func (c Caller) Can(permission string) bool {
	if c.Perms == nil {
		return false
	}
	return c.Perms.HasPermission(permission)
}

// CanRead reports whether the caller may read a type at all.
func (c Caller) CanRead(t Type) bool {
	d, ok := descriptors[t]
	if !ok {
		return false
	}
	return c.Can(d.ReadPermission)
}

// Valid reports whether the caller is usable: an unauthenticated or tenant-less
// caller is refused before any query runs, so a nil tenant can never reach a
// repository and turn into an unscoped read.
func (c Caller) Valid() bool {
	return c.TenantID != uuid.Nil && c.Perms != nil
}

// Resolver turns an (id, tenant) pair into a drawer. One per entity type.
//
// A resolver is handed a Caller and MUST use its TenantID for every read. It
// returns domain.ErrNotFound for an id that does not exist in that tenant — and
// for an id that exists in ANOTHER tenant, which is the same answer on purpose:
// distinguishing them would let a caller enumerate the other tenant's ids (§31).
type Resolver interface {
	// Summary loads the entity head. Returns a typed not-found error when the id
	// does not resolve within the caller's tenant.
	Summary(ctx context.Context, c Caller, id string) (*Summary, error)
	// Relations loads the edges. It is called only after Summary succeeded, so it
	// may assume the entity is readable — but it must still resolve each target
	// through that target's own tenant-scoped repository.
	Relations(ctx context.Context, c Caller, id string) ([]RelationGroup, error)
	// Actions lists what the caller may do. Only allowed actions are returned.
	Actions(ctx context.Context, c Caller, id string) []Action
}

// Registry binds types to resolvers.
//
// A type with no registered resolver is *unsupported*, not broken: the service
// answers a typed validation error naming the type. That is what lets this ship
// with a resolver missing rather than crashing when one is not wired.
type Registry struct {
	resolvers map[Type]Resolver
}

func NewRegistry() *Registry {
	return &Registry{resolvers: map[Type]Resolver{}}
}

// Register binds a resolver to a type, replacing any previous binding.
func (r *Registry) Register(t Type, res Resolver) *Registry {
	if res == nil {
		return r
	}
	r.resolvers[t] = res
	return r
}

// Resolver returns the resolver for a type.
func (r *Registry) Resolver(t Type) (Resolver, error) {
	res, ok := r.resolvers[t]
	if !ok || res == nil {
		return nil, domain.NewValidationError(fmt.Sprintf("entity type %q is not available on this deployment", t))
	}
	return res, nil
}

// Supported reports the types that actually have a resolver wired.
func (r *Registry) Supported() []Type {
	out := make([]Type, 0, len(r.resolvers))
	for _, t := range Types {
		if _, ok := r.resolvers[t]; ok {
			out = append(out, t)
		}
	}
	return out
}
