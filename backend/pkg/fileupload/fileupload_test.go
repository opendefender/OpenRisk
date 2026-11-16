// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package fileupload

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

var (
	pdfBytes  = []byte("%PDF-1.7\n1 0 obj\n<<>>\nendobj\n")
	pngBytes  = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00")
	zipBytes  = []byte("PK\x03\x04\x14\x00\x00\x00\x08\x00word/document.xml")
	exeBytes  = []byte("MZ\x90\x00\x03\x00\x00\x00\x04\x00\x00\x00\xff\xff\x00\x00")
	htmlBytes = []byte("<html><script>alert(1)</script></html>")
	svgBytes  = []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`)
	xmlSVG    = []byte(`<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`)
)

func TestEvidencePolicy_AcceptsAllowlistBySniffing(t *testing.T) {
	for name, tc := range map[string]struct {
		data []byte
		ext  string
	}{
		"pdf": {pdfBytes, ".pdf"}, "png": {pngBytes, ".png"}, "zip": {zipBytes, ".zip"},
		"text": {[]byte("control,owner\nA.5.1,ops\n"), ".txt"},
	} {
		got, err := Evidence.Read(bytes.NewReader(tc.data))
		if err != nil || got.Extension != tc.ext {
			t.Errorf("%s: %+v, %v", name, got, err)
		}
	}
}

func TestEvidencePolicy_RefusesActiveContentAndUnknownBinaries(t *testing.T) {
	for name, body := range map[string][]byte{
		"html": htmlBytes, "svg": svgBytes, "xml-svg": xmlSVG, "exe": exeBytes, "nul": {0, 1, 2, 3},
	} {
		if _, err := Evidence.Read(bytes.NewReader(body)); !errors.Is(err, ErrUnsupported) {
			t.Errorf("%s: want ErrUnsupported, got %v", name, err)
		}
	}
}

func TestEvidencePolicy_RefusesOversizeAndEmpty(t *testing.T) {
	big := append(append([]byte{}, pdfBytes...), make([]byte, EvidenceMaxBytes)...)
	if _, err := Evidence.Read(bytes.NewReader(big)); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("oversize: got %v", err)
	}
	if _, err := Evidence.Read(bytes.NewReader(nil)); !errors.Is(err, ErrEmpty) {
		t.Fatalf("empty: got %v", err)
	}
}

func TestCSVPolicy_RefusesBinaryAcceptsText(t *testing.T) {
	if _, err := CSV.Check(pngBytes); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("png: got %v", err)
	}
	if _, err := CSV.Check(exeBytes); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("exe: got %v", err)
	}
	if _, err := CSV.Check([]byte("title,impact\nx,3\n")); err != nil {
		t.Fatalf("csv: %v", err)
	}
}

func TestAttachmentHeader_HostileNameCannotSplitTheHeader(t *testing.T) {
	h := AttachmentHeader("a\"\r\nX-Evil: 1.pdf")
	if strings.ContainsAny(h, "\r\n") {
		t.Fatalf("header carries CR/LF: %q", h)
	}
	if strings.Count(h, `"`) != 2 {
		t.Fatalf("an inner quote survived: %q", h)
	}
	if !strings.HasPrefix(h, `attachment; filename="aX-Evil: 1.pdf"`) {
		t.Fatalf("unexpected fallback: %q", h)
	}
}

func TestAttachmentHeader_UTF8NameIsPercentEncoded(t *testing.T) {
	h := AttachmentHeader("rapport sécurité.pdf")
	want := `attachment; filename="rapport s_curit_.pdf"; filename*=UTF-8''rapport%20s%C3%A9curit%C3%A9.pdf`
	if h != want {
		t.Fatalf("got  %q\nwant %q", h, want)
	}
	if got := AttachmentHeader("plain.pdf"); got != `attachment; filename="plain.pdf"` {
		t.Fatalf("plain: %q", got)
	}
	if got := AttachmentHeader("\r\n"); got != `attachment; filename="download"` {
		t.Fatalf("empty after cleaning: %q", got)
	}
}
