// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package controlchain

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/domain/canonical"
)

// #1416 — the result use cases, against an in-memory repository that honours the
// tenant, the test and the evidence the way the real one does.

type fakeResults struct {
	chain     *fakeChain
	results   map[uuid.UUID]canonical.ControlTestResult
	resultOf  map[uuid.UUID][2]uuid.UUID // result id -> (tenant, test)
	evidence  map[uuid.UUID]uuid.UUID    // evidence id -> tenant
	links     map[[3]uuid.UUID]ResultEvidenceView
	audits    []*domain.AuditEvent
	linkCount int
}

func newFakeResults(chain *fakeChain) *fakeResults {
	return &fakeResults{
		chain: chain, results: map[uuid.UUID]canonical.ControlTestResult{}, resultOf: map[uuid.UUID][2]uuid.UUID{},
		evidence: map[uuid.UUID]uuid.UUID{}, links: map[[3]uuid.UUID]ResultEvidenceView{},
	}
}

func (f *fakeResults) RecordResult(_ context.Context, tenant, testID uuid.UUID, r canonical.ControlTestResult, a *domain.AuditEvent) error {
	if _, ok := f.chain.liveTest(tenant, testID); !ok {
		return testNotFoundErr()
	}
	f.results[r.ResultID] = r
	f.resultOf[r.ResultID] = [2]uuid.UUID{tenant, testID}
	f.audits = append(f.audits, a)
	return nil
}

func (f *fakeResults) ordered(tenant, testID uuid.UUID) []canonical.ControlTestResult {
	var out []canonical.ControlTestResult
	for id, r := range f.results {
		if f.resultOf[id] == [2]uuid.UUID{tenant, testID} {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if !a.ObservedAt.Equal(b.ObservedAt) {
			return a.ObservedAt.After(b.ObservedAt)
		}
		if !a.RecordedAt.Equal(b.RecordedAt) {
			return a.RecordedAt.After(b.RecordedAt)
		}
		return a.ResultID.String() > b.ResultID.String()
	})
	return out
}

