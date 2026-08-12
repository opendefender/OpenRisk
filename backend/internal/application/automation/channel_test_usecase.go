// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial
// This file is part of the OpenRisk Enterprise Edition and is NOT covered by the
// AGPL; it is licensed under the OpenRisk Commercial License (see LICENSE.commercial).

package automation

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/pkg/notify"
)

// ---------------------------------------------------------------------------
// Testing one channel at a time.
//
// "Alerts don't arrive" is the hardest automation bug to chase, because six
// things can be wrong and the rule only tells you "notify: skipped". Testing a
// single channel with a real delivery removes the guessing: either a message
// lands in Slack, or you get back the exact error the webhook returned.
//
// Every test performs a REAL send. A test that quietly returned success without
// leaving the process would be worse than no test at all.
// ---------------------------------------------------------------------------

// ChannelTestResult is the outcome of testing one channel.
type ChannelTestResult struct {
	Channel string `json:"channel"`
	// Configured says whether the tenant has the channel set up at all — the
	// difference between "not configured" and "configured but broken", which is
	// the first thing an operator needs to know.
	Configured bool   `json:"configured"`
	Delivered  bool   `json:"delivered"`
	Detail     string `json:"detail"`
	Error      string `json:"error,omitempty"`
	// Recipients describes who would have received it, so a "delivered" result
	// that went to the wrong place is still visible.
	Recipients string    `json:"recipients,omitempty"`
	DurationMS int64     `json:"duration_ms"`
	TestedAt   time.Time `json:"tested_at"`
}

// InAppSender delivers a test in-app notification to the requesting user.
type InAppSender interface {
	NotifyInApp(userID, tenantID uuid.UUID, notifType domain.NotificationType, subject, message string, resourceID *uuid.UUID, resourceType string) error
}

// EmailSender delivers a test e-mail. The signature mirrors the transport the
// rest of the platform already uses.
type EmailSender interface {
	SendEmail(ctx context.Context, to, subject, body string) error
}

// ChannelTester performs a real single-channel delivery.
type ChannelTester struct {
	repo  domain.AutomationChannelRepository
	inApp InAppSender
	email EmailSender
	http  notify.HTTPDoer
	// userEmail resolves the requesting user's address for the e-mail test, so a
	// test lands in the tester's inbox rather than a shared alias.
	userEmail func(ctx context.Context, userID uuid.UUID) string
}

// NewChannelTester builds the tester. Every collaborator is optional: a missing
// one produces an honest "not available on this deployment", never a fake pass.
func NewChannelTester(repo domain.AutomationChannelRepository) *ChannelTester {
	return &ChannelTester{repo: repo}
}

func (t *ChannelTester) WithInApp(s InAppSender) *ChannelTester        { t.inApp = s; return t }
func (t *ChannelTester) WithEmail(s EmailSender) *ChannelTester        { t.email = s; return t }
func (t *ChannelTester) WithHTTPDoer(d notify.HTTPDoer) *ChannelTester { t.http = d; return t }
func (t *ChannelTester) WithUserEmail(f func(ctx context.Context, userID uuid.UUID) string) *ChannelTester {
	t.userEmail = f
	return t
}

