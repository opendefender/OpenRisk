// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package imageupload reads an uploaded picture (an avatar, an organization
// logo) and decides what it is from its bytes. The filename and the client's
// Content-Type are never consulted: both are whatever the uploader says.
package imageupload

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// MaxBytes is the largest accepted picture.
const MaxBytes = 1 << 20

var (
	// ErrTooLarge is returned for content over MaxBytes.
	ErrTooLarge = errors.New("image is larger than 1 MB")
	// ErrUnsupported is returned for anything that is not PNG, JPEG or WebP.
	ErrUnsupported = errors.New("image must be PNG, JPEG or WebP")
	// ErrEmpty is returned for an empty upload.
	ErrEmpty = errors.New("image is empty")
)

// allowed maps a sniffed content type to the extension it is stored under.
// SVG is excluded on purpose: it is a document that can carry script.
var allowed = map[string]string{
	"image/png":  ".png",
	"image/jpeg": ".jpg",
	"image/webp": ".webp",
}

// Image is a validated picture held in memory.
type Image struct {
	ContentType string
	Extension   string
	Data        []byte
}

// Read consumes r, refusing it past MaxBytes, and sniffs its type.
func Read(r io.Reader) (*Image, error) {
	data, err := io.ReadAll(io.LimitReader(r, MaxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("imageupload: read: %w", err)
	}
	if len(data) == 0 {
		return nil, ErrEmpty
	}
	if len(data) > MaxBytes {
		return nil, ErrTooLarge
	}
	ct := http.DetectContentType(data)
	ext, ok := allowed[ct]
	if !ok {
		return nil, ErrUnsupported
	}
	return &Image{ContentType: ct, Extension: ext, Data: data}, nil
}

// Reader returns a fresh reader over the image bytes.
func (i *Image) Reader() io.Reader { return bytes.NewReader(i.Data) }

// Sniff reports the content type of stored bytes, for serving them back.
func Sniff(data []byte) (string, bool) {
	ct := http.DetectContentType(data)
	_, ok := allowed[ct]
	return ct, ok
}
