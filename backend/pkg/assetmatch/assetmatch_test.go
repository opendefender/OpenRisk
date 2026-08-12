// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

package assetmatch

import "testing"

var (
	web01 = Candidate{
		ID: "asset-web01", Name: "web-01",
		Hostnames: []string{"web-01.corp.local"},
		IPs:       []string{"10.0.0.5"},
		CPEs:      []string{"cpe:2.3:a:apache:log4j:2.14.0"},
	}
	db02 = Candidate{
		ID: "asset-db02", Name: "db-02",
		Hostnames: []string{"db-02.corp.local"},
		IPs:       []string{"10.0.0.9"},
	}
	bucket = Candidate{
		ID: "asset-bucket", Name: "backups",
		CloudResourceID: "arn:aws:s3:::acme-backups",
	}
)

func TestCorrelate_NoMatchIsNoMatch(t *testing.T) {
	res := Correlate(Finding{AssetName: "totally-unknown"}, []Candidate{web01, db02})
	if res.Best != nil {
		t.Fatalf("expected no match, got %+v", res.Best)
	}
	if res.Ambiguous {
		t.Error("nothing matched, so nothing is ambiguous")
	}
}

// A cloud resource id is provider-issued and globally unique — it must outrank
// everything and be safe to auto-assign.
func TestCorrelate_CloudIDIsStrongest(t *testing.T) {
	res := Correlate(Finding{CloudID: "arn:aws:s3:::acme-backups"}, []Candidate{web01, bucket})
	if res.Best == nil || res.Best.AssetID != bucket.ID {
		t.Fatalf("expected the bucket, got %+v", res.Best)
	}
	if res.Best.Methods[0] != MethodCloudID {
		t.Errorf("expected cloud_id as the leading method, got %v", res.Best.Methods)
	}
	if res.Best.Confidence < AutoAssignThreshold {
		t.Errorf("a cloud id match must be auto-assignable, got %.2f", res.Best.Confidence)
	}
}

// The most common real case: the scanner reports an FQDN, the inventory stores
// the short name.
func TestCorrelate_FQDNAgainstShortName(t *testing.T) {
	res := Correlate(Finding{AssetName: "WEB-01.corp.local"}, []Candidate{web01, db02})
	if res.Best == nil || res.Best.AssetID != web01.ID {
		t.Fatalf("expected web-01, got %+v", res.Best)
	}
	if res.Best.Confidence < AutoAssignThreshold {
		t.Errorf("an exact FQDN match should be auto-assignable, got %.2f", res.Best.Confidence)
	}
}

// Independent signals agreeing IS the evidence; corroboration must raise
// confidence above any one of them alone.
func TestCorrelate_CorroborationRaisesConfidence(t *testing.T) {
	alone := Correlate(Finding{Hostname: "web-01"}, []Candidate{web01})
	together := Correlate(Finding{
		Hostname: "web-01",
		IPs:      []string{"10.0.0.5"},
		CPEs:     []string{"cpe:2.3:a:apache:log4j:2.14.1"},
	}, []Candidate{web01})

	if alone.Best == nil || together.Best == nil {
		t.Fatal("both should match")
	}
	if together.Best.Confidence <= alone.Best.Confidence {
		t.Errorf("three agreeing signals (%.2f) must beat one (%.2f)",
			together.Best.Confidence, alone.Best.Confidence)
	}
	if len(together.Best.Methods) != 3 {
		t.Errorf("expected 3 methods, got %v", together.Best.Methods)
	}
}

// Weak signals must never out-rank a cloud resource id, however many of them
// there are. Summing confidences would break this.
func TestCorrelate_WeakSignalsNeverBeatCloudID(t *testing.T) {
	decoy := Candidate{
		ID: "asset-decoy", Name: "backups",
		Hostnames: []string{"backups"},
		CPEs:      []string{"cpe:2.3:a:apache:log4j:2.14.0"},
	}
	res := Correlate(Finding{
		CloudID:   "arn:aws:s3:::acme-backups",
		AssetName: "backups",
		CPEs:      []string{"cpe:2.3:a:apache:log4j:2.14.1"},
	}, []Candidate{decoy, bucket})

	if res.Best == nil || res.Best.AssetID != bucket.ID {
		t.Fatalf("the cloud id must win, got %+v", res.Best)
	}
}

// Two assets answering to the same name is a real inventory problem. Picking
// one at random hides exactly the problem the user needs to see.
func TestCorrelate_ReportsAmbiguity(t *testing.T) {
	twinA := Candidate{ID: "asset-a", Name: "web-01", Hostnames: []string{"web-01.corp.local"}}
	twinB := Candidate{ID: "asset-b", Name: "web-01", Hostnames: []string{"web-01.dr.local"}}

	res := Correlate(Finding{Hostname: "web-01"}, []Candidate{twinA, twinB})
	if !res.Ambiguous {
		t.Fatalf("two equally-good candidates must be reported ambiguous: %+v", res.Candidates)
	}
	if len(res.Candidates) != 2 {
		t.Errorf("both candidates must be returned for a human to choose, got %d", len(res.Candidates))
	}
}

