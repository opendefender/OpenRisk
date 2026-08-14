// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package report

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"strings"
	"testing"
)

// readPackage opens the produced bytes as a zip and returns each part.
// Hand-written OOXML has one failure mode that matters: a package a reader
// refuses to open. Parsing it back is the closest a unit test gets to that.
func readPackage(t *testing.T, b []byte) map[string]string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		t.Fatalf("output is not a readable zip: %v", err)
	}
	parts := map[string]string{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("cannot open %s: %v", f.Name, err)
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatalf("cannot read %s: %v", f.Name, err)
		}
		parts[f.Name] = string(data)
	}
	return parts
}

// assertWellFormedXML parses every XML part. A malformed part produces a file
// that downloads fine and then will not open, which the user experiences as the
// product being broken with no explanation.
func assertWellFormedXML(t *testing.T, parts map[string]string) {
	t.Helper()
	for name, body := range parts {
		if !strings.HasSuffix(name, ".xml") && !strings.HasSuffix(name, ".rels") {
			continue
		}
		dec := xml.NewDecoder(strings.NewReader(body))
		for {
			_, err := dec.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("%s is not well-formed XML: %v", name, err)
			}
		}
	}
}

func TestRenderDOCX_ProducesAReadablePackage(t *testing.T) {
	out, err := RenderDOCX(DocxDocument{
		Title:    "Rapport de conformité — COBAC",
		Subtitle: "Période : 2026-01-01 au 2026-06-30",
		Blocks: []DocxBlock{
			{Heading: 2, Text: "Synthèse"},
			{Text: "45 contrôles sur 45, dont 12 implémentés."},
			{Table: [][]string{
				{"Code", "Contrôle", "Statut"},
				{"R-2016/04 art. 12", "Dispositif de contrôle interne", "implemented"},
				{"R-2016/04 art. 18", "Séparation des tâches", "not_implemented"},
			}},
		},
		Footer: "Empreinte d'intégrité : a1b2c3d4e5f60718",
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	parts := readPackage(t, out)
	assertWellFormedXML(t, parts)

	for _, required := range []string{
		"[Content_Types].xml", "_rels/.rels",
		"word/document.xml", "word/styles.xml", "word/_rels/document.xml.rels",
	} {
		if _, ok := parts[required]; !ok {
			t.Errorf("package is missing %s — readers will refuse it", required)
		}
	}

	doc := parts["word/document.xml"]
	// Accented French must survive intact: the register is written in it.
	if !strings.Contains(doc, "Rapport de conformité — COBAC") {
		t.Error("the title did not survive into the document")
	}
	if !strings.Contains(doc, "Séparation des tâches") {
		t.Error("accented table content did not survive")
	}
	if !strings.Contains(doc, "a1b2c3d4e5f60718") {
		t.Error("the integrity hash must appear in the document itself")
	}
}

// The escaper is the whole defence against a corrupt download: a description
// containing an ampersand or an angle bracket is ordinary in a control register.
func TestRenderDOCX_EscapesContentThatWouldBreakTheFile(t *testing.T) {
	out, err := RenderDOCX(DocxDocument{
		Title: `Politique "sécurité" & <accès>`,
		Blocks: []DocxBlock{
			{Text: "R&D <script>alert('x')</script> — contrôle d'accès"},
			{Table: [][]string{{"A & B"}, {"<td>"}}},
		},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	parts := readPackage(t, out)
	assertWellFormedXML(t, parts)

	doc := parts["word/document.xml"]
	if strings.Contains(doc, "<script>") {
		t.Error("raw markup leaked into the document body")
	}
	if !strings.Contains(doc, "&amp;") {
		t.Error("ampersands should be escaped, not dropped")
	}
}

func TestRenderXLSX_ProducesAReadableWorkbook(t *testing.T) {
	out, err := RenderXLSX([]XlsxSheet{
		{Name: "Contrôles", Rows: [][]string{
			{"Code", "Nom", "Statut", "Preuves"},
			{"A.5.1", "Politiques de sécurité", "implemented", "2"},
			{"A.5.15", "Contrôle d'accès", "in_progress", "0"},
		}},
		{Name: "Écarts", Rows: [][]string{{"Code", "Écart"}, {"A.5.15", "Aucune preuve valide"}}},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	parts := readPackage(t, out)
	assertWellFormedXML(t, parts)

	for _, required := range []string{
		"[Content_Types].xml", "_rels/.rels", "xl/workbook.xml",
		"xl/_rels/workbook.xml.rels", "xl/worksheets/sheet1.xml", "xl/worksheets/sheet2.xml",
	} {
		if _, ok := parts[required]; !ok {
			t.Errorf("workbook is missing %s", required)
		}
	}

	sheet := parts["xl/worksheets/sheet1.xml"]
	// Reference codes must stay text. Written as numbers, "A.5.1" survives but
	// an identifier like "8.2.2" or one with a leading zero would be mangled —
	// which is why every cell is an inline string.
	if !strings.Contains(sheet, "A.5.1") {
		t.Error("the reference code did not survive")
	}
	if !strings.Contains(sheet, `t="inlineStr"`) {
		t.Error("cells should be inline strings so codes are not reinterpreted as numbers")
	}
	if !strings.Contains(parts["xl/workbook.xml"], "Contrôles") {
		t.Error("the sheet name did not survive")
	}
}

// Excel refuses a workbook whose sheet name is too long or contains []:*?/\ —
// producing a file that will not open at all.
func TestRenderXLSX_SanitisesSheetNames(t *testing.T) {
	out, err := RenderXLSX([]XlsxSheet{
		{Name: "Conformité / Écarts [2026]: rapport très long au-delà de la limite", Rows: [][]string{{"a"}}},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	parts := readPackage(t, out)
	assertWellFormedXML(t, parts)

	wb := parts["xl/workbook.xml"]
	for _, forbidden := range []string{"[", "]", ":", "*", "?", "/", "\\"} {
		// The name attribute is what matters; the XML around it legitimately
		// contains slashes in namespace URLs, so check the extracted name.
		name := betweenQuotes(wb, `<sheet name="`)
		if strings.Contains(name, forbidden) {
			t.Errorf("sheet name still contains %q: %s", forbidden, name)
		}
		if len([]rune(name)) > 31 {
			t.Errorf("sheet name is %d runes, Excel allows 31: %s", len([]rune(name)), name)
		}
	}
}

func TestRenderXLSX_RefusesAnEmptyWorkbook(t *testing.T) {
	if _, err := RenderXLSX(nil); err == nil {
		t.Fatal("a workbook with no sheets must be refused, not produced")
	}
}

// Byte-identical output for identical input is what makes the integrity hash
// meaningful: regenerating the same report must produce the same hash.
func TestRenderers_AreDeterministic(t *testing.T) {
	doc := DocxDocument{Title: "T", Blocks: []DocxBlock{{Text: "x"}, {Table: [][]string{{"a", "b"}}}}}
	a, err := RenderDOCX(doc)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	b, err := RenderDOCX(doc)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !bytes.Equal(a, b) {
		t.Error("two renders of the same document differ — the integrity hash would be meaningless")
	}

	sheets := []XlsxSheet{{Name: "S", Rows: [][]string{{"a"}, {"b"}}}}
	x, _ := RenderXLSX(sheets)
	y, _ := RenderXLSX(sheets)
	if !bytes.Equal(x, y) {
		t.Error("two renders of the same workbook differ")
	}
}

func TestColumnName(t *testing.T) {
	cases := map[int]string{0: "A", 1: "B", 25: "Z", 26: "AA", 27: "AB", 51: "AZ", 52: "BA"}
	for i, want := range cases {
		if got := columnName(i); got != want {
			t.Errorf("columnName(%d) = %s, want %s", i, got, want)
		}
	}
}

func betweenQuotes(s, prefix string) string {
	i := strings.Index(s, prefix)
	if i < 0 {
		return ""
	}
	rest := s[i+len(prefix):]
	j := strings.Index(rest, `"`)
	if j < 0 {
		return ""
	}
	return rest[:j]
}
