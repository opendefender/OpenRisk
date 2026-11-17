// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package controlchain

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/domain/canonical"
)

// AttachResultEvidenceUseCase attaches an evidence of the library to a result.
type AttachResultEvidenceUseCase struct {
	repo ResultRepository
	now  Clock
}

// NewAttachResultEvidenceUseCase builds the use case.
func NewAttachResultEvidenceUseCase(repo ResultRepository, now Clock) *AttachResultEvidenceUseCase {
	return &AttachResultEvidenceUseCase{repo: repo, now: now}
}

// Execute builds the evidenced_by edge (test -> evidence, qualifier the result id)
// with the user_entered envelope and inserts it after the test, the result (of
// this test, in this tenant) and the evidence (live, in this tenant) resolved:
// any miss, a foreign tenant's included, is ErrNotFound. created is false when
// the pair already had its edge; the stored edge is returned and nothing is
// audited again. The same artifact may back two results of one test.
func (u *AttachResultEvidenceUseCase) Execute(ctx context.Context, c Caller, testID, resultID, evidenceID uuid.UUID) (link *ResultEvidenceView, created bool, err error) {
	if err := c.authorize(PermissionAttachEvidence); err != nil {
		return nil, false, err
	}
	conf, err := userConfidence()
	if err != nil {
		return nil, false, validation(err)
	}
	now := u.now().UTC()
	edgeID := uuid.New()
	qualifier := resultID
	edge := canonical.Edge{
		ID: edgeID, TenantID: c.TenantID,
		From:       canonical.RecordRef{Type: canonical.TypeControlTest, ID: testID},
		Verb:       canonical.VerbEvidencedBy,
		To:         canonical.EdgeTarget{Type: canonical.TypeEvidence, ID: evidenceID, TenantID: c.TenantID},
		Qualifier:  &qualifier,
		Source:     canonical.SourceOpenRiskUser,
		SourceID:   edgeID.String(),
		Confidence: conf,
		Provenance: userProvenance(c.UserID),
		CreatedAt:  now, UpdatedAt: now,
	}
	if err := validation(edge.Validate()); err != nil {
		return nil, false, err
	}
	after := domain.JSONMap{
		"test_id": testID.String(), "result_id": resultID.String(), "evidence_id": evidenceID.String(), "edge_id": edgeID.String(),
	}
	ev := auditEventOf(auditResultType, c, domain.AuditActionUpdate, "evidence attached to a control test result", nil, after,
		domain.StringList{"evidence"}, resultID, now)
	link, created, err = u.repo.AttachEvidence(ctx, c.TenantID, testID, resultID, edge, ev)
	if err != nil {
		return nil, false, fmt.Errorf("application.AttachControlTestResultEvidence: %w", err)
	}
	return link, created, nil
}
