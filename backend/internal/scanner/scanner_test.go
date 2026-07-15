// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package scanner

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
)

// fakeKV is an in-memory KV + Locker for tests.
type fakeKV struct {
	mu       sync.Mutex
	store    map[string]string
	messages []string
}

func newFakeKV() *fakeKV { return &fakeKV{store: map[string]string{}} }

func (f *fakeKV) Set(_ context.Context, key, value string, _ time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.store[key] = value
	return nil
}
func (f *fakeKV) Get(_ context.Context, key string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.store[key], nil
}
func (f *fakeKV) Del(_ context.Context, keys ...string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, k := range keys {
		delete(f.store, k)
	}
	return nil
}
func (f *fakeKV) Publish(_ context.Context, _ string, _ interface{}) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.messages = append(f.messages, "msg")
	return nil
}
func (f *fakeKV) SetNX(_ context.Context, key, value string, _ time.Duration) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.store[key]; ok {
		return false, nil
	}
	f.store[key] = value
	return true, nil
}

func strp(s string) *string { return &s }

// --- Target scope validation ----------------------------------------------

func TestValidateTarget(t *testing.T) {
	ok := []string{"10.0.0.5", "192.168.1.0/24", "10.0.0.1/32", "host.internal", "srv-01"}
	for _, tgt := range ok {
		assert.NoError(t, ValidateTarget(tgt), "expected %q to be valid", tgt)
	}
	bad := []string{"", "10.0.0.0/16", "0.0.0.0/0", "192.168.0.0/8", "not a host!", "10.0.0.0/23"}
	for _, tgt := range bad {
		assert.Error(t, ValidateTarget(tgt), "expected %q to be rejected", tgt)
	}
}

// --- Criticality inference + normalization ---------------------------------

func TestNormalizeAsset_InfersCriticality(t *testing.T) {
	prod := normalizeAsset(AssetDiscovery{Name: "db", Environment: "PROD", Tags: []string{"database"}})
	dev := normalizeAsset(AssetDiscovery{Name: "sandbox", Environment: "dev"})
	assert.Greater(t, prod.Criticality, dev.Criticality)
	assert.LessOrEqual(t, prod.Criticality, 3.0)
	assert.GreaterOrEqual(t, dev.Criticality, 0.1)
	// Explicit criticality is preserved (not overwritten by inference).
	explicit := normalizeAsset(AssetDiscovery{Name: "x", Criticality: 2.0, Environment: "dev"})
	assert.Equal(t, 2.0, explicit.Criticality)
	// Unknown type is coerced.
	assert.Equal(t, domain.AssetTypeUnknown, prod.Type)
}

func TestNormalizeCPEList(t *testing.T) {
	got := normalizeCPEList([]string{"CPE:2.3:o:Linux", " cpe:2.3:o:linux ", "cpe:2.3:a:nginx", ""})
	// lower-cased, trimmed, de-duped, sorted
	assert.Equal(t, []string{"cpe:2.3:a:nginx", "cpe:2.3:o:linux"}, got)
}

func TestCriticalityLabel(t *testing.T) {
	assert.Equal(t, domain.CriticalityCritical, CriticalityLabel(3.0))
	assert.Equal(t, domain.CriticalityHigh, CriticalityLabel(2.5))
	assert.Equal(t, domain.CriticalityMedium, CriticalityLabel(1.5))
	assert.Equal(t, domain.CriticalityLow, CriticalityLabel(0.5))
}

// --- Dedup -----------------------------------------------------------------

func TestDedupeAssets_MergesByExternalID(t *testing.T) {
	in := []AssetDiscovery{
		{ExternalID: "i-1", Name: "a", CPE: []string{"cpe:x"}, Criticality: 1.0, Tags: []string{"t1"}},
		{ExternalID: "i-1", Name: "a", CPE: []string{"cpe:y"}, Criticality: 2.5, Tags: []string{"t2"}, IP: strp("10.0.0.1")},
		{ExternalID: "i-2", Name: "b"},
	}
	out := dedupeAssets(in)
	require.Len(t, out, 2)
	assert.Equal(t, 2.5, out[0].Criticality)                        // max
	assert.ElementsMatch(t, []string{"cpe:x", "cpe:y"}, out[0].CPE) // merged
	assert.ElementsMatch(t, []string{"t1", "t2"}, out[0].Tags)
	require.NotNil(t, out[0].IP) // hole filled from the second occurrence
}

