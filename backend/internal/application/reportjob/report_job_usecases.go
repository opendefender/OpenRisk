// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package reportjob turns a report request into an addressable job.
//
// The navigation reason this exists: asking for a compliance report used to
// navigate the user to the Reports screen, whose "Compliance" tile navigated
// them back to Compliance — an infinite round trip that never produced a
// document. A report now has a URL of its own (/reports/jobs/:id), so the
// request terminates somewhere: on the artifact it asked for.
package reportjob

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"gorm.io/datatypes"
)

// Generator renders one report kind into bytes.
//
// Kept as an interface so a new report kind is a new Generator plus a registry
// entry, with no change to the job lifecycle, the handler or the frontend.
type Generator interface {
	// Kind is the ReportKind this generator serves.
	Kind() domain.ReportKind
	// Generate renders the report. The returned title/filename are stored on the
	// job so the list and the download read correctly without re-resolving.
	Generate(ctx context.Context, tenantID, requestedBy uuid.UUID, params map[string]any) (res GeneratedReport, err error)
}

// GeneratedReport is a rendered artifact.
type GeneratedReport struct {
	Title       string
	Filename    string
	ContentType string
	Bytes       []byte
}

// UseCase creates, runs and reads report jobs.
type UseCase struct {
	repo       domain.ReportJobRepository
	generators map[domain.ReportKind]Generator
}

// New builds the use case with the generators it can dispatch to.
func New(repo domain.ReportJobRepository, generators ...Generator) *UseCase {
	m := make(map[domain.ReportKind]Generator, len(generators))
	for _, g := range generators {
		if g != nil {
			m[g.Kind()] = g
		}
	}
	return &UseCase{repo: repo, generators: m}
}

// CreateInput is a report request.
type CreateInput struct {
	Kind   domain.ReportKind
	Params map[string]any
}

// Create records the job and renders it.
//
// Rendering is synchronous. The compliance PDF takes well under a second even
// for a 200-control framework, and a goroutine would buy nothing but a race
// against the client's first poll — while costing the caller any chance of
// seeing a validation failure (an unknown framework, say) as anything other
// than a job that mysteriously failed. The job model is here for addressability
// and for the stored artifact, not to escape a slow render. If a future kind is
// genuinely slow, it can return queued and complete out of band; the status
// field and the polling client already support that.
func (uc *UseCase) Create(ctx context.Context, tenantID, requestedBy uuid.UUID, in CreateInput) (*domain.ReportJob, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewValidationError("tenant is required")
	}
	if !in.Kind.Valid() {
		return nil, domain.NewValidationError("unknown report kind: " + string(in.Kind))
	}
	gen, ok := uc.generators[in.Kind]
	if !ok {
		return nil, domain.NewValidationError("no generator configured for " + string(in.Kind))
	}

	params := in.Params
	if params == nil {
		params = map[string]any{}
	}
	encoded, err := json.Marshal(params)
	if err != nil {
		return nil, domain.NewValidationError("invalid report parameters")
	}

	job := &domain.ReportJob{
		TenantID:    tenantID,
		Kind:        in.Kind,
		Status:      domain.ReportJobRunning,
		Params:      datatypes.JSON(encoded),
		RequestedBy: requestedBy,
	}
	if err := uc.repo.Create(ctx, job); err != nil {
		return nil, domain.NewInternalError("failed to record report job: " + err.Error())
	}

	res, genErr := gen.Generate(ctx, tenantID, requestedBy, params)
	now := time.Now()
	job.CompletedAt = &now

	if genErr != nil {
		job.Status = domain.ReportJobFailed
		// A user-safe reason: domain errors carry one, anything else does not and
		// must not be echoed back (RULE #6 / no raw internals to clients).
		if appErr, ok := genErr.(*domain.AppError); ok {
			job.Error = appErr.Message
		} else {
			job.Error = "report generation failed"
		}
		if err := uc.repo.Update(ctx, job); err != nil {
			return nil, domain.NewInternalError("failed to record report failure: " + err.Error())
		}
		// The job itself is returned, not an error: a failed report is a real
		// job the user can open, read the reason on, and retry from. Returning
		// an error here would leave them on the Reports screen with a toast,
		// which is the dead end this package exists to remove.
		return job, nil
	}

	job.Status = domain.ReportJobSucceeded
	job.Title = res.Title
	job.Filename = res.Filename
	job.ContentType = res.ContentType
	job.Artifact = res.Bytes
	job.SizeBytes = len(res.Bytes)
	if err := uc.repo.Update(ctx, job); err != nil {
		return nil, domain.NewInternalError("failed to store report: " + err.Error())
	}
	return job, nil
}

// Get returns one job (without its artifact stripped — callers that serialise it
// rely on Artifact being json:"-").
func (uc *UseCase) Get(ctx context.Context, tenantID, id uuid.UUID) (*domain.ReportJob, error) {
	return uc.repo.GetByID(ctx, tenantID, id)
}

// List returns the tenant's recent jobs, newest first.
func (uc *UseCase) List(ctx context.Context, tenantID uuid.UUID, limit int) ([]domain.ReportJob, error) {
	return uc.repo.List(ctx, tenantID, limit)
}
