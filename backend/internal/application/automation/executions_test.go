// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package automation

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
)

func TestExecutionListByRule_Success(t *testing.T) {
	tenant, ruleID := uuid.New(), uuid.New()
	rules, execs := newMockRuleRepo(), newMockExecRepo()
	rules.add(&domain.AutomationRule{ID: ruleID, TenantID: tenant})
	execs.execs[uuid.New()] = &domain.AutomationExecution{TenantID: tenant, RuleID: ruleID}
	svc := NewExecutionService(execs, rules)

	out, err := svc.ListByRule(context.Background(), tenant, ruleID, 50)

	require.NoError(t, err)
	assert.Len(t, out, 1)
}

// #1520: a rule of another tenant and an unknown rule answer alike.
func TestExecutionListByRule_NotFound_ForeignAndUnknownAlike(t *testing.T) {
	tenant, other, foreign := uuid.New(), uuid.New(), uuid.New()
	rules, execs := newMockRuleRepo(), newMockExecRepo()
	rules.add(&domain.AutomationRule{ID: foreign, TenantID: other})
	execs.execs[uuid.New()] = &domain.AutomationExecution{TenantID: other, RuleID: foreign}
	svc := NewExecutionService(execs, rules)

	_, errForeign := svc.ListByRule(context.Background(), tenant, foreign, 50)
	_, errUnknown := svc.ListByRule(context.Background(), tenant, uuid.New(), 50)

	require.ErrorIs(t, errForeign, domain.ErrNotFound)
	require.ErrorIs(t, errUnknown, domain.ErrNotFound)
	assert.Equal(t, domain.MessageFromError(errForeign), domain.MessageFromError(errUnknown))
	assert.Equal(t, domain.HTTPStatusFromError(errForeign), domain.HTTPStatusFromError(errUnknown))
}

func TestExecutionListByRule_Unauthorized(t *testing.T) {
	svc := NewExecutionService(newMockExecRepo(), newMockRuleRepo())

	_, err := svc.ListByRule(context.Background(), uuid.Nil, uuid.New(), 50)

	require.ErrorIs(t, err, domain.ErrUnauthorized)
}
