// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package canonical

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Envelope is the eleven fields every canonical entity carries (ADR 0008 D2).
// Entities embed it; TestCanonical_EveryEntityCarriesTheEnvelope fails on an
// exported entity type that does not.
//
// No envelope field is read from a request body: tenant_id comes from the
// authenticated context, and source, source_id, source_version, confidence and
// provenance are set by the use case or ingest path that owns the source (D2).
type Envelope struct {
	ID            uuid.UUID  `json:"id"`
	TenantID      uuid.UUID  `json:"tenant_id"`
	Source        Source     `json:"source"`
	SourceID      string     `json:"source_id"`
	SourceVersion *string    `json:"source_version"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	ObservedAt    *time.Time `json:"observed_at"`
	Confidence    float64    `json:"confidence"`
	Provenance    Provenance `json:"provenance"`
	Relationships List[Edge] `json:"relationships"`
}

// observedRule is D4's optionality of observed_at for one entity.
type observedRule int

const (
	observedForbidden observedRule = iota // D4 "null": Requirement, Control, ControlTest
	observedOptional                      // D4 "yes, nullable": ControlImplementation, Risk
	observedRequired                      // D4 "required": Evidence, Finding
)

// validate checks the envelope for an entity of type self. rc carries the
// review state the user_entered uplift reads; entities without a review pass
// the zero value, which never qualifies.
func (e Envelope) validate(self EntityType, observed observedRule, rc ReviewContext) (SourceClass, error) {
	if err := requireID("tenant_id", e.TenantID); err != nil {
		return "", err
	}
	if err := requireID("id", e.ID); err != nil {
		return "", err
	}
	class, err := validateSource("source", e.Source)
	if err != nil {
		return "", err
	}
	if err := requireText("source_id", e.SourceID, 255); err != nil {
		return "", err
	}
	if err := optionalText("source_version", e.SourceVersion, 64); err != nil {
		return "", err
	}
	if err := requireTime("created_at", e.CreatedAt); err != nil {
		return "", err
	}
	if err := requireTime("updated_at", e.UpdatedAt); err != nil {
		return "", err
	}
	switch observed {
	case observedForbidden:
		if e.ObservedAt != nil {
			return "", invalid("observed_at", "must be null on %s (ADR 0008 D4)", self)
		}
	case observedRequired:
		if e.ObservedAt == nil || e.ObservedAt.IsZero() {
			return "", invalid("observed_at", "is required on %s (ADR 0008 D4)", self)
		}
	case observedOptional:
		if err := optionalTime("observed_at", e.ObservedAt); err != nil {
			return "", err
		}
	}
	rc.ActorID = e.Provenance.Actor.ID
	if err := validateConfidence("confidence", e.Source, e.Confidence, rc); err != nil {
		return "", err
	}
	if err := e.Provenance.Validate(e.Source); err != nil {
		return "", err
	}
	for i, edge := range e.Relationships {
		field := fmt.Sprintf("relationships[%d]", i)
		if edge.From.Type != self || edge.From.ID != e.ID {
			return "", invalid(field+".from", "an outgoing edge must start at this record")
		}
		if edge.TenantID != e.TenantID {
			return "", invalid(field+".tenant_id", "an edge inherits the tenant of its source end")
		}
		if err := within(field, edge.Validate()); err != nil {
			return "", err
		}
	}
	return class, nil
}

// Ownership is the `domain.Ownership` block, unchanged (D6): user ids until
// `Owner` becomes an entity in #1217.
type Ownership struct {
	OwnerID    *uuid.UUID `json:"owner_id"`
	AssigneeID *uuid.UUID `json:"assignee_id"`
	ReviewerID *uuid.UUID `json:"reviewer_id"`
}

func (o Ownership) validate() error {
	if err := optionalID("owner_id", o.OwnerID); err != nil {
		return err
	}
	if err := optionalID("assignee_id", o.AssigneeID); err != nil {
		return err
	}
	return optionalID("reviewer_id", o.ReviewerID)
}
