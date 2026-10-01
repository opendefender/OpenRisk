// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package risk

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
)

// fakeTx stages creates and only "commits" them when fn succeeds, which is the
// property the import depends on.
type fakeTx struct {
	committed []*domain.Risk
	failOn    int // 1-based create call that errors; 0 never
	assets    []*domain.Asset
}

func (f *fakeTx) run(ctx context.Context, fn func(repo domain.RiskRepository, assets RiskAssetStore) error) error {
	var staged []*domain.Risk
	calls := 0
	write := func(r *domain.Risk) error {
		calls++
		if calls == f.failOn {
			return errors.New("disk full")
		}
		staged = append(staged, r)
		return nil
	}
	repo := &MockRiskRepository{createFunc: func(_ context.Context, r *domain.Risk) error { return write(r) }}
	if err := fn(repo, &fakeImportAssetStore{tx: f, write: write}); err != nil {
		return err
	}
	f.committed = append(f.committed, staged...)
	return nil
}

// list is the ImportAssetLister over the fake's assets, tenant-scoped.
func (f *fakeTx) list(_ context.Context, tenantID uuid.UUID) ([]ImportAssetRef, error) {
	var out []ImportAssetRef
	for _, a := range f.assets {
		if a.TenantID == tenantID {
			out = append(out, ImportAssetRef{ID: a.ID, Name: a.Name})
		}
	}
	return out, nil
}

type fakeImportAssetStore struct {
	tx    *fakeTx
	write func(*domain.Risk) error
}

func (s *fakeImportAssetStore) FindByIDs(_ context.Context, tenantID uuid.UUID, ids []uuid.UUID) ([]*domain.Asset, error) {
	var out []*domain.Asset
	for _, a := range s.tx.assets {
		for _, id := range ids {
			if a.ID == id && a.TenantID == tenantID {
				out = append(out, a)
			}
		}
	}
	return out, nil
}

func (s *fakeImportAssetStore) SaveWithAssets(_ context.Context, r *domain.Risk, assets []*domain.Asset, _ bool) error {
	r.Assets = assets
	return s.write(r)
}

func importCSV(t *testing.T, tx *fakeTx, csv string) (*ImportRisksResult, error) {
	t.Helper()
	return NewImportRisksUseCase(tx.run).Execute(context.Background(), uuid.New(), ImportRisksInput{
		CSV: []byte(csv), ImportedBy: uuid.New(),
	})
}

func rejectedErrors(t *testing.T, err error) []ImportRowError {
	t.Helper()
	var rej *ImportRejectedError
	require.ErrorAs(t, err, &rej)
	require.ErrorIs(t, err, domain.ErrValidation)
	assert.Equal(t, 0, rej.Result.Created)
	return rej.Result.Errors
}

func TestImportRisks_Success(t *testing.T) {
	tx := &fakeTx{}
	res, err := importCSV(t, tx, "title,description,probability,impact,tags,frameworks\n"+
		"Phishing,Credential theft,0.6,8,\"email;people\",ISO27001\n"+
		"Ransomware,,0.3,10,,\n")
	require.NoError(t, err)

	assert.Equal(t, 2, res.Created)
	assert.Equal(t, 0, res.Rejected)
	assert.Empty(t, res.Errors)
	require.Len(t, tx.committed, 2)

	r := tx.committed[0]
	assert.Equal(t, "Phishing", r.Title)
	assert.Equal(t, domain.SourceImport, r.Source)
	assert.Equal(t, []string{"email", "people"}, []string(r.Tags))
	assert.Equal(t, []string{"ISO27001"}, []string(r.Frameworks))
	// Scored exactly as CreateRiskUseCase scores a hand-made risk.
	assert.InDelta(t, 4.8, r.Score, 1e-9)
	assert.Equal(t, domain.CriticalityFromScore(4.8), r.Criticality)
	assert.Equal(t, []uuid.UUID{tx.committed[0].ID, tx.committed[1].ID}, res.RiskIDs)
}

func TestImportRisks_NotFound(t *testing.T) {
	// The import's "not found" case is a file with nothing in it to import.
	for name, csv := range map[string]string{
		"empty":       "",
		"header only": "title,probability,impact\n",
		"blank rows":  "title,probability,impact\n,,\n\n",
	} {
		t.Run(name, func(t *testing.T) {
			tx := &fakeTx{}
			_, err := importCSV(t, tx, csv)
			errs := rejectedErrors(t, err)
			require.Len(t, errs, 1)
			assert.Equal(t, 0, errs[0].Line)
			assert.Empty(t, tx.committed)
		})
	}
}

func TestImportRisks_Unauthorized(t *testing.T) {
	tx := &fakeTx{}
	_, err := NewImportRisksUseCase(tx.run).Execute(context.Background(), uuid.Nil, ImportRisksInput{
		CSV: []byte("title,probability,impact\nX,0.5,5\n"),
	})
	require.ErrorIs(t, err, domain.ErrUnauthorized)
	assert.Empty(t, tx.committed)
}

