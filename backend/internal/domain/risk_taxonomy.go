// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Three concepts used to share one column and one badge. They are not the same
// thing, and pretending they were is why a user's free-text label showed up in
// the register's "Référentiel" column wearing a framework badge:
//
//	tags             — free text, user-authored, unbounded      → column "Étiquettes"
//	categories       — controlled vocabulary, configured per tenant → column "Catégorie"
//	control_mappings — a reference to a real compliance control  → column "Référentiel"
//
// Only the third one may ever be rendered as a framework, because only the third
// one IS one: it points at a row in compliance_controls that the tenant actually
// imported. Tags stay on Risk.Tags (already free text and already correct);
// this file adds the other two.

// ---------------------------------------------------------------------------
// Categories — controlled vocabulary, per tenant
// ---------------------------------------------------------------------------

// RiskCategory is one entry of a tenant's controlled classification vocabulary
// ("Fraude", "Cyber", "Conformité", …).
//
// Controlled means: an admin curates the list, and a risk may only point at an
// entry that exists. That is the whole difference with a tag — you cannot invent
// a category by typing it, which is what makes it aggregatable in a dashboard.
type RiskCategory struct {
	ID       uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	TenantID uuid.UUID `gorm:"type:uuid;not null;index:idx_risk_categories_tenant_slug,unique,priority:1" json:"tenant_id"`

	Name string `gorm:"size:120;not null" json:"name"`
	// Slug is the stable machine key, unique per tenant. Renaming a category
	// changes Name, never Slug, so links and saved filters survive.
	Slug        string `gorm:"size:120;not null;index:idx_risk_categories_tenant_slug,unique,priority:2" json:"slug"`
	Description string `gorm:"type:text" json:"description"`
	// Color is a UI token name (not a hex value) so categories stay theme-aware
	// in both light and dark.
	Color     string `gorm:"size:32;default:'neutral'" json:"color"`
	SortOrder int    `gorm:"default:0" json:"sort_order"`
	// Active false keeps history readable (risks already classified keep their
	// category) while removing the entry from the picker. Categories are never
	// hard-deleted out from under existing risks.
	Active bool `gorm:"default:true;index" json:"active"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// Computed, NOT persisted — how many risks currently carry this category.
	RiskCount int64 `gorm:"-" json:"risk_count"`
}

func (RiskCategory) TableName() string { return "risk_categories" }

var slugPattern = regexp.MustCompile(`[^a-z0-9]+`)

// Slugify derives a stable machine key from a display name.
func Slugify(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = strings.NewReplacer(
		"à", "a", "â", "a", "ä", "a", "é", "e", "è", "e", "ê", "e", "ë", "e",
		"î", "i", "ï", "i", "ô", "o", "ö", "o", "ù", "u", "û", "u", "ü", "u", "ç", "c",
	).Replace(s)
	s = slugPattern.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

// Validate enforces the invariants of a category.
func (c *RiskCategory) Validate() error {
	if strings.TrimSpace(c.Name) == "" {
		return NewValidationError("category name is required")
	}
	if len(c.Name) > 120 {
		return NewValidationError("category name must be 120 characters or less")
	}
	if c.Slug == "" {
		c.Slug = Slugify(c.Name)
	}
	if c.Slug == "" {
		return NewValidationError("category name must contain at least one letter or digit")
	}
	return nil
}

// DefaultRiskCategories is the starting vocabulary seeded for a tenant that has
// none. A tenant with an empty category list cannot classify anything, so the
// "Catégorie" column would ship dead on arrival; these are a defensible GRC
// starting point that an admin can rename, reorder or deactivate.
func DefaultRiskCategories() []RiskCategory {
	seed := []struct{ name, color, desc string }{
		{"Cybersécurité", "critical", "Menaces techniques : intrusion, malware, exfiltration."},
		{"Conformité", "high", "Manquement à une exigence réglementaire ou contractuelle."},
		{"Opérationnel", "medium", "Défaillance de processus, d'outil ou de personne."},
		{"Financier", "high", "Perte, fraude ou exposition monétaire directe."},
		{"Fournisseurs & tiers", "medium", "Dépendance à un prestataire ou à un sous-traitant."},
		{"Continuité d'activité", "high", "Indisponibilité durable d'un service essentiel."},
		{"Données personnelles", "critical", "Traitement de données à caractère personnel."},
		{"Réputation", "low", "Atteinte à l'image auprès des clients ou du régulateur."},
	}
	out := make([]RiskCategory, 0, len(seed))
	for i, s := range seed {
		out = append(out, RiskCategory{
			Name: s.name, Slug: Slugify(s.name), Description: s.desc,
			Color: s.color, SortOrder: i, Active: true,
		})
	}
	return out
}

// RiskCategoryRepository is the tenant-scoped port for the vocabulary.
type RiskCategoryRepository interface {
	Create(ctx context.Context, c *RiskCategory) error
	GetByID(ctx context.Context, id, tenantID uuid.UUID) (*RiskCategory, error)
	// List returns the tenant's categories, ordered by SortOrder then Name.
	// includeInactive false hides deactivated entries from the picker.
	List(ctx context.Context, tenantID uuid.UUID, includeInactive bool) ([]RiskCategory, error)
	Update(ctx context.Context, c *RiskCategory) error
	Delete(ctx context.Context, id, tenantID uuid.UUID) error
	// ExistsBySlug guards the per-tenant uniqueness before an insert.
	ExistsBySlug(ctx context.Context, tenantID uuid.UUID, slug string, excludeID *uuid.UUID) (bool, error)
	// CountRisks fills RiskCount for the admin screen, in one grouped query.
	CountRisks(ctx context.Context, tenantID uuid.UUID) (map[uuid.UUID]int64, error)
	// SeedDefaults inserts DefaultRiskCategories for a tenant that has none.
	// Idempotent: a tenant that already has categories is left alone.
	SeedDefaults(ctx context.Context, tenantID uuid.UUID) error
}

// ---------------------------------------------------------------------------
// Control mappings — a risk's reference to a compliance control
// ---------------------------------------------------------------------------

// RiskControlMapping links a risk to the compliance control (or, at a coarser
// grain, the framework) it answers to.
//
// This is what the "Référentiel" column renders — and the ONLY thing it may
// render. Previously that column read from a free-text `frameworks` array
// populated by a hard-coded dropdown, and fell back to the risk's first TAG when
// the array was empty. Both halves of that were wrong.
//
// ControlID is nullable on purpose: the migration can honestly say "this risk
// relates to ISO 27001" without inventing a control it never named, and the UI
// links to the framework's control list in that case instead of a single control.
type RiskControlMapping struct {
	ID       uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	TenantID uuid.UUID `gorm:"type:uuid;not null;index" json:"tenant_id"`

	RiskID uuid.UUID `gorm:"type:uuid;not null;index" json:"risk_id"`
	// FrameworkID is always set. ControlID narrows it to a specific control.
	FrameworkID uuid.UUID  `gorm:"type:uuid;not null;index" json:"framework_id"`
	ControlID   *uuid.UUID `gorm:"type:uuid;index" json:"control_id"`

	Note      string     `gorm:"type:text" json:"note"`
	CreatedBy *uuid.UUID `gorm:"type:uuid" json:"created_by"`
	// Source records where the link came from, so a link inferred by the 0046
	// data migration is distinguishable from one a human made.
	Source RiskSource `gorm:"type:varchar(20);default:'manual'" json:"source"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// Computed, NOT persisted — filled by the list use case so the register can
	// render a badge and a deep link without a round-trip per row.
	FrameworkName string `gorm:"-" json:"framework_name,omitempty"`
	ControlCode   string `gorm:"-" json:"control_code,omitempty"`
	ControlName   string `gorm:"-" json:"control_name,omitempty"`
}

