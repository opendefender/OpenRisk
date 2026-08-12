// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Where an incident came from, who needs to know, and what was learned.
//
// Three gaps this closes:
//
//  1. Provenance. An incident that appeared on its own is unnerving until you
//     can see which rule opened it and from what. "Créé automatiquement par la
//     règle X depuis la source Y" is not decoration — it is the difference
//     between an alert people trust and one they mute.
//  2. Links. RiskID was a *uint while risks use UUIDs, so incident → risk was
//     structurally impossible. Assets were free text. Both are now real ids.
//  3. Learning. An incident that closes without a post-mortem teaches nothing,
//     and the same incident happens again. For a CRITICAL incident that is not
//     a style preference, so closing one requires a published post-mortem.
// ---------------------------------------------------------------------------

// IncidentOrigin says who or what opened an incident.
const (
	// OriginManual — a human declared it.
	OriginManual = "manual"
	// OriginAutomation — an automation rule opened it.
	OriginAutomation = "automation"
	// OriginScanner — the infrastructure scanner opened it from a finding.
	OriginScanner = "scanner"
	// OriginCTI — threat intelligence opened it (KEV, active exploitation).
	OriginCTI = "cti"
	// OriginIntegration — an external tool pushed it in (webhook, ITSM).
	OriginIntegration = "integration"
)

// IsAutomaticOrigin reports whether an incident was opened without a human.
// These are the ones that need the provenance banner.
func IsAutomaticOrigin(origin string) bool {
	switch strings.ToLower(strings.TrimSpace(origin)) {
	case OriginAutomation, OriginScanner, OriginCTI, OriginIntegration:
		return true
	}
	return false
}

// IncidentOriginInfo describes an origin for the UI, in one sentence a
// non-technical reader can act on.
type IncidentOriginInfo struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	LabelEN     string `json:"label_en"`
	Description string `json:"description"`
	Automatic   bool   `json:"automatic"`
	// WhereToConfigure points at the screen that controls this source, so
	// "why did I get this?" has an answer that ends in a setting.
	WhereToConfigure string `json:"where_to_configure,omitempty"`
}

// IncidentOrigins is the catalogue behind the "Where do incidents come from?"
// help page. It is derived from the code paths that actually create incidents —
// if a new producer is added without an entry here, the help page lies.
func IncidentOrigins() []IncidentOriginInfo {
	return []IncidentOriginInfo{
		{
			Key: OriginManual, Label: "Déclaration manuelle", LabelEN: "Manual declaration",
			Description: "Quelqu'un a cliqué sur « Déclarer un incident ». La source la plus courante, et la seule qui parte d'un jugement humain.",
			Automatic:   false,
		},
		{
			Key: OriginAutomation, Label: "Règle d'automatisation", LabelEN: "Automation rule",
			Description: "Une règle SOAR s'est déclenchée sur un événement de la plateforme et a ouvert cet incident. La règle et l'exécution exactes sont indiquées sur l'incident.",
			Automatic:   true, WhereToConfigure: "/automation?tab=rules",
		},
		{
			Key: OriginScanner, Label: "Scanner d'infrastructure", LabelEN: "Infrastructure scanner",
			Description: "Un scan a détecté quelque chose qui justifiait l'ouverture d'un incident, et non une simple vulnérabilité à trier.",
			Automatic:   true, WhereToConfigure: "/infrastructure",
		},
		{
			Key: OriginCTI, Label: "Renseignement sur les menaces", LabelEN: "Threat intelligence",
			Description: "Une vulnérabilité de votre parc est passée sur la liste CISA des vulnérabilités activement exploitées : elle est exploitée maintenant, pas en théorie.",
			Automatic:   true, WhereToConfigure: "/threat-intel",
		},
		{
			Key: OriginIntegration, Label: "Outil externe", LabelEN: "External tool",
			Description: "Un outil tiers (EDR, SIEM, ITSM) a poussé l'incident via un webhook.",
			Automatic:   true, WhereToConfigure: "/vulnerabilities?tab=integrations",
		},
	}
}

// FindIncidentOrigin resolves a catalogue entry.
func FindIncidentOrigin(key string) (IncidentOriginInfo, bool) {
	for _, o := range IncidentOrigins() {
		if o.Key == strings.ToLower(strings.TrimSpace(key)) {
			return o, true
		}
	}
	return IncidentOriginInfo{}, false
}