func TestImportRisks_Validation_OneBadRowImportsNothing(t *testing.T) {
	tx := &fakeTx{}
	_, err := importCSV(t, tx, "title,probability,impact\n"+
		"Good,0.5,5\n"+
		",1.5,abc\n"+
		"Also good,0.1,2\n"+
		"Too big,0.2,11\n")
	errs := rejectedErrors(t, err)
	assert.Empty(t, tx.committed, "a file with an invalid row must persist nothing")

	got := map[string]bool{}
	for _, e := range errs {
		got[fmt.Sprintf("%d:%s", e.Line, e.Column)] = true
	}
	assert.Equal(t, map[string]bool{
		"3:title": true, "3:probability": true, "3:impact": true, "5:impact": true,
	}, got, "every error is reported with its line and column")

	var rej *ImportRejectedError
	require.ErrorAs(t, err, &rej)
	assert.Equal(t, 2, rej.Result.Rejected)
}

func TestImportRisks_WriteFailureRollsBackEverything(t *testing.T) {
	tx := &fakeTx{failOn: 2}
	_, err := importCSV(t, tx, "title,probability,impact\nA,0.5,5\nB,0.5,5\nC,0.5,5\n")
	require.Error(t, err)
	assert.Empty(t, tx.committed)
}

func TestImportRisks_LegacyScaleIsRefusedNotConverted(t *testing.T) {
	for name, csv := range map[string]string{
		"old template": "Title,Description,Probability,Impact\n\"Web API\",\"x\",3,4\n\"DB\",\"y\",4,5\n",
		// Every probability is 1: valid on the 0–1 scale, but this is the old
		// file's floor, and reading it as "certain" would be silent.
		"all ones": "title,probability,impact\nA,1,2\nB,1,5\n",
	} {
		t.Run(name, func(t *testing.T) {
			tx := &fakeTx{}
			_, err := importCSV(t, tx, csv)
			errs := rejectedErrors(t, err)
			require.Len(t, errs, 1)
			assert.Contains(t, errs[0].Message, "1–5 scale")
			assert.Empty(t, tx.committed)
		})
	}
}

func TestImportRisks_FrenchSpreadsheetExport(t *testing.T) {
	tx := &fakeTx{}
	res, err := importCSV(t, tx, "\xef\xbb\xbftitre;probability;impact\n")
	errs := rejectedErrors(t, err)
	assert.Equal(t, "titre", errs[0].Column, "unknown columns are named, not ignored")
	assert.Nil(t, res)

	res, err = importCSV(t, tx, "\xef\xbb\xbfTitle;Probability;Impact;Tags\r\nFuite;0,4;7,5;a, b\r\n")
	require.NoError(t, err)
	require.Equal(t, 1, res.Created)
	assert.InDelta(t, 0.4, tx.committed[0].Probability, 1e-9)
	assert.InDelta(t, 7.5, tx.committed[0].Impact, 1e-9)
	assert.Equal(t, []string{"a", "b"}, []string(tx.committed[0].Tags))
}

func TestImportRisks_HeaderErrors(t *testing.T) {
	tx := &fakeTx{}
	_, err := importCSV(t, tx, "title,probability,status\nA,0.5,Open\n")
	errs := rejectedErrors(t, err)
	cols := []string{}
	for _, e := range errs {
		assert.Equal(t, 1, e.Line)
		cols = append(cols, e.Column)
	}
	assert.ElementsMatch(t, []string{"status", "impact"}, cols)
}

func TestImportRisks_TooManyRows(t *testing.T) {
	var b strings.Builder
	b.WriteString("title,probability,impact\n")
	for i := 0; i <= MaxImportRows; i++ {
		fmt.Fprintf(&b, "R%d,0.5,5\n", i)
	}
	tx := &fakeTx{}
	_, err := importCSV(t, tx, b.String())
	errs := rejectedErrors(t, err)
	assert.Contains(t, errs[0].Message, "more than")
}

func TestImportRisks_OverCapacityWritesNothing(t *testing.T) {
	tx := &fakeTx{}
	uc := NewImportRisksUseCase(tx.run).WithCapacity(func(context.Context, uuid.UUID) (int, error) { return 1, nil })
	_, err := uc.Execute(context.Background(), uuid.New(), ImportRisksInput{
		CSV: []byte("title,probability,impact\nA,0.5,5\nB,0.5,5\n"),
	})
	var over *ImportOverCapacityError
	require.ErrorAs(t, err, &over)
	assert.Equal(t, 2, over.Requested)
	assert.Equal(t, 1, over.Remaining)
	assert.Empty(t, tx.committed)

	// Unlimited (-1) lets it through.
	uc = NewImportRisksUseCase(tx.run).WithCapacity(func(context.Context, uuid.UUID) (int, error) { return -1, nil })
	res, err := uc.Execute(context.Background(), uuid.New(), ImportRisksInput{
		CSV: []byte("title,probability,impact\nA,0.5,5\nB,0.5,5\n"),
	})
	require.NoError(t, err)
	assert.Equal(t, 2, res.Created)
}

