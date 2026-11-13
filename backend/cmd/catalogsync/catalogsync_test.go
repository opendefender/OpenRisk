// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"strings"
	"testing"
)

// A cut-down Official Journal page with the shapes the real ones use: an
// article whose paragraphs carry points, a point list followed by a
// subparagraph, roman sub-points inside a point, and a chapter heading that
// must not leak into the article before it.
const ojFixture = `<html><body>
<p>(1) Considérant que l'article 2 s'applique.</p>
<p class="oj-ti-art">Article 2</p><p class="oj-sti-art">Notification</p>
<p>1. Les entités notifient <span>sans retard</span>.</p>
<p>Deuxième alinéa du paragraphe 1.</p>
<p>2. Les entités soumettent:</p>
<table><tr><td><p>a)</p></td><td><p>une alerte précoce;</p></td></tr></table>
<table><tr><td><p>b)</p></td><td><p>un rapport final comprenant:</p></td></tr></table>
<table><tr><td><p>i)</p></td><td><p>une description;</p></td></tr></table>
<table><tr><td><p>ii)</p></td><td><p>la cause;</p></td></tr></table>
<table><tr><td><p>c)</p></td><td><p>un rapport d&#8217;avancement.</p></td></tr></table>
<p>Par dérogation au point b), un prestataire notifie autrement.</p>
<p>3. Les autorités répondent.</p>
<p>CHAPITRE IV</p><p>Tests</p>
<p class="oj-ti-art">Article 3</p><p class="oj-sti-art">Sans paragraphe</p>
<p>Les entités testent leurs systèmes.</p>
<p>ANNEXE</p>
<p>Exigences techniques</p>
<p>1. Politique de sécurité [article 21, paragraphe 2, point a), de la directive (UE) 2022/2555]</p>
<p>1.1. Politique relative à la sécurité</p>
<p>1.1.1.</p><p>La politique définit l'approche.</p>
<p>1.2.</p><p>Rôles</p><p>1.2.1.</p><p>Les rôles sont définis.</p>
<p>2. Évaluation [article 21, paragraphe 2, point f), de la directive (UE) 2022/2555]</p>
<p>2.1.</p><p>Les entités évaluent l'efficacité.</p>
</body></html>`

