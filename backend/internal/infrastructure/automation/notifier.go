// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial
// This file is part of the OpenRisk Enterprise Edition and is NOT covered by the
// AGPL; it is licensed under the OpenRisk Commercial License (see LICENSE.commercial).

// Package automation (infrastructure) wires the SOAR engine's abstract action
// ports to concrete capabilities: the multi-channel alert Notifier, the ITSM
// Ticketer, and the risk/scan actions. It is the composition-side adapter — the
// pure engine lives in internal/application/automation.
package automation

import (
	"context"
	"strings"

	"github.com/google/uuid"
	appauto "github.com/opendefender/openrisk/internal/application/automation"
	notificationapp "github.com/opendefender/openrisk/internal/application/notification"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/email"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
	"github.com/opendefender/openrisk/pkg/notify"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

// Notifier is the concrete multi-channel dispatcher implementing
// appauto.Notifier. It resolves user-addressed recipients (in-app/email) from
// the risk owner + a target role, and posts to the tenant Slack/Teams webhooks
// for the chat channels. Every channel is best-effort: a failure on one channel
// never blocks the others, and the delivered channels are returned so the
// engine can record an honest step.
type Notifier struct {
	channels domain.AutomationChannelRepository
	inApp    *notificationapp.UseCase
	email    email.Service
	users    *repository.GormUserRepository
	db       *gorm.DB
	logger   zerolog.Logger
}

// NewNotifier builds the dispatcher. Any dependency may be nil; the matching
// channel is simply skipped.
func NewNotifier(
	channels domain.AutomationChannelRepository,
	inApp *notificationapp.UseCase,
	emailSvc email.Service,
	users *repository.GormUserRepository,
	db *gorm.DB,
	logger zerolog.Logger,
) *Notifier {
	return &Notifier{channels: channels, inApp: inApp, email: emailSvc, users: users, db: db, logger: logger}
}

var _ appauto.Notifier = (*Notifier)(nil)

type recipient struct {
	id    uuid.UUID
	email string
}

// Notify dispatches an alert. Returns the distinct channels that delivered.
func (n *Notifier) Notify(ctx context.Context, req appauto.NotifyRequest) ([]string, error) {
	var cfg *domain.AutomationChannelConfig
	if n.channels != nil {
		if c, err := n.channels.Get(ctx, req.TenantID); err == nil {
			cfg = c
		}
	}
	if cfg == nil {
		cfg = &domain.AutomationChannelConfig{TenantID: req.TenantID, EmailEnabled: true}
	}

	channels := normaliseChannels(req.Channels, cfg)
	recips := n.resolveRecipients(ctx, req.TenantID, req.OwnerID, req.TargetRole, cfg)

	delivered := map[string]struct{}{}
	for _, ch := range channels {
		switch domain.NotificationChannel(ch) {
		case domain.NotificationChannelSlack:
			if cfg.SlackEnabled && cfg.SlackWebhookURL != "" {
				if err := notify.PostSlack(ctx, cfg.SlackWebhookURL, toChatMessage(req), nil); err != nil {
					n.logger.Warn().Err(err).Msg("automation notify: slack failed")
				} else {
					delivered["slack"] = struct{}{}
				}
			}
		case domain.NotificationChannelTeams:
			if cfg.TeamsEnabled && cfg.TeamsWebhookURL != "" {
				if err := notify.PostTeams(ctx, cfg.TeamsWebhookURL, toChatMessage(req), nil); err != nil {
					n.logger.Warn().Err(err).Msg("automation notify: teams failed")
				} else {
					delivered["teams"] = struct{}{}
				}
			}
		case domain.NotificationChannelEmail:
			if cfg.EmailEnabled && n.email != nil {
				if n.deliverEmail(ctx, req, recips, cfg) {
					delivered["email"] = struct{}{}
				}
			}
		case domain.NotificationChannelInApp:
			if n.inApp != nil && n.deliverInApp(req, recips) {
				delivered["in_app"] = struct{}{}
			}
		}
	}

	out := make([]string, 0, len(delivered))
	for ch := range delivered {
		out = append(out, ch)
	}
	return out, nil
}

func (n *Notifier) deliverEmail(ctx context.Context, req appauto.NotifyRequest, recips []recipient, cfg *domain.AutomationChannelConfig) bool {
	targets := map[string]struct{}{}
	for _, r := range recips {
		if r.email != "" {
			targets[r.email] = struct{}{}
		}
	}
	if len(targets) == 0 && cfg.DefaultEmail != "" {
		targets[cfg.DefaultEmail] = struct{}{}
	}
	if len(targets) == 0 {
		return false
	}
	sent := false
	for addr := range targets {
		if err := n.email.SendEmail(ctx, addr, req.Subject, req.Message); err != nil {
			n.logger.Warn().Err(err).Str("to", addr).Msg("automation notify: email failed")
			continue
		}
		sent = true
	}
	return sent
}

func (n *Notifier) deliverInApp(req appauto.NotifyRequest, recips []recipient) bool {
	sent := false
	for _, r := range recips {
		if r.id == uuid.Nil {
			continue
		}
		if err := n.inApp.NotifyInApp(r.id, req.TenantID, domain.NotificationTypeAutomation,
			req.Subject, req.Message, req.ResourceID, req.ResourceType); err != nil {
			n.logger.Warn().Err(err).Str("user", r.id.String()).Msg("automation notify: in-app failed")
			continue
		}
		sent = true
	}
	return sent
}

// resolveRecipients gathers the owner (if any) plus every user holding the
// target role in the tenant. Degrades gracefully — a lookup failure yields
// fewer recipients, never an error.
func (n *Notifier) resolveRecipients(ctx context.Context, tenantID uuid.UUID, ownerID *uuid.UUID, role string, cfg *domain.AutomationChannelConfig) []recipient {
	seen := map[uuid.UUID]*recipient{}

	if ownerID != nil && *ownerID != uuid.Nil && n.users != nil {
		rec := &recipient{id: *ownerID}
		if u, err := n.users.GetByID(ctx, *ownerID); err == nil && u != nil {
			rec.email = u.Email
		}
		seen[*ownerID] = rec
	}

	if role != "" && n.db != nil {
		for _, id := range n.memberIDsByRole(ctx, tenantID, role) {
			if _, ok := seen[id]; !ok {
				seen[id] = &recipient{id: id}
			}
		}
		// Fill emails for role recipients in one query.
		missing := make([]uuid.UUID, 0, len(seen))
		for id, r := range seen {
			if r.email == "" {
				missing = append(missing, id)
			}
		}
		if len(missing) > 0 && n.users != nil {
			if emails, err := n.users.EmailsByIDs(ctx, missing); err == nil {
				for id, addr := range emails {
					if r, ok := seen[id]; ok {
						r.email = addr
					}
				}
			}
		}
	}

	out := make([]recipient, 0, len(seen))
	for _, r := range seen {
		out = append(out, *r)
	}
	return out
}

// memberIDsByRole maps a target role hint to the tenant's members holding it.
// "manager" has no dedicated role in the model → treated as admin.
func (n *Notifier) memberIDsByRole(ctx context.Context, tenantID uuid.UUID, role string) []uuid.UUID {
	var roles []domain.MemberRole
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "admin", "manager", "": // manager folds into admin
		roles = []domain.MemberRole{domain.RoleAdmin, domain.RoleRoot}
	case "root":
		roles = []domain.MemberRole{domain.RoleRoot}
	default:
		roles = []domain.MemberRole{domain.MemberRole(role)}
	}
	var ids []uuid.UUID
	if err := n.db.WithContext(ctx).
		Model(&domain.OrganizationMember{}).
		Where("organization_id = ? AND role IN ?", tenantID, roles).
		Pluck("user_id", &ids).Error; err != nil {
		n.logger.Debug().Err(err).Msg("automation notify: role lookup failed")
	}
	return ids
}

func normaliseChannels(requested []string, cfg *domain.AutomationChannelConfig) []string {
	if len(requested) > 0 {
		return requested
	}
	// Default: every configured channel.
	out := []string{"in_app"}
	if cfg.EmailEnabled {
		out = append(out, "email")
	}
	if cfg.SlackEnabled && cfg.SlackWebhookURL != "" {
		out = append(out, "slack")
	}
	if cfg.TeamsEnabled && cfg.TeamsWebhookURL != "" {
		out = append(out, "teams")
	}
	return out
}

func toChatMessage(req appauto.NotifyRequest) notify.ChatMessage {
	facts := make([]notify.ChatFact, 0, len(req.Facts))
	for _, f := range req.Facts {
		facts = append(facts, notify.ChatFact{Label: f.Label, Value: f.Value})
	}
	return notify.ChatMessage{
		Title:    req.Subject,
		Text:     req.Message,
		Severity: req.Severity,
		Facts:    facts,
		LinkURL:  req.LinkURL,
	}
}
