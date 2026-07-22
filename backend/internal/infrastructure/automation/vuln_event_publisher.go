// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial
// This file is part of the OpenRisk Enterprise Edition and is NOT covered by the
// AGPL; it is licensed under the OpenRisk Commercial License (see LICENSE.commercial).

package automation

import (
	"context"

	vulnapp "github.com/opendefender/openrisk/internal/application/vulnerability"
	"github.com/opendefender/openrisk/internal/domain"
	redisclient "github.com/opendefender/openrisk/internal/infrastructure/redis"
	"github.com/opendefender/openrisk/pkg/events"
)

// VulnEventPublisher publishes the vulnerability.detected Redis event that fires
// the SOAR engine's vulnerability_detected trigger. It implements
// vulnapp.VulnEventPublisher.
type VulnEventPublisher struct {
	redis *redisclient.Client
}

// NewVulnEventPublisher builds the publisher.
func NewVulnEventPublisher(redis *redisclient.Client) *VulnEventPublisher {
	return &VulnEventPublisher{redis: redis}
}

var _ vulnapp.VulnEventPublisher = (*VulnEventPublisher)(nil)

// PublishVulnerabilityDetected maps a persisted vulnerability to the event
// payload and publishes it. Best-effort — a publish failure is returned to the
// (nil-safe) caller which never blocks ingest on it.
func (p *VulnEventPublisher) PublishVulnerabilityDetected(ctx context.Context, v *domain.Vulnerability) error {
	assetID := ""
	if v.AssetID != nil {
		assetID = v.AssetID.String()
	}
	evt := events.VulnerabilityDetectedEvent{
		VulnerabilityID: v.ID.String(),
		TenantID:        v.TenantID.String(),
		CVEID:           v.CVEID,
		Title:           v.Title,
		Severity:        string(v.Severity),
		CVSS:            v.CVSSScore,
		KEV:             v.KEV,
		PriorityTier:    v.PriorityTier,
		AssetID:         assetID,
		AssetName:       v.AssetName,
		Source:          string(v.Source),
		TriggeredBy:     "system",
	}
	return p.redis.Publish(ctx, events.VulnerabilityDetected, evt)
}