func TestDedupeFindings(t *testing.T) {
	cve := "CVE-2024-1"
	in := []FindingDiscovery{
		{AssetExternalID: "i-1", CVE: &cve, AffectedCPE: []string{"cpe:x"}},
		{AssetExternalID: "i-1", CVE: &cve, AffectedCPE: []string{"cpe:x"}}, // dup
		{AssetExternalID: "i-2", CVE: &cve, AffectedCPE: []string{"cpe:x"}}, // diff asset
	}
	out := dedupeFindings(in)
	assert.Len(t, out, 2)
}

// --- Auto-mitigation detection ---------------------------------------------

func TestDetectMitigations(t *testing.T) {
	cveA, cveB := "CVE-A", "CVE-B"
	prev := []FindingDiscovery{
		{AssetExternalID: "i-1", CVE: &cveA, Title: "A", Severity: "high", Evidence: "port 22"},
		{AssetExternalID: "i-1", CVE: &cveB, Title: "B", Severity: "low"},
	}
	cur := []FindingDiscovery{
		{AssetExternalID: "i-1", CVE: &cveB, Title: "B", Severity: "low"}, // A gone → mitigated
	}
	mits := detectMitigations(prev, cur, time.Now())
	require.Len(t, mits, 1)
	assert.Equal(t, "CVE-A", *mits[0].CVE)
	assert.Equal(t, "i-1", mits[0].AssetExternalID)

	// Nothing removed → no mitigations.
	assert.Empty(t, detectMitigations(cur, cur, time.Now()))
}

func TestSeverityAtLeast(t *testing.T) {
	assert.True(t, SeverityAtLeast("critical", "medium"))
	assert.True(t, SeverityAtLeast("MEDIUM", "medium"))
	assert.False(t, SeverityAtLeast("low", "high"))
}

// --- Scanner Validate ------------------------------------------------------

func TestCloudScannerValidate(t *testing.T) {
	aws := NewAWSScanner(nil)
	assert.False(t, aws.IsAgentBased())
	// Missing creds → error.
	err := aws.Validate(context.Background(), ScanConfig{Provider: domain.ProviderAWS})
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrValidation)
	// Present → ok.
	err = aws.Validate(context.Background(), ScanConfig{
		Provider:    domain.ProviderAWS,
		Credentials: map[string]string{"access_key_id": "AKIA", "secret_access_key": "s"},
	})
	assert.NoError(t, err)
	// Provider mismatch → error.
	assert.Error(t, aws.Validate(context.Background(), ScanConfig{Provider: domain.ProviderGCP}))
}

func TestAgentScannerValidate(t *testing.T) {
	nmap := NewNmapScanner()
	assert.True(t, nmap.IsAgentBased())
	assert.Error(t, nmap.Validate(context.Background(), ScanConfig{Provider: domain.ProviderNmap}))                            // no targets
	assert.Error(t, nmap.Validate(context.Background(), ScanConfig{Provider: domain.ProviderNmap, Targets: []string{"10/8"}})) // bad
	assert.NoError(t, nmap.Validate(context.Background(), ScanConfig{Provider: domain.ProviderNmap, Targets: []string{"10.0.0.0/24"}}))
}

func TestCloudScanner_Unavailable_EmitsErrorNoAssets(t *testing.T) {
	aws := NewAWSScanner(nil) // no collector
	assetCh, findingCh, errCh := aws.Scan(context.Background(), ScanConfig{Provider: domain.ProviderAWS})
	var assets int
	for range assetCh {
		assets++
	}
	for range findingCh {
	}
	var errs int
	for range errCh {
		errs++
	}
	assert.Equal(t, 0, assets)
	assert.Equal(t, 1, errs) // the "not configured" error
}

// --- Preview store ---------------------------------------------------------

