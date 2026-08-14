// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	appreport "github.com/opendefender/openrisk/internal/application/report"
	"github.com/opendefender/openrisk/internal/domain"
)

// ReportStore is what the worker needs from the repository.
type ReportStore interface {
	ClaimQueued(ctx context.Context) (*domain.Report, error)
	Update(ctx context.Context, r *domain.Report) error
	UpdateProgress(ctx context.Context, id uuid.UUID, progress int, step string) error
}

// ProgressPublisher pushes progress to whoever is watching. Optional: without it
// the client falls back to polling the report, which still works.
type ProgressPublisher func(ctx context.Context, tenantID, reportID uuid.UUID, progress int, step, runState string)

// OrgNameLookup resolves the organisation name for the cover page.
type OrgNameLookup func(ctx context.Context, tenantID uuid.UUID) string

// ReportWorker renders queued reports.
//
// A poll loop rather than a queue subscription. The cadence is short enough that
// a user does not notice, and the alternative — publishing a job event — adds a
// failure mode where a report sits queued forever because a message was dropped
// while Redis restarted. Here the claim is the source of truth: anything queued
// gets picked up on the next tick, whatever happened in between.
type ReportWorker struct {
	store    ReportStore
	sources  appreport.Sources
	publish  ProgressPublisher
	orgName  OrgNameLookup
	logger   zerolog.Logger
	interval time.Duration
	// concurrency bounds simultaneous renders. A report is CPU-bound (PDF
	// layout) and memory-bound (the whole document in RAM); rendering twenty at
	// once would let one tenant's bulk export starve every other request.
	concurrency int
}

func NewReportWorker(store ReportStore, sources appreport.Sources, logger zerolog.Logger) *ReportWorker {
	return &ReportWorker{
		store:       store,
		sources:     sources,
		logger:      logger,
		interval:    2 * time.Second,
		concurrency: 2,
	}
}

// WithProgressPublisher attaches the live progress channel.
func (w *ReportWorker) WithProgressPublisher(p ProgressPublisher) *ReportWorker {
	w.publish = p
	return w
}

// WithOrgLookup attaches the organisation-name resolver.
func (w *ReportWorker) WithOrgLookup(f OrgNameLookup) *ReportWorker {
	w.orgName = f
	return w
}

// WithInterval overrides the poll cadence (tests).
func (w *ReportWorker) WithInterval(d time.Duration) *ReportWorker {
	if d > 0 {
		w.interval = d
	}
	return w
}

func (w *ReportWorker) Start(ctx context.Context) {
	w.logger.Info().Int("workers", w.concurrency).Msg("Report worker started (async generation)")
	for i := 0; i < w.concurrency; i++ {
		go w.loop(ctx)
	}
}

func (w *ReportWorker) loop(ctx context.Context) {
	t := time.NewTicker(w.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			// Drain: several reports queued at once should not take one tick each.
			for {
				worked, err := w.RunOnce(ctx)
				if err != nil {
					w.logger.Warn().Err(err).Msg("report worker: could not claim a job")
					break
				}
				if !worked {
					break
				}
			}
		}
	}
}

// RunOnce claims and renders at most one report. Exported so a test can drive a
// single render without a ticker.
func (w *ReportWorker) RunOnce(ctx context.Context) (bool, error) {
	rep, err := w.store.ClaimQueued(ctx)
	if err != nil {
		return false, err
	}
	if rep == nil {
		return false, nil
	}
	w.render(ctx, rep)
	return true, nil
}