func (RiskControlMapping) TableName() string { return "risk_control_mappings" }

// Label is what the "Référentiel" badge shows: the framework, narrowed to the
// control when there is one.
func (m RiskControlMapping) Label() string {
	if m.ControlCode != "" {
		return fmt.Sprintf("%s · %s", m.FrameworkName, m.ControlCode)
	}
	return m.FrameworkName
}

// URL is the deep link the badge points at — the control when the mapping names
// one, the framework's control list otherwise. Never a dead badge.
func (m RiskControlMapping) URL() string {
	if m.ControlID != nil {
		return fmt.Sprintf("/compliance/frameworks/%s/controls/%s", m.FrameworkID, *m.ControlID)
	}
	return fmt.Sprintf("/compliance/frameworks/%s/controls", m.FrameworkID)
}

// RiskControlMappingRepository is the tenant-scoped port for risk↔control links.
type RiskControlMappingRepository interface {
	Create(ctx context.Context, m *RiskControlMapping) error
	GetByID(ctx context.Context, id, tenantID uuid.UUID) (*RiskControlMapping, error)
	// ListByRisk returns a single risk's mappings, enriched.
	ListByRisk(ctx context.Context, tenantID, riskID uuid.UUID) ([]RiskControlMapping, error)
	// ListByRisks batches the lookup for a page of the register — one query for
	// the whole page rather than one per row.
	ListByRisks(ctx context.Context, tenantID uuid.UUID, riskIDs []uuid.UUID) (map[uuid.UUID][]RiskControlMapping, error)
	Delete(ctx context.Context, id, tenantID uuid.UUID) error
	// Exists guards against linking the same risk to the same control twice.
	Exists(ctx context.Context, tenantID, riskID, frameworkID uuid.UUID, controlID *uuid.UUID) (bool, error)
	// UnmappedRiskIDs lists the tenant's risks that have no mapping at all —
	// the /risks/unmapped screen.
	UnmappedRiskIDs(ctx context.Context, tenantID uuid.UUID) ([]uuid.UUID, error)
}
