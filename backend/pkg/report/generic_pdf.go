// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package report

import (
	"bytes"
	"fmt"

	"github.com/go-pdf/fpdf"
)

// A PDF renderer for the generic report content model — the counterpart of
// RenderDOCX and RenderXLSX, so the three formats of a report say the same thing.
//
// It uses the print palette rather than the screen one throughout, and every
// colour it puts on the page is one the palette test has already checked at
// 4.5:1 on white.

// GenericDoc is the flat document the reporting engine hands over.
type GenericDoc struct {
	Title    string
	Subtitle string
	Meta     [][2]string
	Summary  []GenericFigure
	Sections []GenericSection
	Footer   string
	// Landscape for wide registers. A twelve-column risk register on portrait A4
	// is a table nobody can read.
	Landscape bool
}

// GenericFigure is a labelled figure, optionally tinted by status.
type GenericFigure struct {
	Label  string
	Value  string
	Status string
}

// GenericSection is a heading with prose and/or a table.
type GenericSection struct {
	Heading string
	Body    []string
	Columns []string
	Rows    [][]string
	// Widths are relative column weights; empty spreads evenly.
	Widths []float64
	// StatusColumn tints a row by the value in that column. -1 for none.
	StatusColumn int
	Empty        string
}

const (
	pdfMargin     = 14.0
	pdfLineHeight = 5.0
)

// RenderGenericPDF renders the document.
func RenderGenericPDF(doc GenericDoc) ([]byte, error) {
	orientation := "P"
	if doc.Landscape {
		orientation = "L"
	}
	pdf := fpdf.New(orientation, "mm", "A4", "")
	pdf.SetMargins(pdfMargin, pdfMargin, pdfMargin)
	pdf.SetAutoPageBreak(true, 18)

	// The core fonts are Latin-1; the translator maps the accented French the
	// register is written in. Without it, "Séparation des tâches" reaches the page
	// as mojibake — the failure that made the compliance PDF unreadable before.
	tr := pdf.UnicodeTranslatorFromDescriptor("")

	// The footer carries the integrity hash on every page: a reader holding page
	// four of a printout can still check what they are holding.
	pdf.SetFooterFunc(func() {
		pdf.SetY(-14)
		pdf.SetFont("Helvetica", "", 7.5)
		setTextColor(pdf, PrintInkMuted)
		if doc.Footer != "" {
			pdf.CellFormat(0, 4, tr(doc.Footer), "", 1, "L", false, 0, "")
		}
		pdf.SetFont("Helvetica", "", 7.5)
		pdf.CellFormat(0, 4, fmt.Sprintf("%d", pdf.PageNo()), "", 0, "R", false, 0, "")
	})

	pdf.AddPage()

	// Title block.
	setTextColor(pdf, PrintInk)
	pdf.SetFont("Helvetica", "B", 18)
	pdf.MultiCell(0, 8, tr(doc.Title), "", "L", false)
	if doc.Subtitle != "" {
		pdf.SetFont("Helvetica", "", 10.5)
		setTextColor(pdf, PrintInkMuted)
		pdf.MultiCell(0, 5.5, tr(doc.Subtitle), "", "L", false)
	}
	pdf.Ln(3)

	// Identity block — two columns of label/value.
	if len(doc.Meta) > 0 {
		pdf.SetFont("Helvetica", "", 9)
		for _, kv := range doc.Meta {
			setTextColor(pdf, PrintInkMuted)
			pdf.CellFormat(45, 5, tr(kv[0]), "", 0, "L", false, 0, "")
			setTextColor(pdf, PrintInk)
			pdf.MultiCell(0, 5, tr(kv[1]), "", "L", false)
		}
		pdf.Ln(2)
		setDrawColor(pdf, PrintRule)
		y := pdf.GetY()
		pdf.Line(pdfMargin, y, pageWidth(pdf)-pdfMargin, y)
		pdf.Ln(4)
	}

	// Summary figures, as cards across the page.
	if len(doc.Summary) > 0 {
		drawFigures(pdf, tr, doc.Summary)
		pdf.Ln(4)
	}

	for _, s := range doc.Sections {
		drawSection(pdf, tr, s)
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("report: could not write the PDF: %w", err)
	}
	return buf.Bytes(), nil
}

