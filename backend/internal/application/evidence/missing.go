// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package evidence

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

// MissingKind distinguishes the two ways a control can lack proof, because they
// are different jobs for different people on different days.
type MissingKind string

const (
	// MissingNone: the control has at least one currently-valid artifact.
	MissingNone MissingKind = "covered"
	// MissingNever: nobody has ever attached proof. Someone must go and collect it.
	MissingNever MissingKind = "no_evidence"
	// MissingStale: proof exists but has expired or was rejected. Someone must go
	// and REFRESH it — usually a much smaller job, and one that is invisible in
	// tools that only count attachments.
	MissingStale MissingKind = "stale_evidence"
	// MissingExpiring: proof is valid but falls due inside the warning window.
	MissingExpiring MissingKind = "expiring_soon"
)

// MissingControl is one control and the state of its proof.
type MissingControl struct {
	ControlID     uuid.UUID            `json:"control_id"`
	ReferenceCode string               `json:"reference_code"`
	Name          string               `json:"name"`
	Status        domain.ControlStatus `json:"control_status"`
	Kind          MissingKind          `json:"kind"`
	// TotalEvidence counts every attached artifact; CoveringEvidence counts only
	// those that currently substantiate. The gap between the two numbers is the
	// story: "4 documents, none of them still valid".
	TotalEvidence    int        `json:"total_evidence"`
	CoveringEvidence int        `json:"covering_evidence"`
	NearestExpiry    *time.Time `json:"nearest_expiry,omitempty"`
}

// FrameworkEvidenceCoverage is the per-framework roll-up.
type FrameworkEvidenceCoverage struct {
	FrameworkID   uuid.UUID `json:"framework_id"`
	FrameworkName string    `json:"framework_name"`
	Version       string    `json:"version"`

	TotalControls   int `json:"total_controls"`
	CoveredControls int `json:"covered_controls"`
	NoEvidence      int `json:"no_evidence"`
	StaleEvidence   int `json:"stale_evidence"`
	ExpiringSoon    int `json:"expiring_soon"`

	// PercentCovered is over controls that need proof. Controls marked
	// not-applicable are excluded from BOTH sides of the ratio — asking a tenant
	// to evidence a control they have formally scoped out would manufacture gaps.
	PercentCovered float64 `json:"percent_covered"`

	// Controls lacking proof, worst first. Covered controls are omitted: this is
	// a worklist, not an inventory.
	Missing []MissingControl `json:"missing"`
}

// MissingEvidence answers "what proof am I missing?" for one framework, or for
// every framework the tenant owns.
//
// This is the view that makes the module a GRC tool rather than a file cabinet.
// A control marked "implemented" whose only evidence expired last quarter is the
// single most dangerous row in a compliance register: it reads green everywhere
// else in the product and collapses under the first question an auditor asks.
func (s *Service) MissingEvidence(ctx context.Context, tenantID, frameworkID uuid.UUID) ([]FrameworkEvidenceCoverage, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewValidationError("tenant is required")
	}

	var frameworks []domain.ComplianceFramework
	if frameworkID != uuid.Nil {
		fw, err := s.controls.GetFrameworkByID(ctx, frameworkID, tenantID)
		if err != nil {
			return nil, err
		}
		if fw == nil {
			return nil, domain.NewNotFoundError("framework", frameworkID)
		}
		frameworks = []domain.ComplianceFramework{*fw}
	} else {
		all, err := s.controls.ListFrameworks(ctx, tenantID)
		if err != nil {
			return nil, err
		}
		frameworks = all
	}

	now := s.now()
	out := make([]FrameworkEvidenceCoverage, 0, len(frameworks))

	for _, fw := range frameworks {
		controls, err := s.controls.ListControlsByFramework(ctx, tenantID, fw.ID)
		if err != nil {
			return nil, err
		}

		// Two grouped queries per framework, never one per control: the artifacts
		// currently covering, and (below) everything attached at all.
		covering, err := s.repo.CountCoveringByFramework(ctx, tenantID, fw.ID, now)
		if err != nil {
			return nil, err
		}

		cov := FrameworkEvidenceCoverage{
			FrameworkID: fw.ID, FrameworkName: fw.Name, Version: fw.Version,
			Missing: make([]MissingControl, 0),
		}

		applicable := 0
		for _, c := range controls {
			cov.TotalControls++
			if c.Status == domain.ControlStatusNotApplicable {
				continue
			}
			applicable++

			coveringN := covering[c.ID]
			mc := MissingControl{
				ControlID: c.ID, ReferenceCode: c.ReferenceCode, Name: c.Name,
				Status: c.Status, CoveringEvidence: coveringN,
			}

			// Only reach for the full artifact list of controls that are NOT covered,
			// plus covered ones (to find the nearest expiry). In a mature register
			// most controls have some evidence, so this is bounded by the framework's
			// control count and returns small rows.
			attached, err := s.repo.ListByControl(ctx, tenantID, c.ID)
			if err != nil {
				return nil, err
			}
			mc.TotalEvidence = len(attached)

			var nearest *time.Time
			for i := range attached {
				a := &attached[i]
				if !a.Covers(now) || a.ValidUntil == nil {
					continue
				}
				if nearest == nil || a.ValidUntil.Before(*nearest) {
					nearest = a.ValidUntil
				}
			}
			mc.NearestExpiry = nearest

			switch {
			case coveringN == 0 && mc.TotalEvidence == 0:
				mc.Kind = MissingNever
				cov.NoEvidence++
			case coveringN == 0:
				// Attached but none of it still counts.
				mc.Kind = MissingStale
				cov.StaleEvidence++
			case nearest != nil && nearest.Before(now.Add(domain.EvidenceExpiryWindow)):
				mc.Kind = MissingExpiring
				cov.CoveredControls++
				cov.ExpiringSoon++
			default:
				mc.Kind = MissingNone
				cov.CoveredControls++
			}

			if mc.Kind != MissingNone {
				cov.Missing = append(cov.Missing, mc)
			}
		}

		if applicable > 0 {
			cov.PercentCovered = float64(cov.CoveredControls) / float64(applicable) * 100
		}

		// Worst first: never-evidenced before stale before expiring, then by code so
		// the order is stable between refreshes.
		sortMissing(cov.Missing)
		out = append(out, cov)
	}

	return out, nil
}

// kindRank orders the worklist by how much work each row implies.
func kindRank(k MissingKind) int {
	switch k {
	case MissingNever:
		return 0
	case MissingStale:
		return 1
	case MissingExpiring:
		return 2
	default:
		return 3
	}
}

// sortMissing is an insertion sort: framework control counts are in the tens to
// low hundreds, and this keeps the ordering rule readable in one place.
func sortMissing(rows []MissingControl) {
	for i := 1; i < len(rows); i++ {
		for j := i; j > 0; j-- {
			a, b := rows[j-1], rows[j]
			if kindRank(a.Kind) < kindRank(b.Kind) {
				break
			}
			if kindRank(a.Kind) == kindRank(b.Kind) && a.ReferenceCode <= b.ReferenceCode {
				break
			}
			rows[j-1], rows[j] = rows[j], rows[j-1]
		}
	}
}
