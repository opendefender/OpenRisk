// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package notification

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// OutboxStore is what the sender needs from the outbox
// (repository.GormNoticeEmailOutbox).
type OutboxStore interface {
	ClaimDue(ctx context.Context, limit int, lease time.Duration) ([]domain.NoticeEmailOutbox, error)
	MarkSent(ctx context.Context, row domain.NoticeEmailOutbox) error
	Retry(ctx context.Context, row domain.NoticeEmailOutbox, at time.Time, reason string) error
	Close(ctx context.Context, row domain.NoticeEmailOutbox, status, reason string) error
}

const (
	// OutboxMaxAttempts is how many times a row is tried before it is marked
	// failed. With the backoff below that is about seven hours of retries.
	OutboxMaxAttempts = 12
	// OutboxLease is how long a claimed row is held. It outlasts one send, so
	// only a replica that died lets it go.
	OutboxLease = 2 * time.Minute
)

// ResourceStateReader reads an item as it stands, tenant-scoped
// (repository.GormNoticeResourceState). nil, nil for a kind it does not know.
type ResourceStateReader interface {
	NoticeResourceState(ctx context.Context, tenantID uuid.UUID, resourceType string, resourceID uuid.UUID) (*domain.NoticeResourceState, error)
}

// ErrOutboxFailed reports a row marked failed after its last attempt. The
// message carries the row's ids and the redacted reason, never an address.
var ErrOutboxFailed = errors.New("notice e-mail failed after its last attempt")

// OutboxSender sends one outbox row: it resolves the recipient again, keeps
// only the events they still want by e-mail, words a digest when there are
// several, and records the outcome.
type OutboxSender struct {
	store     OutboxStore
	directory RecipientDirectory
	prefs     *UseCase
	mailer    NoticeMailer
	link      LinkFunc
	state     ResourceStateReader
	timeout   time.Duration
	now       func() time.Time
}

// WithCurrentState makes each digest end on the item as it stands when the
// digest is sent, read by reader.
func (s *OutboxSender) WithCurrentState(reader ResourceStateReader) *OutboxSender {
	s.state = reader
	return s
}

func NewOutboxSender(store OutboxStore, directory RecipientDirectory, prefs *UseCase, mailer NoticeMailer, link LinkFunc) *OutboxSender {
	return &OutboxSender{
		store: store, directory: directory, prefs: prefs, mailer: mailer, link: link,
		timeout: defaultSendTimeout, now: time.Now,
	}
}

// Claim leases up to limit due rows.
func (s *OutboxSender) Claim(ctx context.Context, limit int) ([]domain.NoticeEmailOutbox, error) {
	return s.store.ClaimDue(ctx, limit, OutboxLease)
}

// Send delivers one claimed row and records what happened. It returns an
// error only for a row marked failed, or when the outcome could not be saved.
//
// A send cut short by ctx (shutdown) is put back, due now, whatever its count:
// it was interrupted, not refused.
func (s *OutboxSender) Send(ctx context.Context, row domain.NoticeEmailOutbox) error {
	// Bookkeeping survives the send's cancellation.
	keep, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()

	rcpt, err := s.directory.NoticeRecipient(ctx, row.TenantID, row.UserID)
	if err != nil {
		return s.retry(keep, ctx, row, fmt.Errorf("recipient lookup: %w", err))
	}
	// The #757 guard, again at send time: someone removed from the tenant
	// since the notice was queued gets nothing.
	if rcpt == nil {
		return s.store.Close(keep, row, domain.NoticeEmailSkipped, "recipient is no longer a member")
	}
	if strings.TrimSpace(rcpt.Email) == "" {
		return s.store.Close(keep, row, domain.NoticeEmailSkipped, "recipient has no address")
	}
	var events []domain.NoticeEmailEvent
	for _, ev := range row.Events {
		if s.prefs == nil || s.prefs.ShouldNotify(row.UserID, row.TenantID, ev.Type, domain.NotificationChannelEmail) {
			events = append(events, ev)
		}
	}
	if len(events) == 0 {
		return s.store.Close(keep, row, domain.NoticeEmailSkipped, "recipient turned this e-mail off")
	}

	// A digest waited for its window, so it ends on the item as it stands now,
	// even when it holds one change. If the state cannot be read, the digest
	// still goes out, ending on the latest change.
	var current *domain.NoticeResourceState
	if row.Kind == domain.NoticeEmailDigest && s.state != nil && row.ResourceID != nil {
		if st, err := s.state.NoticeResourceState(ctx, row.TenantID, row.ResourceType, *row.ResourceID); err == nil {
			current = st
		}
	}
	msg := DigestMessage(events, row.ResourceTitle, row.ResourceType, row.Locale, current)
	link := ""
	if s.link != nil {
		link = s.link(row.ResourceType, row.ResourceID)
	}
	sendCtx, cancelSend := context.WithTimeout(ctx, s.timeout)
	err = s.mailer.SendNotice(sendCtx, rcpt.Email, rcpt.FullName, row.Locale, msg, link)
	cancelSend()
	if err != nil {
		return s.retry(keep, ctx, row, redactedEmailError(err, rcpt.Email))
	}
	return s.store.MarkSent(keep, row)
}