func fixtureAct(t *testing.T) act {
	t.Helper()
	a, err := parseAct(textLines([]byte(ojFixture)))
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestParseAct_ArticlesTitlesAndDivisions(t *testing.T) {
	a := fixtureAct(t)
	if got := a.articles[2].title; got != "Notification" {
		t.Errorf("title = %q", got)
	}
	for _, l := range a.articles[2].body {
		if strings.Contains(l, "CHAPITRE") || l == "Tests" {
			t.Errorf("chapter heading leaked into article 2: %q", l)
		}
	}
	if got := joinText(a.articles[3].body); got != "Les entités testent leurs systèmes." {
		t.Errorf("article 3 = %q", got)
	}
}

func TestEuText_Paragraph_IncludesItsSubparagraphs(t *testing.T) {
	got, err := euText(fixtureAct(t), euRow{Code: "x", Art: 2, Para: 1})
	if err != nil {
		t.Fatal(err)
	}
	if want := "1. Les entités notifient sans retard. Deuxième alinéa du paragraphe 1."; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestEuText_Point_KeepsChapeauAndRomanSubPoints(t *testing.T) {
	got, err := euText(fixtureAct(t), euRow{Code: "x", Art: 2, Para: 2, Point: "b"})
	if err != nil {
		t.Fatal(err)
	}
	want := "2. Les entités soumettent: […] b) un rapport final comprenant: i) une description; ii) la cause;"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// The subparagraph after the last point belongs to the paragraph, not to it.
func TestEuText_LastPoint_StopsAtTheNextSubparagraph(t *testing.T) {
	got, err := euText(fixtureAct(t), euRow{Code: "x", Art: 2, Para: 2, Point: "c"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "Par dérogation") {
		t.Errorf("point c) swallowed the next subparagraph: %q", got)
	}
	if !strings.HasSuffix(got, "c) un rapport d’avancement.") {
		t.Errorf("got %q", got)
	}
}

func TestEuText_MissingProvision_IsAnError(t *testing.T) {
	a := fixtureAct(t)
	for _, r := range []euRow{
		{Code: "art", Art: 9},
		{Code: "para", Art: 2, Para: 7},
		{Code: "point", Art: 2, Para: 2, Point: "f"},
	} {
		if _, err := euText(a, r); err == nil {
			t.Errorf("%s: a provision absent from the source must fail the generation", r.Code)
		}
	}
}

func TestCheckCoverage(t *testing.T) {
	a := fixtureAct(t)
	rows := []euRow{{Art: 2, Para: 1}, {Art: 2, Para: 2, Point: "a"}, {Art: 3}}
	if err := checkCoverage(a, rows, []euExclusion{{Art: 2, Paras: []int{3}, Reason: "autorités"}}, 2, 3); err != nil {
		t.Errorf("fully covered: %v", err)
	}
	err := checkCoverage(a, rows, nil, 2, 3)
	if err == nil || !strings.Contains(err.Error(), "art. 2(3)") {
		t.Errorf("an uncovered paragraph must be named, got %v", err)
	}
}

func TestParseAnnex_InlineAndSeparateHeadings(t *testing.T) {
	s := parseAnnex(fixtureAct(t).annex)
	if len(s) != 3 {
		t.Fatalf("want 3 sections, got %d: %+v", len(s), s)
	}
	if s[0].Num != "1.1" || s[0].Heading != "Politique relative à la sécurité" {
		t.Errorf("inline heading: %+v", s[0])
	}
	if s[1].Num != "1.2" || s[1].Heading != "Rôles" || joinText(s[1].Body) != "1.2.1. Les rôles sont définis." {
		t.Errorf("separate heading: %+v", s[1])
	}
	if s[2].Heading != "" || joinText(s[2].Body) != "Les entités évaluent l'efficacité." {
		t.Errorf("headless section: %+v", s[2])
	}
	title, ref := splitHeading(s[2].Parent)
	if title != "Évaluation" || ref != "article 21, paragraphe 2, point f)" {
		t.Errorf("splitHeading = %q, %q", title, ref)
	}
}

func TestNIST_StatementWithParameters(t *testing.T) {
	params := map[string]oscalParam{
		"p1": {ID: "p1", Label: "frequency"},
		"p2": {ID: "p2", Label: "organization-defined personnel or roles"},
		"p3": {ID: "p3", Select: &struct {
			HowMany string   `json:"how-many"`
			Choice  []string `json:"choice"`
		}{HowMany: "one-or-more", Choice: []string{"system-level", "{{ insert: param, p1 }}"}}},
	}
	parts := []oscalPart{{Name: "statement", Parts: []oscalPart{
		{Name: "item", Props: []oscalProp{{Name: "label", Value: "a."}}, Prose: "Review {{ insert: param, p1 }} with {{ insert: param, p2 }};"},
		{Name: "item", Props: []oscalProp{{Name: "label", Value: "b."}}, Prose: "Apply at {{ insert: param, p3 }}."},
	}}, {Name: "guidance", Prose: "Not part of the statement."}}
	got := statementText(parts, params)
	want := "a. Review [Assignment: organization-defined frequency] with [Assignment: organization-defined personnel or roles]; " +
		"b. Apply at [Selection (one or more): system-level; [Assignment: organization-defined frequency]]."
	if got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

func TestNIST_Rows_EnhancementsAndWithdrawn(t *testing.T) {
	var cat oscalCatalog
	cat.Catalog.Metadata.Version = "5.2.0"
	stmt := func(p string) []oscalPart { return []oscalPart{{Name: "statement", Prose: p}} }
	lbl := func(v string) []oscalProp {
		return []oscalProp{{Name: "label", Value: v}, {Name: "label", Value: "AC-02", Class: "zero-padded"}}
	}
	cat.Catalog.Groups = []oscalGroup{{ID: "ac", Controls: []oscalControl{{
		ID: "ac-2", Title: "Account Management", Props: lbl("AC-2"), Parts: stmt("Manage accounts."),
		Controls: []oscalControl{
			{ID: "ac-2.1", Title: "Automated System Account Management", Props: lbl("AC-2(1)"), Parts: stmt("Automate.")},
			{ID: "ac-2.10", Title: "Gone", Props: append(lbl("AC-2(10)"), oscalProp{Name: "status", Value: "withdrawn"})},
		},
	}}}}
	rows, err := nistRows(cat, "https://example/src.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("withdrawn enhancement must be dropped, got %+v", rows)
	}
	if rows[1].Code != "AC-2(1)" || rows[1].Name != "Account Management | Automated System Account Management" {
		t.Errorf("enhancement row = %+v", rows[1])
	}
	if rows[0].Source != "NIST SP 800-53 Rev. 5.2.0, AC-2 (MODERATE baseline) — https://example/src.json" {
		t.Errorf("source = %q", rows[0].Source)
	}
}

// The spec tables themselves: no code twice, and every DORA exclusion has a reason.
func TestSpecs_UniqueCodesAndReasonedExclusions(t *testing.T) {
	for name, spec := range map[string][]euRow{"dora": doraRows, "nis2": nis2Rows} {
		seen := map[string]bool{}
		for _, r := range spec {
			if seen[r.Code] {
				t.Errorf("%s: %s twice", name, r.Code)
			}
			seen[r.Code] = true
			if strings.TrimSpace(r.Name) == "" {
				t.Errorf("%s: %s has no name", name, r.Code)
			}
		}
	}
	for _, e := range doraExcluded {
		if e.Reason == "" {
			t.Errorf("art. %d excluded without a reason", e.Art)
		}
	}
}
