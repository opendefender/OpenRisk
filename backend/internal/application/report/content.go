// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package report

import (
	"fmt"
	"strings"
	"time"

	"github.com/opendefender/openrisk/internal/domain"
	pkgreport "github.com/opendefender/openrisk/pkg/report"
)

// Content is what a report SAYS, independent of the container it is delivered in.
//
// One content model rendered three ways, rather than three generators per type.
// The alternative — a PDF builder, a DOCX builder and an XLSX builder per report
// — is how the same report comes to say different things in different formats:
// someone fixes a total in the PDF path and the spreadsheet keeps the old one.
// Here a fix lands once.
type Content struct {
	Title    string
	Subtitle string
	// Meta is the identity block: organisation, period, language, template
	// version, and the integrity hash. Printed on every format.
	Meta []KeyValue
	// Summary is the handful of figures that answer "how are we doing".
	Summary  []KeyValue
	Sections []Section
	// Footer carries the confidentiality line and the hash.
	Footer string
}

// KeyValue is a labelled figure.
type KeyValue struct {
	Label string
	Value string
	// Status tints the value on paper (severity, implementation state). Empty
	// leaves it in ink.
	Status string
}

// Section is a heading with either prose or a table under it.
type Section struct {
	Heading string
	Body    []string
	Table   *Table
	// Empty is what to say when the table has no rows. A section that silently
	// disappears reads as "nothing to report"; one that says "no incident in the
	// period" is a statement the reader can rely on.
	Empty string
}

// Table is a header row plus rows.
type Table struct {
	Columns []string
	Rows    [][]string
	// StatusColumn indexes the column whose value tints the row on paper. -1 for
	// none.
	StatusColumn int
}

// Localise picks a string by document language.
func Localise(locale domain.ReportLocale, fr, en string) string {
	if locale == domain.ReportLocaleEN {
		return en
	}
	return fr
}

// FormatDate renders a date in the document's language, not the server's.
func FormatDate(locale domain.ReportLocale, t time.Time) string {
	if locale == domain.ReportLocaleEN {
		return t.Format("2 January 2006")
	}
	months := []string{"janvier", "février", "mars", "avril", "mai", "juin",
		"juillet", "août", "septembre", "octobre", "novembre", "décembre"}
	return fmt.Sprintf("%d %s %d", t.Day(), months[int(t.Month())-1], t.Year())
}

// Percent formats a ratio for display.
func Percent(v float64) string { return fmt.Sprintf("%.1f %%", v) }

// RenderDOCX turns Content into a Word document.
func (c Content) RenderDOCX() ([]byte, error) {
	doc := pkgreport.DocxDocument{
		Title:    c.Title,
		Subtitle: c.Subtitle,
		Footer:   c.Footer,
	}

	if len(c.Meta) > 0 {
		rows := [][]string{{"", ""}}
		rows = rows[:0]
		for _, kv := range c.Meta {
			rows = append(rows, []string{kv.Label, kv.Value})
		}
		doc.Blocks = append(doc.Blocks, pkgreport.DocxBlock{Table: rows})
	}

	if len(c.Summary) > 0 {
		doc.Blocks = append(doc.Blocks, pkgreport.DocxBlock{Heading: 2, Text: "Synthèse"})
		rows := make([][]string, 0, len(c.Summary)+1)
		rows = append(rows, []string{"Indicateur", "Valeur"})
		for _, kv := range c.Summary {
			rows = append(rows, []string{kv.Label, kv.Value})
		}
		doc.Blocks = append(doc.Blocks, pkgreport.DocxBlock{Table: rows})
	}

	for _, s := range c.Sections {
		doc.Blocks = append(doc.Blocks, pkgreport.DocxBlock{Heading: 2, Text: s.Heading})
		for _, p := range s.Body {
			doc.Blocks = append(doc.Blocks, pkgreport.DocxBlock{Text: p})
		}
		if s.Table != nil && len(s.Table.Rows) > 0 {
			rows := make([][]string, 0, len(s.Table.Rows)+1)
			rows = append(rows, s.Table.Columns)
			rows = append(rows, s.Table.Rows...)
			doc.Blocks = append(doc.Blocks, pkgreport.DocxBlock{Table: rows})
		} else if s.Table != nil && s.Empty != "" {
			doc.Blocks = append(doc.Blocks, pkgreport.DocxBlock{Text: s.Empty})
		}
	}

	return pkgreport.RenderDOCX(doc)
}

