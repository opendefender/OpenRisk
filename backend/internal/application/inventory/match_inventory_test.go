// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package inventory

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	vulnapp "github.com/opendefender/openrisk/internal/application/vulnerability"
	"github.com/opendefender/openrisk/internal/domain"
)

type fakeOSV struct {
	advs  map[string][]Advisory // package name → advisories
	err   error
	asked []PackageQuery
}

func (f *fakeOSV) Lookup(_ context.Context, qs []PackageQuery) (map[PackageQuery][]Advisory, error) {
	f.asked = append(f.asked, qs...)
	if f.err != nil {
		return nil, f.err
	}
	out := map[PackageQuery][]Advisory{}
	for _, q := range qs {
		if a, ok := f.advs[q.Name]; ok {
			out[q] = a
		}
	}
	return out, nil
}

type fakeNVD struct{ known map[string][]Advisory }

func (f *fakeNVD) Lookup(_ context.Context, _, name, _ string) ([]Advisory, bool, error) {
	a, ok := f.known[name]
	return a, ok, nil
}

type fakeWriter struct{ batches []vulnapp.IngestInput }

func (w *fakeWriter) Execute(_ context.Context, _ uuid.UUID, in vulnapp.IngestInput) (*vulnapp.IngestResult, error) {
	w.batches = append(w.batches, in)
	return &vulnapp.IngestResult{Created: len(in.Findings)}, nil
}

type fakeCloser struct {
	calls []struct {
		prefix string
		keep   []string
	}
}

func (c *fakeCloser) CloseAbsent(_ context.Context, _ uuid.UUID, _ domain.VulnSource, prefix string, keep []string, _ time.Time) (int64, error) {
	c.calls = append(c.calls, struct {
		prefix string
		keep   []string
	}{prefix, keep})
	return 1, nil
}

type denyAll struct{}

func (denyAll) OnlineLookupEnabled(context.Context, uuid.UUID) bool { return false }

func storeInv(t *testing.T, repo *fakeRepo, tenant uuid.UUID, platform, version string, pkgs []domain.HostPackageInput) *domain.HostInventory {
	t.Helper()
	inv := &domain.HostInventory{TenantID: tenant, HostKey: "srv-core-01", Hostname: "srv-core-01",
		Platform: platform, OSVersion: version, PackageCount: len(pkgs)}
	require.NoError(t, repo.Replace(context.Background(), inv, pkgs))
	return inv
}

var ssl = Advisory{ID: "UBUNTU-CVE-2024-0727", CVE: "CVE-2024-0727", Summary: "PKCS12 NULL dereference",
	CVSS: 5.5, Severity: "low", Fixed: "3.0.2-0ubuntu1.14"}

func TestMatchInventory_Success(t *testing.T) {
	tenant := uuid.New()
	repo := newFakeRepo()
	inv := storeInv(t, repo, tenant, "ubuntu", "22.04.3 LTS", []domain.HostPackageInput{
		{Name: "openssl", Version: "3.0.2-0ubuntu1.10", Source: "deb"},
		{Name: "bash", Version: "5.1-6ubuntu1", Source: "deb"},
		{Name: "librpm", Version: "4.17", Source: "rpm"}, // not this platform's ecosystem
	})
	// Debian ships one flaw twice (DEBIAN-CVE and DSA): one finding.
	osv := &fakeOSV{advs: map[string][]Advisory{"openssl": {ssl, {ID: "USN-6622-1", CVE: "CVE-2024-0727"}}}}
	w, c := &fakeWriter{}, &fakeCloser{}
	uc := NewMatchInventoryUseCase(repo, osv, nil, w, c)

	res, err := uc.Execute(context.Background(), tenant, inv.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, res.Findings)
	assert.Equal(t, 1, res.Uncheckable)
	require.Len(t, w.batches, 1)
	in := w.batches[0]
	assert.True(t, in.FromAgent && in.ReopenRemediated)
	assert.Equal(t, domain.VulnSourceAgent, in.Source)
	f := in.Findings[0]
	assert.Equal(t, FindingID("srv-core-01", "Ubuntu:22.04:LTS", "openssl", "CVE-2024-0727"), f["external_id"])
	assert.Equal(t, "3.0.2-0ubuntu1.14", f["fixed_version"])
	assert.Equal(t, LookupOSV, f["lookup"])
	assert.Equal(t, "srv-core-01", f["hostname"])

	require.Len(t, c.calls, 1)
	assert.Equal(t, "srv-core-01|", c.calls[0].prefix)
	assert.Equal(t, []string{FindingID("srv-core-01", "Ubuntu:22.04:LTS", "openssl", "CVE-2024-0727")}, c.calls[0].keep)

	got, _ := repo.Get(context.Background(), tenant, inv.ID)
	assert.NotNil(t, got.MatchedAt)
	assert.Empty(t, got.MatchError)
	assert.Equal(t, 1, got.UncheckableCount)
}

