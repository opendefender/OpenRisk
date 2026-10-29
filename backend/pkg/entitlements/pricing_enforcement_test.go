// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package entitlements

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// TestPricingDoc_FeatureMatrixMatchesTheCode proves the page says what the
// MATRIX declares. That is not the same as what the backend ENFORCES (#613): a
// Free "Basic" and a Business "Advanced" compliance meant nothing while every
// gate branched on a boolean.
//
// This test reads the published table and, for every step up a row — "—" to ✓,
// "Basic" to "Standard", ✓ to "Advanced" — asks whether some gate in the
// backend source tells the two apart. A step nothing distinguishes is a depth
// the page sells and the code does not deliver.
//
// The gates it recognises are the four ways the backend reads an entitlement,
// written with the package imported as `ent`:
//
//	RequireFeature(svc, ent.FeatX)            any enabled level
//	svc.Allowed(ctx, tenant, ent.FeatX)       any enabled level
//	RequireLevel(svc, ent.FeatX, ent.LevelY)  Y or deeper
//	FeatureLevelAtLeast(c, ent.FeatX, ent.LevelY)
//
// A gate written any other way is not seen, which can only make this test
// stricter, never let a gap through. Line comments are stripped first: a gate
// quoted in a comment enforces nothing and must not close a gap.
//
// knownUnenforced is the measured gap. An unlisted gap fails; a listed one that
// a gate now closes also fails, so the list cannot go stale in either direction.
// Support and SLA are descriptors (see levelRank) and are not checked.
// ---------------------------------------------------------------------------

// knownUnenforced lists every advertised step no gate distinguishes, keyed
// "<feature>:<the higher level>". Closing one means adding the gate and
// deleting the line.
var knownUnenforced = map[string]string{
	"api:on":              "Free's Limited API is the same API as Pro's — no quota or scope reads the level",
	"automation:on":       "no route reads FeatAutomation",
	"automation:advanced": "no route reads FeatAutomation",
	"ai_advisor:advanced": "the AI routes gate on/off only",
	"scanner:on":          "no route reads FeatScanner",
	"compliance:standard": "no route reads FeatCompliance; every plan gets the same compliance module",
	"compliance:advanced": "no route reads FeatCompliance",
	"compliance:custom":   "no route reads FeatCompliance",
	"cti:on":              "no route reads FeatCTI",
	"cti:advanced":        "no route reads FeatCTI",
	"governance:on":       "no route reads FeatGovernance",
	"governance:advanced": "no route reads FeatGovernance",
	"multi_tenant:on":     "no route reads FeatMultiTenant",
	"on_premise:on":       "a deployment property, not a route — nothing to gate in the request path",
}

var (
	constFeatureRe = regexp.MustCompile(`(Feat\w+)\s+Feature\s*=\s*"([^"]*)"`)
	constLevelRe   = regexp.MustCompile(`(Level\w+)\s+Level\s*=\s*"([^"]*)"`)
	lineCommentRe  = regexp.MustCompile(`(?m)//.*$`)

	gateAnyRe = []*regexp.Regexp{
		regexp.MustCompile(`RequireFeature\(\s*\w+\s*,\s*ent\.(Feat\w+)\s*\)`),
		regexp.MustCompile(`\.Allowed\([^()]*ent\.(Feat\w+)\s*\)`),
	}
	gateLevelRe = []*regexp.Regexp{
		regexp.MustCompile(`RequireLevel\(\s*\w+\s*,\s*ent\.(Feat\w+)\s*,\s*ent\.(Level\w+)\s*\)`),
		regexp.MustCompile(`FeatureLevelAtLeast\(\s*\w+\s*,\s*ent\.(Feat\w+)\s*,\s*ent\.(Level\w+)\s*\)`),
	}
)

