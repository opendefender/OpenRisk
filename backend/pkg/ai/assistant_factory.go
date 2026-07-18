// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

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
