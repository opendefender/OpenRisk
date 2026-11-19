// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package canonical

import "encoding/json"

// List is a contract array that ADR 0008 types "no, may be empty" (D2
// relationships; D6 tags, results, identifiers, smart_score.factors). Go's nil
// slice would encode as null, which the contract does not allow; a nil List
// encodes as []. Decoding is the default slice decoding.
//
// The method is on the list, not on Envelope: a MarshalJSON on the embedded
// Envelope would be promoted and replace each entity's whole encoding.
type List[T any] []T

// MarshalJSON writes [] for a nil list.
func (l List[T]) MarshalJSON() ([]byte, error) {
	if l == nil {
		return []byte("[]"), nil
	}
	return json.Marshal([]T(l))
}
