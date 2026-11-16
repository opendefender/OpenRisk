// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package fileupload decides whether an uploaded document is acceptable from
// its bytes, and builds the header it is served back with (#1316).
//
// Neither the filename nor the client's Content-Type is consulted: both are
// whatever the uploader says. The type is sniffed with http.DetectContentType
// and must be on the policy's allowlist. It is the document counterpart of
// pkg/imageupload, which stays the model for pictures.
package fileupload

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"
)

var (
	// ErrTooLarge is returned for content over the policy's limit.
	ErrTooLarge = errors.New("file is too large")
	// ErrUnsupported is returned for a sniffed type that is not on the allowlist.
	ErrUnsupported = errors.New("file type is not allowed")
	// ErrEmpty is returned for an empty upload.
	ErrEmpty = errors.New("file is empty")
)

// EvidenceMaxBytes is the largest accepted evidence file. The product promises
// no figure (the upload control carries no size copy), so the limit is an
// explicit constant kept under the API's 4 MB request-body ceiling: a file
// between this and 4 MB is refused here with a clear message instead of being
// accepted on a technicality, and multipart framing never pushes a file at the
// limit over the body ceiling.
const EvidenceMaxBytes = 3 << 20

// CSVMaxBytes matches the import use cases' own 2 MB limit.
const CSVMaxBytes = 2 << 20

// Policy is a size limit and a sniffed-type allowlist.
type Policy struct {
	// MaxBytes is the largest accepted upload.
	MaxBytes int64
	// Allowed maps a sniffed media type (no parameters) to the extension the
	// file is stored under. The extension is server-chosen, never the client's.
	Allowed map[string]string
	// RequireUTF8 makes a text/plain upload fail unless the whole body is valid
	// UTF-8. Off for CSV: the importers check encoding themselves and tell the
	// user to re-save as CSV UTF-8 (422 not_utf8), which a 400 here would hide.
	RequireUTF8 bool
	// RefuseMarkup makes a text/plain upload fail when it contains active-content
	// markers (see markupMarkers). Set for files that are stored and served back.
	RefuseMarkup bool
}

// markupMarkers are refused (case-insensitively) anywhere in text evidence: the
// sniffer only looks at the first 512 bytes and a leading tag it does not know
// (<a:svg>, <!DOCTYPE svg>) reads as plain text.
var markupMarkers = []string{"<svg", ":svg", "<script", "<html", "<!doctype", "<iframe", "<object", "<embed", "javascript:"}

// Evidence is the allowlist for proof artifacts. text/html and text/xml (SVG)
// are absent on purpose: they are documents that carry script.
var Evidence = Policy{
	MaxBytes:     EvidenceMaxBytes,
	RequireUTF8:  true,
	RefuseMarkup: true,
	Allowed: map[string]string{
		"application/pdf": ".pdf",
		"image/png":       ".png",
		"image/jpeg":      ".jpg",
		"image/webp":      ".webp",
		"text/plain":      ".txt", // UTF-8 text, CSV included
		"application/zip": ".zip", // OOXML: .docx .xlsx .pptx
	},
}

// CSV accepts text only; the importers parse it, check its encoding and store
// nothing.
var CSV = Policy{
	MaxBytes: CSVMaxBytes,
	Allowed:  map[string]string{"text/plain": ".csv"},
}

// File is a validated upload held in memory.
type File struct {
	ContentType string
	Extension   string
	Data        []byte
}

// Reader returns a fresh reader over the file bytes.
func (f *File) Reader() io.Reader { return bytes.NewReader(f.Data) }

