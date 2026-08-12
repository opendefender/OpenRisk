// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package incident (infrastructure) delivers incident alerts and answers the
// post-mortem gate. It composes the existing multi-channel dispatcher rather
// than growing a second one — an incident alert and an automation alert should
// reach the same Slack channel through the same code.
package incident

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	appauto "github.com/opendefender/openrisk/internal/application/automation"
	appinc "github.com/opendefender/openrisk/internal/application/incident"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/service"
)

// ChannelDispatcher is the multi-channel sender (the automation Notifier).
type ChannelDispatcher interface {
	Notify(ctx context.Context, req appauto.NotifyRequest) (delivered []string, err error)
}

// Notifier alerts an incident's stakeholders when it is declared and whenever
// its severity moves.
//
// Severity changes get their own alert on purpose: a medium incident that
// becomes critical needs the people who were not watching it, and those people
// have no reason to be re-reading an incident they already triaged.
type Notifier struct {
	dispatch ChannelDispatcher
	logger   zerolog.Logger
}

// NewNotifier builds the incident alerting adapter.
func NewNotifier(dispatch ChannelDispatcher, logger zerolog.Logger) *Notifier {
	return &Notifier{dispatch: dispatch, logger: logger}
}

var _ service.IncidentNotifier = (*Notifier)(nil)

// NotifyDeclared alerts the stakeholders recorded on the incident.
func (n *Notifier) NotifyDeclared(ctx context.Context, inc *domain.Incident) {
	if n == nil || n.dispatch == nil || inc == nil {
		return
	}
	tenantID, err := uuid.Parse(inc.TenantID)
	if err != nil {
		return
	}
	subject := fmt.Sprintf("[%s] Incident declared: %s", strings.ToUpper(inc.Severity), inc.Title)
	n.send(ctx, tenantID, inc, subject, n.declaredBody(inc))
}

// NotifySeverityChanged alerts on a severity move, in both directions: a
// downgrade matters too, because it tells people they can stand down.
func (n *Notifier) NotifySeverityChanged(ctx context.Context, inc *domain.Incident, from, to string) {
	if n == nil || n.dispatch == nil || inc == nil {
		return
	}
	tenantID, err := uuid.Parse(inc.TenantID)
	if err != nil {
		return
	}
	direction := "raised to"
	if severityRank(to) < severityRank(from) {
		direction = "lowered to"
	}
	subject := fmt.Sprintf("[%s] Incident severity %s %s: %s",
		strings.ToUpper(to), direction, strings.ToUpper(to), inc.Title)
	body := fmt.Sprintf("Incident INC-%d changed from %s to %s.\n\n%s",
		inc.ID, strings.ToUpper(from), strings.ToUpper(to), inc.Description)
	n.send(ctx, tenantID, inc, subject, body)
}

// send fans the alert out to every stakeholder on the channels they were
// recorded with. A stakeholder with no channels falls back to in-app + email:
// being on the list at all means somebody decided you need to know.
func (n *Notifier) send(ctx context.Context, tenantID uuid.UUID, inc *domain.Incident, subject, body string) {
	targets := stakeholderTargets(inc)
	if len(targets) == 0 {
		// Nobody was named. Fall back to the tenant's default audience rather
		// than dropping the alert — an incident nobody was told about is the
		// failure this module exists to prevent.
		targets = []target{{role: "admin", channels: []string{domain.ChannelInApp, domain.ChannelEmail}}}
	}
	facts := incidentFacts(inc)
	for _, t := range targets {
		req := appauto.NotifyRequest{
			TenantID:     tenantID,
			Channels:     t.channels,
			Severity:     inc.Severity,
			Subject:      subject,
			Message:      body,
			TargetRole:   t.role,
			Facts:        facts,
			ResourceType: "incident",
		}
		if t.userID != uuid.Nil {
			id := t.userID
			req.OwnerID = &id
		}
		delivered, err := n.dispatch.Notify(ctx, req)
		if err != nil && len(delivered) == 0 {
			n.logger.Warn().Err(err).Uint("incident", inc.ID).
				Msg("incident alert could not be delivered on any channel")
			continue
		}
		n.logger.Info().Uint("incident", inc.ID).
			Strs("channels", delivered).Msg("incident alert delivered")
	}
}