// Test delivers one test message on one channel.
func (t *ChannelTester) Test(ctx context.Context, tenantID, userID uuid.UUID, channel string) (*ChannelTestResult, error) {
	channel = strings.ToLower(strings.TrimSpace(channel))
	if !domain.IsAutomationChannel(channel) {
		return nil, domain.NewValidationError("unknown channel: " + channel)
	}
	cfg, err := t.repo.Get(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		cfg = &domain.AutomationChannelConfig{TenantID: tenantID}
	}

	start := time.Now()
	res := &ChannelTestResult{Channel: channel, TestedAt: start.UTC()}
	msg := testMessage()

	switch channel {
	case domain.ChannelInApp:
		res.Configured = true // in-app needs no setup
		res.Recipients = "you"
		if t.inApp == nil {
			res.Detail = "in-app notifications are not available on this deployment"
			break
		}
		if userID == uuid.Nil {
			res.Detail = "no signed-in user to notify"
			break
		}
		if err := t.inApp.NotifyInApp(userID, tenantID, domain.NotificationTypeAutomation,
			msg.Title, msg.Text, nil, "automation_channel_test"); err != nil {
			res.Error = err.Error()
			res.Detail = "the notification could not be stored"
			break
		}
		res.Delivered = true
		res.Detail = "a test notification is waiting in your bell menu"

	case domain.ChannelEmail:
		to := strings.TrimSpace(cfg.DefaultEmail)
		if t.userEmail != nil && userID != uuid.Nil {
			if addr := t.userEmail(ctx, userID); addr != "" {
				to = addr
			}
		}
		res.Configured = cfg.EmailEnabled && to != ""
		res.Recipients = to
		switch {
		case !cfg.EmailEnabled:
			res.Detail = "e-mail delivery is turned off for this tenant"
		case to == "":
			res.Detail = "no recipient: set a fallback address, or sign in with a user that has one"
		case t.email == nil:
			res.Detail = "no mail transport is configured on this deployment"
		default:
			if err := t.email.SendEmail(ctx, to, msg.Title, msg.Text); err != nil {
				res.Error = err.Error()
				res.Detail = "the mail transport refused the message"
				break
			}
			res.Delivered = true
			res.Detail = "sent to " + to
		}

	case domain.ChannelSlack:
		res.Configured = cfg.SlackEnabled && cfg.SlackWebhookURL != ""
		if !res.Configured {
			res.Detail = "no Slack webhook is configured"
			break
		}
		if err := notify.PostSlack(ctx, cfg.SlackWebhookURL, msg, t.http); err != nil {
			res.Error = err.Error()
			res.Detail = "Slack rejected the message"
			break
		}
		res.Delivered = true
		res.Detail = "posted to the configured Slack webhook"

	case domain.ChannelTeams:
		res.Configured = cfg.TeamsEnabled && cfg.TeamsWebhookURL != ""
		if !res.Configured {
			res.Detail = "no Teams webhook is configured"
			break
		}
		if err := notify.PostTeams(ctx, cfg.TeamsWebhookURL, msg, t.http); err != nil {
			res.Error = err.Error()
			res.Detail = "Teams rejected the message"
			break
		}
		res.Delivered = true
		res.Detail = "posted to the configured Teams webhook"

	case domain.ChannelWebhook:
		res.Configured = cfg.WebhookEnabled && cfg.WebhookURL != ""
		if !res.Configured {
			res.Detail = "no outbound webhook is configured"
			break
		}
		if err := notify.PostWebhook(ctx, cfg.WebhookURL, cfg.WebhookSecret, msg, t.http); err != nil {
			res.Error = err.Error()
			res.Detail = "the endpoint did not accept the request"
			break
		}
		res.Delivered = true
		res.Detail = "the endpoint accepted the request"
		if cfg.WebhookSecret != "" {
			res.Detail += " (signed with X-OpenRisk-Signature)"
		}

	case domain.ChannelSMS:
		recipients := notify.SplitRecipients(cfg.SMSRecipients)
		res.Configured = cfg.SMSEnabled && cfg.SMSGatewayURL != "" && len(recipients) > 0
		res.Recipients = fmt.Sprintf("%d number(s)", len(recipients))
		if !res.Configured {
			res.Detail = "the SMS gateway is not configured, or has no recipients"
			break
		}
		sent, err := notify.SendSMS(ctx, notify.SMSConfig{
			GatewayURL:  cfg.SMSGatewayURL,
			APIKey:      cfg.SMSAPIKey,
			Sender:      cfg.SMSSender,
			ToField:     cfg.SMSToField,
			TextField:   cfg.SMSTextField,
			SenderField: cfg.SMSSenderField,
		}, recipients, msg.Title+" — "+msg.Text, t.http)
		if err != nil {
			res.Error = err.Error()
			res.Detail = fmt.Sprintf("the gateway accepted %d of %d recipients before failing", sent, len(recipients))
			break
		}
		res.Delivered = true
		res.Detail = fmt.Sprintf("the gateway accepted %d recipient(s)", sent)
	}

	res.DurationMS = time.Since(start).Milliseconds()
	return res, nil
}

func testMessage() notify.ChatMessage {
	return notify.ChatMessage{
		Title:    "OpenRisk channel test",
		Text:     "This is a test message sent from OpenRisk → Automation → Channels. If you can read it, this channel works.",
		Severity: "low",
		Facts: []notify.ChatFact{
			{Label: "Kind", Value: "channel test"},
			{Label: "Sent at", Value: time.Now().UTC().Format(time.RFC3339)},
		},
	}
}

// ConfiguredChannels implements the dry-run ChannelProbe: which channels this
// tenant could actually deliver on right now.
func (s *ChannelService) ConfiguredChannels(ctx context.Context, tenantID uuid.UUID) ([]string, error) {
	cfg, err := s.repo.Get(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		return []string{domain.ChannelInApp}, nil
	}
	// In-app always works: it needs no external system.
	out := []string{domain.ChannelInApp}
	if cfg.EmailEnabled {
		out = append(out, domain.ChannelEmail)
	}
	if cfg.SlackEnabled && cfg.SlackWebhookURL != "" {
		out = append(out, domain.ChannelSlack)
	}
	if cfg.TeamsEnabled && cfg.TeamsWebhookURL != "" {
		out = append(out, domain.ChannelTeams)
	}
	if cfg.WebhookEnabled && cfg.WebhookURL != "" {
		out = append(out, domain.ChannelWebhook)
	}
	if cfg.SMSEnabled && cfg.SMSGatewayURL != "" {
		out = append(out, domain.ChannelSMS)
	}
	return out, nil
}