// Read consumes r, refusing it past MaxBytes, and sniffs its type.
func (p Policy) Read(r io.Reader) (*File, error) {
	data, err := io.ReadAll(io.LimitReader(r, p.MaxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("fileupload: read: %w", err)
	}
	return p.Check(data)
}

// Check validates bytes already read.
func (p Policy) Check(data []byte) (*File, error) {
	if len(data) == 0 {
		return nil, ErrEmpty
	}
	if int64(len(data)) > p.MaxBytes {
		return nil, ErrTooLarge
	}
	ct := Sniff(data)
	ext, ok := p.Allowed[ct]
	if !ok {
		return nil, ErrUnsupported
	}
	if ct == "text/plain" && !acceptableText(data, p.RequireUTF8, p.RefuseMarkup) {
		return nil, ErrUnsupported
	}
	return &File{ContentType: ct, Extension: ext, Data: data}, nil
}

// acceptableText applies to text/plain, which the sniffer grants to any bytes
// without a binary marker in the first 512: UTF-16 (BOM) is refused, with
// requireUTF8 the whole body must be valid UTF-8, and with refuseMarkup
// active-content markers are refused too.
func acceptableText(data []byte, requireUTF8, refuseMarkup bool) bool {
	if bytes.HasPrefix(data, []byte{0xFE, 0xFF}) || bytes.HasPrefix(data, []byte{0xFF, 0xFE}) {
		return false
	}
	if requireUTF8 && !utf8.Valid(data) {
		return false
	}
	if refuseMarkup {
		low := strings.ToLower(string(data))
		for _, m := range markupMarkers {
			if strings.Contains(low, m) {
				return false
			}
		}
	}
	return true
}

// Sniff returns the media type of data, without parameters. DetectContentType
// reports a bare <svg> document as plain text; it is reclassified here because
// SVG carries script and must never ride through the text allowlist.
func Sniff(data []byte) string {
	ct := http.DetectContentType(data)
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = ct[:i]
	}
	if ct == "text/plain" {
		head := data
		if len(head) > 512 {
			head = head[:512]
		}
		head = bytes.TrimPrefix(head, []byte("\xef\xbb\xbf"))
		head = bytes.TrimLeft(head, " \t\r\n\f\v")
		if len(head) >= 4 && strings.EqualFold(string(head[:4]), "<svg") {
			return "image/svg+xml"
		}
	}
	return ct
}

// AttachmentHeader builds an RFC 6266 Content-Disposition value for a download.
//
// The filename comes from the uploader, so it is hostile input: the ASCII
// fallback drops quotes, backslashes, path separators and every control
// character (a CR or LF there would split the header), and the real name
// travels percent-encoded in filename*, so a UTF-8 name survives intact.
func AttachmentHeader(name string) string {
	fallback := asciiFallback(name)
	h := `attachment; filename="` + fallback + `"`
	if !isPlainASCII(name) || fallback != name {
		if enc := percentEncode(cleanName(name)); enc != "" {
			h += "; filename*=UTF-8''" + enc
		}
	}
	return h
}

// cleanName removes control characters and path separators but keeps Unicode.
func cleanName(name string) string {
	var b strings.Builder
	for _, r := range name {
		if r < 0x20 || r == 0x7f || (r >= 0x80 && r < 0xa0) || r == '/' || r == '\\' || r == 0x2028 || r == 0x2029 {
			continue
		}
		b.WriteRune(r)
	}
	out := strings.TrimSpace(b.String())
	if len(out) > 200 {
		out = strings.ToValidUTF8(out[:200], "")
	}
	return out
}

func asciiFallback(name string) string {
	var b strings.Builder
	for _, r := range cleanName(name) {
		switch {
		case r == '"' || r == '%' || r == ';':
			continue
		case r > 0x7e:
			b.WriteByte('_')
		default:
			b.WriteRune(r)
		}
	}
	out := strings.TrimSpace(b.String())
	if out == "" {
		return "download"
	}
	return out
}

func isPlainASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

// percentEncode encodes every byte outside the RFC 5987 attr-char set.
func percentEncode(s string) string {
	const hex = "0123456789ABCDEF"
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' ||
			strings.IndexByte("!#$&+-.^_`|~", c) >= 0 {
			b.WriteByte(c)
			continue
		}
		b.WriteByte('%')
		b.WriteByte(hex[c>>4])
		b.WriteByte(hex[c&0xf])
	}
	return b.String()
}