// The page translates errors from Code and Params, so every error carries a
// code and the codes below are a contract with importRisksSchema.ts.
func TestImportRisks_ErrorsCarryCodesForTranslation(t *testing.T) {
	tx := &fakeTx{}
	_, err := importCSV(t, tx, "title,probability,impact\n"+
		",1.5,abc\n"+
		"Too big,0.2,11\n")
	errs := rejectedErrors(t, err)

	got := map[string]ImportRowError{}
	for _, e := range errs {
		require.NotEmpty(t, e.Code, "error without a code: %+v", e)
		got[fmt.Sprintf("%d:%s", e.Line, e.Column)] = e
	}
	assert.Equal(t, "required", got["2:title"].Code)
	assert.Equal(t, "out_of_range", got["2:probability"].Code)
	assert.Equal(t, map[string]string{"min": "0", "max": "1", "value": "1.5"}, got["2:probability"].Params)
	assert.Equal(t, "not_a_number", got["2:impact"].Code)
	assert.Equal(t, "abc", got["2:impact"].Params["value"])
	assert.Equal(t, "out_of_range", got["3:impact"].Code)
	assert.Equal(t, "10", got["3:impact"].Params["max"])

	_, err = importCSV(t, tx, "title,probability,impact\nA,3,4\n")
	assert.Equal(t, "legacy_scale", rejectedErrors(t, err)[0].Code)
	_, err = importCSV(t, tx, "titre,probability,impact\n")
	assert.Equal(t, "unknown_column", rejectedErrors(t, err)[0].Code)
	assert.Empty(t, tx.committed)
}

// #755 + #792: the "assets" column links each risk to the tenant's assets by
// name or id, and the stored score is the engine's with their criticality.
func TestImportRisks_AssetsColumnLinksAndScores(t *testing.T) {
	tenant := uuid.New()
	critical := &domain.Asset{ID: uuid.New(), TenantID: tenant, Name: "Core banking DB", Criticality: domain.CriticalityCritical}
	low := &domain.Asset{ID: uuid.New(), TenantID: tenant, Name: "Kiosk", Criticality: domain.CriticalityLow}
	tx := &fakeTx{assets: []*domain.Asset{critical, low}}

	res, err := NewImportRisksUseCase(tx.run).WithAssets(tx.list).Execute(context.Background(), tenant, ImportRisksInput{
		CSV: []byte("title,probability,impact,assets\n" +
			"Fraud,0.5,6,core banking db\n" +
			"Theft,0.5,6," + low.ID.String() + "\n" +
			"Unlinked,0.5,6,\n"),
		ImportedBy: uuid.New(),
	})
	require.NoError(t, err)
	require.Equal(t, 3, res.Created)
	require.Len(t, tx.committed, 3)

	fraud, theft, unlinked := tx.committed[0], tx.committed[1], tx.committed[2]
	require.Len(t, fraud.Assets, 1)
	assert.Equal(t, critical.ID, fraud.Assets[0].ID, "names match case-insensitively")
	require.Len(t, theft.Assets, 1)
	assert.Equal(t, low.ID, theft.Assets[0].ID, "ids are accepted")
	assert.Empty(t, unlinked.Assets)
	assert.Greater(t, fraud.Score, unlinked.Score, "a critical asset raises the score")
	assert.Less(t, theft.Score, unlinked.Score, "a low-criticality asset lowers it")
}

func TestImportRisks_UnknownOrAmbiguousAssetImportsNothing(t *testing.T) {
	tenant := uuid.New()
	other := uuid.New()
	tx := &fakeTx{assets: []*domain.Asset{
		{ID: uuid.New(), TenantID: tenant, Name: "Web"},
		{ID: uuid.New(), TenantID: tenant, Name: "web"},
		{ID: uuid.New(), TenantID: other, Name: "Payroll"},
	}}
	_, err := NewImportRisksUseCase(tx.run).WithAssets(tx.list).Execute(context.Background(), tenant, ImportRisksInput{
		CSV:        []byte("title,probability,impact,assets\nA,0.5,6,Web\nB,0.5,6,Payroll\nC,2,6,Ghost\n"),
		ImportedBy: uuid.New(),
	})
	errs := rejectedErrors(t, err)
	assert.Empty(t, tx.committed)

	codes := map[string]bool{}
	for _, e := range errs {
		codes[fmt.Sprintf("%d:%s:%s", e.Line, e.Column, e.Code)] = true
	}
	assert.Equal(t, map[string]bool{
		"2:assets:ambiguous_asset":   true,
		"3:assets:unknown_asset":     true, // another tenant's asset is not visible
		"4:probability:out_of_range": true,
		"4:assets:unknown_asset":     true, // checked even on a row with other errors
	}, codes)
}

func TestImportRisks_AssetsColumnWithoutResolverIsRefused(t *testing.T) {
	tx := &fakeTx{}
	_, err := importCSV(t, tx, "title,probability,impact,assets\nA,0.5,6,Web\n")
	assert.Equal(t, "assets_unavailable", rejectedErrors(t, err)[0].Code)
	assert.Empty(t, tx.committed)
}
