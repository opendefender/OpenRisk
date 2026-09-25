// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

package vulngate

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"
)

func day(s string) time.Time {
	t, err := time.Parse(dateLayout, s)
	if err != nil {
		panic(err)
	}
	return t
}

func validException() Exception {
	return Exception{
		ID:      "CVE-2026-41567",
		Package: "github.com/docker/docker",
		Reason:  "test-only dependency of golang-migrate, not linked into the server",
		Owner:   "alex-dembele",
		Issue:   487,
		Added:   "2026-09-24",
		Expires: "2026-12-23",
	}
}

func joinErrs(errs []error) string {
	parts := make([]string, len(errs))
	for i, e := range errs {
		parts[i] = e.Error()
	}
	return strings.Join(parts, "\n")
}

func TestValidateExceptions_Valid(t *testing.T) {
	if _, errs := ValidateExceptions([]Exception{validException()}, day("2026-09-24")); len(errs) != 0 {
		t.Fatalf("valid exception rejected:\n%s", joinErrs(errs))
	}
	// The expiry date itself is still covered.
	if _, errs := ValidateExceptions([]Exception{validException()}, day("2026-12-23")); len(errs) != 0 {
		t.Fatalf("exception rejected on its expiry date:\n%s", joinErrs(errs))
	}
}

func TestValidateExceptions_MissingFields(t *testing.T) {
	cases := map[string]func(*Exception){
		"id":      func(e *Exception) { e.ID = "" },
		"package": func(e *Exception) { e.Package = "" },
		"reason":  func(e *Exception) { e.Reason = "  " },
		"owner":   func(e *Exception) { e.Owner = "" },
		"issue":   func(e *Exception) { e.Issue = 0 },
		"added":   func(e *Exception) { e.Added = "" },
		"expires": func(e *Exception) { e.Expires = "" },
	}
	for field, mutate := range cases {
		t.Run(field, func(t *testing.T) {
			e := validException()
			mutate(&e)
			_, errs := ValidateExceptions([]Exception{e}, day("2026-09-24"))
			if !strings.Contains(joinErrs(errs), "missing "+field) {
				t.Fatalf("want a 'missing %s' error, got:\n%s", field, joinErrs(errs))
			}
		})
	}
}

func TestValidateExceptions_Expired(t *testing.T) {
	valid, errs := ValidateExceptions([]Exception{validException()}, day("2026-12-24"))
	if len(valid) != 0 {
		t.Fatalf("an expired exception must cover nothing, got %+v", valid)
	}
	got := joinErrs(errs)
	if len(errs) != 1 || !strings.Contains(got, "EXPIRED on 2026-12-23") ||
		!strings.Contains(got, "CVE-2026-41567") || !strings.Contains(got, "@alex-dembele") {
		t.Fatalf("expired exception must fail naming the entry and its owner, got:\n%s", got)
	}
}

func TestValidateExceptions_LongerThan90Days(t *testing.T) {
	e := validException()
	e.Expires = "2026-12-24" // 91 days after 2026-09-24
	_, errs := ValidateExceptions([]Exception{e}, day("2026-09-24"))
	if !strings.Contains(joinErrs(errs), "more than 90 days") {
		t.Fatalf("want a 90-day error, got:\n%s", joinErrs(errs))
	}

	e.Expires = "2026-09-01"
	_, errs = ValidateExceptions([]Exception{e}, day("2026-08-01"))
	if !strings.Contains(joinErrs(errs), "before added") {
		t.Fatalf("want an expires-before-added error, got:\n%s", joinErrs(errs))
	}
}

