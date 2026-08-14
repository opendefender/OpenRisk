// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package report

import "math"

// A palette for paper, separate from the one on screen.
//
// Screen colours are chosen against a dark or off-white app background, often
// with a hue that carries brand more than contrast. Printed on white — or
// photocopied, or read on a projector in a board room — the same values lose the
// contrast they had. A "high" severity chip that reads clearly in the product
// can land at 2.4:1 on paper, below the 4.5:1 WCAG AA asks for body text.
//
// So the document gets its own palette, and the ratios are ASSERTED IN A TEST
// (print_palette_test.go) rather than eyeballed. If someone later prefers a
// lighter amber, the test fails before the report ships.

// PrintColor is an RGB triple in the 0-255 range fpdf works in.
type PrintColor struct{ R, G, B int }

// Hex renders the colour as RRGGBB, for the OOXML writers.
func (c PrintColor) Hex() string {
	const digits = "0123456789ABCDEF"
	out := make([]byte, 6)
	for i, v := range []int{c.R, c.G, c.B} {
		out[i*2] = digits[(v>>4)&0xF]
		out[i*2+1] = digits[v&0xF]
	}
	return string(out)
}

// The print palette. Every foreground here clears 4.5:1 on PrintPaper.
var (
	// PrintPaper is the page. Pure white, because that is what a printer gives
	// you whatever the PDF says, and designing against an off-white the reader
	// will not have is how contrast estimates drift.
	PrintPaper = PrintColor{255, 255, 255}
	// PrintInk is body text and headings.
	PrintInk = PrintColor{17, 24, 39}
	// PrintInkMuted is secondary text (captions, footers). Still AA at body size.
	PrintInkMuted = PrintColor{75, 85, 99}
	// PrintRule is table borders and separators. Not held to a text ratio —
	// it is never text — but dark enough to survive a photocopy.
	PrintRule = PrintColor{156, 163, 175}
	// PrintHeaderFill is the table header band. Light, so the bold ink on top of
	// it keeps its contrast rather than the usual white-on-mid-tone.
	PrintHeaderFill = PrintColor{243, 244, 246}

	// Status colours, darkened for paper. These are the ones that fail most
	// often on screen-tuned values: amber especially.
	PrintCritical = PrintColor{153, 27, 27} // deep red
	PrintHigh     = PrintColor{154, 52, 18} // burnt orange, not amber
	PrintMedium   = PrintColor{133, 77, 14} // dark ochre
	PrintLow      = PrintColor{22, 101, 52} // forest green
	PrintNeutral  = PrintColor{55, 65, 81}  // slate
	PrintAccent   = PrintColor{30, 64, 175} // navy, for links and emphasis
)

// Hex forms, for the DOCX/XLSX writers which take strings.
var (
	printInkHex        = PrintInk.Hex()
	printRuleHex       = PrintRule.Hex()
	printHeaderFillHex = PrintHeaderFill.Hex()
)

// StatusPrintColor maps a severity or status word to its print colour.
//
// Falls back to neutral rather than guessing: an unrecognised status rendered in
// a colour picked at random tells the reader something the data does not say.
func StatusPrintColor(status string) PrintColor {
	switch status {
	case "critical", "CRITICAL", "critique":
		return PrintCritical
	case "high", "HIGH", "eleve", "élevé":
		return PrintHigh
	case "medium", "MEDIUM", "moyen":
		return PrintMedium
	case "low", "LOW", "faible":
		return PrintLow
	case "implemented", "approved", "valid", "published":
		return PrintLow
	case "not_implemented", "expired", "rejected", "failed":
		return PrintCritical
	case "in_progress", "in_review", "expiring_soon", "partial":
		return PrintMedium
	default:
		return PrintNeutral
	}
}

// relativeLuminance implements WCAG 2.1's definition.
func relativeLuminance(c PrintColor) float64 {
	channel := func(v int) float64 {
		s := float64(v) / 255.0
		if s <= 0.03928 {
			return s / 12.92
		}
		return math.Pow((s+0.055)/1.055, 2.4)
	}
	return 0.2126*channel(c.R) + 0.7152*channel(c.G) + 0.0722*channel(c.B)
}

// ContrastRatio returns the WCAG contrast ratio between two colours (1.0 to 21.0).
func ContrastRatio(a, b PrintColor) float64 {
	la, lb := relativeLuminance(a), relativeLuminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// MeetsAA reports whether a foreground clears WCAG AA on a background at body
// size (4.5:1). Exported so a generator can assert it at render time rather than
// trusting that the palette was not edited since.
func MeetsAA(foreground, background PrintColor) bool {
	return ContrastRatio(foreground, background) >= 4.5
}
