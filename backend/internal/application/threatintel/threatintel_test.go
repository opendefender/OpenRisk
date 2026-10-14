// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package threatintel

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/pkg/cti"
)

// fakeTenant keeps every tenant's data apart, as the repository does.
type fakeTenant struct {
	assets    map[uuid.UUID][]AssetCPEs
	findings  map[uuid.UUID][]Finding
	dismissed map[uuid.UUID]map[string]time.Time
}

func newFakeTenant() *fakeTenant {
	return &fakeTenant{
		assets:    map[uuid.UUID][]AssetCPEs{},
		findings:  map[uuid.UUID][]Finding{},
		dismissed: map[uuid.UUID]map[string]time.Time{},
	}
}

func (f *fakeTenant) AssetsWithCPEs(_ context.Context, t uuid.UUID) ([]AssetCPEs, error) {
	return f.assets[t], nil
}
func (f *fakeTenant) FindingsWithCVE(_ context.Context, t uuid.UUID) ([]Finding, error) {
	return f.findings[t], nil
}
func (f *fakeTenant) Dismissals(_ context.Context, t uuid.UUID) (map[string]time.Time, error) {
	return f.dismissed[t], nil
}
func (f *fakeTenant) Dismiss(_ context.Context, d *domain.CTIThreatDismissal) error {
	if f.dismissed[d.TenantID] == nil {
		f.dismissed[d.TenantID] = map[string]time.Time{}
	}
	if _, ok := f.dismissed[d.TenantID][d.CVEID]; !ok {
		f.dismissed[d.TenantID][d.CVEID] = d.CreatedAt
	}
	return nil
}
func (f *fakeTenant) Restore(_ context.Context, t uuid.UUID, cve string) (bool, error) {
	if _, ok := f.dismissed[t][cve]; !ok {
		return false, nil
	}
	delete(f.dismissed[t], cve)
	return true, nil
}

type fakeCatalogue struct {
	vulns []cti.CTIVulnerability
	err   error
}

func (c *fakeCatalogue) ByCPEs(_ context.Context, cpes []string) ([]cti.CTIVulnerability, error) {
	if c.err != nil {
		return nil, c.err
	}
	want := map[string]bool{}
	for _, x := range cpes {
		want[x] = true
	}
	var out []cti.CTIVulnerability
	for _, v := range c.vulns {
		for _, a := range v.AffectedCPE {
			if want[a] {
				out = append(out, v)
				break
			}
		}
	}
	return out, nil
}
func (c *fakeCatalogue) ByCVEs(_ context.Context, ids []string) ([]cti.CTIVulnerability, error) {
	want := map[string]bool{}
	for _, x := range ids {
		want[x] = true
	}
	var out []cti.CTIVulnerability
	for _, v := range c.vulns {
		if want[strings.ToUpper(v.CVEID)] {
			out = append(out, v)
		}
	}
	return out, nil
}
func (c *fakeCatalogue) GetByCVE(_ context.Context, id string) (*cti.CTIVulnerability, error) {
	for _, v := range c.vulns {
		if v.CVEID == id {
			vv := v
			return &vv, nil
		}
	}
	return nil, nil
}

const sslvpn = "cpe:2.3:o:vendor:sslvpn:7.4.5"

func catalogue() *fakeCatalogue {
	return &fakeCatalogue{vulns: []cti.CTIVulnerability{
		{CVEID: "CVE-2026-21873", CVSSV3: 9.8, Severity: "CRITICAL", CISAKnown: true,
			PublishedAt:     time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC),
			MitreTechniques: []string{"T1190", "T1133"}, AffectedCPE: []string{sslvpn}},
		{CVEID: "CVE-2026-17760", CVSSV3: 5.4, Severity: "MEDIUM", AffectedCPE: []string{"cpe:2.3:a:kora:rh:3"}},
		{CVEID: "CVE-2026-0001", CVSSV3: 7.0, Severity: "HIGH", AffectedCPE: []string{"cpe:2.3:a:nobody:uses:1"}},
	}}
}

func TestListThreats_Success(t *testing.T) {
	tenant := uuid.New()
	fw, hr := uuid.New(), uuid.New()
	ft := newFakeTenant()
	ft.assets[tenant] = []AssetCPEs{
		{ID: fw, Name: "Pare-feu périmétrique", Criticality: "CRITICAL", CPEs: []string{sslvpn}},
		{ID: hr, Name: "Kora RH", Criticality: "MEDIUM", CPEs: []string{"cpe:2.3:a:kora:rh:3"}},
	}
	// The scanner also saw the SSL-VPN flaw on the firewall.
	ft.findings[tenant] = []Finding{{CVEID: "cve-2026-21873", Title: "Exécution de code à distance — SSL-VPN",
		AssetID: &fw, AssetName: "Pare-feu périmétrique", Criticality: "CRITICAL", EPSS: 0.94, Source: "nessus"}}

	res, err := NewListThreatsUseCase(ft, catalogue()).Execute(context.Background(), tenant)
	require.NoError(t, err)
	require.Len(t, res.Threats, 2, "a catalogue entry touching no asset is not a threat")
	assert.Equal(t, 2, res.AssetsWithCPEs)

	top := res.Threats[0]
	assert.Equal(t, "CVE-2026-21873", top.CVEID)
	assert.Equal(t, 100, top.Relevance)
	assert.Equal(t, "Exécution de code à distance — SSL-VPN", top.Title)
	require.Len(t, top.Assets, 1, "one asset reached two ways is listed once")
	assert.ElementsMatch(t, []string{"cpe", "scan"}, top.Assets[0].Via)
	assert.Equal(t, []string{"nvd", "cisa_kev", "mitre_attack", "nessus"}, top.Sources)
	assert.InDelta(t, 0.94, top.EPSS, 1e-9)
	assert.Nil(t, top.DismissedAt)

	low := res.Threats[1]
	assert.Equal(t, "CVE-2026-17760", low.CVEID)
	assert.Equal(t, 20+0+11+5, low.Relevance)
	assert.Equal(t, []string{}, low.Techniques)
}