func drawFigures(pdf *fpdf.Fpdf, tr func(string) string, figures []GenericFigure) {
	perRow := 3
	usable := pageWidth(pdf) - 2*pdfMargin
	w := (usable - float64(perRow-1)*3) / float64(perRow)

	for i := 0; i < len(figures); i += perRow {
		end := i + perRow
		if end > len(figures) {
			end = len(figures)
		}
		startY := pdf.GetY()
		x := pdfMargin
		for _, f := range figures[i:end] {
			pdf.SetXY(x, startY)
			setFillColor(pdf, PrintHeaderFill)
			pdf.Rect(x, startY, w, 16, "F")

			pdf.SetXY(x+3, startY+2.5)
			pdf.SetFont("Helvetica", "", 7.5)
			setTextColor(pdf, PrintInkMuted)
			pdf.CellFormat(w-6, 4, tr(f.Label), "", 2, "L", false, 0, "")

			pdf.SetX(x + 3)
			pdf.SetFont("Helvetica", "B", 13)
			setTextColor(pdf, StatusPrintColor(f.Status))
			pdf.CellFormat(w-6, 7, tr(f.Value), "", 0, "L", false, 0, "")

			x += w + 3
		}
		pdf.SetXY(pdfMargin, startY+19)
	}
}

func drawSection(pdf *fpdf.Fpdf, tr func(string) string, s GenericSection) {
	if s.Heading != "" {
		pdf.SetFont("Helvetica", "B", 12)
		setTextColor(pdf, PrintInk)
		pdf.MultiCell(0, 6, tr(s.Heading), "", "L", false)
		pdf.Ln(1)
	}
	for _, p := range s.Body {
		pdf.SetFont("Helvetica", "", 9.5)
		setTextColor(pdf, PrintInk)
		pdf.MultiCell(0, pdfLineHeight, tr(p), "", "L", false)
		pdf.Ln(1.5)
	}

	if len(s.Columns) == 0 {
		pdf.Ln(2)
		return
	}
	if len(s.Rows) == 0 {
		// Say so. A section that just stops reads as an omission.
		pdf.SetFont("Helvetica", "I", 9)
		setTextColor(pdf, PrintInkMuted)
		msg := s.Empty
		if msg == "" {
			msg = "Aucun élément."
		}
		pdf.MultiCell(0, pdfLineHeight, tr(msg), "", "L", false)
		pdf.Ln(3)
		return
	}

	widths := resolveWidths(pdf, s)
	drawTableHeader(pdf, tr, s.Columns, widths)

	pdf.SetFont("Helvetica", "", 8.5)
	for _, row := range s.Rows {
		// Height first: a wrapped cell decides the row, and drawing before
		// measuring is how table borders end up cutting through text.
		lines := 1
		cells := make([][]string, len(row))
		for i, cell := range row {
			if i >= len(widths) {
				break
			}
			wrapped := wrapToWidth(pdf, cell, widths[i]-3)
			cells[i] = wrapped
			if len(wrapped) > lines {
				lines = len(wrapped)
			}
		}
		h := float64(lines) * 4.6

		if pdf.GetY()+h > pageHeight(pdf)-20 {
			pdf.AddPage()
			drawTableHeader(pdf, tr, s.Columns, widths)
			pdf.SetFont("Helvetica", "", 8.5)
		}

		y := pdf.GetY()
		x := pdfMargin
		tint := PrintInk
		if s.StatusColumn >= 0 && s.StatusColumn < len(row) {
			tint = StatusPrintColor(row[s.StatusColumn])
		}

		for i := range widths {
			if i >= len(cells) {
				break
			}
			setDrawColor(pdf, PrintRule)
			pdf.Rect(x, y, widths[i], h, "D")
			if i == s.StatusColumn {
				setTextColor(pdf, tint)
			} else {
				setTextColor(pdf, PrintInk)
			}
			for li, line := range cells[i] {
				pdf.SetXY(x+1.5, y+float64(li)*4.6+0.8)
				pdf.CellFormat(widths[i]-3, 4, tr(line), "", 0, "L", false, 0, "")
			}
			x += widths[i]
		}
		pdf.SetXY(pdfMargin, y+h)
	}
	pdf.Ln(4)
}

