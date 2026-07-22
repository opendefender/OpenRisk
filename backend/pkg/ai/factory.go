// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial
// This file is part of the OpenRisk Enterprise Edition and is NOT covered by the
// AGPL; it is licensed under the OpenRisk Commercial License (see LICENSE.commercial).

package ai

import "strings"

// NewAdvisor picks the best available advisor: a ClaudeAdvisor when an API key
// is supplied, otherwise the deterministic TemplateAdvisor. The board-report use
// case additionally keeps a TemplateAdvisor as a runtime fallback, so even a
// ClaudeAdvisor that errors mid-request never blocks report generation.
//
// apiKey and model typically come from ANTHROPIC_API_KEY / ANTHROPIC_MODEL, read
// in the composition root (cmd/server/main.go) — this package never touches env.
func NewAdvisor(apiKey, model string) Advisor {
	if strings.TrimSpace(apiKey) == "" {
		return NewTemplateAdvisor()
	}
	return NewClaudeAdvisor(apiKey, model)
}
