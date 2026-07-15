// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package domain

import "github.com/google/uuid"

// MitigationProgressChanged event - published when progress calculation changes
type MitigationProgressChanged struct {
	PlanID   uuid.UUID `json:"plan_id"`
	Progress int       `json:"progress"`
	Status   string    `json:"status"` // New status if changed
}

// MitigationCompleted event - published when plan is validated to done (manually or via auto-completion of all subactions)
type MitigationCompleted struct {
	PlanID uuid.UUID `json:"plan_id"`
	RiskID uuid.UUID `json:"risk_id"`
	Source string    `json:"source"` // "manual" | "scanner" | "ai"
}

// MitigationAutoCompleted event - published when scanner auto-completes a subaction
type MitigationAutoCompleted struct {
	PlanID       uuid.UUID `json:"plan_id"`
	SubActionID  uuid.UUID `json:"sub_action_id"`
	ScannerJobID string    `json:"scanner_job_id"` // Reference to the scan run
	Evidence     string    `json:"evidence"`       // JSON or URL to scanner findings
}

// MitigationReverted event - published when a subaction is marked as incomplete
type MitigationReverted struct {
	PlanID      uuid.UUID `json:"plan_id"`
	SubActionID uuid.UUID `json:"sub_action_id"`
	RevertedBy  uuid.UUID `json:"reverted_by"`
}