func (w *ReportWorker) render(ctx context.Context, rep *domain.Report) {
	started := time.Now()
	log := w.logger.With().Str("report_id", rep.ID.String()).Str("type", string(rep.Type)).Logger()

	// A render must never take the server down with it. fpdf panics on some
	// inputs (it did on a rune above 255 once), and a panic here would kill the
	// process for every tenant over one malformed description.
	defer func() {
		if r := recover(); r != nil {
			log.Error().Interface("panic", r).Msg("report worker: render panicked")
			w.fail(ctx, rep, "the document could not be produced")
		}
	}()

	tpl, err := appreport.TemplateFor(rep.Type)
	if err != nil {
		w.fail(ctx, rep, err.Error())
		return
	}

	req := appreport.Request{
		TenantID: rep.TenantID,
		Type:     rep.Type,
		Locale:   rep.Locale,
		Template: tpl,
	}
	if w.orgName != nil {
		req.OrgName = w.orgName(ctx, rep.TenantID)
	}
	applyParams(&req, rep.Params)

	progress := func(pct int, step string) {
		_ = w.store.UpdateProgress(ctx, rep.ID, pct, step)
		if w.publish != nil {
			w.publish(ctx, rep.TenantID, rep.ID, pct, step, string(domain.ReportRunRunning))
		}
	}
	progress(10, "collecting data")

	content, err := appreport.Build(ctx, w.sources, req, progress)
	if err != nil {
		// A user-safe reason: domain errors carry one, anything else must not be
		// echoed back (RULE #6).
		msg := "the report could not be generated"
		if appErr, ok := err.(*domain.AppError); ok {
			msg = appErr.Message
		}
		log.Warn().Err(err).Msg("report worker: build failed")
		w.fail(ctx, rep, msg)
		return
	}

	// Fingerprint the CONTENT, then render once with that fingerprint printed on
	// the page.
	//
	// The document cannot carry the hash of its own bytes — printing it would
	// change them. An earlier version rendered twice, printed the first render's
	// hash and stored the second's, which produced a number on the page that
	// never matched the number the API served: checkable-looking and wrong. The
	// fingerprint is over what the report SAYS, so it is stable across formats
	// and re-renders of the same data, and the file hash is computed from the
	// bytes afterwards.
	progress(85, "rendering")
	fingerprint := appreport.FingerprintContent(content)
	content.Footer = fmt.Sprintf("%s · %s : %s",
		appreport.Localise(rep.Locale, "Document confidentiel", "Confidential document"),
		appreport.Localise(rep.Locale, "empreinte du contenu", "content fingerprint"),
		fingerprint[:16])

	landscape := rep.Type == domain.ReportTypeRiskRegister
	final, err := content.Render(rep.Format, landscape)
	if err != nil {
		log.Warn().Err(err).Msg("report worker: render failed")
		w.fail(ctx, rep, "the document could not be produced")
		return
	}

	now := time.Now()
	rep.Artifact = final
	rep.SizeBytes = len(final)
	rep.ContentHash = domain.ComputeContentHash(final)
	rep.ContentFingerprint = fingerprint
	rep.ContentType = rep.Format.ContentType()
	rep.Filename = filenameFor(rep, content.Title)
	rep.Title = content.Title
	rep.RunState = domain.ReportRunSucceeded
	rep.Progress = 100
	rep.Step = "done"
	rep.CompletedAt = &now

	if err := w.store.Update(ctx, rep); err != nil {
		log.Error().Err(err).Msg("report worker: could not store the document")
		return
	}
	if w.publish != nil {
		w.publish(ctx, rep.TenantID, rep.ID, 100, "done", string(domain.ReportRunSucceeded))
	}
	log.Info().
		Int("bytes", len(final)).
		Str("format", string(rep.Format)).
		Dur("took", time.Since(started)).
		Msg("report worker: document produced")
}

func (w *ReportWorker) fail(ctx context.Context, rep *domain.Report, reason string) {
	now := time.Now()
	rep.RunState = domain.ReportRunFailed
	rep.Error = reason
	rep.Step = "failed"
	rep.CompletedAt = &now
	if err := w.store.Update(ctx, rep); err != nil {
		w.logger.Error().Err(err).Str("report_id", rep.ID.String()).
			Msg("report worker: could not record the failure")
	}
	if w.publish != nil {
		w.publish(ctx, rep.TenantID, rep.ID, rep.Progress, "failed", string(domain.ReportRunFailed))
	}
}

// applyParams reads the configurator's answers off the stored job.
//
// A malformed value is skipped rather than failing the render: the report is
// still worth producing at a wider scope, and the parameters were validated when
// the request was accepted.
func applyParams(req *appreport.Request, raw []byte) {
	if len(raw) == 0 {
		return
	}
	var params map[string]any
	if err := json.Unmarshal(raw, &params); err != nil {
		return
	}
	if s, ok := params["from"].(string); ok {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			req.From = t
		}
	}
	if s, ok := params["to"].(string); ok {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			req.To = t
		}
	}
	if s, ok := params["framework_id"].(string); ok {
		if id, err := uuid.Parse(s); err == nil {
			req.FrameworkID = id
		}
	}
	if s, ok := params["audit_id"].(string); ok {
		if id, err := uuid.Parse(s); err == nil {
			req.AuditID = id
		}
	}
	if list, ok := params["recipients"].([]any); ok {
		for _, r := range list {
			if s, ok := r.(string); ok {
				req.Recipients = append(req.Recipients, s)
			}
		}
	}
}

// filenameFor builds a download name that means something in a downloads folder.
func filenameFor(rep *domain.Report, title string) string {
	safe := make([]rune, 0, len(title))
	for _, r := range title {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			safe = append(safe, r)
		case r == ' ', r == '-', r == '_':
			safe = append(safe, '-')
		}
	}
	name := string(safe)
	if name == "" {
		name = string(rep.Type)
	}
	if len(name) > 60 {
		name = name[:60]
	}
	return fmt.Sprintf("%s-%s-v%d%s", name, rep.Locale, rep.Version, rep.Format.Extension())
}