func (f *fakeResults) ListResults(_ context.Context, tenant, testID uuid.UUID, limit, offset int) ([]canonical.ControlTestResult, error) {
	if _, ok := f.chain.liveTest(tenant, testID); !ok {
		return nil, testNotFoundErr()
	}
	out := f.ordered(tenant, testID)
	if offset > len(out) {
		offset = len(out)
	}
	out = out[offset:]
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (f *fakeResults) LatestResult(_ context.Context, tenant, testID uuid.UUID) (*canonical.ControlTestResult, error) {
	if _, ok := f.chain.liveTest(tenant, testID); !ok {
		return nil, testNotFoundErr()
	}
	out := f.ordered(tenant, testID)
	if len(out) == 0 {
		return nil, domain.InternalControlError(domain.ErrNotFound, domain.IntCtlNoResult)
	}
	return &out[0], nil
}

func (f *fakeResults) AttachEvidence(_ context.Context, tenant, testID, resultID uuid.UUID, e canonical.Edge, a *domain.AuditEvent) (*ResultEvidenceView, bool, error) {
	if _, ok := f.chain.liveTest(tenant, testID); !ok {
		return nil, false, testNotFoundErr()
	}
	if f.resultOf[resultID] != [2]uuid.UUID{tenant, testID} {
		return nil, false, domain.InternalControlError(domain.ErrNotFound, domain.IntCtlResultNotFound)
	}
	if t, ok := f.evidence[e.To.ID]; !ok || t != tenant {
		return nil, false, domain.NewNotFoundError("Evidence", e.To.ID)
	}
	key := [3]uuid.UUID{testID, resultID, e.To.ID}
	if stored, ok := f.links[key]; ok {
		return &stored, false, nil
	}
	v := ResultEvidenceView{
		ID: e.ID, TestID: testID, ResultID: resultID, EvidenceID: e.To.ID, Source: e.Source, SourceID: e.SourceID,
		Confidence: e.Confidence, Provenance: e.Provenance, CreatedAt: e.CreatedAt,
	}
	f.links[key] = v
	f.linkCount++
	f.audits = append(f.audits, a)
	return &v, true, nil
}

func (f *fakeResults) ListResultEvidence(_ context.Context, tenant, testID, resultID uuid.UUID) ([]ResultEvidenceView, error) {
	if _, ok := f.chain.liveTest(tenant, testID); !ok {
		return nil, testNotFoundErr()
	}
	if f.resultOf[resultID] != [2]uuid.UUID{tenant, testID} {
		return nil, domain.InternalControlError(domain.ErrNotFound, domain.IntCtlResultNotFound)
	}
	var out []ResultEvidenceView
	for k, v := range f.links {
		if k[0] == testID && k[1] == resultID {
			out = append(out, v)
		}
	}
	return out, nil
}

type resultFixture struct {
	*chainFixture
	results *fakeResults
	test    *TestView
	evA     uuid.UUID
	evB     uuid.UUID // tenant B's evidence
}

func newResultFixture() *resultFixture {
	cf := newChainFixture()
	impl := cf.newImpl(cf.assetA)
	rf := &resultFixture{chainFixture: cf, results: newFakeResults(cf.chain), test: cf.newTest(impl.ID)}
	rf.caller.Permissions = append(append([]string{}, rf.caller.Permissions...), PermissionAttachEvidence)
	rf.other.Permissions = append(append([]string{}, rf.other.Permissions...), PermissionAttachEvidence)
	rf.evA, rf.evB = uuid.New(), uuid.New()
	rf.results.evidence[rf.evA] = tenantA
	rf.results.evidence[rf.evB] = tenantB
	return rf
}

func (rf *resultFixture) record(c Caller, in RecordResultInput) (*ResultView, error) {
	return NewRecordResultUseCase(rf.results, rf.owners, clock).Execute(context.Background(), c, rf.test.ID, in)
}

func passAt(at time.Time) RecordResultInput {
	return RecordResultInput{Outcome: "pass", ObservedAt: at}
}

// ---------------------------------------------------------------------------
// RecordResult

func TestRecordControlTestResult_Success(t *testing.T) {
	rf := newResultFixture()
	observed := now.Add(-2 * time.Hour).In(time.FixedZone("x", 3600))
	in := passAt(observed)
	in.Note = str("quarterly run")
	v, err := rf.record(rf.caller, in)
	require.NoError(t, err)
	assert.Equal(t, rf.test.ID, v.TestID)
	assert.Equal(t, canonical.OutcomePass, v.Outcome)
	assert.True(t, observed.Equal(v.ObservedAt))
	assert.Equal(t, time.UTC, v.ObservedAt.Location(), "observed_at is normalised to UTC")
	assert.Equal(t, now, v.RecordedAt, "recorded_at is the server clock")
	assert.Equal(t, canonical.SourceOpenRiskUser, v.Source)
	assert.Equal(t, v.ID.String(), v.SourceID)
	assert.Equal(t, 0.70, v.Confidence)
	assert.Equal(t, canonical.Actor{Kind: canonical.ActorUser, ID: rf.caller.UserID.String()}, v.ExecutedBy)
	last := rf.results.audits[len(rf.results.audits)-1]
	assert.Equal(t, "control_test_result", last.EntityType)
	assert.Equal(t, v.ID.String(), last.EntityID)
	assert.Equal(t, rf.caller.TenantID, last.TenantID)

	// Another executor must be a member of the tenant.
	in = passAt(now)
	in.ExecutedBy = &rf.owner
	v, err = rf.record(rf.caller, in)
	require.NoError(t, err)
	assert.Equal(t, rf.owner.String(), v.ExecutedBy.ID)
	stranger := uuid.New()
	in.ExecutedBy = &stranger
	_, err = rf.record(rf.caller, in)
	require.ErrorIs(t, err, domain.ErrValidation)
}

func TestRecordControlTestResult_NotFound(t *testing.T) {
	rf := newResultFixture()
	uc := NewRecordResultUseCase(rf.results, rf.owners, clock)
	_, err := uc.Execute(context.Background(), rf.other, rf.test.ID, passAt(now))
	require.ErrorIs(t, err, domain.ErrNotFound, "another tenant's test")
	_, err = uc.Execute(context.Background(), rf.caller, uuid.New(), passAt(now))
	require.ErrorIs(t, err, domain.ErrNotFound)
	assert.Empty(t, rf.results.results)
}

func TestRecordControlTestResult_Unauthorized(t *testing.T) {
	rf := newResultFixture()
	uc := NewRecordResultUseCase(rf.results, rf.owners, clock)
	_, err := uc.Execute(context.Background(), Caller{UserID: uuid.New(), Permissions: rf.caller.Permissions}, rf.test.ID, passAt(now))
	require.ErrorIs(t, err, domain.ErrUnauthorized, "no tenant")
	_, err = uc.Execute(context.Background(), Caller{TenantID: tenantA, Permissions: rf.caller.Permissions}, rf.test.ID, passAt(now))
	require.ErrorIs(t, err, domain.ErrUnauthorized, "no user")
	readOnly := rf.caller
	readOnly.Permissions = []string{PermissionRead, PermissionAttachEvidence}
	_, err = uc.Execute(context.Background(), readOnly, rf.test.ID, passAt(now))
	require.ErrorIs(t, err, domain.ErrForbidden)
	assert.Empty(t, rf.results.results)
}

func TestRecordControlTestResult_RejectsBadInput(t *testing.T) {
	rf := newResultFixture()
	long := string(make([]rune, maxLongText+1))
	for name, in := range map[string]RecordResultInput{
		"no observed_at":    {Outcome: "pass"},
		"unknown outcome":   passAtOutcome("maybe", now),
		"empty outcome":     passAtOutcome("", now),
		"future observed":   passAt(now.Add(maxObservedSkew + time.Second)),
		"note too long":     {Outcome: "pass", ObservedAt: now, Note: &long},
		"nil executor uuid": {Outcome: "pass", ObservedAt: now, ExecutedBy: &uuid.UUID{}},
	} {
		_, err := rf.record(rf.caller, in)
		require.ErrorIs(t, err, domain.ErrValidation, name)
	}
	assert.Empty(t, rf.results.results)
	// Within the skew, and arbitrarily old, is fine.
	_, err := rf.record(rf.caller, passAt(now.Add(maxObservedSkew)))
	require.NoError(t, err)
	_, err = rf.record(rf.caller, passAt(now.AddDate(-5, 0, 0)))
	require.NoError(t, err)
}

func passAtOutcome(outcome string, at time.Time) RecordResultInput {
	return RecordResultInput{Outcome: outcome, ObservedAt: at}
}

// ---------------------------------------------------------------------------
// ListResults and Latest

func TestListControlTestResults_Success(t *testing.T) {
	rf := newResultFixture()
	older, err := rf.record(rf.caller, passAt(now.Add(-48*time.Hour)))
	require.NoError(t, err)
	newer, err := rf.record(rf.caller, passAtOutcome("fail", now.Add(-time.Hour)))
	require.NoError(t, err)
	got, err := NewListResultsUseCase(rf.results).Execute(context.Background(), rf.caller, rf.test.ID, 0, 0)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, newer.ID, got[0].ID, "newest observed first")
	assert.Equal(t, older.ID, got[1].ID)

	page, err := NewListResultsUseCase(rf.results).Execute(context.Background(), rf.caller, rf.test.ID, 1, 1)
	require.NoError(t, err)
	require.Len(t, page, 1)
	assert.Equal(t, older.ID, page[0].ID)
	_, err = NewListResultsUseCase(rf.results).Execute(context.Background(), rf.caller, rf.test.ID, MaxListLimit+1, 0)
	require.ErrorIs(t, err, domain.ErrValidation)
}