// The upgraded host reports no flaw: nothing is written and every open
// finding of the host is closed.
func TestMatchInventory_FixedVersionCloses(t *testing.T) {
	tenant := uuid.New()
	repo := newFakeRepo()
	inv := storeInv(t, repo, tenant, "ubuntu", "22.04 LTS", []domain.HostPackageInput{
		{Name: "openssl", Version: "3.0.2-0ubuntu1.14", Source: "deb"}})
	w, c := &fakeWriter{}, &fakeCloser{}
	res, err := NewMatchInventoryUseCase(repo, &fakeOSV{}, nil, w, c).Execute(context.Background(), tenant, inv.ID)
	require.NoError(t, err)
	assert.Empty(t, w.batches)
	require.Len(t, c.calls, 1)
	assert.Empty(t, c.calls[0].keep)
	assert.Equal(t, 1, res.Closed)
}

// OSV down: the error is recorded and nothing is closed.
func TestMatchInventory_UnreachableClosesNothing(t *testing.T) {
	tenant := uuid.New()
	repo := newFakeRepo()
	inv := storeInv(t, repo, tenant, "debian", "12", []domain.HostPackageInput{{Name: "openssl", Version: "1", Source: "deb"}})
	c := &fakeCloser{}
	_, err := NewMatchInventoryUseCase(repo, &fakeOSV{err: errors.New("dial tcp: timeout")}, nil, &fakeWriter{}, c).
		Execute(context.Background(), tenant, inv.ID)
	require.Error(t, err)
	assert.Empty(t, c.calls)
	got, _ := repo.Get(context.Background(), tenant, inv.ID)
	assert.Equal(t, MatchErrUnreachable, got.MatchError)
}

// Windows: a program NVD resolves is checked and marked as a CPE match; one
// it cannot resolve is counted not checkable.
func TestMatchInventory_Windows(t *testing.T) {
	tenant := uuid.New()
	repo := newFakeRepo()
	inv := storeInv(t, repo, tenant, "windows", "10.0.22631", []domain.HostPackageInput{
		{Name: "7-Zip 23.01 (x64)", Version: "23.01", Publisher: "Igor Pavlov", Source: "programs"},
		{Name: "In-house tool", Version: "1.0", Publisher: "Contoso", Source: "programs"},
	})
	nvd := &fakeNVD{known: map[string][]Advisory{"7-Zip 23.01 (x64)": {{ID: "CVE-2024-11477", CVE: "CVE-2024-11477", CVSS: 7.8, Severity: "high"}}}}
	w := &fakeWriter{}
	res, err := NewMatchInventoryUseCase(repo, &fakeOSV{}, nvd, w, &fakeCloser{}).Execute(context.Background(), tenant, inv.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, res.Findings)
	assert.Equal(t, 1, res.Uncheckable)
	assert.Equal(t, LookupNVDCPE, w.batches[0].Findings[0]["lookup"])
}

type flakyNVD struct{}

func (flakyNVD) Lookup(_ context.Context, _, name, _ string) ([]Advisory, bool, error) {
	if name == "Broken" {
		return nil, true, errors.New("NVD HTTP 400")
	}
	return []Advisory{{ID: "CVE-2024-11477", CVE: "CVE-2024-11477", CVSS: 7.8}}, true, nil
}

