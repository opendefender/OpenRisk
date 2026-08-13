// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package report

import "testing"

// The point of the print palette is that its contrast is checked, not assumed.
// If someone later prefers a lighter amber because it looks better on screen,
// this fails before the document reaches a regulator.
func TestPrintPalette_MeetsAAOnPaper(t *testing.T) {
	cases := []struct {
		name string
		fg   PrintColor
	}{
		{"body ink", PrintInk},
		{"muted ink", PrintInkMuted},
		{"critical", PrintCritical},
		{"high", PrintHigh},
		{"medium", PrintMedium},
		{"low", PrintLow},
		{"neutral", PrintNeutral},
		{"accent", PrintAccent},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ratio := ContrastRatio(tc.fg, PrintPaper)
			if ratio < 4.5 {
				t.Errorf("%s on paper is %.2f:1, below the 4.5:1 WCAG AA needs at body size", tc.name, ratio)
			}
			if !MeetsAA(tc.fg, PrintPaper) {
				t.Errorf("%s should pass MeetsAA", tc.name)
			}
		})
	}
}

// The table header band is the place print contrast usually fails: a mid-tone
// fill with white text looks smart on screen and turns to mush on paper. Here
// the fill is light and the text is ink, so the ratio has to hold.
func TestPrintPalette_TableHeaderTextIsReadable(t *testing.T) {
	if ratio := ContrastRatio(PrintInk, PrintHeaderFill); ratio < 4.5 {
		t.Errorf("header text on its fill is %.2f:1, below AA", ratio)
	}
	// And white on that fill must NOT be used — this asserts the trap is real,
	// so the constant is not "improved" into it later.
	if ContrastRatio(PrintPaper, PrintHeaderFill) >= 4.5 {
		t.Error("white on the header fill should be unreadable; if this passes the fill got too dark")
	}
}

// A ratio calculator that cannot reproduce the two ends of the scale is not
// measuring anything.
func TestContrastRatio_KnownValues(t *testing.T) {
	black := PrintColor{0, 0, 0}
	white := PrintColor{255, 255, 255}

	if got := ContrastRatio(black, white); got < 20.9 || got > 21.1 {
		t.Errorf("black on white should be 21:1, got %.2f", got)
	}
	if got := ContrastRatio(white, white); got < 0.99 || got > 1.01 {
		t.Errorf("white on white should be 1:1, got %.2f", got)
	}
	// Symmetric: order of arguments must not change the answer.
	if a, b := ContrastRatio(PrintInk, PrintPaper), ContrastRatio(PrintPaper, PrintInk); a != b {
		t.Errorf("contrast should be symmetric, got %.4f and %.4f", a, b)
	}
}

func TestStatusPrintColor_FallsBackRatherThanGuessing(t *testing.T) {
	if StatusPrintColor("critical") != PrintCritical {
		t.Error("critical should map to the deep red")
	}
	if StatusPrintColor("implemented") != PrintLow {
		t.Error("implemented should read as good")
	}
	// An unknown status must not be given a colour that asserts something.
	if StatusPrintColor("wibble") != PrintNeutral {
		t.Error("an unrecognised status must fall back to neutral, not pick a meaning")
	}
	// Whatever comes back is still readable.
	for _, s := range []string{"critical", "high", "medium", "low", "implemented", "wibble"} {
		if !MeetsAA(StatusPrintColor(s), PrintPaper) {
			t.Errorf("the colour chosen for %q is not readable on paper", s)
		}
	}
}

func TestPrintColor_Hex(t *testing.T) {
	if got := (PrintColor{255, 255, 255}).Hex(); got != "FFFFFF" {
		t.Errorf("want FFFFFF, got %s", got)
	}
	if got := (PrintColor{0, 0, 0}).Hex(); got != "000000" {
		t.Errorf("want 000000, got %s", got)
	}
	if got := (PrintColor{17, 24, 39}).Hex(); got != "111827" {
		t.Errorf("want 111827, got %s", got)
	}
}