func TestValidateExceptions_BadFormats(t *testing.T) {
	e := validException()
	e.ID = "CVE-2026-42306"
	e.Added = "24/09/2026"
	e.Owner = "not a handle"
	valid, errs := ValidateExceptions([]Exception{e, validException(), validException()}, day("2026-09-24"))
	if len(valid) != 1 {
		t.Errorf("only the first well-formed, non-duplicate entry is usable; got %d", len(valid))
	}
	got := joinErrs(errs)
	for _, want := range []string{"added \"24/09/2026\" is not a YYYY-MM-DD date", "is not a GitHub handle", "exception #3 (CVE-2026-41567): duplicates exception #2"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

func TestParseExceptions_RejectsUnknownKeys(t *testing.T) {
	_, err := ParseExceptions(strings.NewReader("exceptions:\n  - id: CVE-1\n    expiry: 2026-01-01\n"))
	if err == nil || !strings.Contains(err.Error(), "expiry") {
		t.Fatalf("want an unknown-field error naming 'expiry', got %v", err)
	}
}

func TestParseExceptions_RepoFile(t *testing.T) {
	f, err := os.Open("../../../security/vulnerability-exceptions.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	excs, err := ParseExceptions(f)
	if err != nil {
		t.Fatal(err)
	}
	// Only the format is checked here: expiry depends on the calendar and is
	// enforced by the gate itself, daily.
	_, errs := ValidateExceptions(excs, day("2000-01-01"))
	for _, e := range errs {
		if !strings.Contains(e.Error(), "EXPIRED") {
			t.Error(e)
		}
	}
}

func TestParseTrivy_KeepsHighAndCritical(t *testing.T) {
	f, err := os.Open("testdata/trivy-gomod.json")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	got, err := ParseTrivy(f)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("want the 2 HIGH findings, MEDIUM dropped; got %+v", got)
	}
	var crypto Finding
	for _, g := range got {
		if g.ID == "CVE-2026-56854" {
			crypto = g
		}
	}
	if crypto.Package != "golang.org/x/crypto" || crypto.Installed != "v0.54.0" || crypto.Fixed != "0.55.0" || crypto.Target != "go.mod" {
		t.Fatalf("x/crypto finding parsed wrong: %+v", crypto)
	}
}

func TestParseGovulncheck_KeepsReachableOnly(t *testing.T) {
	f, err := os.Open("testdata/govulncheck.json")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	got, err := ParseGovulncheck(f)
	if err != nil {
		t.Fatal(err)
	}
	want := Finding{Source: "govulncheck", Target: "backend", ID: "GO-2026-6348",
		Package: "google.golang.org/grpc", Installed: "v1.82.1", Fixed: "v1.83.1", Severity: "REACHABLE"}
	if len(got) != 1 || got[0] != want {
		t.Fatalf("want only the reachable grpc finding %+v, got %+v", want, got)
	}
}

func TestApply_ExactPairOnly(t *testing.T) {
	exc := validException()
	findings := []Finding{
		{Source: "trivy", Target: "go.mod", ID: exc.ID, Package: exc.Package, Installed: "v28.5.2", Severity: "HIGH"},
		// Same package, other advisory: not covered.
		{Source: "trivy", Target: "go.mod", ID: "CVE-2026-42306", Package: exc.Package, Installed: "v28.5.2", Severity: "HIGH"},
		// Same advisory, other package: not covered.
		{Source: "trivy", Target: "go.mod", ID: exc.ID, Package: "github.com/moby/moby", Installed: "v1", Severity: "HIGH"},
		// Exact duplicate of the first: counted once.
		{Source: "trivy", Target: "go.mod", ID: exc.ID, Package: exc.Package, Installed: "v28.5.2", Severity: "HIGH"},
	}
	res := Apply(findings, []Exception{exc, {ID: "CVE-0000-0001", Package: "nothing"}})
	if len(res.Excepted) != 1 || len(res.Blocking) != 2 {
		t.Fatalf("want 1 excepted and 2 blocking, got %d and %d", len(res.Excepted), len(res.Blocking))
	}
	if len(res.Unused) != 1 || res.Unused[0].ID != "CVE-0000-0001" {
		t.Fatalf("want the unmatched exception reported as unused, got %+v", res.Unused)
	}
}

func TestRender_FailsAndNamesEachFinding(t *testing.T) {
	var out bytes.Buffer
	res := Result{Blocking: []Finding{{Source: "trivy", Target: "package-lock.json", ID: "GHSA-xxxx",
		Package: "lodash", Installed: "4.17.20", Fixed: "4.17.21", Severity: "CRITICAL"}}}
	if Render(&out, res, nil) {
		t.Fatal("gate passed with a blocking finding")
	}
	for _, want := range []string{"GHSA-xxxx", "lodash", "4.17.20", "4.17.21", "Dependency gate: FAIL"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output lacks %q:\n%s", want, out.String())
		}
	}

	out.Reset()
	if !Render(&out, Result{}, nil) || !strings.Contains(out.String(), "PASS") {
		t.Fatalf("empty result must pass:\n%s", out.String())
	}
}
