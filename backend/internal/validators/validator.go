package validators

import (
    "github.com/go-playground/validator/v10"
)

var v *validator.Validate

func init() {
    v = validator.New()
}

// ValidateStruct validates a struct using go-playground/validator tags.
func ValidateStruct(s interface{}) error {
    return v.Struct(s)
}
