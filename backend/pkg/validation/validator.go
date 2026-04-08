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
