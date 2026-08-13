// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package compliance

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

// --- fakes -----------------------------------------------------------------

type fakeCrosswalkRepo struct {
	links []domain.ControlCrosswalk
}

func (f *fakeCrosswalkRepo) Create(_ context.Context, m *domain.ControlCrosswalk) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	f.links = append(f.links, *m)
	return nil
}

func (f *fakeCrosswalkRepo) Exists(_ context.Context, tenantID, a, b uuid.UUID) (bool, error) {
	for _, l := range f.links {
		if l.TenantID != tenantID {
			continue
		}
		if (l.SourceControlID == a && l.TargetControlID == b) ||
			(l.SourceControlID == b && l.TargetControlID == a) {
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeCrosswalkRepo) List(_ context.Context, tenantID uuid.UUID, controlID *uuid.UUID) ([]domain.ControlCrosswalk, error) {
	var out []domain.ControlCrosswalk
	for _, l := range f.links {
		if l.TenantID != tenantID {
			continue
		}
		if controlID != nil && l.SourceControlID != *controlID && l.TargetControlID != *controlID {
			continue
		}
		out = append(out, l)
	}
	return out, nil
}

func (f *fakeCrosswalkRepo) Delete(context.Context, uuid.UUID, uuid.UUID) error { return nil }

func (f *fakeCrosswalkRepo) ListByFramework(_ context.Context, tenantID, _ uuid.UUID) ([]domain.ControlCrosswalk, error) {
	// The real query narrows by framework in SQL; the fixture holds one tenant's
	// links and the use case works out which end is which, so returning them all
	// exercises exactly that logic.
	var out []domain.ControlCrosswalk
	for _, l := range f.links {
		if l.TenantID == tenantID {
			out = append(out, l)
		}
	}
	return out, nil
}

// fakeEvidenceCounts answers only the one method inherited coverage needs.
type fakeEvidenceCounts struct {
	byFramework map[uuid.UUID]map[uuid.UUID]int
}

func (f *fakeEvidenceCounts) CountCoveringByFramework(_ context.Context, _, frameworkID uuid.UUID, _ time.Time) (map[uuid.UUID]int, error) {
	return f.byFramework[frameworkID], nil
}

func (f *fakeEvidenceCounts) Create(context.Context, *domain.Evidence) error { return nil }
func (f *fakeEvidenceCounts) Update(context.Context, *domain.Evidence) error { return nil }
func (f *fakeEvidenceCounts) GetByID(context.Context, uuid.UUID, uuid.UUID) (*domain.Evidence, error) {
	return nil, nil
}
func (f *fakeEvidenceCounts) List(context.Context, uuid.UUID, domain.EvidenceFilter) ([]domain.Evidence, int64, error) {
	return nil, 0, nil
}
func (f *fakeEvidenceCounts) Delete(context.Context, uuid.UUID, uuid.UUID) error      { return nil }
func (f *fakeEvidenceCounts) Link(context.Context, *domain.EvidenceControlLink) error { return nil }
func (f *fakeEvidenceCounts) Unlink(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error {
	return nil
}
func (f *fakeEvidenceCounts) ListLinks(context.Context, uuid.UUID, []uuid.UUID) ([]domain.EvidenceControlLink, error) {
	return nil, nil
}
func (f *fakeEvidenceCounts) ListByControl(context.Context, uuid.UUID, uuid.UUID) ([]domain.Evidence, error) {
	return nil, nil
}
func (f *fakeEvidenceCounts) ControlsWithCoverage(context.Context, uuid.UUID, uuid.UUID, time.Time) (map[uuid.UUID]bool, error) {
	return nil, nil
}
func (f *fakeEvidenceCounts) ListExpiring(context.Context, time.Time, time.Duration, int) ([]domain.Evidence, error) {
	return nil, nil
}
func (f *fakeEvidenceCounts) MarkReminded(context.Context, uuid.UUID, time.Time) error { return nil }

// --- fixture ---------------------------------------------------------------

type coverageFixture struct {
	uc       *GetInheritedCoverageUseCase
	repo     *MockComplianceRepository
	links    *fakeCrosswalkRepo
	tenant   uuid.UUID
	isoID    uuid.UUID
	socID    uuid.UUID
	iso      []domain.ComplianceControl
	soc      []domain.ComplianceControl
	evidence *fakeEvidenceCounts
}

func setupCoverage(t *testing.T) *coverageFixture {
	t.Helper()
	tenant, isoID, socID := uuid.New(), uuid.New(), uuid.New()

	mk := func(fw uuid.UUID, code string) domain.ComplianceControl {
		return domain.ComplianceControl{
			ID: uuid.New(), TenantID: tenant, FrameworkID: fw,
			ReferenceCode: code, Name: code, Status: domain.ControlStatusNotImplemented,
		}
	}
	// SOC 2 is the framework being imported; ISO is the mature one.
	iso := []domain.ComplianceControl{mk(isoID, "A.5.1"), mk(isoID, "A.5.15"), mk(isoID, "A.5.3")}
	soc := []domain.ComplianceControl{mk(socID, "CC5.3"), mk(socID, "CC6.1"), mk(socID, "CC1.5"), mk(socID, "CC2.1")}

	byID := map[uuid.UUID]domain.ComplianceControl{}
	for _, c := range append(append([]domain.ComplianceControl{}, iso...), soc...) {
		byID[c.ID] = c
	}

	repo := &MockComplianceRepository{
		getFrameworkByIDFunc: func(_ context.Context, id, tid uuid.UUID) (*domain.ComplianceFramework, error) {
			if tid != tenant {
				return nil, nil
			}
			switch id {
			case isoID:
				return &domain.ComplianceFramework{ID: isoID, TenantID: tenant, Name: "ISO/IEC 27001", CatalogKey: "iso27001-2022"}, nil
			case socID:
				return &domain.ComplianceFramework{ID: socID, TenantID: tenant, Name: "SOC 2", CatalogKey: "soc2-tsc"}, nil
			}
			return nil, nil
		},
		listControlsByFrameworkFunc: func(_ context.Context, tid, fid uuid.UUID) ([]domain.ComplianceControl, error) {
			if tid != tenant {
				return nil, nil
			}
			switch fid {
			case isoID:
				return iso, nil
			case socID:
				return soc, nil
			}
			return nil, nil
		},
		getControlByIDFunc: func(_ context.Context, id, tid uuid.UUID) (*domain.ComplianceControl, error) {
			if tid != tenant {
				return nil, nil
			}
			c, ok := byID[id]
			if !ok {
				return nil, nil
			}
			return &c, nil
		},
	}

	ev := &fakeEvidenceCounts{byFramework: map[uuid.UUID]map[uuid.UUID]int{}}
	links := &fakeCrosswalkRepo{}
	uc := NewGetInheritedCoverageUseCase(repo, links, ev)

	return &coverageFixture{uc: uc, repo: repo, links: links, tenant: tenant,
		isoID: isoID, socID: socID, iso: iso, soc: soc, evidence: ev}
}

func (f *coverageFixture) link(src, tgt domain.ComplianceControl, cov domain.CrosswalkCoverage) {
	f.links.links = append(f.links.links, domain.ControlCrosswalk{
		ID: uuid.New(), TenantID: f.tenant,
		SourceControlID: src.ID, TargetControlID: tgt.ID,
		Coverage: cov, Rationale: "same evidence answers both", Origin: domain.CrosswalkOriginCurated,
	})
}

func (f *coverageFixture) evidenceOn(c domain.ComplianceControl, n int) {
	if f.evidence.byFramework[c.FrameworkID] == nil {
		f.evidence.byFramework[c.FrameworkID] = map[uuid.UUID]int{}
	}
	f.evidence.byFramework[c.FrameworkID][c.ID] = n
}

// --- tests -----------------------------------------------------------------

// The headline. Two of SOC 2's four controls crosswalk fully to ISO controls
// that hold valid proof, so half the framework is already answered.
func TestInheritedCoverage_CountsOnlyEvidencedFullMatches(t *testing.T) {
	f := setupCoverage(t)

	f.link(f.iso[0], f.soc[0], domain.CoverageFull) // A.5.1  -> CC5.3, evidenced
	f.link(f.iso[1], f.soc[1], domain.CoverageFull) // A.5.15 -> CC6.1, evidenced
	f.link(f.iso[2], f.soc[2], domain.CoveragePartial)
	f.evidenceOn(f.iso[0], 1)
	f.evidenceOn(f.iso[1], 3)
	f.evidenceOn(f.iso[2], 2)

	cov, err := f.uc.Execute(context.Background(), f.tenant, f.socID)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	if cov.TotalControls != 4 {
		t.Fatalf("want 4 controls, got %d", cov.TotalControls)
	}
	if cov.CrosswalkedControls != 3 {
		t.Fatalf("want 3 crosswalked, got %d", cov.CrosswalkedControls)
	}
	if cov.AlreadyCoveredControls != 2 {
		t.Fatalf("want 2 already covered, got %d", cov.AlreadyCoveredControls)
	}
	if cov.PercentAlreadyCovered != 50 {
		t.Fatalf("want 50%%, got %.1f", cov.PercentAlreadyCovered)
	}
	// The partial link contributes something, but not a claim of coverage.
	if cov.PartiallyCoveredControls != 1 {
		t.Fatalf("want 1 partial, got %d", cov.PartiallyCoveredControls)
	}
	// A control with no crosswalk is simply absent: this is a head start, not an
	// inventory.
	for _, c := range cov.Controls {
		if c.ReferenceCode == "CC2.1" {
			t.Fatal("an uncrosswalked control must not appear on the head-start list")
		}
	}
}

// The rule that keeps the number honest: a crosswalk to a control nobody has
// evidenced inherits nothing. Claiming otherwise tells a compliance officer they
// can stop working on something they have never proved.
func TestInheritedCoverage_UnevidencedSourceInheritsNothing(t *testing.T) {
	f := setupCoverage(t)
	f.link(f.iso[0], f.soc[0], domain.CoverageFull)
	// No evidence anywhere.

	cov, err := f.uc.Execute(context.Background(), f.tenant, f.socID)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if cov.AlreadyCoveredControls != 0 {
		t.Fatalf("an unevidenced source must inherit nothing, got %d", cov.AlreadyCoveredControls)
	}
	if cov.PercentAlreadyCovered != 0 {
		t.Fatalf("want 0%%, got %.1f", cov.PercentAlreadyCovered)
	}
	// It still appears, because the link is real and worth acting on — it just
	// reads as "you have the mapping, go and evidence the ISO control".
	if cov.CrosswalkedControls != 1 || len(cov.Controls) != 1 {
		t.Fatalf("the crosswalk itself should still be reported")
	}
	if cov.Controls[0].AlreadyEvidenced {
		t.Fatal("must not read as already evidenced")
	}
	if cov.Controls[0].Sources[0].EvidenceCount != 0 {
		t.Fatal("the source's evidence count should read zero")
	}
}

// A partial crosswalk never counts as covered, however much proof sits behind it.
func TestInheritedCoverage_PartialNeverCounts(t *testing.T) {
	f := setupCoverage(t)
	f.link(f.iso[2], f.soc[2], domain.CoveragePartial)
	f.evidenceOn(f.iso[2], 5)

	cov, _ := f.uc.Execute(context.Background(), f.tenant, f.socID)
	if cov.AlreadyCoveredControls != 0 {
		t.Fatalf("partial coverage is not coverage, got %d", cov.AlreadyCoveredControls)
	}
	if cov.Controls[0].Coverage != domain.CoveragePartial {
		t.Fatalf("want partial, got %q", cov.Controls[0].Coverage)
	}
}

// Direction must not matter: a crosswalk written ISO→SOC has to be read the same
// way when SOC is the framework being examined, or coverage would work only for
// whichever framework happened to be imported second.
func TestInheritedCoverage_ReadsCrosswalksInEitherDirection(t *testing.T) {
	f := setupCoverage(t)
	// Stored the "wrong" way round for this query: SOC control as source.
	f.links.links = append(f.links.links, domain.ControlCrosswalk{
		ID: uuid.New(), TenantID: f.tenant,
		SourceControlID: f.soc[1].ID, TargetControlID: f.iso[1].ID,
		Coverage: domain.CoverageFull, Rationale: "access control", Origin: domain.CrosswalkOriginCurated,
	})
	f.evidenceOn(f.iso[1], 2)

	cov, _ := f.uc.Execute(context.Background(), f.tenant, f.socID)
	if cov.AlreadyCoveredControls != 1 {
		t.Fatalf("stored direction must not matter, got %d covered", cov.AlreadyCoveredControls)
	}
	if cov.Controls[0].Sources[0].ReferenceCode != "A.5.15" {
		t.Fatalf("the far end should be the ISO control, got %q", cov.Controls[0].Sources[0].ReferenceCode)
	}
}

// Every reported source carries the reasoning: the number is one an auditor will
// question, so the tenant must be able to read why the product claims it.
func TestInheritedCoverage_SourcesCarryTheirRationale(t *testing.T) {
	f := setupCoverage(t)
	f.link(f.iso[1], f.soc[1], domain.CoverageFull)
	f.evidenceOn(f.iso[1], 1)

	cov, _ := f.uc.Execute(context.Background(), f.tenant, f.socID)
	src := cov.Controls[0].Sources[0]
	if src.Rationale == "" {
		t.Fatal("a source with no rationale is a claim nobody can defend")
	}
	if src.Origin != domain.CrosswalkOriginCurated {
		t.Fatalf("origin should say who asserted it, got %q", src.Origin)
	}
	if src.FrameworkName != "ISO/IEC 27001" {
		t.Fatalf("the source should name its framework, got %q", src.FrameworkName)
	}
}

// Links whose two ends are both inside the framework under examination inherit
// nothing — that is a within-framework relation, not a head start.
func TestInheritedCoverage_IgnoresLinksInsideTheSameFramework(t *testing.T) {
	f := setupCoverage(t)
	f.link(f.soc[0], f.soc[1], domain.CoverageFull)
	f.evidenceOn(f.soc[0], 4)

	cov, _ := f.uc.Execute(context.Background(), f.tenant, f.socID)
	if cov.CrosswalkedControls != 0 {
		t.Fatalf("a link inside the framework inherits nothing, got %d", cov.CrosswalkedControls)
	}
}

func TestInheritedCoverage_UnknownFrameworkIsNotFound(t *testing.T) {
	f := setupCoverage(t)
	if _, err := f.uc.Execute(context.Background(), f.tenant, uuid.New()); err == nil {
		t.Fatal("unknown framework must be a 404")
	}
	if _, err := f.uc.Execute(context.Background(), uuid.New(), f.socID); err == nil {
		t.Fatal("another tenant's framework must not be readable")
	}
}
