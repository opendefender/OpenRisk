// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Notice e-mail delivery (#997). A notice e-mail is never sent from the
// request: it is written to an outbox and a worker sends it. For each person
// and item, the first e-mail goes out at once; what follows within the
// organization's window is gathered into one digest, sent when the window
// ends. Nothing is dropped.

// NoticeEmailWindowDefault is the grouping window of an organization that
// never opened the setting.
const NoticeEmailWindowDefault = 60

// NoticeEmailWindows are the windows an administrator may pick, in minutes.
// A short list on purpose: a free number invites "1 minute", which is no
// grouping at all, and "a week", which is a silent inbox.
var NoticeEmailWindows = []int{15, 30, 60, 240, 1440}

// NoticeEmailPolicy is one organization's choice of grouping window.
type NoticeEmailPolicy struct {
	// No `default:gen_random_uuid()`: it is Postgres-only and the repository
	// tests run on sqlite. Writers assign it.
	ID       uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	TenantID uuid.UUID `gorm:"type:uuid;not null;uniqueIndex" json:"tenant_id"`
	// WindowMinutes is one of NoticeEmailWindows.
	WindowMinutes int        `gorm:"not null" json:"window_minutes"`
	UpdatedByID   *uuid.UUID `gorm:"type:uuid" json:"updated_by_id,omitempty"`
	CreatedAt     time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt     time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

func (NoticeEmailPolicy) TableName() string { return "notice_email_policies" }

// Validate refuses a window outside the list.
func (p NoticeEmailPolicy) Validate() error {
	if !ValidNoticeEmailWindow(p.WindowMinutes) {
		return NewValidationError("window_minutes must be one of 15, 30, 60, 240, 1440")
	}
	return nil
}

// Window is the policy as a duration. A stored value outside the list (a
// direct SQL edit) reads as the default rather than as no grouping.
func (p *NoticeEmailPolicy) Window() time.Duration {
	if p == nil || !ValidNoticeEmailWindow(p.WindowMinutes) {
		return NoticeEmailWindowDefault * time.Minute
	}
	return time.Duration(p.WindowMinutes) * time.Minute
}

func ValidNoticeEmailWindow(minutes int) bool {
	for _, w := range NoticeEmailWindows {
		if w == minutes {
			return true
		}
	}
	return false
}

// NoticeEmailThrottle is the last e-mail sent to one person about one item. It
// is also the lock that serializes the decision "send now or add to the
// digest" for that key, across replicas.
type NoticeEmailThrottle struct {
	TenantID     uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID       uuid.UUID `gorm:"type:uuid;primaryKey"`
	ResourceType string    `gorm:"type:text;primaryKey"`
	ResourceID   uuid.UUID `gorm:"type:uuid;primaryKey"`
	SentAt       time.Time `gorm:"type:timestamptz;not null"`
}

func (NoticeEmailThrottle) TableName() string { return "notice_email_throttle" }

// Outbox row kinds.
const (
	NoticeEmailSingle = "single" // one notice, its own e-mail
	NoticeEmailDigest = "digest" // the notices gathered during a window
)

// Outbox row states. pending → sending → sent | failed | skipped; a send that
// fails returns to pending with a later next_attempt_at.
const (
	NoticeEmailPending = "pending"
	NoticeEmailSending = "sending"
	NoticeEmailSent    = "sent"
	NoticeEmailFailed  = "failed"
	// NoticeEmailSkipped is a row the worker closed without sending because
	// the recipient was no longer a member, or turned that e-mail off.
	NoticeEmailSkipped = "skipped"
)

// NoticeEmailEvent is one notice as worded for its recipient.
type NoticeEmailEvent struct {
	Type    NotificationType `json:"type"`
	At      time.Time        `json:"at"`
	Subject string           `json:"subject"`
	Lines   []string         `json:"lines"`
}

// NoticeEmailEvents is stored as a JSON array.
type NoticeEmailEvents []NoticeEmailEvent

func (e NoticeEmailEvents) Value() (driver.Value, error) {
	if e == nil {
		return "[]", nil
	}
	b, err := json.Marshal(e)
	return string(b), err
}

func (e *NoticeEmailEvents) Scan(value interface{}) error {
	switch v := value.(type) {
	case nil:
		*e = nil
		return nil
	case []byte:
		return json.Unmarshal(v, e)
	case string:
		return json.Unmarshal([]byte(v), e)
	default:
		return errors.New("notice e-mail events: unsupported type " + fmt.Sprintf("%T", value))
	}
}

// NoticeEmailOutbox is one e-mail to send: a single notice, or a digest.
//
// The address is not stored: the worker resolves the recipient again when it
// sends, so a member removed in the meantime gets nothing, and the outbox holds
// no contact data.
type NoticeEmailOutbox struct {
	ID            uuid.UUID  `gorm:"type:uuid;primaryKey"`
	TenantID      uuid.UUID  `gorm:"type:uuid;not null;index:idx_notice_email_outbox_key,priority:1"`
	UserID        uuid.UUID  `gorm:"type:uuid;not null;index:idx_notice_email_outbox_key,priority:2"`
	ResourceType  string     `gorm:"type:text;not null;default:'';index:idx_notice_email_outbox_key,priority:3"`
	ResourceID    *uuid.UUID `gorm:"type:uuid;index:idx_notice_email_outbox_key,priority:4"`
	ResourceTitle string     `gorm:"type:text;not null;default:''"`
	Kind          string     `gorm:"type:text;not null"`
	// Grouped rows count toward the window: sending one restamps the throttle.
	// A deadline reminder is not grouped. No `default:` tag: GORM would drop
	// an explicit false on insert in favour of it.
	Grouped     bool              `gorm:"not null"`
	Locale      string            `gorm:"type:text;not null"`
	Events      NoticeEmailEvents `gorm:"type:jsonb;not null"`
	Status      string            `gorm:"type:text;not null;index:idx_notice_email_outbox_due,priority:1"`
	DueAt       time.Time         `gorm:"type:timestamptz;not null;index:idx_notice_email_outbox_due,priority:2"`
	Attempts    int               `gorm:"not null;default:0"`
	LockedUntil *time.Time        `gorm:"type:timestamptz"`
	LastError   string            `gorm:"type:text;not null;default:''"`
	CreatedAt   time.Time         `gorm:"type:timestamptz;not null"`
	SentAt      *time.Time        `gorm:"type:timestamptz"`
}

func (NoticeEmailOutbox) TableName() string { return "notice_email_outbox" }
