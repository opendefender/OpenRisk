package scoring

import (
	"errors"
	"fmt"
)

// ErrValidation est l'erreur typée retournée pour tout paramètre
// hors range. Wrappée avec le message précis du paramètre invalide.
var ErrValidation = errors.New("scoring validation error")

// NewValidationError crée une ErrValidation wrappée avec le contexte.
// Format: "ErrValidation: <param> must be between <min> and <max>, got <value>"
func NewValidationError(param string, value float64, min, max float64) error {
	return fmt.Errorf("%w: %s must be between %.1f and %.1f, got %.4f",
		ErrValidation, param, min, max, value)
}
