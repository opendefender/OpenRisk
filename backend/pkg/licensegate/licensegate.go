// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

// Package licensegate decides whether the licences of OpenRisk's dependencies
// may ship (#457, D-017).
//
// It reads the npm tree from frontend/package-lock.json and the installed
// package.json files, the Go tree from `go list` plus a Trivy licence report,
// and checks every dependency against security/license-policy.yaml. A licence
// that is not allowlisted fails the gate. So does a dependency whose licence
// cannot be resolved: UNKNOWN never passes on its own, it needs an exception
// that says what the licence really is and why.
package licensegate

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Unknown is the licence of a dependency whose metadata declares none.
const Unknown = "UNKNOWN"

// Dependency is one package of either tree, with its licence as declared.
type Dependency struct {
	Ecosystem string // "npm" or "go"
	Name      string
	Version   string
	License   string // SPDX expression as declared, or Unknown
	Dev       bool   // npm devDependency, never bundled
}

func (d Dependency) label() string {
	return fmt.Sprintf("%s %s@%s", d.Ecosystem, d.Name, d.Version)
}

// Allowed is one licence any dependency may carry.
type Allowed struct {
	ID     string `yaml:"id"`
	Reason string `yaml:"reason"`
}

// Exception accepts exactly one package whose declared licence is not
// allowlisted, or is missing. Declared must match the metadata exactly, so a
// package that changes its licence upstream falls out of its exception.
type Exception struct {
	Ecosystem string `yaml:"ecosystem"`
	Package   string `yaml:"package"`
	Declared  string `yaml:"declared"`
	License   string `yaml:"license"`
	Reason    string `yaml:"reason"`
	Issue     int    `yaml:"issue"`
}

func (e Exception) key() string { return e.Ecosystem + "\x00" + e.Package }

// Policy is security/license-policy.yaml.
type Policy struct {
	Allowed    []Allowed   `yaml:"allowed"`
	Exceptions []Exception `yaml:"exceptions"`
}

// ParsePolicy reads the policy file. Unknown keys are rejected so a misspelt
// field is reported instead of silently read as empty.
func ParsePolicy(r io.Reader) (Policy, error) {
	dec := yaml.NewDecoder(r)
	dec.KnownFields(true)
	var p Policy
	if err := dec.Decode(&p); err != nil && !errors.Is(err, io.EOF) {
		return Policy{}, fmt.Errorf("license policy: %w", err)
	}
	return p, nil
}

// Validate returns one error per problem in the policy, in file order. Any
// problem fails the gate: an allowlist nobody can read is not a policy.
func (p Policy) Validate() []error {
	var errs []error
	seen := map[string]bool{}
	for i, a := range p.Allowed {
		where := fmt.Sprintf("allowed #%d (%s)", i+1, a.ID)
		if a.ID == "" || strings.ContainsAny(a.ID, " ()") {
			errs = append(errs, fmt.Errorf("%s: id must be a single SPDX identifier", where))
		}
		if strings.EqualFold(a.ID, Unknown) {
			errs = append(errs, fmt.Errorf("%s: %s cannot be allowlisted; add a per-package exception", where, Unknown))
		}
		if strings.TrimSpace(a.Reason) == "" {
			errs = append(errs, fmt.Errorf("%s: reason is required", where))
		}
		if seen[a.ID] {
			errs = append(errs, fmt.Errorf("%s: duplicate", where))
		}
		seen[a.ID] = true
	}
	seenExc := map[string]bool{}
	for i, e := range p.Exceptions {
		where := fmt.Sprintf("exception #%d (%s %s)", i+1, e.Ecosystem, e.Package)
		if e.Ecosystem != "npm" && e.Ecosystem != "go" {
			errs = append(errs, fmt.Errorf("%s: ecosystem must be npm or go", where))
		}
		if e.Package == "" || e.Declared == "" || e.License == "" {
			errs = append(errs, fmt.Errorf("%s: package, declared and license are required", where))
		}
		if strings.TrimSpace(e.Reason) == "" {
			errs = append(errs, fmt.Errorf("%s: reason is required", where))
		}
		if e.Issue <= 0 {
			errs = append(errs, fmt.Errorf("%s: issue is required", where))
		}
		if seenExc[e.key()] {
			errs = append(errs, fmt.Errorf("%s: duplicate", where))
		}
		seenExc[e.key()] = true
	}
	return errs
}

// ---------------------------------------------------------------------------
// SPDX expressions
// ---------------------------------------------------------------------------

