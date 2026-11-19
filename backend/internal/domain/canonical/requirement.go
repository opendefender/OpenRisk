// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package canonical

import "github.com/google/uuid"

// Obligation is the strength of the source's wording (D6, closed).
type Obligation string

// D6's obligation values. `unspecified` is the honest value for imported
// catalogues that do not record it; it is never guessed.
const (
	ObligationShall       Obligation = "shall"
	ObligationShould      Obligation = "should"
	ObligationMay         Obligation = "may"
	ObligationUnspecified Obligation = "unspecified"
)

// Framework is which framework states a requirement. A framework is a
// grouping, not an entity (D6).
type Framework struct {
	// Key is the catalogue key (compliance_frameworks.catalog_key), empty for a
	// tenant-authored framework.
	Key     string `json:"key"`
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Requirement is an obligation stated by a source, independent of how a tenant
// answers it (ADR 0008 D6). observed_at is always null (D4).
type Requirement struct {
	Envelope
	Framework     Framework  `json:"framework"`
	ReferenceCode string     `json:"reference_code"`
	Title         string     `json:"title"`
	Statement     string     `json:"statement"`
	Citation      *string    `json:"citation"`
	Obligation    Obligation `json:"obligation"`
	ParentID      *uuid.UUID `json:"parent_id"`
	Language      string     `json:"language"`
}

// Validate applies D2, D3, D4 and D6 to a Requirement.
func (r Requirement) Validate() error {
	class, err := r.Envelope.validate(TypeRequirement, observedForbidden, ReviewContext{})
	if err != nil {
		return err
	}
	if err := requireText("framework.name", r.Framework.Name, 0); err != nil {
		return err
	}
	if err := maxLen("reference_code", r.ReferenceCode, 50); err != nil {
		return err
	}
	if err := requireText("title", r.Title, 255); err != nil {
		return err
	}
	if err := requireText("statement", r.Statement, 0); err != nil {
		return err
	}
	if err := optionalText("citation", r.Citation, 255); err != nil {
		return err
	}
	if class == ClassRegulatoryCatalogue {
		// D6: citation is not null for regulatory_catalogue. The empty
		// framework key and reference code are allowed only for a
		// tenant-authored requirement, which a catalogue entry is not.
		if blankPtr(r.Citation) {
			return invalid("citation", "is required for a %s requirement", ClassRegulatoryCatalogue)
		}
		if blank(r.Framework.Key) {
			return invalid("framework.key", "is required for a %s requirement", ClassRegulatoryCatalogue)
		}
		if blank(r.ReferenceCode) {
			return invalid("reference_code", "is required for a %s requirement", ClassRegulatoryCatalogue)
		}
	}
	switch r.Obligation {
	case ObligationShall, ObligationShould, ObligationMay, ObligationUnspecified:
	default:
		return invalid("obligation", "must be shall, should, may or unspecified")
	}
	if err := optionalID("parent_id", r.ParentID); err != nil {
		return err
	}
	if !localeTag.MatchString(r.Language) {
		return invalid("language", "must be a BCP-47 locale code (ADR 0002)")
	}
	return nil
}