// A clearly better candidate is not ambiguous even when a weaker one also
// matches — otherwise every CPE overlap would demand human review.
func TestCorrelate_ClearWinnerIsNotAmbiguous(t *testing.T) {
	weak := Candidate{ID: "asset-weak", CPEs: []string{"cpe:2.3:a:apache:log4j:2.14.9"}}
	res := Correlate(Finding{
		Hostname: "web-01.corp.local",
		IPs:      []string{"10.0.0.5"},
		CPEs:     []string{"cpe:2.3:a:apache:log4j:2.14.1"},
	}, []Candidate{web01, weak})

	if res.Best == nil || res.Best.AssetID != web01.ID {
		t.Fatalf("expected web-01, got %+v", res.Best)
	}
	if res.Ambiguous {
		t.Errorf("a clear winner must not be flagged ambiguous: %+v", res.Candidates)
	}
}

// A finding that only knows an IP still lands, and lands on the right host.
func TestCorrelate_IPOnly(t *testing.T) {
	res := Correlate(Finding{AssetName: "10.0.0.9"}, []Candidate{web01, db02})
	if res.Best == nil || res.Best.AssetID != db02.ID {
		t.Fatalf("expected db-02 by IP, got %+v", res.Best)
	}
	if res.Best.Methods[0] != MethodIP {
		t.Errorf("expected an IP match, got %v", res.Best.Methods)
	}
}

// CPE versions drift between the scanner and the inventory. Matching on the
// exact version would fail precisely when the asset IS the vulnerable one.
func TestCPEProduct_IgnoresVersion(t *testing.T) {
	a := cpeProduct("cpe:2.3:a:apache:log4j:2.14.1")
	b := cpeProduct("cpe:2.3:a:apache:log4j:2.17.0")
	if a != b {
		t.Errorf("versions must not affect the product key: %q vs %q", a, b)
	}
	if cpeProduct("cpe:2.3:a:apache:log4j:2.14.1") == cpeProduct("cpe:2.3:a:apache:struts:2.5.0") {
		t.Error("different products must not collide")
	}
}

// Ranking must not depend on map iteration order.
func TestCorrelate_IsDeterministic(t *testing.T) {
	f := Finding{Hostname: "web-01", IPs: []string{"10.0.0.5"}}
	first := Correlate(f, []Candidate{web01, db02, bucket})
	for i := 0; i < 25; i++ {
		got := Correlate(f, []Candidate{bucket, db02, web01})
		if got.Best == nil || first.Best == nil || got.Best.AssetID != first.Best.AssetID {
			t.Fatalf("ranking changed between runs: %+v vs %+v", got.Best, first.Best)
		}
		if got.Best.Confidence != first.Best.Confidence {
			t.Fatalf("confidence changed between runs: %v vs %v", got.Best.Confidence, first.Best.Confidence)
		}
	}
}

func TestLooksLikeIP(t *testing.T) {
	for _, s := range []string{"10.0.0.1", "192.168.1.254", "fe80::1", "2001:db8::dead:beef"} {
		if !looksLikeIP(s) {
			t.Errorf("%q should look like an IP", s)
		}
	}
	for _, s := range []string{"web-01", "web-01.corp.local", "", "10.0.0"} {
		if looksLikeIP(s) {
			t.Errorf("%q should not look like an IP", s)
		}
	}
}

func TestShortHost_LeavesIPsAlone(t *testing.T) {
	if got := shortHost("web-01.corp.local"); got != "web-01" {
		t.Errorf("expected web-01, got %q", got)
	}
	if got := shortHost("10.0.0.5"); got != "10.0.0.5" {
		t.Errorf("an IP must not be truncated at its first dot, got %q", got)
	}
}

// Confidence is a probability, not a score — it must never leave [0,1], and
// must never claim certainty, because a human can always be right instead.
func TestConfidence_StaysInRange(t *testing.T) {
	res := Correlate(Finding{
		AssetExternalID: "ext-1",
		CloudID:         "arn:aws:s3:::acme-backups",
		Hostname:        "backups",
		IPs:             []string{"10.0.0.1"},
		CPEs:            []string{"cpe:2.3:a:apache:log4j:1.0"},
	}, []Candidate{{
		ID: "everything", Name: "backups", ExternalID: "ext-1",
		CloudResourceID: "arn:aws:s3:::acme-backups",
		Hostnames:       []string{"backups"},
		IPs:             []string{"10.0.0.1"},
		CPEs:            []string{"cpe:2.3:a:apache:log4j:1.0"},
	}})
	if res.Best == nil {
		t.Fatal("expected a match")
	}
	if res.Best.Confidence <= 0 || res.Best.Confidence > 0.99 {
		t.Errorf("confidence out of range: %v", res.Best.Confidence)
	}
}