// Nothing to match on is an empty list with the reason, never an error.
func TestListThreats_NotFound(t *testing.T) {
	res, err := NewListThreatsUseCase(newFakeTenant(), catalogue()).Execute(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Empty(t, res.Threats)
	assert.Equal(t, 0, res.AssetsWithCPEs)
}

// Another tenant's assets, findings and dismissals never shape this list.
func TestListThreats_Unauthorized(t *testing.T) {
	mine, theirs := uuid.New(), uuid.New()
	ft := newFakeTenant()
	ft.assets[theirs] = []AssetCPEs{{ID: uuid.New(), Name: "their firewall", Criticality: "CRITICAL", CPEs: []string{sslvpn}}}
	ft.assets[mine] = []AssetCPEs{{ID: uuid.New(), Name: "my firewall", Criticality: "HIGH", CPEs: []string{sslvpn}}}
	ft.dismissed[theirs] = map[string]time.Time{"CVE-2026-21873": time.Now()}

	res, err := NewListThreatsUseCase(ft, catalogue()).Execute(context.Background(), mine)
	require.NoError(t, err)
	require.Len(t, res.Threats, 1)
	assert.Equal(t, "my firewall", res.Threats[0].Assets[0].Name)
	assert.Nil(t, res.Threats[0].DismissedAt, "their dismissal is not mine")

	failing := NewListThreatsUseCase(ft, &fakeCatalogue{err: errors.New("down")})
	_, err = failing.Execute(context.Background(), mine)
	assert.Error(t, err)
}

func TestDismissThreat_Success(t *testing.T) {
	tenant := uuid.New()
	ft := newFakeTenant()
	ft.assets[tenant] = []AssetCPEs{{ID: uuid.New(), Name: "fw", Criticality: "CRITICAL", CPEs: []string{sslvpn}}}
	uc := NewDismissThreatUseCase(ft, catalogue())

	require.NoError(t, uc.Dismiss(context.Background(), tenant, uuid.New(), " cve-2026-21873 "))
	require.NoError(t, uc.Dismiss(context.Background(), tenant, uuid.New(), "CVE-2026-21873"), "idempotent")

	res, err := NewListThreatsUseCase(ft, catalogue()).Execute(context.Background(), tenant)
	require.NoError(t, err)
	require.Len(t, res.Threats, 1, "a dismissed threat stays listed")
	assert.NotNil(t, res.Threats[0].DismissedAt)

	require.NoError(t, uc.Restore(context.Background(), tenant, "CVE-2026-21873"))
	res, _ = NewListThreatsUseCase(ft, catalogue()).Execute(context.Background(), tenant)
	assert.Nil(t, res.Threats[0].DismissedAt)
}

func TestDismissThreat_NotFound(t *testing.T) {
	uc := NewDismissThreatUseCase(newFakeTenant(), catalogue())
	err := uc.Dismiss(context.Background(), uuid.New(), uuid.New(), "CVE-1999-0001")
	assert.True(t, errors.Is(err, domain.ErrNotFound), "a CVE outside the catalogue: %v", err)
	err = uc.Restore(context.Background(), uuid.New(), "CVE-2026-21873")
	assert.True(t, errors.Is(err, domain.ErrNotFound), "nothing to restore: %v", err)
	err = uc.Dismiss(context.Background(), uuid.New(), uuid.New(), "'; drop table")
	assert.True(t, errors.Is(err, domain.ErrValidation), "not a CVE id: %v", err)
}

// One tenant cannot withdraw another tenant's dismissal.
func TestDismissThreat_Unauthorized(t *testing.T) {
	mine, theirs := uuid.New(), uuid.New()
	ft := newFakeTenant()
	uc := NewDismissThreatUseCase(ft, catalogue())
	require.NoError(t, uc.Dismiss(context.Background(), theirs, uuid.New(), "CVE-2026-21873"))

	err := uc.Restore(context.Background(), mine, "CVE-2026-21873")
	assert.True(t, errors.Is(err, domain.ErrNotFound))
	assert.Contains(t, ft.dismissed[theirs], "CVE-2026-21873", "their dismissal stands")
}