// enforcedThresholds scans the backend's non-test sources and returns, per
// feature, every depth some gate compares against.
func enforcedThresholds(t *testing.T) map[Feature][]Level {
	t.Helper()
	src, err := os.ReadFile("entitlements.go")
	if err != nil {
		t.Fatal(err)
	}
	features := map[string]Feature{}
	for _, m := range constFeatureRe.FindAllStringSubmatch(string(src), -1) {
		features[m[1]] = Feature(m[2])
	}
	levels := map[string]Level{}
	for _, m := range constLevelRe.FindAllStringSubmatch(string(src), -1) {
		levels[m[1]] = Level(m[2])
	}

	out := map[Feature][]Level{}
	add := func(file, featName string, lvl Level) {
		f, ok := features[featName]
		if !ok {
			t.Errorf("%s: gate names %s, which is not a Feature constant", file, featName)
			return
		}
		out[f] = append(out[f], lvl)
	}

	for _, root := range []string{filepath.Join("..", "..", "cmd"), filepath.Join("..", "..", "internal")} {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			code := lineCommentRe.ReplaceAllString(string(b), "")
			for _, re := range gateAnyRe {
				for _, m := range re.FindAllStringSubmatch(code, -1) {
					add(path, m[1], LevelLimited) // the shallowest enabled depth
				}
			}
			for _, re := range gateLevelRe {
				for _, m := range re.FindAllStringSubmatch(code, -1) {
					lvl, ok := levels[m[2]]
					if !ok {
						t.Errorf("%s: gate names %s, which is not a Level constant", path, m[2])
						continue
					}
					add(path, m[1], lvl)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("scanning %s: %v", root, err)
		}
	}
	return out
}

// distinguishes reports whether a gate at depth th tells lo from hi: hi passes
// it and lo does not.
func distinguishes(th, lo, hi Level) bool {
	return hi.AtLeast(th) && !lo.AtLeast(th)
}

func TestPricingDoc_AdvertisedDepthsAreEnforced(t *testing.T) {
	raw, err := os.ReadFile(pricingDocPath())
	if err != nil {
		t.Fatalf("docs/PRICING.md must be readable: %v", err)
	}
	thresholds := enforcedThresholds(t)

	gaps := map[string]bool{}
	for _, cells := range tableRows(t, string(raw), "## Feature matrix", "## Pricing") {
		feat, ok := docFeatures[cells[0]]
		if !ok || feat == FeatSupport || feat == FeatSLA || len(cells) != 5 {
			continue // limits, descriptors, malformed rows: the other test owns them
		}
		// The distinct depths the row advertises, shallowest first.
		var ladder []Level
		for _, cell := range cells[1:] {
			lvl, known := docLevels[cell]
			if !known || !lvl.Ordered() {
				t.Errorf("%s: %q is not a depth", cells[0], cell)
				continue
			}
			if len(ladder) == 0 || !ladder[len(ladder)-1].AtLeast(lvl) {
				ladder = append(ladder, lvl)
			}
		}
		for i := 1; i < len(ladder); i++ {
			lo, hi := ladder[i-1], ladder[i]
			closed := false
			for _, th := range thresholds[feat] {
				if distinguishes(th, lo, hi) {
					closed = true
					break
				}
			}
			if !closed {
				gaps[string(feat)+":"+string(hi)] = true
			}
		}
	}

	var unlisted, stale []string
	for g := range gaps {
		if _, ok := knownUnenforced[g]; !ok {
			unlisted = append(unlisted, g)
		}
	}
	for g := range knownUnenforced {
		if !gaps[g] {
			stale = append(stale, g)
		}
	}
	sort.Strings(unlisted)
	sort.Strings(stale)
	for _, g := range unlisted {
		t.Errorf("PRICING.md advertises %s and no gate enforces it — add the gate, or "+
			"record the gap in knownUnenforced so it is measured, not hidden", g)
	}
	for _, g := range stale {
		t.Errorf("knownUnenforced lists %s, but a gate now enforces it (or the page "+
			"no longer advertises it) — delete the line", g)
	}
}

// TestPricingDoc_EnforcementScanSeesTheFinancialGates pins the scanner itself:
// if it stopped recognising the gates main.go mounts, every gap would look
// unenforced and the test above would fail for the wrong reason — or, worse,
// a rewrite could make it see nothing and pass vacuously with a full list.
func TestPricingDoc_EnforcementScanSeesTheFinancialGates(t *testing.T) {
	got := map[Level]bool{}
	for _, l := range enforcedThresholds(t)[FeatFinancialQuant] {
		got[l] = true
	}
	for _, want := range []Level{LevelBasic, LevelOn} {
		if !got[want] {
			t.Errorf("the scan found no financial_quantification gate at %q; found %v", want, got)
		}
	}
	if len(enforcedThresholds(t)[FeatVendorRisk]) == 0 {
		t.Error("the scan found no vendor_risk gate; main.go mounts RequireFeature for it")
	}
}
