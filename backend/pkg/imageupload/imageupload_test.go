// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package imageupload

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

func pngBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestRead_AcceptsPNGAndJPEGBySniffing(t *testing.T) {
	got, err := Read(bytes.NewReader(pngBytes(t)))
	if err != nil || got.ContentType != "image/png" || got.Extension != ".png" {
		t.Fatalf("png: %+v, %v", got, err)
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 2, 2)), nil); err != nil {
		t.Fatal(err)
	}
	if got, err = Read(&buf); err != nil || got.ContentType != "image/jpeg" {
		t.Fatalf("jpeg: %+v, %v", got, err)
	}
	webp := append([]byte("RIFF\x00\x00\x00\x00WEBPVP8 "), make([]byte, 32)...)
	if got, err = Read(bytes.NewReader(webp)); err != nil || got.ContentType != "image/webp" {
		t.Fatalf("webp: %+v, %v", got, err)
	}
}

func TestRead_RefusesSVGGIFAndScript(t *testing.T) {
	for name, body := range map[string][]byte{
		"svg":    []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`),
		"gif":    []byte("GIF89a\x01\x00\x01\x00\x00\x00\x00;"),
		"html":   []byte("<html><script>alert(1)</script></html>"),
		"binary": {0x00, 0x01, 0x02},
	} {
		if _, err := Read(bytes.NewReader(body)); !errors.Is(err, ErrUnsupported) {
			t.Errorf("%s: want ErrUnsupported, got %v", name, err)
		}
	}
}

func TestRead_RefusesOversizeAndEmpty(t *testing.T) {
	big := append(pngBytes(t), make([]byte, MaxBytes)...)
	if _, err := Read(bytes.NewReader(big)); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("oversize: got %v", err)
	}
	if _, err := Read(bytes.NewReader(nil)); !errors.Is(err, ErrEmpty) {
		t.Fatalf("empty: got %v", err)
	}
}
