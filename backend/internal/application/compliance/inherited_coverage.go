// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package compliance

import (
	"context"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

// InheritedSource is one control elsewhere in the tenant's programme that
// already answers a control of the framework being examined.
type InheritedSource struct {
	ControlID     uuid.UUID                `json:"control_id"`
	ReferenceCode string                   `json:"reference_code"`
	Name          string                   `json:"name"`
	FrameworkID   uuid.UUID                `json:"framework_id"`
	FrameworkName string                   `json:"framework_name"`
	Coverage      domain.CrosswalkCoverage `json:"coverage"`
	Rationale     string                   `json:"rationale"`
	Origin        domain.CrosswalkOrigin   `json:"origin"`
	// EvidenceCount is how much CURRENTLY VALID proof the source control holds.
	// Zero means the crosswalk exists but inherits nothing — the distinction the
	// headline number turns on.
	EvidenceCount int `json:"evidence_count"`
}

// InheritedControl is one control of the framework under examination.
type InheritedControl struct {
	ControlID     uuid.UUID         `json:"control_id"`
	ReferenceCode string            `json:"reference_code"`
	Name          string            `json:"name"`
	Sources       []InheritedSource `json:"sources"`
	// Coverage is the strongest inheritance available: full if any source fully
	// answers it AND holds valid proof, partial if the best on offer is partial
	// or the source is unevidenced, empty if nothing crosswalks to it.
	Coverage domain.CrosswalkCoverage `json:"coverage,omitempty"`
	// AlreadyEvidenced is true when at least one FULL source holds valid proof —
	// this is what the headline percentage counts.
	AlreadyEvidenced bool `json:"already_evidenced"`
}

// InheritedCoverage answers "how much of this framework do I already have?".
//
// This is the number that decides whether a second framework is a quarter's work
// or a fortnight's. It counts only controls whose crosswalked source holds
// CURRENTLY VALID evidence — a link to a control someone evidenced with a
// certificate that expired last year inherits nothing, and saying otherwise
// would tell a compliance officer they can stop working.
type InheritedCoverage struct {
	FrameworkID   uuid.UUID `json:"framework_id"`
	FrameworkName string    `json:"framework_name"`

	TotalControls int `json:"total_controls"`
	// CrosswalkedControls have at least one correspondence, evidenced or not.
	CrosswalkedControls int `json:"crosswalked_controls"`
	// AlreadyCoveredControls are fully crosswalked to a control holding valid
	// proof. The headline.
	AlreadyCoveredControls int `json:"already_covered_controls"`
	// PartiallyCoveredControls have something to inherit but not the whole thing.
	PartiallyCoveredControls int `json:"partially_covered_controls"`

	// PercentAlreadyCovered is AlreadyCovered / applicable controls. The "47%".
	PercentAlreadyCovered float64 `json:"percent_already_covered"`

	// Controls carries only those with something to inherit — this is a head
	// start, not an inventory.
	Controls []InheritedControl `json:"controls"`
}

// GetInheritedCoverageUseCase computes the head start a tenant gets on a
// framework from proof they already hold elsewhere.
type GetInheritedCoverageUseCase struct {
	repo       domain.ComplianceRepository
	crosswalks domain.ControlCrosswalkRepository
	evidence   domain.EvidenceRepository
	now        func() time.Time
}

func NewGetInheritedCoverageUseCase(
	repo domain.ComplianceRepository,
	crosswalks domain.ControlCrosswalkRepository,
	evidence domain.EvidenceRepository,
) *GetInheritedCoverageUseCase {
	return &GetInheritedCoverageUseCase{repo: repo, crosswalks: crosswalks, evidence: evidence, now: time.Now}
}

// WithClock overrides the clock (tests).
func (uc *GetInheritedCoverageUseCase) WithClock(f func() time.Time) *GetInheritedCoverageUseCase {
	if f != nil {
		uc.now = f
	}
	return uc
}

func (uc *GetInheritedCoverageUseCase) Execute(ctx context.Context, tenantID, frameworkID uuid.UUID) (*InheritedCoverage, error) {
	fw, err := uc.repo.GetFrameworkByID(ctx, frameworkID, tenantID)
	if err != nil {
		return nil, err
	}
	if fw == nil {
		return nil, domain.NewNotFoundError("framework", frameworkID)
	}

	controls, err := uc.repo.ListControlsByFramework(ctx, tenantID, frameworkID)
	if err != nil {
		return nil, err
	}

	links, err := uc.crosswalks.ListByFramework(ctx, tenantID, frameworkID)
	if err != nil {
		return nil, err
	}

	out := &InheritedCoverage{
		FrameworkID:   fw.ID,
		FrameworkName: fw.Name,
		Controls:      make([]InheritedControl, 0),
	}

	// Index this framework's controls so the crosswalk's two ends can be told
	// apart: a link has one foot here and one foot elsewhere, and which is
	// "source" in storage says nothing about which side we are reporting on.
	mine := make(map[uuid.UUID]domain.ComplianceControl, len(controls))
	applicable := 0
	for _, c := range controls {
		mine[c.ID] = c
		out.TotalControls++
		if c.Status != domain.ControlStatusNotApplicable {
			applicable++
		}
	}

	// Gather the far ends, then resolve each once. A framework with a hundred
	// crosswalks pointing at a dozen distinct controls costs a dozen lookups.
	type sourceRef struct {
		coverage  domain.CrosswalkCoverage
		rationale string
		origin    domain.CrosswalkOrigin
	}
	byControl := make(map[uuid.UUID]map[uuid.UUID]sourceRef, len(controls))
	farEnds := make(map[uuid.UUID]bool)

	for _, l := range links {
		var here, there uuid.UUID
		switch {
		case mine[l.SourceControlID].ID != uuid.Nil && mine[l.TargetControlID].ID == uuid.Nil:
			here, there = l.SourceControlID, l.TargetControlID
		case mine[l.TargetControlID].ID != uuid.Nil && mine[l.SourceControlID].ID == uuid.Nil:
			here, there = l.TargetControlID, l.SourceControlID
		default:
			// Both ends inside this framework (or neither): nothing is inherited
			// from elsewhere, which is the only thing this view reports.
			continue
		}
		if byControl[here] == nil {
			byControl[here] = map[uuid.UUID]sourceRef{}
		}
		byControl[here][there] = sourceRef{coverage: l.Coverage, rationale: l.Rationale, origin: l.Origin}
		farEnds[there] = true
	}

	// Resolve each far end once: its identity, its framework, and how much valid
	// proof it holds.
	type resolved struct {
		control       domain.ComplianceControl
		frameworkName string
		evidenceCount int
	}
	resolvedEnds := make(map[uuid.UUID]resolved, len(farEnds))
	frameworkNames := map[uuid.UUID]string{}
	coverageByFramework := map[uuid.UUID]map[uuid.UUID]int{}
	now := uc.now()

	for id := range farEnds {
		c, err := uc.repo.GetControlByID(ctx, id, tenantID)
		if err != nil || c == nil {
			// A crosswalk to a control that has since been deleted inherits nothing.
			// Skipping beats failing the whole view over one stale link.
			continue
		}
		name, ok := frameworkNames[c.FrameworkID]
		if !ok {
			if f, err := uc.repo.GetFrameworkByID(ctx, c.FrameworkID, tenantID); err == nil && f != nil {
				name = f.Name
			}
			frameworkNames[c.FrameworkID] = name
		}

		// One grouped evidence query per SOURCE framework, not per control.
		counts, ok := coverageByFramework[c.FrameworkID]
		if !ok {
			counts = map[uuid.UUID]int{}
			if uc.evidence != nil {
				if m, err := uc.evidence.CountCoveringByFramework(ctx, tenantID, c.FrameworkID, now); err == nil {
					counts = m
				}
			}
			coverageByFramework[c.FrameworkID] = counts
		}

		resolvedEnds[id] = resolved{control: *c, frameworkName: name, evidenceCount: counts[c.ID]}
	}

	for _, c := range controls {
		sources := byControl[c.ID]
		if len(sources) == 0 {
			continue
		}

		row := InheritedControl{
			ControlID: c.ID, ReferenceCode: c.ReferenceCode, Name: c.Name,
			Sources: make([]InheritedSource, 0, len(sources)),
		}

		for id, ref := range sources {
			r, ok := resolvedEnds[id]
			if !ok {
				continue
			}
			row.Sources = append(row.Sources, InheritedSource{
				ControlID: r.control.ID, ReferenceCode: r.control.ReferenceCode, Name: r.control.Name,
				FrameworkID: r.control.FrameworkID, FrameworkName: r.frameworkName,
				Coverage: ref.coverage, Rationale: ref.rationale, Origin: ref.origin,
				EvidenceCount: r.evidenceCount,
			})

			switch {
			case ref.coverage == domain.CoverageFull && r.evidenceCount > 0:
				row.AlreadyEvidenced = true
				row.Coverage = domain.CoverageFull
			case row.Coverage == "":
				row.Coverage = domain.CoveragePartial
			}
		}
		if len(row.Sources) == 0 {
			continue
		}

		// Strongest first, so the reason shown next to a control is the best one
		// available rather than whichever link happened to be created first.
		sort.Slice(row.Sources, func(i, j int) bool {
			a, b := row.Sources[i], row.Sources[j]
			if (a.EvidenceCount > 0) != (b.EvidenceCount > 0) {
				return a.EvidenceCount > 0
			}
			if a.Coverage != b.Coverage {
				return a.Coverage == domain.CoverageFull
			}
			return a.ReferenceCode < b.ReferenceCode
		})

		out.CrosswalkedControls++
		if row.AlreadyEvidenced {
			out.AlreadyCoveredControls++
		} else {
			out.PartiallyCoveredControls++
		}
		out.Controls = append(out.Controls, row)
	}

	if applicable > 0 {
		out.PercentAlreadyCovered = float64(out.AlreadyCoveredControls) / float64(applicable) * 100
	}

	sort.Slice(out.Controls, func(i, j int) bool {
		if out.Controls[i].AlreadyEvidenced != out.Controls[j].AlreadyEvidenced {
			return out.Controls[i].AlreadyEvidenced
		}
		return out.Controls[i].ReferenceCode < out.Controls[j].ReferenceCode
	})

	return out, nil
}