func TestListControlTestResults_NotFound(t *testing.T) {
	rf := newResultFixture()
	_, err := rf.record(rf.caller, passAt(now))
	require.NoError(t, err)
	uc := NewListResultsUseCase(rf.results)
	_, err = uc.Execute(context.Background(), rf.other, rf.test.ID, 0, 0)
	require.ErrorIs(t, err, domain.ErrNotFound)
	_, err = uc.Execute(context.Background(), rf.caller, uuid.New(), 0, 0)
	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestListControlTestResults_Unauthorized(t *testing.T) {
	rf := newResultFixture()
	uc := NewListResultsUseCase(rf.results)
	_, err := uc.Execute(context.Background(), Caller{UserID: uuid.New(), Permissions: rf.caller.Permissions}, rf.test.ID, 0, 0)
	require.ErrorIs(t, err, domain.ErrUnauthorized)
	_, err = uc.Execute(context.Background(), noPerms(rf.caller), rf.test.ID, 0, 0)
	require.ErrorIs(t, err, domain.ErrForbidden)
}

func TestLatestControlTestResult_Success(t *testing.T) {
	rf := newResultFixture()
	_, err := rf.record(rf.caller, passAt(now.Add(-48*time.Hour)))
	require.NoError(t, err)
	newest, err := rf.record(rf.caller, passAtOutcome("inconclusive", now.Add(-time.Hour)))
	require.NoError(t, err)
	// Recorded last, observed earlier: not the latest.
	_, err = rf.record(rf.caller, passAt(now.Add(-72*time.Hour)))
	require.NoError(t, err)
	v, err := NewLatestResultUseCase(rf.results).Execute(context.Background(), rf.caller, rf.test.ID)
	require.NoError(t, err)
	assert.Equal(t, newest.ID, v.ID)
}

func TestLatestControlTestResult_NotFound(t *testing.T) {
	rf := newResultFixture()
	uc := NewLatestResultUseCase(rf.results)
	_, err := uc.Execute(context.Background(), rf.caller, rf.test.ID)
	require.ErrorIs(t, err, domain.ErrNotFound)
	assert.Equal(t, domain.IntCtlNoResult, code(err), "a test that never ran")

	_, err = uc.Execute(context.Background(), rf.caller, uuid.New())
	require.ErrorIs(t, err, domain.ErrNotFound)
	assert.Equal(t, domain.IntCtlTestNotFound, code(err), "no such test")
	_, err = uc.Execute(context.Background(), rf.other, rf.test.ID)
	assert.Equal(t, domain.IntCtlTestNotFound, code(err), "another tenant's test reads like an absent one")
}

func TestLatestControlTestResult_Unauthorized(t *testing.T) {
	rf := newResultFixture()
	uc := NewLatestResultUseCase(rf.results)
	_, err := uc.Execute(context.Background(), Caller{UserID: uuid.New(), Permissions: rf.caller.Permissions}, rf.test.ID)
	require.ErrorIs(t, err, domain.ErrUnauthorized)
	_, err = uc.Execute(context.Background(), noPerms(rf.caller), rf.test.ID)
	require.ErrorIs(t, err, domain.ErrForbidden)
}

// ---------------------------------------------------------------------------
// AttachEvidence

func TestAttachControlTestResultEvidence_Success(t *testing.T) {
	rf := newResultFixture()
	r1, err := rf.record(rf.caller, passAt(now))
	require.NoError(t, err)
	r2, err := rf.record(rf.caller, passAt(now.Add(-time.Hour)))
	require.NoError(t, err)
	uc := NewAttachResultEvidenceUseCase(rf.results, clock)
	link, created, err := uc.Execute(context.Background(), rf.caller, rf.test.ID, r1.ID, rf.evA)
	require.NoError(t, err)
	assert.True(t, created)
	assert.Equal(t, r1.ID, link.ResultID, "the edge carries the result id as its qualifier")
	assert.Equal(t, rf.test.ID, link.TestID)
	assert.Equal(t, rf.evA, link.EvidenceID)
	assert.Equal(t, canonical.SourceOpenRiskUser, link.Source)
	assert.Equal(t, link.ID.String(), link.SourceID)
	assert.Equal(t, 0.70, link.Confidence)
	assert.Equal(t, canonical.Actor{Kind: canonical.ActorUser, ID: rf.caller.UserID.String()}, link.Provenance.Actor)
	last := rf.results.audits[len(rf.results.audits)-1]
	assert.Equal(t, "control_test_result", last.EntityType)
	assert.Equal(t, r1.ID.String(), last.EntityID)

	// The same pair again: the stored edge, no second audit entry.
	audits := len(rf.results.audits)
	again, created, err := uc.Execute(context.Background(), rf.caller, rf.test.ID, r1.ID, rf.evA)
	require.NoError(t, err)
	assert.False(t, created)
	assert.Equal(t, link.ID, again.ID, "the stored edge, not the new one")
	assert.Len(t, rf.results.audits, audits)

	// The same artifact backs a second result of the same test.
	_, created, err = uc.Execute(context.Background(), rf.caller, rf.test.ID, r2.ID, rf.evA)
	require.NoError(t, err)
	assert.True(t, created)
	got, err := NewListResultEvidenceUseCase(rf.results).Execute(context.Background(), rf.caller, rf.test.ID, r1.ID)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, rf.evA, got[0].EvidenceID)
}