// One program NVD refuses does not hide the others, and nothing is closed
// while any lookup failed (security review of #910).
func TestMatchInventory_OneProgramFails(t *testing.T) {
	tenant := uuid.New()
	repo := newFakeRepo()
	inv := storeInv(t, repo, tenant, "windows", "10.0", []domain.HostPackageInput{
		{Name: "Broken", Version: "1\\x", Source: "programs"},
		{Name: "7-Zip", Version: "23.01", Source: "programs"},
	})
	w, c := &fakeWriter{}, &fakeCloser{}
	res, err := NewMatchInventoryUseCase(repo, &fakeOSV{}, flakyNVD{}, w, c).Execute(context.Background(), tenant, inv.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, res.Findings)
	assert.Equal(t, 1, res.Uncheckable)
	assert.Empty(t, c.calls, "no closing after a failed lookup")
	got, _ := repo.Get(context.Background(), tenant, inv.ID)
	assert.Equal(t, MatchErrUnreachable, got.MatchError)
}

func TestFindingID_FixedLength(t *testing.T) {
	long := strings.Repeat("x", 256)
	id := FindingID(strings.Repeat("h", 63), "Debian:12", long, "CVE-2024-0727")
	assert.LessOrEqual(t, len(id), 255)
	assert.True(t, strings.HasPrefix(id, strings.Repeat("h", 63)+"|"))
	assert.NotEqual(t, id, FindingID(strings.Repeat("h", 63), "Debian:12", long, "CVE-2024-0728"))
}

func TestMatchInventory_Disabled(t *testing.T) {
	tenant := uuid.New()
	repo := newFakeRepo()
	inv := storeInv(t, repo, tenant, "debian", "12", []domain.HostPackageInput{{Name: "openssl", Version: "1", Source: "deb"}})
	osv, c := &fakeOSV{}, &fakeCloser{}
	_, err := NewMatchInventoryUseCase(repo, osv, nil, &fakeWriter{}, c).WithPolicy(denyAll{}).
		Execute(context.Background(), tenant, inv.ID)
	require.NoError(t, err)
	assert.Empty(t, osv.asked, "nothing leaves when the tenant switched lookups off")
	assert.Empty(t, c.calls)
	got, _ := repo.Get(context.Background(), tenant, inv.ID)
	assert.Equal(t, MatchErrDisabled, got.MatchError)
}

func TestMatchInventory_NotFound(t *testing.T) {
	_, err := NewMatchInventoryUseCase(newFakeRepo(), &fakeOSV{}, nil, &fakeWriter{}, &fakeCloser{}).
		Execute(context.Background(), uuid.New(), uuid.New())
	assert.True(t, errors.Is(err, domain.ErrNotFound))
}

// Another tenant's inventory id reads as not found, and nothing is looked up.
func TestMatchInventory_Unauthorized(t *testing.T) {
	mine, theirs := uuid.New(), uuid.New()
	repo := newFakeRepo()
	inv := storeInv(t, repo, theirs, "debian", "12", []domain.HostPackageInput{{Name: "openssl", Version: "1", Source: "deb"}})
	osv := &fakeOSV{}
	uc := NewMatchInventoryUseCase(repo, osv, nil, &fakeWriter{}, &fakeCloser{})
	_, err := uc.Execute(context.Background(), mine, inv.ID)
	assert.True(t, errors.Is(err, domain.ErrNotFound))
	assert.Empty(t, osv.asked)
	_, err = uc.Execute(context.Background(), uuid.Nil, inv.ID)
	assert.True(t, errors.Is(err, domain.ErrUnauthorized))
}

func TestFirstCVE(t *testing.T) {
	assert.Equal(t, "CVE-2024-0727", FirstCVE("UBUNTU-CVE-2024-0727", []string{"CVE-2024-0727"}))
	assert.Equal(t, "CVE-2023-12345", FirstCVE("cve-2023-12345", nil), "the id itself, upper-cased")
	assert.Equal(t, "", FirstCVE("DSA-5000-1", []string{"DEBIAN-123"}))
}
