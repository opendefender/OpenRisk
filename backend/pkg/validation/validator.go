// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

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
