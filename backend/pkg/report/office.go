// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package report

import (
	"archive/zip"
	"bytes"
	"fmt"
	"strings"
)

// Office Open XML writers for DOCX and XLSX.
//
// Written by hand against the OOXML packaging rules rather than pulled from a
// library. Both formats are a zip of a few XML parts, the subset a report needs
// is small — paragraphs, a table, a sheet of rows — and the alternative was two
// more dependencies (each large, each with its own transitive tree) inside a
// product that ships to regulated environments and has to justify what it
// bundles. The parts written here are the minimum a conforming reader requires:
// Word and Excel open the output, as do LibreOffice and Google Docs.
//
// What this deliberately does NOT do: images, charts, styles beyond bold and
// headings, or cell formats beyond text and number. A report that needs those is
// a PDF.

// DocxDocument is a simple flow of blocks — the shape a report actually has.
type DocxDocument struct {
	Title    string
	Subtitle string
	Blocks   []DocxBlock
	// Footer is repeated on every page (integrity hash, confidentiality).
	Footer string
}

// DocxBlock is one element of the flow.
type DocxBlock struct {
	// Heading level 1-3; 0 means body text.
	Heading int
	Text    string
	Bold    bool
	// Table, when non-nil, renders instead of Text. First row is the header.
	Table [][]string
}

