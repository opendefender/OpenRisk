// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

package validation

import (
	"strings"
	"testing"
)

type sample struct {
	Title  string  `json:"title" validate:"required"`
	Impact float64 `json:"impact" validate:"required,min=0,max=10"`
	Email  string  `json:"email" validate:"omitempty,email"`
}

// TestHumanizeErrors_NoRawValidatorText is the regression for audit-2026 #245:
// the client must never receive the raw "Key: 'sample.Title' Error:Field
// validation for 'Title' failed on the 'required' tag" string.
func TestHumanizeErrors_NoRawValidatorText(t *testing.T) {
	err := GetValidator().Struct(sample{Impact: 20, Email: "not-an-email"})
	msgs := HumanizeErrors(err)
	if msgs == nil {
		t.Fatal("expected validation errors, got none")
	}
	// json field names, not Go struct fields.
	if _, ok := msgs["title"]; !ok {
		t.Fatalf("expected a 'title' error keyed by json name, got keys %v", keys(msgs))
	}
	if got := msgs["title"]; !strings.Contains(got, "required") {
		t.Errorf("title message = %q, want a human 'required' message", got)
	}
	if got := msgs["impact"]; !strings.Contains(got, "maximum") {
		t.Errorf("impact message = %q, want a human 'maximum' message", got)
	}
	if got := msgs["email"]; !strings.Contains(strings.ToLower(got), "email") {
		t.Errorf("email message = %q, want a human email message", got)
	}
	// No leakage of the raw validator format.
	for _, m := range msgs {
		if strings.Contains(m, "Key:") || strings.Contains(m, "Error:Field validation") {
			t.Errorf("message leaks raw validator text: %q", m)
		}
	}
}

func keys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