func TestAttachControlTestResultEvidence_NotFound(t *testing.T) {
	rf := newResultFixture()
	r, err := rf.record(rf.caller, passAt(now))
	require.NoError(t, err)
	uc := NewAttachResultEvidenceUseCase(rf.results, clock)
	ctx := context.Background()
	_, _, err = uc.Execute(ctx, rf.caller, rf.test.ID, r.ID, rf.evB)
	require.ErrorIs(t, err, domain.ErrNotFound, "another tenant's evidence")
	_, _, err = uc.Execute(ctx, rf.caller, rf.test.ID, r.ID, uuid.New())
	require.ErrorIs(t, err, domain.ErrNotFound, "no such evidence")
	_, _, err = uc.Execute(ctx, rf.caller, rf.test.ID, uuid.New(), rf.evA)
	require.ErrorIs(t, err, domain.ErrNotFound, "no such result")
	_, _, err = uc.Execute(ctx, rf.caller, uuid.New(), r.ID, rf.evA)
	require.ErrorIs(t, err, domain.ErrNotFound, "no such test")
	_, _, err = uc.Execute(ctx, rf.other, rf.test.ID, r.ID, rf.evB)
	require.ErrorIs(t, err, domain.ErrNotFound, "another tenant's test")

	// A result of another test of the same tenant is not this test's result.
	other := NewCreateTestUseCase(rf.chain, clock)
	impl := rf.newImpl(rf.assetA2)
	t2, err := other.Execute(ctx, rf.caller, impl.ID, manualTest("other"))
	require.NoError(t, err)
	_, _, err = uc.Execute(ctx, rf.caller, t2.ID, r.ID, rf.evA)
	require.ErrorIs(t, err, domain.ErrNotFound, "a result of another test")
	assert.Zero(t, rf.results.linkCount)
}