// Permits reports whether expr is acceptable when only the ids in allowed are.
// OR needs one acceptable side (we elect it), AND needs both, and a
// "X WITH exception" term needs "X WITH exception" itself to be allowlisted.
// npm metadata spells the operators in either case; both are accepted.
func Permits(expr string, allowed map[string]bool) (bool, error) {
	p := &exprParser{toks: tokenize(expr)}
	if len(p.toks) == 0 {
		return false, fmt.Errorf("empty licence expression")
	}
	ok, err := p.or(allowed)
	if err != nil {
		return false, fmt.Errorf("licence %q: %w", expr, err)
	}
	if p.pos != len(p.toks) {
		return false, fmt.Errorf("licence %q: unexpected %q", expr, p.toks[p.pos])
	}
	return ok, nil
}

func tokenize(s string) []string {
	s = strings.NewReplacer("(", " ( ", ")", " ) ").Replace(s)
	return strings.Fields(s)
}

type exprParser struct {
	toks []string
	pos  int
}

func (p *exprParser) peek() string {
	if p.pos < len(p.toks) {
		return p.toks[p.pos]
	}
	return ""
}

func (p *exprParser) or(allowed map[string]bool) (bool, error) {
	ok, err := p.and(allowed)
	if err != nil {
		return false, err
	}
	for strings.EqualFold(p.peek(), "OR") {
		p.pos++
		right, err := p.and(allowed)
		if err != nil {
			return false, err
		}
		ok = ok || right
	}
	return ok, nil
}

func (p *exprParser) and(allowed map[string]bool) (bool, error) {
	ok, err := p.term(allowed)
	if err != nil {
		return false, err
	}
	for strings.EqualFold(p.peek(), "AND") {
		p.pos++
		right, err := p.term(allowed)
		if err != nil {
			return false, err
		}
		ok = ok && right
	}
	return ok, nil
}

func (p *exprParser) term(allowed map[string]bool) (bool, error) {
	t := p.peek()
	switch {
	case t == "":
		return false, fmt.Errorf("expression ends early")
	case t == "(":
		p.pos++
		ok, err := p.or(allowed)
		if err != nil {
			return false, err
		}
		if p.peek() != ")" {
			return false, fmt.Errorf("missing )")
		}
		p.pos++
		return ok, nil
	case t == ")" || isOperator(t):
		return false, fmt.Errorf("unexpected %q", t)
	}
	p.pos++
	id := strings.TrimSuffix(t, "+") // "GPL-2.0+" is still GPL-2.0 for us
	if strings.EqualFold(p.peek(), "WITH") {
		p.pos++
		exc := p.peek()
		if exc == "" || exc == "(" || exc == ")" || isOperator(exc) {
			return false, fmt.Errorf("WITH needs an exception id")
		}
		p.pos++
		id += " WITH " + exc
	}
	return allowed[id], nil
}

func isOperator(t string) bool {
	return strings.EqualFold(t, "AND") || strings.EqualFold(t, "OR") || strings.EqualFold(t, "WITH")
}

// ---------------------------------------------------------------------------
// npm tree
// ---------------------------------------------------------------------------

type lockfile struct {
	LockfileVersion int                  `json:"lockfileVersion"`
	Packages        map[string]lockEntry `json:"packages"`
}

type lockEntry struct {
	Name     string          `json:"name"`
	Version  string          `json:"version"`
	License  json.RawMessage `json:"license"`
	Dev      bool            `json:"dev"`
	Optional bool            `json:"optional"`
	Link     bool            `json:"link"`
	OS       []string        `json:"os"`
	CPU      []string        `json:"cpu"`
}

type manifest struct {
	License  json.RawMessage   `json:"license"`
	Licenses []json.RawMessage `json:"licenses"`
}

