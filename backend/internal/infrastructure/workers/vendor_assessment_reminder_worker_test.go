// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package workers

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/application/tprm"
)

type fakeReminderSweeper struct {
	mu       sync.Mutex
	calls    int
	outcomes []tprm.ReminderOutcome
	err      error
}

func (f *fakeReminderSweeper) Execute(context.Context) ([]tprm.ReminderOutcome, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	return f.outcomes, f.err
}

func (f *fakeReminderSweeper) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func TestVendorAssessmentReminderWorker_SweepsAtBootAndStopsWithTheContext(t *testing.T) {
	sweeper := &fakeReminderSweeper{}
	w := NewVendorAssessmentReminderWorker(sweeper, zerolog.Nop()).WithInterval(time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		w.Start(ctx)
		close(done)
	}()
	require.Eventually(t, func() bool { return sweeper.count() == 1 }, time.Second, 5*time.Millisecond, "one sweep at boot")

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("the worker did not stop when its context was cancelled")
	}
}

func TestVendorAssessmentReminderWorker_LogsOutcomesWithoutAnyLink(t *testing.T) {
	var buf bytes.Buffer
	assessment := uuid.New()
	sweeper := &fakeReminderSweeper{outcomes: []tprm.ReminderOutcome{
		{TenantID: uuid.New(), AssessmentID: assessment, Offset: 3, Status: tprm.ReminderMailFailed, Err: errors.New("smtp: 421 try later")},
		{TenantID: uuid.New(), AssessmentID: uuid.New(), Offset: 7, Status: tprm.ReminderSent},
	}}
	w := NewVendorAssessmentReminderWorker(sweeper, zerolog.New(&buf))

	w.Sweep(context.Background())

	out := buf.String()
	assert.Contains(t, out, assessment.String())
	assert.Contains(t, out, `"outcome":"mail_failed"`)
	assert.Contains(t, out, `"offset_days":3`)
	assert.Contains(t, out, "smtp: 421 try later")
	assert.Contains(t, out, `"outcome":"sent"`)
	assert.NotContains(t, out, "vendor-questionnaire#", "no link, and so no token, is ever logged")
}

func TestVendorAssessmentReminderWorker_AListingFailureIsLoggedNotFatal(t *testing.T) {
	var buf bytes.Buffer
	w := NewVendorAssessmentReminderWorker(&fakeReminderSweeper{err: errors.New("db down")}, zerolog.New(&buf))

	assert.NotPanics(t, func() { w.Sweep(context.Background()) })
	assert.Contains(t, buf.String(), "db down")
}
