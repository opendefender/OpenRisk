// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package canonical

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
)

// shape is the set of keys a closed JSON object may carry. A key mapped to nil
// holds a scalar or opaque JSON and is not descended into; a key mapped to a
// shape holds an object, or an array of objects, of that shape.
type shape map[string]shape

// provenanceShape is every key D3 lists plus every key some source allowlists
// (D8). Which source may use which key is Validate's job; this is spelling.
var provenanceShape = shape{
	"method":                  nil,
	"actor":                   {"kind": nil, "id": nil},
	"via":                     nil,
	"ingest_id":               nil,
	"derived_from":            {"type": nil, "id": nil},
	"external_refs":           {"scheme": nil, "value": nil},
	"rule":                    nil,
	"confidence_rule":         nil,
	"raw_ref":                 nil,
	"channel":                 nil,
	"template_id":             nil,
	"template_version":        nil,
	"submitted_with_token_id": nil,
}

// attributesShape is every edge attribute D5 names.
var attributesShape = shape{
	attrCoverage:                      nil,
	attrRationale:                     nil,
	attrOrigin:                        nil,
	attrNote:                          nil,
	attrMatchConfidence:               nil,
	attrMatchMethod:                   nil,
	attrMatchAmbiguous:                nil,
	attrMatchCandidates:               nil, // opaque JSON (A7)
	attrResolvedBy:                    nil,
	attrAssetCriticalityAtObservation: nil,
}

// checkKeys refuses, at every nesting level the shape describes, a key not
// spelled exactly as a known key and a key given twice. encoding/json matches
// keys case-insensitively and keeps the last duplicate, so without this pass
// `{"RULE": <credential URL>, "rule": "ok"}` would decode and validate, and
// two spellings of one key could disagree. Neither the key nor any value is
// echoed in the error. Trailing data after the value is refused.
func checkKeys(field string, raw []byte, s shape) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := walkKeys(dec, field, s, 0); err != nil {
		return err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return invalid(field, "malformed JSON")
	}
	return nil
}

// maxJSONDepth bounds the token walk: deeper nesting is refused rather than
// followed, so a hostile document cannot make the walk recurse without limit.
const maxJSONDepth = 64

func walkKeys(dec *json.Decoder, field string, s shape, depth int) error {
	if depth > maxJSONDepth {
		return invalid(field, "nested too deeply")
	}
	tok, err := dec.Token()
	if err != nil {
		return invalid(field, "malformed JSON")
	}
	switch tok {
	case json.Delim('{'):
		seen := map[string]bool{}
		for dec.More() {
			keyTok, err := dec.Token()
			if err != nil {
				return invalid(field, "malformed JSON")
			}
			key, _ := keyTok.(string)
			var child shape
			if s != nil {
				known, ok := s[key]
				if !ok {
					return invalid(field, "unknown key")
				}
				if seen[key] {
					return invalid(field, "duplicate key")
				}
				seen[key] = true
				child = known
				if err := walkKeys(dec, field+"."+key, child, depth+1); err != nil {
					return err
				}
				continue
			}
			if err := walkKeys(dec, field, nil, depth+1); err != nil {
				return err
			}
		}
		if _, err := dec.Token(); err != nil {
			return invalid(field, "malformed JSON")
		}
	case json.Delim('['):
		for dec.More() {
			if err := walkKeys(dec, field, s, depth+1); err != nil {
				return err
			}
		}
		if _, err := dec.Token(); err != nil {
			return invalid(field, "malformed JSON")
		}
	}
	return nil
}

// decodeStrict runs checkKeys, then decodes into v refusing unknown fields.
func decodeStrict(field string, raw []byte, s shape, v any) error {
	if err := checkKeys(field, raw, s); err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return invalid(field, "%s", decodeReason(err))
	}
	return nil
}

// decodeReason describes a decoding failure without echoing a key or a value:
// a refused value may be a credential (ABSOLUTE RULE 6). A type error names
// the field path, which by then is made only of known keys.
func decodeReason(err error) string {
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) && typeErr.Field != "" {
		return typeErr.Field + " has the wrong type"
	}
	if strings.HasPrefix(err.Error(), "json: unknown field") {
		return "unknown key"
	}
	return "malformed JSON"
}