// NPMTree lists every package of a lockfileVersion 2/3 tree. The licence comes
// from the lockfile when npm recorded it there, otherwise from the installed
// package.json under root, so `npm ci` must have run. An optional,
// platform-specific build that is not installed on this machine (esbuild's
// darwin binary on a Linux runner) is skipped and counted, not reported: the
// build that is installed for this platform is checked instead. Any other
// package that is not installed is an error, not a pass.
func NPMTree(lockPath string) (deps []Dependency, skipped int, err error) {
	raw, err := os.ReadFile(lockPath)
	if err != nil {
		return nil, 0, err
	}
	var lf lockfile
	if err := json.Unmarshal(raw, &lf); err != nil {
		return nil, 0, fmt.Errorf("%s: %w", lockPath, err)
	}
	if lf.LockfileVersion < 2 {
		return nil, 0, fmt.Errorf("%s: lockfileVersion %d has no packages map; need 2 or 3", lockPath, lf.LockfileVersion)
	}
	root := filepath.Dir(lockPath)
	keys := make([]string, 0, len(lf.Packages))
	for k := range lf.Packages {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		e := lf.Packages[k]
		if k == "" || e.Link || !strings.Contains(k, "node_modules/") {
			continue // the root project itself, or a workspace link
		}
		name := e.Name
		if name == "" {
			name = k[strings.LastIndex(k, "node_modules/")+len("node_modules/"):]
		}
		lic := licenseField(e.License)
		if lic == "" {
			b, rerr := os.ReadFile(filepath.Join(root, filepath.FromSlash(k), "package.json"))
			switch {
			case rerr == nil:
				var m manifest
				if err := json.Unmarshal(b, &m); err != nil {
					return nil, 0, fmt.Errorf("%s/package.json: %w", k, err)
				}
				lic = manifestLicense(m)
			case errors.Is(rerr, os.ErrNotExist) && e.Optional && (len(e.OS) > 0 || len(e.CPU) > 0):
				skipped++
				continue
			case errors.Is(rerr, os.ErrNotExist):
				return nil, 0, fmt.Errorf("%s is in the lockfile but not installed; run npm ci first", k)
			default:
				return nil, 0, rerr
			}
		}
		if lic == "" {
			lic = Unknown
		}
		deps = append(deps, Dependency{Ecosystem: "npm", Name: name, Version: e.Version, License: lic, Dev: e.Dev})
	}
	return deps, skipped, nil
}

// licenseField reads `"license": "MIT"` and the legacy `"license": {"type": "MIT"}`.
func licenseField(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return strings.TrimSpace(s)
	}
	var o struct {
		Type string `json:"type"`
	}
	if json.Unmarshal(raw, &o) == nil {
		return strings.TrimSpace(o.Type)
	}
	return ""
}

// manifestLicense also reads the legacy `"licenses": [{"type": …}, …]` array,
// which npm documents as a choice between the entries, so it becomes an OR.
func manifestLicense(m manifest) string {
	if l := licenseField(m.License); l != "" {
		return l
	}
	var alts []string
	for _, r := range m.Licenses {
		if l := licenseField(r); l != "" {
			alts = append(alts, "("+l+")")
		}
	}
	return strings.Join(alts, " OR ")
}

// ---------------------------------------------------------------------------
// Go tree
// ---------------------------------------------------------------------------

// ParseGoModules reads `go list -deps -test -f '{{with .Module}}{{if not
// .Main}}{{.Path}} {{.Version}}{{end}}{{end}}' ./...` output: one "path
// version" per line. Duplicates and blank lines are dropped.
func ParseGoModules(r io.Reader) ([]Dependency, error) {
	seen := map[string]bool{}
	var out []Dependency
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) == 0 {
			continue
		}
		if len(f) != 2 {
			return nil, fmt.Errorf("go modules: want \"path version\", got %q", sc.Text())
		}
		if seen[f[0]] {
			continue
		}
		seen[f[0]] = true
		out = append(out, Dependency{Ecosystem: "go", Name: f[0], Version: f[1]})
	}
	return out, sc.Err()
}

type trivyReport struct {
	Results []struct {
		Licenses []struct {
			PkgName string `json:"PkgName"`
			Name    string `json:"Name"`
		} `json:"Licenses"`
	} `json:"Results"`
}

// ParseTrivyLicenses reads `trivy fs --scanners license --format json` and
// returns the licences found per package. Trivy reports one entry per licence
// file it classified, so a module with two licence files gets both, and must
// satisfy both.
func ParseTrivyLicenses(r io.Reader) (map[string][]string, error) {
	var rep trivyReport
	if err := json.NewDecoder(r).Decode(&rep); err != nil {
		return nil, fmt.Errorf("trivy licence report: %w", err)
	}
	out := map[string][]string{}
	for _, res := range rep.Results {
		for _, l := range res.Licenses {
			if l.PkgName == "" || l.Name == "" {
				continue // a loose licence file, not tied to a dependency
			}
			out[l.PkgName] = appendUnique(out[l.PkgName], l.Name)
		}
	}
	return out, nil
}

// GoTree gives each module `go list` built the licences Trivy found for it.
// A module Trivy found nothing for is Unknown: Trivy stays silent about a
// package with no classifiable licence, so silence must not read as a pass.
func GoTree(mods []Dependency, found map[string][]string) []Dependency {
	out := make([]Dependency, len(mods))
	for i, m := range mods {
		lics := found[m.Name]
		switch len(lics) {
		case 0:
			m.License = Unknown
		case 1:
			m.License = lics[0]
		default:
			sorted := append([]string(nil), lics...)
			sort.Strings(sorted)
			m.License = "(" + strings.Join(sorted, ") AND (") + ")"
		}
		out[i] = m
	}
	return out
}