// RenderXLSX turns Content into a workbook: one sheet of identity and figures,
// then one per table.
func (c Content) RenderXLSX() ([]byte, error) {
	summary := pkgreport.XlsxSheet{Name: "Synthèse"}
	summary.Rows = append(summary.Rows, []string{c.Title})
	if c.Subtitle != "" {
		summary.Rows = append(summary.Rows, []string{c.Subtitle})
	}
	summary.Rows = append(summary.Rows, []string{})
	for _, kv := range c.Meta {
		summary.Rows = append(summary.Rows, []string{kv.Label, kv.Value})
	}
	if len(c.Summary) > 0 {
		summary.Rows = append(summary.Rows, []string{})
		for _, kv := range c.Summary {
			summary.Rows = append(summary.Rows, []string{kv.Label, kv.Value})
		}
	}

	sheets := []pkgreport.XlsxSheet{summary}
	for _, s := range c.Sections {
		if s.Table == nil {
			continue
		}
		sheet := pkgreport.XlsxSheet{Name: s.Heading}
		sheet.Rows = append(sheet.Rows, s.Table.Columns)
		if len(s.Table.Rows) == 0 && s.Empty != "" {
			// Say it rather than shipping an empty sheet the reader has to
			// interpret.
			sheet.Rows = append(sheet.Rows, []string{s.Empty})
		}
		sheet.Rows = append(sheet.Rows, s.Table.Rows...)
		sheets = append(sheets, sheet)
	}

	// A workbook of prose sections and no tables would be a single sheet; that is
	// fine and still readable, which is why no error is raised here.
	return pkgreport.RenderXLSX(sheets)
}

// RenderPDF turns Content into a PDF through the generic renderer, so the three
// formats are three views of one model rather than three implementations.
func (c Content) RenderPDF(landscape bool) ([]byte, error) {
	doc := pkgreport.GenericDoc{
		Title:     c.Title,
		Subtitle:  c.Subtitle,
		Footer:    c.Footer,
		Landscape: landscape,
	}
	for _, kv := range c.Meta {
		doc.Meta = append(doc.Meta, [2]string{kv.Label, kv.Value})
	}
	for _, kv := range c.Summary {
		doc.Summary = append(doc.Summary, pkgreport.GenericFigure{
			Label: kv.Label, Value: kv.Value, Status: kv.Status,
		})
	}
	for _, s := range c.Sections {
		sec := pkgreport.GenericSection{
			Heading: s.Heading, Body: s.Body, Empty: s.Empty, StatusColumn: -1,
		}
		if s.Table != nil {
			sec.Columns = s.Table.Columns
			sec.Rows = s.Table.Rows
			sec.StatusColumn = s.Table.StatusColumn
		}
		doc.Sections = append(doc.Sections, sec)
	}
	return pkgreport.RenderGenericPDF(doc)
}

// Render dispatches on format.
func (c Content) Render(format domain.ReportFormat, landscape bool) ([]byte, error) {
	switch format {
	case domain.ReportFormatDOCX:
		return c.RenderDOCX()
	case domain.ReportFormatXLSX:
		return c.RenderXLSX()
	default:
		return c.RenderPDF(landscape)
	}
}

// FingerprintContent hashes what the report SAYS, independent of the container
// it is delivered in.
//
// This is the value printed on the document. It is deliberately NOT the hash of
// the file: printing a file's own hash is impossible, because printing it
// changes the bytes being hashed. What this gives instead is a number two people
// can compare to know they are reading the same content — including across
// formats, since the PDF and the XLSX of one report fingerprint identically.
//
// The serialisation is written by hand rather than taken from encoding/json so
// the value cannot drift when a struct field is added for display purposes: only
// what a reader would notice contributes to it.
func FingerprintContent(c Content) string {
	var b strings.Builder
	b.WriteString(c.Title)
	b.WriteByte('\n')
	b.WriteString(c.Subtitle)
	b.WriteByte('\n')

	// Meta deliberately excluded: it carries the generation timestamp, so
	// including it would make every regeneration of unchanged data fingerprint
	// differently — which is exactly the comparison this exists to support.
	for _, kv := range c.Summary {
		b.WriteString(kv.Label)
		b.WriteByte('\x1f')
		b.WriteString(kv.Value)
		b.WriteByte('\x1e')
	}
	for _, s := range c.Sections {
		b.WriteString(s.Heading)
		b.WriteByte('\x1e')
		for _, p := range s.Body {
			b.WriteString(p)
			b.WriteByte('\x1e')
		}
		if s.Table != nil {
			for _, col := range s.Table.Columns {
				b.WriteString(col)
				b.WriteByte('\x1f')
			}
			b.WriteByte('\x1e')
			for _, row := range s.Table.Rows {
				for _, cell := range row {
					b.WriteString(cell)
					b.WriteByte('\x1f')
				}
				b.WriteByte('\x1e')
			}
		}
	}
	return domain.ComputeContentHash([]byte(b.String()))
}