// RenderDOCX writes a Word document.
func RenderDOCX(doc DocxDocument) ([]byte, error) {
	var body strings.Builder

	if doc.Title != "" {
		body.WriteString(docxParagraph(doc.Title, 1, false))
	}
	if doc.Subtitle != "" {
		body.WriteString(docxParagraph(doc.Subtitle, 0, false))
	}

	for _, b := range doc.Blocks {
		if b.Table != nil {
			body.WriteString(docxTable(b.Table))
			continue
		}
		body.WriteString(docxParagraph(b.Text, b.Heading, b.Bold))
	}
	if doc.Footer != "" {
		body.WriteString(docxParagraph(doc.Footer, 0, false))
	}

	document := xmlHeader + `
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
<w:body>` + body.String() + `
<w:sectPr><w:pgSz w:w="11906" w:h="16838"/><w:pgMar w:top="1134" w:right="1134" w:bottom="1134" w:left="1134"/></w:sectPr>
</w:body></w:document>`

	return zipParts(map[string]string{
		"[Content_Types].xml": xmlHeader + `
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
<Default Extension="xml" ContentType="application/xml"/>
<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
<Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/>
</Types>`,
		"_rels/.rels": xmlHeader + `
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/_rels/document.xml.rels": xmlHeader + `
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>
</Relationships>`,
		"word/styles.xml":   docxStyles,
		"word/document.xml": document,
	})
}

const xmlHeader = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`

// docxStyles defines the handful of styles the reports use. Heading colours are
// the print palette from print_palette.go: dark enough on white to clear WCAG AA
// at body size, which screen-tuned brand colours generally are not.
var docxStyles = xmlHeader + `
<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
<w:style w:type="paragraph" w:styleId="Heading1"><w:name w:val="heading 1"/>
<w:pPr><w:spacing w:before="240" w:after="120"/></w:pPr>
<w:rPr><w:b/><w:sz w:val="32"/><w:color w:val="` + printInkHex + `"/></w:rPr></w:style>
<w:style w:type="paragraph" w:styleId="Heading2"><w:name w:val="heading 2"/>
<w:pPr><w:spacing w:before="200" w:after="100"/></w:pPr>
<w:rPr><w:b/><w:sz w:val="26"/><w:color w:val="` + printInkHex + `"/></w:rPr></w:style>
<w:style w:type="paragraph" w:styleId="Heading3"><w:name w:val="heading 3"/>
<w:pPr><w:spacing w:before="160" w:after="80"/></w:pPr>
<w:rPr><w:b/><w:sz w:val="22"/><w:color w:val="` + printInkHex + `"/></w:rPr></w:style>
</w:styles>`

func docxParagraph(text string, heading int, bold bool) string {
	var pPr, rPr string
	if heading >= 1 && heading <= 3 {
		pPr = fmt.Sprintf(`<w:pPr><w:pStyle w:val="Heading%d"/></w:pPr>`, heading)
	} else if bold {
		rPr = `<w:rPr><w:b/></w:rPr>`
	}
	return fmt.Sprintf(`<w:p>%s<w:r>%s<w:t xml:space="preserve">%s</w:t></w:r></w:p>`,
		pPr, rPr, escapeXML(text))
}

func docxTable(rows [][]string) string {
	if len(rows) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<w:tbl><w:tblPr><w:tblW w:w="5000" w:type="pct"/>` +
		`<w:tblBorders>` +
		`<w:top w:val="single" w:sz="4" w:color="` + printRuleHex + `"/>` +
		`<w:left w:val="single" w:sz="4" w:color="` + printRuleHex + `"/>` +
		`<w:bottom w:val="single" w:sz="4" w:color="` + printRuleHex + `"/>` +
		`<w:right w:val="single" w:sz="4" w:color="` + printRuleHex + `"/>` +
		`<w:insideH w:val="single" w:sz="4" w:color="` + printRuleHex + `"/>` +
		`<w:insideV w:val="single" w:sz="4" w:color="` + printRuleHex + `"/>` +
		`</w:tblBorders></w:tblPr>`)

	for i, row := range rows {
		b.WriteString(`<w:tr>`)
		for _, cell := range row {
			shade := ""
			runPr := ""
			if i == 0 {
				// Header row: a light fill, and bold text in ink — not white on a
				// mid-tone, which is where print contrast usually fails.
				shade = `<w:shd w:val="clear" w:fill="` + printHeaderFillHex + `"/>`
				runPr = `<w:rPr><w:b/></w:rPr>`
			}
			b.WriteString(`<w:tc><w:tcPr>` + shade + `</w:tcPr><w:p><w:r>` + runPr +
				`<w:t xml:space="preserve">` + escapeXML(cell) + `</w:t></w:r></w:p></w:tc>`)
		}
		b.WriteString(`</w:tr>`)
	}
	b.WriteString(`</w:tbl><w:p/>`)
	return b.String()
}

// XlsxSheet is one worksheet: a name and rows of cells.
type XlsxSheet struct {
	Name string
	Rows [][]string
}

// RenderXLSX writes a workbook.
//
// Everything is written as an inline string. Numbers-as-text is a real
// limitation — Excel will not sum a column of them — but the alternative,
// guessing which strings are numbers, silently mangles reference codes like
// "8.2.2" and identifiers with leading zeroes into figures. For a compliance
// register, keeping "A.5.1" intact matters more than autosum.
func RenderXLSX(sheets []XlsxSheet) ([]byte, error) {
	if len(sheets) == 0 {
		return nil, fmt.Errorf("report: a workbook needs at least one sheet")
	}

	parts := map[string]string{}
	var sheetRefs, contentOverrides, workbookRels strings.Builder

	for i, sh := range sheets {
		n := i + 1
		path := fmt.Sprintf("xl/worksheets/sheet%d.xml", n)
		parts[path] = xlsxSheetXML(sh)

		name := sh.Name
		if name == "" {
			name = fmt.Sprintf("Feuille%d", n)
		}
		// Excel refuses sheet names over 31 characters or containing []:*?/\ —
		// truncating here beats producing a file that will not open.
		name = sanitiseSheetName(name)

		fmt.Fprintf(&sheetRefs, `<sheet name="%s" sheetId="%d" r:id="rId%d"/>`, escapeXML(name), n, n)
		fmt.Fprintf(&contentOverrides,
			`<Override PartName="/%s" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>`, path)
		fmt.Fprintf(&workbookRels,
			`<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet%d.xml"/>`, n, n)
	}

	parts["[Content_Types].xml"] = xmlHeader + `
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
<Default Extension="xml" ContentType="application/xml"/>
<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>` +
		contentOverrides.String() + `</Types>`

	parts["_rels/.rels"] = xmlHeader + `
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>
</Relationships>`

	parts["xl/workbook.xml"] = xmlHeader + `
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"
          xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
<sheets>` + sheetRefs.String() + `</sheets></workbook>`

	parts["xl/_rels/workbook.xml.rels"] = xmlHeader + `
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		workbookRels.String() + `</Relationships>`

	return zipParts(parts)
}

func xlsxSheetXML(sh XlsxSheet) string {
	var b strings.Builder
	b.WriteString(xmlHeader + `
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`)
	for r, row := range sh.Rows {
		fmt.Fprintf(&b, `<row r="%d">`, r+1)
		for c, cell := range row {
			fmt.Fprintf(&b, `<c r="%s%d" t="inlineStr"><is><t xml:space="preserve">%s</t></is></c>`,
				columnName(c), r+1, escapeXML(cell))
		}
		b.WriteString(`</row>`)
	}
	b.WriteString(`</sheetData></worksheet>`)
	return b.String()
}

// columnName converts a zero-based index to a spreadsheet column (A, B, … AA).
func columnName(i int) string {
	name := ""
	for i >= 0 {
		name = string(rune('A'+i%26)) + name
		i = i/26 - 1
	}
	return name
}

func sanitiseSheetName(s string) string {
	replacer := strings.NewReplacer("[", "(", "]", ")", ":", "-", "*", "-", "?", "", "/", "-", "\\", "-")
	s = replacer.Replace(s)
	if len([]rune(s)) > 31 {
		s = string([]rune(s)[:31])
	}
	return s
}

// zipParts writes the OOXML package.
//
// Deflate throughout: a compliance register of a few hundred controls is mostly
// repeated XML tags, and stored entries would make the download several times
// larger for no gain.
func zipParts(parts map[string]string) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	// [Content_Types].xml must come first — some readers assume it.
	order := make([]string, 0, len(parts))
	if _, ok := parts["[Content_Types].xml"]; ok {
		order = append(order, "[Content_Types].xml")
	}
	for name := range parts {
		if name != "[Content_Types].xml" {
			order = append(order, name)
		}
	}
	// Stable order so the same input produces byte-identical output — which is
	// what makes the integrity hash reproducible.
	for i := 1; i < len(order); i++ {
		for j := i; j > 1 && order[j] < order[j-1]; j-- {
			order[j], order[j-1] = order[j-1], order[j]
		}
	}

	for _, name := range order {
		w, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate})
		if err != nil {
			return nil, fmt.Errorf("report: could not add %s: %w", name, err)
		}
		if _, err := w.Write([]byte(parts[name])); err != nil {
			return nil, fmt.Errorf("report: could not write %s: %w", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("report: could not finish the package: %w", err)
	}
	return buf.Bytes(), nil
}

// escapeXML escapes the five XML entities plus the control characters XML 1.0
// forbids outright — a stray 0x0C in a description would otherwise produce a
// file no reader will open, and the user would see a corrupt download with no
// explanation.
func escapeXML(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 16)
	for _, r := range s {
		switch r {
		case '&':
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		case '"':
			b.WriteString("&quot;")
		case '\'':
			b.WriteString("&apos;")
		case '\n':
			b.WriteString("&#10;")
		case '\t', '\r':
			b.WriteRune(r)
		default:
			if r < 0x20 {
				continue // not representable in XML 1.0
			}
			b.WriteRune(r)
		}
	}
	return b.String()
}