func appendUnique(s []string, v string) []string {
	for _, x := range s {
		if x == v {
			return s
		}
	}
	return append(s, v)
}

// ---------------------------------------------------------------------------
// Verdict
// ---------------------------------------------------------------------------

// Violation is a dependency the policy does not accept.
type Violation struct {
	Dependency
	Why string
}

// Result is the verdict over every dependency checked.
type Result struct {
	Checked    []Dependency
	Excepted   []Dependency
	Violations []Violation
	Unused     []Exception // exceptions that matched no dependency
}

// Check applies the policy to deps. An exception covers a package only while
// its declared licence is exactly what the metadata says.
func Check(p Policy, deps []Dependency) Result {
	allowed := map[string]bool{}
	for _, a := range p.Allowed {
		allowed[a.ID] = true
	}
	excs := map[string]Exception{}
	for _, e := range p.Exceptions {
		excs[e.key()] = e
	}
	used := map[string]bool{}

	res := Result{Checked: deps}
	for _, d := range deps {
		k := d.Ecosystem + "\x00" + d.Name
		if e, ok := excs[k]; ok {
			if e.Declared == d.License {
				used[k] = true
				res.Excepted = append(res.Excepted, d)
				continue
			}
			used[k] = true
			res.Violations = append(res.Violations, Violation{d, fmt.Sprintf(
				"exception expects declared %q, metadata now says %q; re-review it", e.Declared, d.License)})
			continue
		}
		if d.License == Unknown {
			res.Violations = append(res.Violations, Violation{d, "no licence could be resolved; establish it and add an exception"})
			continue
		}
		ok, err := Permits(d.License, allowed)
		switch {
		case err != nil:
			res.Violations = append(res.Violations, Violation{d, err.Error()})
		case !ok:
			res.Violations = append(res.Violations, Violation{d, "licence not in the allowlist"})
		}
	}
	for _, e := range p.Exceptions {
		if !used[e.key()] {
			res.Unused = append(res.Unused, e)
		}
	}
	return res
}

// Render writes the census and the verdict, and returns whether the gate
// passes. GitHub Actions turns the ::error:: lines into annotations. An unused
// exception fails too: it would silently accept the next package of that name.
func Render(w io.Writer, res Result, policyErrs []error, skipped int) bool {
	var b bytes.Buffer
	pass := len(res.Violations) == 0 && len(policyErrs) == 0 && len(res.Unused) == 0

	census := map[string]int{}
	eco := map[string]int{}
	for _, d := range res.Checked {
		census[d.License]++
		eco[d.Ecosystem]++
	}
	ids := make([]string, 0, len(census))
	for id := range census {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		if census[ids[i]] != census[ids[j]] {
			return census[ids[i]] > census[ids[j]]
		}
		return ids[i] < ids[j]
	})
	fmt.Fprintf(&b, "Checked %d npm and %d Go dependencies", eco["npm"], eco["go"])
	if skipped > 0 {
		fmt.Fprintf(&b, " (%d npm builds for other platforms not installed, skipped)", skipped)
	}
	b.WriteString("\n\n")
	for _, id := range ids {
		fmt.Fprintf(&b, "%5d  %s\n", census[id], id)
	}
	b.WriteString("\n")

	if len(policyErrs) > 0 {
		fmt.Fprintf(&b, "Policy file: %d problem(s)\n", len(policyErrs))
		for _, e := range policyErrs {
			fmt.Fprintf(&b, "::error title=Licence policy::%s\n", e)
		}
		b.WriteString("\n")
	}

	if len(res.Violations) > 0 {
		fmt.Fprintf(&b, "BLOCKING: %d dependenc(ies) with a licence the policy does not accept\n", len(res.Violations))
		for _, v := range res.Violations {
			dev := ""
			if v.Dev {
				dev = ", dev"
			}
			fmt.Fprintf(&b, "::error title=Licence %s::%s is %q%s — %s\n", v.License, v.label(), v.License, dev, v.Why)
		}
		b.WriteString("\n")
	}

	if len(res.Excepted) > 0 {
		fmt.Fprintf(&b, "Excepted: %d dependenc(ies) covered by a per-package exception\n", len(res.Excepted))
		for _, d := range res.Excepted {
			fmt.Fprintf(&b, "  %s (declared %q)\n", d.label(), d.License)
		}
		b.WriteString("\n")
	}

	for _, e := range res.Unused {
		fmt.Fprintf(&b, "::error title=Unused licence exception::%s %s matched no dependency; remove it\n", e.Ecosystem, e.Package)
	}

	if pass {
		b.WriteString("Licence gate: PASS\n")
	} else {
		b.WriteString("Licence gate: FAIL\n")
	}
	_, _ = w.Write(b.Bytes())
	return pass
}
