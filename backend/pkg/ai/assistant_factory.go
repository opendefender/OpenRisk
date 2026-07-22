// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial
// This file is part of the OpenRisk Enterprise Edition and is NOT covered by the
// AGPL; it is licensed under the OpenRisk Commercial License (see LICENSE.commercial).

package ai

import "strings"

// NewAssistant picks the best available assistant: a ClaudeAssistant when an API
// key is supplied, otherwise the deterministic TemplateAssistant. The application
// layer additionally keeps a TemplateAssistant as a runtime fallback, so even a
// ClaudeAssistant that errors mid-request never blocks a feature.
//
// apiKey and model typically come from ANTHROPIC_API_KEY / ANTHROPIC_MODEL, read
// in the composition root (cmd/server/main.go) — this package never touches env.
func NewAssistant(apiKey, model string) Assistant {
	if strings.TrimSpace(apiKey) == "" {
		return NewTemplateAssistant()
	}
	return NewClaudeAssistant(apiKey, model)
}

// IsLLMBacked reports whether an assistant talks to a real LLM (vs the template).
// The /ai/status endpoint uses it to tell the UI which mode is active.
func IsLLMBacked(a Assistant) bool {
	_, isTemplate := a.(*TemplateAssistant)
	return !isTemplate
}
