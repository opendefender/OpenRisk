// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

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
