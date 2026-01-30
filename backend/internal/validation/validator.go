package validation

import (
    "sync"

    "github.com/go-playground/validator/v"
)

var (
    once sync.Once
    v    validator.Validate
)

// GetValidator returns a singleton validator instance
func GetValidator() validator.Validate {
    once.Do(func() {
        v = validator.New()
        // register uuid tag if needed (go-playground supports uuid validation)
    })
    return v
}
