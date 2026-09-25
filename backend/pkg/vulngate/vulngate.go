// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

// Package vulngate decides whether a set of dependency scan results may ship.
//
// It reads Trivy JSON reports (Go modules, npm lockfile, container image) and
// govulncheck JSON streams, applies the exceptions declared in
// security/vulnerability-exceptions.yaml, and reports every finding that is
// neither fixed nor covered by a valid, unexpired exception. An exception file
// that is malformed, or holds an expired entry, fails the gate on its own:
// an exception nobody renews must not silently keep a vulnerability shippable.
package vulngate

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// MaxExceptionDays is the longest an exception may live, counted from Added.
const MaxExceptionDays = 90

const dateLayout = "2006-01-02"

// githubHandle matches a GitHub login: alphanumerics and single hyphens, no
// leading hyphen, at most 39 characters.
var githubHandle = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9]|-[A-Za-z0-9]){0,38}$`)

// Finding is one vulnerable package reported by a scanner.
type Finding struct {
	Source    string // "trivy" or "govulncheck"
	Target    string // lockfile, image layer or binary the scanner looked at
	ID        string // CVE-, GHSA- or GO- identifier
	Package   string // module, npm package or OS package name
	Installed string
	Fixed     string // empty when no fixed version exists
	Severity  string // HIGH, CRITICAL, or "REACHABLE" for govulncheck
}

func (f Finding) key() string {
	return strings.Join([]string{f.Source, f.Target, f.ID, f.Package, f.Installed}, "\x00")
}

// Exception exempts exactly one (ID, Package) pair until Expires.
type Exception struct {
	ID      string `yaml:"id"`
	Package string `yaml:"package"`
	Reason  string `yaml:"reason"`
	Owner   string `yaml:"owner"`
	Issue   int    `yaml:"issue"`
	Added   string `yaml:"added"`
	Expires string `yaml:"expires"`
}

func (e Exception) label() string {
	return fmt.Sprintf("%s in %s (owner @%s)", e.ID, e.Package, e.Owner)
}

type exceptionFile struct {
	Exceptions []Exception `yaml:"exceptions"`
}

// ParseExceptions reads the exceptions file. Unknown keys are rejected so a
// misspelt field is reported instead of silently read as empty.
func ParseExceptions(r io.Reader) ([]Exception, error) {
	dec := yaml.NewDecoder(r)
	dec.KnownFields(true)
	var f exceptionFile
	if err := dec.Decode(&f); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, nil
		}
		return nil, fmt.Errorf("vulnerability exceptions: %w", err)
	}
	return f.Exceptions, nil
}

// ValidateExceptions returns the entries that may be applied and one error per
// problem found, in file order. An entry with any problem covers nothing, so
// its findings surface as blocking next to the error. An entry is expired when
// today is later than its Expires date; it still holds on that date.
func ValidateExceptions(excs []Exception, today time.Time) ([]Exception, []error) {
	var errs []error
	var valid []Exception
	seen := map[string]int{}
	today = today.UTC().Truncate(24 * time.Hour)

	for i, e := range excs {
		before := len(errs)
		where := fmt.Sprintf("exception #%d", i+1)
		if e.ID != "" {
			where = fmt.Sprintf("exception #%d (%s)", i+1, e.ID)
		}

		var missing []string
		for _, f := range []struct{ name, val string }{
			{"id", e.ID}, {"package", e.Package}, {"reason", e.Reason},
			{"owner", e.Owner}, {"added", e.Added}, {"expires", e.Expires},
		} {
			if strings.TrimSpace(f.val) == "" {
				missing = append(missing, f.name)
			}
		}
		if e.Issue <= 0 {
			missing = append(missing, "issue")
		}
		if len(missing) > 0 {
			errs = append(errs, fmt.Errorf("%s: missing %s", where, strings.Join(missing, ", ")))
		}

		if e.Owner != "" && !githubHandle.MatchString(strings.TrimPrefix(e.Owner, "@")) {
			errs = append(errs, fmt.Errorf("%s: owner %q is not a GitHub handle", where, e.Owner))
		}

		added, addedErr := parseDate(e.Added)
		expires, expiresErr := parseDate(e.Expires)
		if e.Added != "" && addedErr != nil {
			errs = append(errs, fmt.Errorf("%s: added %q is not a YYYY-MM-DD date", where, e.Added))
		}
		if e.Expires != "" && expiresErr != nil {
			errs = append(errs, fmt.Errorf("%s: expires %q is not a YYYY-MM-DD date", where, e.Expires))
		}
		if addedErr == nil && expiresErr == nil {
			switch {
			case expires.Before(added):
				errs = append(errs, fmt.Errorf("%s: expires %s is before added %s", where, e.Expires, e.Added))
			case expires.Sub(added) > MaxExceptionDays*24*time.Hour:
				errs = append(errs, fmt.Errorf("%s: expires %s is more than %d days after added %s",
					where, e.Expires, MaxExceptionDays, e.Added))
			}
		}
		if expiresErr == nil && today.After(expires) {
			errs = append(errs, fmt.Errorf("%s: EXPIRED on %s — %s must renew it with a new justification or remove it",
				where, e.Expires, e.label()))
		}

		if e.ID != "" && e.Package != "" {
			k := e.ID + "\x00" + e.Package
			if prev, dup := seen[k]; dup {
				errs = append(errs, fmt.Errorf("%s: duplicates exception #%d", where, prev))
			}
			seen[k] = i + 1
		}
		if len(errs) == before {
			valid = append(valid, e)
		}
	}
	return valid, errs
}

func parseDate(s string) (time.Time, error) {
	return time.Parse(dateLayout, strings.TrimSpace(s))
}

// Result splits the findings the gate saw.
type Result struct {
	Blocking []Finding
	Excepted []Finding
	Unused   []Exception // exceptions that matched nothing; safe to delete
}

// Apply matches findings against exceptions on the exact (ID, Package) pair.
// Pass it only the valid entries returned by ValidateExceptions.
func Apply(findings []Finding, excs []Exception) Result {
	used := make([]bool, len(excs))
	var res Result
	seen := map[string]bool{}
	for _, f := range findings {
		if seen[f.key()] {
			continue
		}
		seen[f.key()] = true
		matched := false
		for i, e := range excs {
			if e.ID == f.ID && e.Package == f.Package {
				used[i], matched = true, true
			}
		}
		if matched {
			res.Excepted = append(res.Excepted, f)
		} else {
			res.Blocking = append(res.Blocking, f)
		}
	}
	for i, e := range excs {
		if !used[i] {
			res.Unused = append(res.Unused, e)
		}
	}
	sortFindings(res.Blocking)
	sortFindings(res.Excepted)
	return res
}

func sortFindings(fs []Finding) {
	sort.SliceStable(fs, func(i, j int) bool {
		if fs[i].Package != fs[j].Package {
			return fs[i].Package < fs[j].Package
		}
		return fs[i].ID < fs[j].ID
	})
}

// ---------------------------------------------------------------------------
// Trivy
// ---------------------------------------------------------------------------

type trivyReport struct {
	ArtifactName string `json:"ArtifactName"`
	Results      []struct {
		Target          string `json:"Target"`
		Vulnerabilities []struct {
			VulnerabilityID  string `json:"VulnerabilityID"`
			PkgName          string `json:"PkgName"`
			InstalledVersion string `json:"InstalledVersion"`
			FixedVersion     string `json:"FixedVersion"`
			Severity         string `json:"Severity"`
		} `json:"Vulnerabilities"`
	} `json:"Results"`
}

// ParseTrivy reads a `trivy --format json` report and keeps HIGH and CRITICAL
// findings. Lower severities are reported by Trivy itself, never gated.
func ParseTrivy(r io.Reader) ([]Finding, error) {
	var rep trivyReport
	if err := json.NewDecoder(r).Decode(&rep); err != nil {
		return nil, fmt.Errorf("trivy report: %w", err)
	}
	var out []Finding
	for _, res := range rep.Results {
		for _, v := range res.Vulnerabilities {
			sev := strings.ToUpper(v.Severity)
			if sev != "HIGH" && sev != "CRITICAL" {
				continue
			}
			out = append(out, Finding{
				Source:    "trivy",
				Target:    res.Target,
				ID:        v.VulnerabilityID,
				Package:   v.PkgName,
				Installed: v.InstalledVersion,
				Fixed:     v.FixedVersion,
				Severity:  sev,
			})
		}
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// govulncheck
// ---------------------------------------------------------------------------

type govulnMessage struct {
	Finding *struct {
		OSV          string `json:"osv"`
		FixedVersion string `json:"fixed_version"`
		Trace        []struct {
			Module   string `json:"module"`
			Version  string `json:"version"`
			Function string `json:"function"`
		} `json:"trace"`
	} `json:"finding"`
}

// ParseGovulncheck reads the `govulncheck -format json` stream and keeps the
// findings whose vulnerable symbol is actually called by our code. govulncheck
// assigns no severity, so every reachable finding is treated as blocking.
func ParseGovulncheck(r io.Reader) ([]Finding, error) {
	dec := json.NewDecoder(r)
	var out []Finding
	for {
		var m govulnMessage
		err := dec.Decode(&m)
		if errors.Is(err, io.EOF) {
			return out, nil
		}
		if err != nil {
			return nil, fmt.Errorf("govulncheck report: %w", err)
		}
		if m.Finding == nil || len(m.Finding.Trace) == 0 || m.Finding.Trace[0].Function == "" {
			continue
		}
		t := m.Finding.Trace[0]
		out = append(out, Finding{
			Source:    "govulncheck",
			Target:    "backend",
			ID:        m.Finding.OSV,
			Package:   t.Module,
			Installed: t.Version,
			Fixed:     m.Finding.FixedVersion,
			Severity:  "REACHABLE",
		})
	}
}

// ---------------------------------------------------------------------------
// Report
// ---------------------------------------------------------------------------

// Render writes a human-readable account of res, and returns whether the gate
// passes. GitHub Actions turns the ::error:: lines into annotations.
func Render(w io.Writer, res Result, excErrs []error) bool {
	var b bytes.Buffer
	pass := len(res.Blocking) == 0 && len(excErrs) == 0

	if len(excErrs) > 0 {
		fmt.Fprintf(&b, "Exception file: %d problem(s)\n", len(excErrs))
		for _, e := range excErrs {
			fmt.Fprintf(&b, "::error title=Vulnerability exception::%s\n", e)
		}
		b.WriteString("\n")
	}

	if len(res.Blocking) > 0 {
		fmt.Fprintf(&b, "BLOCKING: %d HIGH/CRITICAL or reachable finding(s) with no valid exception\n\n", len(res.Blocking))
		writeTable(&b, res.Blocking)
		for _, f := range res.Blocking {
			fixed := "fixed in " + f.Fixed
			if f.Fixed == "" {
				fixed = "no fixed version"
			}
			fmt.Fprintf(&b, "::error title=%s %s::%s — installed %s, %s (%s, %s)\n",
				f.Severity, f.ID, f.Package, f.Installed, fixed, f.Source, f.Target)
		}
		b.WriteString("\n")
	}

	if len(res.Excepted) > 0 {
		fmt.Fprintf(&b, "Excepted: %d finding(s) covered by security/vulnerability-exceptions.yaml\n\n", len(res.Excepted))
		writeTable(&b, res.Excepted)
		b.WriteString("\n")
	}

	for _, e := range res.Unused {
		fmt.Fprintf(&b, "::warning title=Unused exception::%s matched no finding; remove it\n", e.label())
	}

	if pass {
		b.WriteString("Dependency gate: PASS\n")
	} else {
		b.WriteString("Dependency gate: FAIL\n")
	}
	_, _ = w.Write(b.Bytes())
	return pass
}

func writeTable(b *bytes.Buffer, fs []Finding) {
	rows := [][]string{{"SEVERITY", "ID", "PACKAGE", "INSTALLED", "FIXED", "SOURCE", "TARGET"}}
	for _, f := range fs {
		fixed := f.Fixed
		if fixed == "" {
			fixed = "-"
		}
		rows = append(rows, []string{f.Severity, f.ID, f.Package, f.Installed, fixed, f.Source, f.Target})
	}
	widths := make([]int, len(rows[0]))
	for _, r := range rows {
		for i, c := range r {
			if len(c) > widths[i] {
				widths[i] = len(c)
			}
		}
	}
	for _, r := range rows {
		for i, c := range r {
			if i == len(r)-1 {
				b.WriteString(c)
				continue
			}
			fmt.Fprintf(b, "%-*s  ", widths[i], c)
		}
		b.WriteString("\n")
	}
}