// IncidentStakeholder is somebody who must be told about this incident, and how.
// Recording the channel per person is what makes "notify the stakeholders" a
// promise rather than a hope.
type IncidentStakeholder struct {
	UserID   string   `json:"user_id,omitempty"`
	Email    string   `json:"email,omitempty"`
	Role     string   `json:"role,omitempty"` // org role to resolve at send time
	Name     string   `json:"name,omitempty"`
	Channels []string `json:"channels,omitempty"` // in_app|email|slack|teams|webhook|sms
}

// IncidentStakeholderList is a jsonb array.
type IncidentStakeholderList []IncidentStakeholder

func (l IncidentStakeholderList) Value() (driver.Value, error) {
	if l == nil {
		return "[]", nil
	}
	return json.Marshal(l)
}

func (l *IncidentStakeholderList) Scan(value interface{}) error {
	if value == nil {
		*l = nil
		return nil
	}
	switch v := value.(type) {
	case []byte:
		if len(v) == 0 {
			*l = nil
			return nil
		}
		return json.Unmarshal(v, l)
	case string:
		if v == "" {
			*l = nil
			return nil
		}
		return json.Unmarshal([]byte(v), l)
	default:
		return fmt.Errorf("unsupported type for IncidentStakeholderList: %T", value)
	}
}

// =============================================================================
// Post-mortem
// =============================================================================

// PostMortemStatus is the lifecycle of the write-up.
const (
	PostMortemDraft     = "draft"
	PostMortemPublished = "published"
)

// CorrectiveActionStatus mirrors the mitigation it becomes.
const (
	CorrectiveActionOpen      = "open"
	CorrectiveActionConverted = "converted" // a mitigation plan was created from it
	CorrectiveActionDone      = "done"
	CorrectiveActionDropped   = "dropped"
)

// CorrectiveAction is a decision made in the review that must actually happen.
// The whole point of the field is that it does not stay in a document: publishing
// the post-mortem turns each action into a real mitigation plan, with an owner
// and a due date, in the module that tracks those.
type CorrectiveAction struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description,omitempty"`
	OwnerID     string     `json:"owner_id,omitempty"`
	DueDate     *time.Time `json:"due_date,omitempty"`
	Priority    string     `json:"priority,omitempty"` // critical|high|medium|low
	Status      string     `json:"status"`
	// MitigationID is filled once the action has been converted, so the
	// post-mortem can link to the plan instead of duplicating its state.
	MitigationID string `json:"mitigation_id,omitempty"`
	// RiskID is the risk the mitigation was attached to.
	RiskID string `json:"risk_id,omitempty"`
}

// CorrectiveActionList is a jsonb array.
type CorrectiveActionList []CorrectiveAction

func (l CorrectiveActionList) Value() (driver.Value, error) {
	if l == nil {
		return "[]", nil
	}
	return json.Marshal(l)
}

func (l *CorrectiveActionList) Scan(value interface{}) error {
	if value == nil {
		*l = nil
		return nil
	}
	switch v := value.(type) {
	case []byte:
		if len(v) == 0 {
			*l = nil
			return nil
		}
		return json.Unmarshal(v, l)
	case string:
		if v == "" {
			*l = nil
			return nil
		}
		return json.Unmarshal([]byte(v), l)
	default:
		return fmt.Errorf("unsupported type for CorrectiveActionList: %T", value)
	}
}

// PostMortemTimelineEntry is one moment in the reconstructed story. Separate
// from IncidentTimeline (the system's own log) because a review timeline is
// authored: it says what people believed and decided, not only what the platform
// recorded.
type PostMortemTimelineEntry struct {
	At     time.Time `json:"at"`
	Title  string    `json:"title"`
	Detail string    `json:"detail,omitempty"`
	// Kind marks the moments a review is measured by.
	Kind string `json:"kind,omitempty"` // detection|escalation|mitigation|resolution|note
}

// PostMortemTimeline is a jsonb array.
type PostMortemTimeline []PostMortemTimelineEntry

func (l PostMortemTimeline) Value() (driver.Value, error) {
	if l == nil {
		return "[]", nil
	}
	return json.Marshal(l)
}

