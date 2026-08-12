// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial
// This file is part of the OpenRisk Enterprise Edition and is NOT covered by the
// AGPL; it is licensed under the OpenRisk Commercial License (see LICENSE.commercial).

package automation

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	appauto "github.com/opendefender/openrisk/internal/application/automation"
	"github.com/opendefender/openrisk/internal/domain"
)

// SubjectResolver loads a REAL record from the tenant and rebuilds the trigger
// context a live event would have produced for it. This is what makes a dry run
// a test against production data rather than against a made-up payload: the
// severities, CVSS scores, KEV flags and asset tags are the tenant's own.
//
// Read-only by construction — it never writes, so a dry run cannot have a side
// effect even by accident.
type SubjectResolver struct{ db *gorm.DB }

// NewSubjectResolver builds the resolver.
func NewSubjectResolver(db *gorm.DB) *SubjectResolver { return &SubjectResolver{db: db} }

var _ appauto.SubjectResolver = (*SubjectResolver)(nil)

// Resolve loads the named record, or the tenant's most recent relevant one when
// no id is given. A nil context with no error means "this tenant has nothing to
// trace against yet" — the caller then falls back to a synthetic subject and
// says so, rather than failing or inventing data silently.
func (r *SubjectResolver) Resolve(ctx context.Context, tenantID uuid.UUID, trigger domain.AutomationTrigger, subjectType, subjectID string) (*appauto.TriggerContext, string, error) {
	kind := strings.ToLower(strings.TrimSpace(subjectType))
	if kind == "" {
		kind = defaultSubjectFor(trigger)
	}
	switch kind {
	case "vulnerability":
		return r.vulnerability(ctx, tenantID, subjectID)
	case "risk":
		return r.risk(ctx, tenantID, subjectID)
	case "incident":
		return r.incident(ctx, tenantID, subjectID)
	default:
		return nil, "", nil
	}
}

func defaultSubjectFor(t domain.AutomationTrigger) string {
	switch t {
	case domain.TriggerRiskCreated, domain.TriggerRiskScoreUpdated:
		return "risk"
	case domain.TriggerIncidentCreated:
		return "incident"
	default:
		return "vulnerability"
	}
}

func (r *SubjectResolver) vulnerability(ctx context.Context, tenantID uuid.UUID, id string) (*appauto.TriggerContext, string, error) {
	q := r.db.WithContext(ctx).Model(&domain.Vulnerability{}).Where("tenant_id = ?", tenantID)
	if strings.TrimSpace(id) != "" {
		vid, err := uuid.Parse(id)
		if err != nil {
			return nil, "", domain.NewValidationError("invalid vulnerability id")
		}
		q = q.Where("id = ?", vid)
	} else {
		// Newest first, and the worst of the newest: tracing against the tenant's
		// most severe recent finding is the case an operator actually cares about.
		q = q.Order("priority_score DESC, created_at DESC")
	}
	var v domain.Vulnerability
	if err := q.Take(&v).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, "", nil
		}
		return nil, "", err
	}

	tc := &appauto.TriggerContext{
		TenantID:     tenantID,
		Ref:          "vuln:" + v.ID.String(),
		Subject:      v.Title,
		Title:        v.Title,
		Severity:     strings.ToLower(string(v.Severity)),
		CVSS:         v.CVSSScore,
		KEV:          v.KEV,
		PriorityTier: v.PriorityTier,
		CVEID:        v.CVEID,
	}
	if v.AssetID != nil {
		tc.AssetID = v.AssetID
		tc.AssetName, tc.AssetTags = r.AssetFacts(ctx, tenantID, *v.AssetID)
	}
	label := v.Title
	if v.CVEID != "" {
		label = v.CVEID + " — " + v.Title
	}
	return tc, "live vulnerability: " + label, nil
}

func (r *SubjectResolver) risk(ctx context.Context, tenantID uuid.UUID, id string) (*appauto.TriggerContext, string, error) {
	q := r.db.WithContext(ctx).Model(&domain.Risk{}).Where("tenant_id = ?", tenantID)
	if strings.TrimSpace(id) != "" {
		rid, err := uuid.Parse(id)
		if err != nil {
			return nil, "", domain.NewValidationError("invalid risk id")
		}
		q = q.Where("id = ?", rid)
	} else {
		q = q.Order("score DESC, created_at DESC")
	}
	var risk domain.Risk
	if err := q.Take(&risk).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, "", nil
		}
		return nil, "", err
	}
	rid := risk.ID
	tc := &appauto.TriggerContext{
		TenantID: tenantID,
		Ref:      "risk:" + rid.String(),
		Subject:  risk.Name,
		Title:    risk.Name,
		Severity: strings.ToLower(string(risk.Criticality)),
		RiskID:   &rid,
	}
	if risk.SourceCVEID != nil {
		tc.CVEID = *risk.SourceCVEID
	}
	tc.AssetTags = []string(risk.Tags)
	return tc, fmt.Sprintf("live risk: %s (score %.2f)", risk.Name, risk.Score), nil
}

func (r *SubjectResolver) incident(ctx context.Context, tenantID uuid.UUID, id string) (*appauto.TriggerContext, string, error) {
	q := r.db.WithContext(ctx).Model(&domain.Incident{}).Where("tenant_id = ?", tenantID.String())
	if strings.TrimSpace(id) != "" {
		q = q.Where("id = ?", id)
	} else {
		q = q.Order("created_at DESC")
	}
	var inc domain.Incident
	if err := q.Take(&inc).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, "", nil
		}
		return nil, "", err
	}
	tc := &appauto.TriggerContext{
		TenantID: tenantID,
		Ref:      fmt.Sprintf("incident:%d", inc.ID),
		Subject:  inc.Title,
		Title:    inc.Title,
		Severity: strings.ToLower(inc.Severity),
	}
	return tc, fmt.Sprintf("live incident: %s (INC-%d)", inc.Title, inc.ID), nil
}

// AssetFacts loads the name and the tag vocabulary a rule's asset-tag condition
// gates on.
//
// Assets themselves carry no free-text tags in this model; the vocabulary an
// operator actually writes ("internet-facing", "pci", "prod") lives on the RISKS
// attached to an asset — the same place the Smart Risk engine reads exposure
// from. So the tag set is: the asset's own structural facets (type, criticality)
// plus every tag of every risk linked to it. Without this the asset-tag
// condition would compile, save, and never match anything.
func (r *SubjectResolver) AssetFacts(ctx context.Context, tenantID, assetID uuid.UUID) (string, []string) {
	var asset domain.Asset
	if err := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", assetID, tenantID).
		Take(&asset).Error; err != nil {
		return "", nil
	}
	seen := map[string]bool{}
	var tags []string
	add := func(v string) {
		v = strings.ToLower(strings.TrimSpace(v))
		if v == "" || seen[v] {
			return
		}
		seen[v] = true
		tags = append(tags, v)
	}
	add(asset.Type)
	add(string(asset.Criticality))
	add(asset.Source)

	var linked []domain.Risk
	if err := r.db.WithContext(ctx).
		Joins("JOIN risk_assets ON risk_assets.risk_id = risks.id").
		Where("risk_assets.asset_id = ? AND risks.tenant_id = ?", assetID, tenantID).
		Find(&linked).Error; err == nil {
		for _, rk := range linked {
			for _, t := range rk.Tags {
				add(t)
			}
		}
	}
	return asset.Name, tags
}