func TestAttachControlTestResultEvidence_Unauthorized(t *testing.T) {
	rf := newResultFixture()
	r, err := rf.record(rf.caller, passAt(now))
	require.NoError(t, err)
	uc := NewAttachResultEvidenceUseCase(rf.results, clock)
	ctx := context.Background()
	_, _, err = uc.Execute(ctx, Caller{UserID: uuid.New(), Permissions: rf.caller.Permissions}, rf.test.ID, r.ID, rf.evA)
	require.ErrorIs(t, err, domain.ErrUnauthorized)
	// compliance:controls:update alone is not enough: attaching is an evidence write.
	ctlOnly := rf.caller
	ctlOnly.Permissions = []string{PermissionRead, PermissionCreate, PermissionUpdate, PermissionDelete}
	_, _, err = uc.Execute(ctx, ctlOnly, rf.test.ID, r.ID, rf.evA)
	require.ErrorIs(t, err, domain.ErrForbidden)
	assert.Zero(t, rf.results.linkCount)
	ok := rf.caller
	ok.Permissions = []string{PermissionAttachEvidence}
	_, created, err := uc.Execute(ctx, ok, rf.test.ID, r.ID, rf.evA)
	require.NoError(t, err)
	assert.True(t, created)
}

func TestListControlTestResultEvidence_Success(t *testing.T) {
	rf := newResultFixture()
	r, err := rf.record(rf.caller, passAt(now))
	require.NoError(t, err)
	uc := NewListResultEvidenceUseCase(rf.results)
	got, err := uc.Execute(context.Background(), rf.caller, rf.test.ID, r.ID)
	require.NoError(t, err)
	assert.Empty(t, got)
	assert.NotNil(t, got)
	_, _, err = NewAttachResultEvidenceUseCase(rf.results, clock).Execute(context.Background(), rf.caller, rf.test.ID, r.ID, rf.evA)
	require.NoError(t, err)
	got, err = uc.Execute(context.Background(), rf.caller, rf.test.ID, r.ID)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, rf.evA, got[0].EvidenceID)
}

func TestListControlTestResultEvidence_NotFound(t *testing.T) {
	rf := newResultFixture()
	r, err := rf.record(rf.caller, passAt(now))
	require.NoError(t, err)
	uc := NewListResultEvidenceUseCase(rf.results)
	ctx := context.Background()
	_, err = uc.Execute(ctx, rf.other, rf.test.ID, r.ID)
	require.ErrorIs(t, err, domain.ErrNotFound)
	_, err = uc.Execute(ctx, rf.caller, rf.test.ID, uuid.New())
	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestListControlTestResultEvidence_Unauthorized(t *testing.T) {
	rf := newResultFixture()
	r, err := rf.record(rf.caller, passAt(now))
	require.NoError(t, err)
	uc := NewListResultEvidenceUseCase(rf.results)
	ctx := context.Background()
	_, err = uc.Execute(ctx, noPerms(rf.caller), rf.test.ID, r.ID)
	require.ErrorIs(t, err, domain.ErrForbidden)
	_, err = uc.Execute(ctx, Caller{UserID: uuid.New()}, rf.test.ID, r.ID)
	require.ErrorIs(t, err, domain.ErrUnauthorized)
}
