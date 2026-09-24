// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package workers

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/pkg/events"
	"github.com/opendefender/openrisk/pkg/scoring"
)

type captureAppender struct{ got []*domain.AuditEvent }

func (c *captureAppender) Append(_ context.Context, e *domain.AuditEvent) error {
	c.got = append(c.got, e)
	return nil
}

func TestScoreWorker_JournalsAScoreItMoves(t *testing.T) {
	sink := &captureAppender{}
	w := (&ScoreWorker{logger: zerolog.Nop()}).WithAudit(sink)
	tenant, risk := uuid.New(), uuid.New()
	b, err := scoring.NewEngine().Breakdown(0.7, 8, 1.5, nil)
	if err != nil {
		t.Fatal(err)
	}

	w.journalScoreChange(context.Background(), risk, tenant, 5.6, events.RiskUpdatedEvent{TriggeredBy: "system"}, b)

	if len(sink.got) != 1 {
		t.Fatalf("want one entry for a moved score, got %d", len(sink.got))
	}
	ev := sink.got[0]
	if ev.TenantID != tenant || ev.EntityID != risk.String() {
		t.Fatalf("wrong anchor: %s/%s", ev.TenantID, ev.EntityID)
	}
	if ev.ActorType != domain.AuditActorJob || ev.ActorLabel != domain.AuditJobScoreEngine {
		t.Fatalf("the engine must sign as job %q, got %q/%q", domain.AuditJobScoreEngine, ev.ActorType, ev.ActorLabel)
	}
	if ev.Before["score"] != 5.6 || ev.After["score"] != b.Score {
		t.Fatalf("before/after score: %v → %v", ev.Before, ev.After)
	}
}

func TestScoreWorker_UnchangedScoreIsNotAnEvent(t *testing.T) {
	sink := &captureAppender{}
	w := (&ScoreWorker{logger: zerolog.Nop()}).WithAudit(sink)
	b, _ := scoring.NewEngine().Breakdown(0.7, 8, 1.5, nil)

	w.journalScoreChange(context.Background(), uuid.New(), uuid.New(), b.Score, events.RiskUpdatedEvent{}, b)
	if len(sink.got) != 0 {
		t.Fatalf("a recompute that lands on the same score is not an event, got %d entries", len(sink.got))
	}

	// And without a trail attached the worker still runs.
	(&ScoreWorker{logger: zerolog.Nop()}).journalScoreChange(context.Background(), uuid.New(), uuid.New(), 0, events.RiskUpdatedEvent{}, b)
}