func drawTableHeader(pdf *fpdf.Fpdf, tr func(string) string, columns []string, widths []float64) {
	pdf.SetFont("Helvetica", "B", 8.5)
	y := pdf.GetY()
	x := pdfMargin
	for i, col := range columns {
		if i >= len(widths) {
			break
		}
		setFillColor(pdf, PrintHeaderFill)
		setDrawColor(pdf, PrintRule)
		pdf.Rect(x, y, widths[i], 6.5, "FD")
		// Ink on a light fill, never white on a mid-tone: that is the pairing the
		// palette test pins at 4.5:1.
		setTextColor(pdf, PrintInk)
		pdf.SetXY(x+1.5, y+1.2)
		pdf.CellFormat(widths[i]-3, 4, tr(col), "", 0, "L", false, 0, "")
		x += widths[i]
	}
	pdf.SetXY(pdfMargin, y+6.5)
}

func resolveWidths(pdf *fpdf.Fpdf, s GenericSection) []float64 {
	usable := pageWidth(pdf) - 2*pdfMargin
	if len(s.Widths) == len(s.Columns) {
		total := 0.0
		for _, w := range s.Widths {
			total += w
		}
		out := make([]float64, len(s.Widths))
		for i, w := range s.Widths {
			out[i] = usable * w / total
		}
		return out
	}
	out := make([]float64, len(s.Columns))
	for i := range out {
		out[i] = usable / float64(len(s.Columns))
	}
	return out
}

// wrapToWidth breaks a string to fit, measuring with the font actually set.
//
// Not fpdf's SplitText: it panics on any rune above 255, which includes the em
// dash, the oe ligature and the typographic apostrophe that French descriptions
// are full of. That panic took down report generation once already.
func wrapToWidth(pdf *fpdf.Fpdf, s string, maxW float64) []string {
	if s == "" {
		return []string{""}
	}
	words := splitWords(s)
	lines := []string{}
	current := ""
	for _, w := range words {
		candidate := w
		if current != "" {
			candidate = current + " " + w
		}
		if pdf.GetStringWidth(candidate) <= maxW || current == "" {
			current = candidate
			continue
		}
		lines = append(lines, current)
		current = w
	}
	if current != "" {
		lines = append(lines, current)
	}
	// A single unbreakable token wider than the cell would otherwise overflow the
	// border; truncate it rather than let it run under the next column.
	for i, l := range lines {
		for pdf.GetStringWidth(l) > maxW && len([]rune(l)) > 1 {
			r := []rune(l)
			l = string(r[:len(r)-1])
		}
		lines[i] = l
	}
	if len(lines) == 0 {
		return []string{""}
	}
	return lines
}

func splitWords(s string) []string {
	out := []string{}
	current := []rune{}
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' {
			if len(current) > 0 {
				out = append(out, string(current))
				current = current[:0]
			}
			continue
		}
		current = append(current, r)
	}
	if len(current) > 0 {
		out = append(out, string(current))
	}
	return out
}

func pageWidth(pdf *fpdf.Fpdf) float64 {
	w, _ := pdf.GetPageSize()
	return w
}

func pageHeight(pdf *fpdf.Fpdf) float64 {
	_, h := pdf.GetPageSize()
	return h
}

func setTextColor(pdf *fpdf.Fpdf, c PrintColor) { pdf.SetTextColor(c.R, c.G, c.B) }
func setFillColor(pdf *fpdf.Fpdf, c PrintColor) { pdf.SetFillColor(c.R, c.G, c.B) }
func setDrawColor(pdf *fpdf.Fpdf, c PrintColor) { pdf.SetDrawColor(c.R, c.G, c.B) }
