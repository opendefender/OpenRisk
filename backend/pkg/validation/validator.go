// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package validation

import (
    "fmt"
    "reflect"
    "strings"
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
        // Report the JSON field name (not the Go struct field) in errors, so
        // client-facing messages match the payload the caller actually sent.
        v.RegisterTagNameFunc(func(fld reflect.StructField) string {
            name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
            if name == "-" || name == "" {
                return fld.Name
            }
            return name
        })
    })
    return v
}

// HumanizeErrors turns validator output into a field→message map answering
// WHAT/WHY/HOW instead of leaking the raw
// "Key: 'CreateRiskInput.Title' Error:Field validation..." string
// (audit-2026 #245). Returns nil if err is not a validator.ValidationErrors.
func HumanizeErrors(err error) map[string]string {
    var verrs validator.ValidationErrors
    if err == nil {
        return nil
    }
    var ok bool
    if verrs, ok = err.(validator.ValidationErrors); !ok {
        return nil
    }
    out := make(map[string]string, len(verrs))
    for _, fe := range verrs {
        field := fe.Field()
        label := humanizeField(field)
        param := fe.Param()
        var msg string
        switch fe.Tag() {
        case "required":
            msg = fmt.Sprintf("%s is required.", label)
        case "email":
            msg = fmt.Sprintf("%s must be a valid email address.", label)
        case "uuid4", "uuid":
            msg = fmt.Sprintf("%s must be a valid identifier.", label)
        case "min":
            msg = fmt.Sprintf("%s is too small (minimum %s).", label, param)
        case "max":
            msg = fmt.Sprintf("%s is too large (maximum %s).", label, param)
        case "oneof":
            msg = fmt.Sprintf("%s must be one of: %s.", label, strings.ReplaceAll(param, " ", ", "))
        case "gte":
            msg = fmt.Sprintf("%s must be at least %s.", label, param)
        case "lte":
            msg = fmt.Sprintf("%s must be at most %s.", label, param)
        default:
            msg = fmt.Sprintf("%s is invalid.", label)
        }
        out[field] = msg
    }
    return out
}

// humanizeField turns a json field name ("sle_xaf") into a readable label
// ("Sle xaf").
func humanizeField(name string) string {
    s := strings.ReplaceAll(name, "_", " ")
    if s == "" {
        return s
    }
    return strings.ToUpper(s[:1]) + s[1:]
}

// ValidateStruct validates a struct using go-playground/validator tags.
func ValidateStruct(s interface{}) error {
return GetValidator().Struct(s)
}
