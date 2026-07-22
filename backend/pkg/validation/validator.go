// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package validation

import (
    "sync"

    "github.com/go-playground/validator/v10"
)

var (
    once sync.Once
    v    *validator.Validate
)

// GetValidator returns a singleton validator instance
func GetValidator() *validator.Validate {
    once.Do(func() {
        v = validator.New()
        // register uuid4 tag if needed (go-playground supports uuid validation)
    })
    return v
}

// ValidateStruct validates a struct using go-playground/validator tags.
func ValidateStruct(s interface{}) error {
return GetValidator().Struct(s)
}