func TestPreviewStore_StoreLoadLatest(t *testing.T) {
	kv := newFakeKV()
	ps := NewPreviewStore(kv)
	tenant, cfg, job := uuid.New(), uuid.New(), uuid.New()
	p := &ScanPreview{JobID: job, ConfigID: cfg, TenantID: tenant, CreatedAt: time.Now()}
	require.NoError(t, ps.Store(context.Background(), p))

	got, err := ps.Load(context.Background(), tenant, job)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, job, got.JobID)

	latest, err := ps.LoadLatestForConfig(context.Background(), tenant, cfg)
	require.NoError(t, err)
	require.NotNil(t, latest)
	assert.Equal(t, job, latest.JobID)

	// Another tenant can't read it.
	other, err := ps.Load(context.Background(), uuid.New(), job)
	require.NoError(t, err)
	assert.Nil(t, other)
}

// --- Pipeline (Ingest) -----------------------------------------------------

func TestPipeline_Ingest_NormalizesDedupsStores(t *testing.T) {
	kv := newFakeKV()
	ps := NewPreviewStore(kv)
	reg := NewRegistry()
	p := NewPipeline(reg, ps, NoopNotifier{}, zerolog.Nop())

	tenant, cfg, job := uuid.New(), uuid.New(), uuid.New()
	meta := PreviewMeta{JobID: job, ConfigID: cfg, TenantID: tenant, Provider: domain.ProviderAgent}
	cve := "cve-2024-9"
	assets := []AssetDiscovery{
		{ExternalID: "h1", Name: "h1", Environment: "prod"},
		{ExternalID: "h1", Name: "h1", Environment: "prod"}, // dup
	}
	findings := []FindingDiscovery{
		{AssetExternalID: "h1", CVE: &cve, Title: "x", Severity: "HIGH"},
	}
	preview, err := p.Ingest(context.Background(), meta, assets, findings, nil)
	require.NoError(t, err)
	assert.Len(t, preview.Assets, 1)                      // deduped
	assert.Equal(t, "high", preview.Findings[0].Severity) // normalized
	assert.Equal(t, "CVE-2024-9", *preview.Findings[0].CVE)

	// Stored and reloadable.
	reloaded, err := ps.Load(context.Background(), tenant, job)
	require.NoError(t, err)
	require.NotNil(t, reloaded)
	assert.Equal(t, job, reloaded.JobID)
}

func TestPipeline_Ingest_DetectsMitigationAcrossRuns(t *testing.T) {
	kv := newFakeKV()
	ps := NewPreviewStore(kv)
	p := NewPipeline(NewRegistry(), ps, NoopNotifier{}, zerolog.Nop())
	tenant, cfg := uuid.New(), uuid.New()
	cveA := "CVE-A"

	// Run 1: finding present.
	_, err := p.Ingest(context.Background(), PreviewMeta{JobID: uuid.New(), ConfigID: cfg, TenantID: tenant, Provider: domain.ProviderAgent},
		[]AssetDiscovery{{ExternalID: "h1", Name: "h1"}},
		[]FindingDiscovery{{AssetExternalID: "h1", CVE: &cveA, Title: "A", Severity: "high"}}, nil)
	require.NoError(t, err)

	// Run 2: finding gone → mitigation detected.
	preview, err := p.Ingest(context.Background(), PreviewMeta{JobID: uuid.New(), ConfigID: cfg, TenantID: tenant, Provider: domain.ProviderAgent},
		[]AssetDiscovery{{ExternalID: "h1", Name: "h1"}}, nil, nil)
	require.NoError(t, err)
	require.Len(t, preview.Mitigations, 1)
	assert.Equal(t, "CVE-A", *preview.Mitigations[0].CVE)
}

// --- Agent auth helpers ----------------------------------------------------

func TestHMACPushSignature(t *testing.T) {
	secret, err := GenerateHMACSecret()
	require.NoError(t, err)
	body := []byte(`{"job_id":"x"}`)
	sig := SignPush(secret, body)
	assert.True(t, VerifyPushSignature(secret, body, sig))
	assert.False(t, VerifyPushSignature(secret, body, "deadbeef"))
	assert.False(t, VerifyPushSignature("other", body, sig))
}