// retry puts the row back with a growing delay, or marks it failed after its
// last attempt.
func (s *OutboxSender) retry(keep, ctx context.Context, row domain.NoticeEmailOutbox, cause error) error {
	reason := cause.Error()
	if ctx.Err() != nil {
		return s.store.Retry(keep, row, s.now(), "interrupted: "+reason)
	}
	if row.Attempts >= OutboxMaxAttempts {
		if err := s.store.Close(keep, row, domain.NoticeEmailFailed, reason); err != nil {
			return err
		}
		return fmt.Errorf("%w: outbox row %s (tenant %s): %s", ErrOutboxFailed, row.ID, row.TenantID, reason)
	}
	return s.store.Retry(keep, row, s.now().Add(OutboxBackoff(row.Attempts)), reason)
}

// OutboxBackoff is the wait before attempt n+1: one minute, doubling, capped
// at an hour.
func OutboxBackoff(attempts int) time.Duration {
	if attempts < 1 {
		attempts = 1
	}
	d := time.Minute
	for i := 1; i < attempts && d < time.Hour; i++ {
		d *= 2
	}
	if d > time.Hour {
		d = time.Hour
	}
	return d
}

// DigestMessage words the e-mail for a row. One event is sent as written,
// followed by the current state when one is given (a digest that waited).
// Several become a digest that lists every change in order and ends on the
// item's current state when it is known, else on the latest change.
func DigestMessage(events []domain.NoticeEmailEvent, title, resourceType, locale string, current *domain.NoticeResourceState) domain.NoticeMessage {
	en := locale == "en"
	tr := func(fr, eng string) string {
		if en {
			return eng
		}
		return fr
	}
	if len(events) == 1 {
		msg := domain.NoticeMessage{Subject: events[0].Subject, Lines: append([]string(nil), events[0].Lines...)}
		switch {
		case current != nil && !current.Exists:
			msg.Lines = append(msg.Lines, tr("Cet élément a été supprimé depuis.", "This item has since been deleted."))
		case current != nil:
			msg.Lines = append(msg.Lines, tr("État actuel :", "Current state:"))
			msg.Lines = append(msg.Lines, currentStateLines(current, locale, tr)...)
		}
		return msg
	}
	name := strings.TrimSpace(title)
	if current != nil && current.Exists && current.Title != "" {
		name = current.Title
	}
	if name == "" {
		name = itemLabel(resourceType, en)
	}

	subject := fmt.Sprintf(tr("%d changements sur « %s »", "%d changes to “%s”"), len(events), name)
	lines := make([]string, 0, len(events)+8)
	lines = append(lines, fmt.Sprintf(tr(
		"%d changements ont eu lieu sur « %s » depuis le dernier e-mail à ce sujet :",
		"%d changes were made to “%s” since the last e-mail about it:"), len(events), name))
	for i, ev := range events {
		lines = append(lines, fmt.Sprintf("%d. %s", i+1, ev.Subject))
	}

	switch {
	case current != nil && !current.Exists:
		lines = append(lines, tr("Cet élément a été supprimé depuis.", "This item has since been deleted."))
	case current != nil:
		lines = append(lines, tr("État actuel :", "Current state:"))
		lines = append(lines, currentStateLines(current, locale, tr)...)
	default:
		last := events[len(events)-1]
		latest := strings.Join(last.Lines, " ")
		if latest == "" {
			latest = last.Subject
		}
		lines = append(lines, tr("Le plus récent : ", "Latest: ")+latest)
	}
	return domain.NoticeMessage{Subject: subject, Lines: lines}
}

func currentStateLines(st *domain.NoticeResourceState, locale string, tr func(string, string) string) []string {
	var out []string
	switch {
	case st.MitigationStatus != "":
		out = append(out, tr("Statut : ", "Status: ")+st.MitigationStatus.Label(locale))
	case st.RiskState != "":
		out = append(out, tr("Étape : ", "Stage: ")+st.RiskState.Label(locale))
	}
	if st.Progress != nil {
		out = append(out, fmt.Sprintf(tr("Avancement : %d %%", "Progress: %d%%"), *st.Progress))
	}
	if st.DueDate != nil {
		layout := "02/01/2006"
		if locale == "en" {
			layout = "2006-01-02"
		}
		out = append(out, tr("Échéance : ", "Due: ")+st.DueDate.Format(layout))
	}
	nobody := tr("personne", "nobody")
	or := func(v string) string {
		if v == "" {
			return nobody
		}
		return v
	}
	if st.Owner != "" {
		out = append(out, tr("Responsable : ", "Owner: ")+st.Owner)
	}
	out = append(out, tr("Exécutant : ", "Assignee: ")+or(st.Assignee))
	if st.Reviewer != "" {
		out = append(out, tr("Validateur : ", "Reviewer: ")+st.Reviewer)
	}
	return out
}

func itemLabel(resourceType string, en bool) string {
	labels := map[string][2]string{
		"risk":       {"un risque", "a risk"},
		"mitigation": {"une mitigation", "a mitigation"},
		"incident":   {"un incident", "an incident"},
		"asset":      {"un actif", "an asset"},
	}
	pair, ok := labels[resourceType]
	if !ok {
		pair = [2]string{"un élément", "an item"}
	}
	if en {
		return pair[1]
	}
	return pair[0]
}
