// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package compliance

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// PackBaseHashRule is the domain-separation prefix of the pack-base hash, and
// the version of the hashing rule (ADR 0013 D4, D-122).
//
// The stored pack_base_hash is char(64): the lowercase hex SHA-256 of
//
//	PackBaseHashRule + canonicalJSON(pack_base)
//
// The rule version is part of the HASHED INPUT, never of the stored value, so
// the column stays 64 hex characters. A future rule is "v2": a row's hash is
// then recomputed from its stored pack_base, which the row carries in full.
const PackBaseHashRule = "openrisk-pack-base/v1\n"

// CanonicalJSON encodes v as the canonical JSON the pack-base hash is taken
// over: object keys in sorted order (encoding/json sorts map keys), no
// insignificant whitespace, no HTML escaping, no trailing newline.
//
// v must be built of maps, slices, strings, numbers, bools and nil. Postgres
// jsonb reorders keys and adds whitespace on the way out, which is why the
// hash is always computed from a DECODED document (see HashPackBase), never
// from the bytes a database handed back.
func CanonicalJSON(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, fmt.Errorf("compliance: canonical json: %w", err)
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// HashPackBase returns the pack_base_hash of a pack_base document: the
// document is decoded and re-encoded canonically first, so the same content
// hashes the same whatever key order or whitespace it arrived in.
func HashPackBase(packBase []byte) (string, error) {
	dec := json.NewDecoder(bytes.NewReader(packBase))
	dec.UseNumber()
	var doc map[string]any
	if err := dec.Decode(&doc); err != nil {
		return "", fmt.Errorf("compliance: pack_base is not a JSON object: %w", err)
	}
	if dec.More() {
		return "", fmt.Errorf("compliance: pack_base holds more than one JSON value")
	}
	canon, err := CanonicalJSON(doc)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(append([]byte(PackBaseHashRule), canon...))
	return hex.EncodeToString(sum[:]), nil
}

// PackBase is a pack_base document with its hash, ready to store.
type PackBase struct {
	JSON string // canonical JSON
	Hash string // 64 lowercase hex characters
}

func newPackBase(doc map[string]any) PackBase {
	canon, err := CanonicalJSON(doc)
	if err != nil { // a map of strings cannot fail to encode
		panic(err)
	}
	h, err := HashPackBase(canon)
	if err != nil {
		panic(err)
	}
	return PackBase{JSON: string(canon), Hash: h}
}

// RequirementPackBase is the pack_base of a requirement: the three fields the
// pack manages (ADR 0013 D4) as the pack ships them.
func RequirementPackBase(c CatalogControl) PackBase {
	return newPackBase(map[string]any{
		"name":             c.Name,
		"description":      c.Description,
		"source_reference": c.SourceReference,
	})
}

// CrosswalkPackBase is the pack_base of a crosswalk link: coverage and
// rationale, the two fields the pack manages.
func CrosswalkPackBase(e CrosswalkEntry) PackBase {
	return newPackBase(map[string]any{
		"coverage":  string(e.Coverage),
		"rationale": e.Rationale,
	})
}

// CrosswalkItemKey is the pack_item_key of a crosswalk: "<catalog>:<code>|
// <catalog>:<code>", the two ends ordered by catalog key, then by code, so the
// key does not depend on which way round the entry is stored.
func CrosswalkItemKey(e CrosswalkEntry) string {
	a := e.SourceCatalog + ":" + e.SourceCode
	b := e.TargetCatalog + ":" + e.TargetCode
	if e.SourceCatalog > e.TargetCatalog ||
		(e.SourceCatalog == e.TargetCatalog && e.SourceCode > e.TargetCode) {
		a, b = b, a
	}
	return a + "|" + b
}