type target struct {
	userID   uuid.UUID
	role     string
	channels []string
}

func stakeholderTargets(inc *domain.Incident) []target {
	out := make([]target, 0, len(inc.Stakeholders))
	for _, s := range inc.Stakeholders {
		t := target{role: strings.TrimSpace(s.Role), channels: s.Channels}
		if id, err := uuid.Parse(strings.TrimSpace(s.UserID)); err == nil {
			t.userID = id
		}
		if t.userID == uuid.Nil && t.role == "" {
			continue // a stakeholder we cannot address is not a stakeholder
		}
		if len(t.channels) == 0 {
			t.channels = []string{domain.ChannelInApp, domain.ChannelEmail}
		}
		out = append(out, t)
	}
	return out
}

func (n *Notifier) declaredBody(inc *domain.Incident) string {
	var b strings.Builder
	b.WriteString(inc.Description)
	b.WriteString("\n\n")
	if domain.IsAutomaticOrigin(inc.Origin) {
		// Say up front that no human opened this, and what did — an unexplained
		// automatic alert is the one people learn to ignore.
		label := inc.Origin
		if o, ok := domain.FindIncidentOrigin(inc.Origin); ok {
			label = o.Label
		}
		b.WriteString("Opened automatically (" + label + ")")
		if inc.OriginRuleName != "" {
			b.WriteString(" by rule \"" + inc.OriginRuleName + "\"")
		}
		if inc.OriginDetail != "" {
			b.WriteString(" — " + inc.OriginDetail)
		}
		b.WriteString(".\n")
	}
	if domain.RequiresPostMortem(inc.Severity) {
		b.WriteString("This incident is CRITICAL: it cannot be closed until a post-mortem is published.\n")
	}
	return b.String()
}

func incidentFacts(inc *domain.Incident) []appauto.Fact {
	facts := []appauto.Fact{
		{Label: "Reference", Value: fmt.Sprintf("INC-%d", inc.ID)},
		{Label: "Severity", Value: strings.ToUpper(inc.Severity)},
	}
	if inc.IncidentType != "" {
		facts = append(facts, appauto.Fact{Label: "Type", Value: inc.IncidentType})
	}
	if n := len(inc.AssetIDs); n > 0 {
		facts = append(facts, appauto.Fact{Label: "Assets affected", Value: fmt.Sprintf("%d", n)})
	}
	if n := len(inc.RiskIDs); n > 0 {
		facts = append(facts, appauto.Fact{Label: "Linked risks", Value: fmt.Sprintf("%d", n)})
	}
	return facts
}

func severityRank(s string) int {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "critical":
		return 4
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	}
	return 0
}

// =============================================================================
// Post-mortem gate
// =============================================================================

// PostMortemGate answers whether a critical incident may be closed.
type PostMortemGate struct {
	reviews *appinc.PostMortemService
}

// NewPostMortemGate builds the gate over the review service.
func NewPostMortemGate(reviews *appinc.PostMortemService) *PostMortemGate {
	return &PostMortemGate{reviews: reviews}
}

var _ service.PostMortemGate = (*PostMortemGate)(nil)

// PublishedReviewExists reports whether the review is published and, when it is
// not, what is still missing — so the refusal names the remaining fields instead
// of saying "not allowed".
//
// A gate that cannot check fails CLOSED: if the review cannot be read, refusing
// the closure is the safe answer for a critical incident.
func (g *PostMortemGate) PublishedReviewExists(ctx context.Context, tenantID string, incidentID uint) (bool, string) {
	if g == nil || g.reviews == nil {
		return false, "the post-mortem module is not available on this deployment"
	}
	tid, err := uuid.Parse(tenantID)
	if err != nil {
		return false, "the incident's tenant could not be resolved"
	}
	view, err := g.reviews.Get(ctx, tid, incidentID)
	if err != nil || view == nil {
		return false, "its post-mortem could not be read"
	}
	if view.PostMortem.Status == domain.PostMortemPublished {
		return true, ""
	}
	if len(view.Missing) > 0 {
		return false, "still to write: " + strings.Join(view.Missing, ", ")
	}
	return false, "the post-mortem is complete but not published yet"
}