func (l *PostMortemTimeline) Scan(value interface{}) error {
	if value == nil {
		*l = nil
		return nil
	}
	switch v := value.(type) {
	case []byte:
		if len(v) == 0 {
			*l = nil
			return nil
		}
		return json.Unmarshal(v, l)
	case string:
		if v == "" {
			*l = nil
			return nil
		}
		return json.Unmarshal([]byte(v), l)
	default:
		return fmt.Errorf("unsupported type for PostMortemTimeline: %T", value)
	}
}

// IncidentPostMortem is the structured review of one incident.
type IncidentPostMortem struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	TenantID   uuid.UUID `gorm:"type:uuid;index;not null" json:"tenant_id"`
	IncidentID uint      `gorm:"uniqueIndex;not null" json:"incident_id"`

	Summary string `gorm:"type:text" json:"summary"`
	// RootCause is the answer to "why did this happen", not "what happened".
	RootCause string `gorm:"type:text" json:"root_cause"`
	// ContributingFactors are the conditions that let the root cause bite.
	ContributingFactors string `gorm:"type:text" json:"contributing_factors,omitempty"`
	// Impact is what it cost: users, data, downtime, money, obligations.
	Impact string `gorm:"type:text" json:"impact"`
	// Detection is how it was found — the honest version, including "a customer
	// told us", which is the finding that changes roadmaps.
	Detection string `gorm:"type:text" json:"detection,omitempty"`
	// WhatWentWell is not optimism: a review that only lists failures teaches
	// people to hide incidents.
	WhatWentWell   string `gorm:"type:text" json:"what_went_well,omitempty"`
	LessonsLearned string `gorm:"type:text" json:"lessons_learned,omitempty"`

	Timeline          PostMortemTimeline   `gorm:"type:jsonb" json:"timeline"`
	CorrectiveActions CorrectiveActionList `gorm:"type:jsonb" json:"corrective_actions"`

	Status      string     `gorm:"type:varchar(16);index;default:'draft'" json:"status"`
	AuthorID    *uuid.UUID `gorm:"type:uuid" json:"author_id,omitempty"`
	AuthorEmail string     `gorm:"-" json:"author_email,omitempty"`
	PublishedAt *time.Time `json:"published_at,omitempty"`
	PublishedBy *uuid.UUID `gorm:"type:uuid" json:"published_by,omitempty"`

	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (IncidentPostMortem) TableName() string { return "incident_post_mortems" }

// AuditEntityType makes post-mortems automatically audited.
func (IncidentPostMortem) AuditEntityType() string { return "incident_post_mortem" }

// MissingForPublication lists what still has to be filled in. Returning the list
// rather than a bare error is what lets the UI show a checklist instead of a
// wall — a reviewer should see the three remaining fields, not "invalid".
func (p *IncidentPostMortem) MissingForPublication() []string {
	var missing []string
	if strings.TrimSpace(p.Summary) == "" {
		missing = append(missing, "summary")
	}
	if strings.TrimSpace(p.RootCause) == "" {
		missing = append(missing, "root_cause")
	}
	if strings.TrimSpace(p.Impact) == "" {
		missing = append(missing, "impact")
	}
	if len(p.Timeline) == 0 {
		missing = append(missing, "timeline")
	}
	if len(p.CorrectiveActions) == 0 {
		// A review with no corrective action is a story, not a review.
		missing = append(missing, "corrective_actions")
	}
	return missing
}

// CanPublish reports whether the review is complete enough to publish.
func (p *IncidentPostMortem) CanPublish() bool { return len(p.MissingForPublication()) == 0 }

// RequiresPostMortem reports whether an incident of this severity may not be
// closed without a published review. Critical only: making every incident
// require a full review would produce reviews nobody writes and a rule everyone
// works around.
func RequiresPostMortem(severity string) bool {
	return strings.EqualFold(strings.TrimSpace(severity), "critical")
}

// IncidentPostMortemRepository is the tenant-scoped store.
type IncidentPostMortemRepository interface {
	Get(ctx context.Context, tenantID uuid.UUID, incidentID uint) (*IncidentPostMortem, error)
	Upsert(ctx context.Context, p *IncidentPostMortem) error
	ListByTenant(ctx context.Context, tenantID uuid.UUID, status string, limit int) ([]IncidentPostMortem, error)
}
