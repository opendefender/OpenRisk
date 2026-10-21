// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

// #997 — the notice e-mail outbox on real Postgres. Grouping takes a row lock
// and the worker claims with SKIP LOCKED; sqlite can show neither. Each test
// runs migration 0081 in a schema of its own and drops it afterwards.

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/opendefender/openrisk/internal/domain"
)

const outboxMigration = "../../../../migrations/0081_notice_email_outbox"

// outboxSchema opens a pool whose connections all see a fresh schema. With
// autoMigrate, the tables are first built the way a compose boot builds them.
func outboxSchema(t *testing.T, autoMigrate bool) (*gorm.DB, string) {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	schema := "outbox_997_" + strings.ReplaceAll(uuid.New().String(), "-", "")[:12]
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, admin.Exec(fmt.Sprintf(`CREATE SCHEMA %s`, schema)).Error)
	t.Cleanup(func() {
		_ = admin.Exec(fmt.Sprintf(`DROP SCHEMA %s CASCADE`, schema)).Error
		if sqlDB, err := admin.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})

	u, err := url.Parse(dsn)
	require.NoError(t, err)
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	db, err := gorm.Open(postgres.Open(u.String()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent), TranslateError: true})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(25)
	t.Cleanup(func() { _ = sqlDB.Close() })

	if autoMigrate {
		require.NoError(t, db.AutoMigrate(&domain.NoticeEmailOutbox{}, &domain.NoticeEmailThrottle{}, &domain.NoticeEmailPolicy{}))
	}
	up, err := os.ReadFile(outboxMigration + ".up.sql")
	require.NoError(t, err)
	require.NoError(t, db.Exec(string(up)).Error)
	return db, schema
}

// OutboxSchemaForTest lets the end-to-end test in package repository_test
// build the same schema.
var OutboxSchemaForTest = outboxSchema

type outboxKey struct {
	tenant, user, resource uuid.UUID
}

func newOutboxKey() outboxKey { return outboxKey{uuid.New(), uuid.New(), uuid.New()} }

func (k outboxKey) row(subject string) domain.NoticeEmailOutbox {
	id := k.resource
	return domain.NoticeEmailOutbox{
		TenantID: k.tenant, UserID: k.user, ResourceType: "mitigation", ResourceID: &id,
		ResourceTitle: "Patch VPN", Locale: "fr", Grouped: true,
		Events: domain.NoticeEmailEvents{{Type: domain.NotificationTypeActionAssigned, At: time.Now().UTC(), Subject: subject, Lines: []string{subject}}},
	}
}

func outboxRows(t *testing.T, db *gorm.DB, tenant uuid.UUID) []domain.NoticeEmailOutbox {
	t.Helper()
	var rows []domain.NoticeEmailOutbox
	require.NoError(t, db.Where("tenant_id = ?", tenant).Order("created_at").Find(&rows).Error)
	return rows
}

func TestGormNoticeEmailOutbox_Enqueue_Success(t *testing.T) {
	db, _ := outboxSchema(t, false)
	box := NewGormNoticeEmailOutbox(db)
	ctx := context.Background()
	k := newOutboxKey()

	kind, due, err := box.Enqueue(ctx, k.row("Affecté"), time.Hour)
	require.NoError(t, err)
	require.Equal(t, domain.NoticeEmailSingle, kind, "the first e-mail goes out at once")
	require.WithinDuration(t, time.Now(), due, 5*time.Second)

	kind, due2, err := box.Enqueue(ctx, k.row("Désaffecté"), time.Hour)
	require.NoError(t, err)
	require.Equal(t, domain.NoticeEmailDigest, kind)
	require.WithinDuration(t, due.Add(time.Hour), due2, 5*time.Second, "due when the window ends")

	kind, due3, err := box.Enqueue(ctx, k.row("Réaffecté"), time.Hour)
	require.NoError(t, err)
	require.Equal(t, domain.NoticeEmailDigest, kind)
	require.True(t, due2.Equal(due3), "joins the same digest")

	rows := outboxRows(t, db, k.tenant)
	require.Len(t, rows, 2, "one single, one digest")
	require.Equal(t, domain.NoticeEmailSingle, rows[0].Kind)
	require.Equal(t, domain.NoticeEmailDigest, rows[1].Kind)
	require.Len(t, rows[1].Events, 2)
	require.Equal(t, "Désaffecté", rows[1].Events[0].Subject)
	require.Equal(t, "Réaffecté", rows[1].Events[1].Subject)

	// Another item has its own window.
	other := k
	other.resource = uuid.New()
	kind, _, err = box.Enqueue(ctx, other.row("Affecté"), time.Hour)
	require.NoError(t, err)
	require.Equal(t, domain.NoticeEmailSingle, kind)
}

// Ungrouped notices (deadline reminders) and notices with no item are always
// e-mailed on their own.
func TestGormNoticeEmailOutbox_Enqueue_NotFound(t *testing.T) {
	db, _ := outboxSchema(t, false)
	box := NewGormNoticeEmailOutbox(db)
	ctx := context.Background()
	k := newOutboxKey()

	for range 3 {
		r := k.row("Échéance")
		r.Grouped = false
		kind, _, err := box.Enqueue(ctx, r, 24*time.Hour)
		require.NoError(t, err)
		require.Equal(t, domain.NoticeEmailSingle, kind)
	}
	r := k.row("Sans élément")
	r.ResourceID = nil
	kind, _, err := box.Enqueue(ctx, r, 24*time.Hour)
	require.NoError(t, err)
	require.Equal(t, domain.NoticeEmailSingle, kind)
	require.Len(t, outboxRows(t, db, k.tenant), 4)

	// A deadline reminder does not open a window for the item either.
	kind, _, err = box.Enqueue(ctx, k.row("Affecté"), 24*time.Hour)
	require.NoError(t, err)
	require.Equal(t, domain.NoticeEmailSingle, kind)
}

// Tenant isolation: the same person and item in another tenant is another key;
// a row cannot be steered through another tenant's id.
func TestGormNoticeEmailOutbox_Enqueue_Unauthorized(t *testing.T) {
	db, _ := outboxSchema(t, false)
	box := NewGormNoticeEmailOutbox(db)
	ctx := context.Background()
	k := newOutboxKey()

	_, _, err := box.Enqueue(ctx, k.row("Affecté"), time.Hour)
	require.NoError(t, err)
	other := k
	other.tenant = uuid.New()
	kind, _, err := box.Enqueue(ctx, other.row("Affecté"), time.Hour)
	require.NoError(t, err)
	require.Equal(t, domain.NoticeEmailSingle, kind, "another tenant does not share the window")

	bad := k.row("x")
	bad.TenantID = uuid.Nil
	_, _, err = box.Enqueue(ctx, bad, time.Hour)
	require.Error(t, err)

	row := outboxRows(t, db, k.tenant)[0]
	forged := row
	forged.TenantID = other.tenant
	require.NoError(t, box.MarkSent(ctx, forged))
	require.NoError(t, box.Close(ctx, forged, domain.NoticeEmailFailed, "x"))
	require.NoError(t, box.Retry(ctx, forged, time.Now().Add(time.Hour), "x"))
	require.Equal(t, domain.NoticeEmailPending, outboxRows(t, db, k.tenant)[0].Status, "another tenant's id changes nothing")
}

// #997 — replicas racing on one key: one first e-mail, one digest holding the
// rest, nothing lost.
func TestGormNoticeEmailOutbox_ConcurrentEnqueue_Postgres(t *testing.T) {
	db, _ := outboxSchema(t, false)
	box := NewGormNoticeEmailOutbox(db)
	k := newOutboxKey()

	const racers = 20
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range racers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, _, err := box.Enqueue(context.Background(), k.row(fmt.Sprintf("change %d", i)), time.Hour)
			assert.NoError(t, err)
		}()
	}
	close(start)
	wg.Wait()

	rows := outboxRows(t, db, k.tenant)
	require.Len(t, rows, 2, "exactly one single and one digest")
	singles, events := 0, 0
	for _, r := range rows {
		if r.Kind == domain.NoticeEmailSingle {
			singles++
		}
		events += len(r.Events)
	}
	require.Equal(t, 1, singles)
	require.Equal(t, racers, events, "every notice is in the outbox")
}

// #997 — replicas claiming together never take the same row.
func TestGormNoticeEmailOutbox_ConcurrentClaims_Postgres(t *testing.T) {
	db, _ := outboxSchema(t, false)
	box := NewGormNoticeEmailOutbox(db)
	ctx := context.Background()
	const total = 60
	for range total {
		r := newOutboxKey().row("x")
		r.Grouped = false
		_, _, err := box.Enqueue(ctx, r, time.Hour)
		require.NoError(t, err)
	}

	var mu sync.Mutex
	seen := map[uuid.UUID]int{}
	var wg sync.WaitGroup
	for range 6 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				rows, err := box.ClaimDue(ctx, 4, time.Minute)
				if !assert.NoError(t, err) || len(rows) == 0 {
					return
				}
				mu.Lock()
				for _, r := range rows {
					seen[r.ID]++
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	require.Len(t, seen, total, "every row claimed")
	for id, n := range seen {
		require.Equal(t, 1, n, "row %s claimed %d times", id, n)
	}
}

func TestGormNoticeEmailOutbox_ClaimLeaseAndOutcome(t *testing.T) {
	db, _ := outboxSchema(t, false)
	box := NewGormNoticeEmailOutbox(db)
	ctx := context.Background()
	k := newOutboxKey()
	_, _, err := box.Enqueue(ctx, k.row("Affecté"), time.Hour)
	require.NoError(t, err)
	_, _, err = box.Enqueue(ctx, k.row("Désaffecté"), time.Hour)
	require.NoError(t, err)

	rows, err := box.ClaimDue(ctx, 10, time.Minute)
	require.NoError(t, err)
	require.Len(t, rows, 1, "the digest is not due yet")
	require.Equal(t, domain.NoticeEmailSending, rows[0].Status)
	require.Equal(t, 1, rows[0].Attempts)
	require.Equal(t, "Affecté", rows[0].Events[0].Subject, "events survive the round trip")

	// A replica that died: its lease runs out and the row is claimed again.
	require.NoError(t, db.Exec(`UPDATE notice_email_outbox SET locked_until = now() - interval '1 second' WHERE id = ? AND tenant_id = ?`, rows[0].ID, k.tenant).Error)
	again, err := box.ClaimDue(ctx, 10, time.Minute)
	require.NoError(t, err)
	require.Len(t, again, 1)
	require.Equal(t, rows[0].ID, again[0].ID)
	require.Equal(t, 2, again[0].Attempts)

	// Retry puts it back for later; MarkSent closes it.
	require.NoError(t, box.Retry(ctx, again[0], time.Now().Add(-time.Second), "421"))
	third, err := box.ClaimDue(ctx, 10, time.Minute)
	require.NoError(t, err)
	require.Len(t, third, 1)
	require.Equal(t, "421", third[0].LastError)
	require.NoError(t, box.MarkSent(ctx, third[0]))
	none, err := box.ClaimDue(ctx, 10, time.Minute)
	require.NoError(t, err)
	require.Empty(t, none)

	// The digest is claimed once its window is over, and restamps the throttle.
	require.NoError(t, db.Exec(`UPDATE notice_email_outbox SET due_at = now() - interval '1 second' WHERE kind = 'digest' AND tenant_id = ?`, k.tenant).Error)
	digest, err := box.ClaimDue(ctx, 10, time.Minute)
	require.NoError(t, err)
	require.Len(t, digest, 1)
	require.Equal(t, domain.NoticeEmailDigest, digest[0].Kind)

	// A notice arriving while that digest is sent waits a full window.
	kind, due, err := box.Enqueue(ctx, k.row("Réaffecté"), time.Hour)
	require.NoError(t, err)
	require.Equal(t, domain.NoticeEmailDigest, kind)
	require.WithinDuration(t, time.Now().Add(time.Hour), due, 5*time.Second)
}

func TestGormNoticeEmailOutbox_Prune(t *testing.T) {
	db, _ := outboxSchema(t, false)
	box := NewGormNoticeEmailOutbox(db)
	ctx := context.Background()
	k := newOutboxKey()
	for range 4 {
		r := k.row("x")
		r.Grouped = false
		_, _, err := box.Enqueue(ctx, r, time.Hour)
		require.NoError(t, err)
	}
	ids := outboxRows(t, db, k.tenant)
	set := func(id uuid.UUID, status, age string) {
		require.NoError(t, db.Exec(`UPDATE notice_email_outbox SET status = ?, created_at = now() - ?::interval WHERE id = ? AND tenant_id = ?`, status, age, id, k.tenant).Error)
	}
	set(ids[0].ID, domain.NoticeEmailSent, "8 days")   // pruned
	set(ids[1].ID, domain.NoticeEmailSent, "2 days")   // kept
	set(ids[2].ID, domain.NoticeEmailFailed, "8 days") // kept: failed rows stay a month
	set(ids[3].ID, domain.NoticeEmailPending, "40 days")
	require.NoError(t, db.Exec(`INSERT INTO notice_email_throttle VALUES (?, ?, 'risk', ?, now() - interval '2 days')`, k.tenant, k.user, uuid.New()).Error)

	n, err := box.Prune(ctx)
	require.NoError(t, err)
	require.EqualValues(t, 2, n, "one sent row and one old throttle stamp")
	left := outboxRows(t, db, k.tenant)
	require.Len(t, left, 3)
	for _, r := range left {
		require.NotEqual(t, ids[0].ID, r.ID)
	}
}

func TestGormNoticeEmailPolicy_IsTenantScoped(t *testing.T) {
	db, _ := outboxSchema(t, false)
	repo := NewGormNoticeEmailPolicyRepository(db)
	ctx := context.Background()
	a, b := uuid.New(), uuid.New()

	got, err := repo.GetNoticeEmailPolicy(ctx, a)
	require.NoError(t, err)
	require.Nil(t, got, "never saved")

	require.NoError(t, repo.SaveNoticeEmailPolicy(ctx, &domain.NoticeEmailPolicy{TenantID: a, WindowMinutes: 240}))
	require.NoError(t, repo.SaveNoticeEmailPolicy(ctx, &domain.NoticeEmailPolicy{TenantID: a, WindowMinutes: 15}))
	got, err = repo.GetNoticeEmailPolicy(ctx, a)
	require.NoError(t, err)
	require.Equal(t, 15, got.WindowMinutes, "saved twice, one row")

	got, err = repo.GetNoticeEmailPolicy(ctx, b)
	require.NoError(t, err)
	require.Nil(t, got, "another tenant sees nothing")

	require.Error(t, repo.SaveNoticeEmailPolicy(ctx, &domain.NoticeEmailPolicy{TenantID: b, WindowMinutes: 7}))
	_, err = repo.GetNoticeEmailPolicy(ctx, uuid.Nil)
	require.Error(t, err)
	var count int64
	require.NoError(t, db.Model(&domain.NoticeEmailPolicy{}).Where("tenant_id = ?", a).Count(&count).Error)
	require.EqualValues(t, 1, count)
}

// The tables AutoMigrate builds work for every statement, and 0081 on top of
// them changes nothing.
func TestNoticeEmailOutbox_AutoMigrateTablesWork(t *testing.T) {
	db, _ := outboxSchema(t, true)
	box := NewGormNoticeEmailOutbox(db)
	k := newOutboxKey()
	kind, _, err := box.Enqueue(context.Background(), k.row("a"), time.Hour)
	require.NoError(t, err)
	require.Equal(t, domain.NoticeEmailSingle, kind)
	kind, _, err = box.Enqueue(context.Background(), k.row("b"), time.Hour)
	require.NoError(t, err)
	require.Equal(t, domain.NoticeEmailDigest, kind)
	rows, err := box.ClaimDue(context.Background(), 10, time.Minute)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.NoError(t, NewGormNoticeEmailPolicyRepository(db).SaveNoticeEmailPolicy(context.Background(),
		&domain.NoticeEmailPolicy{TenantID: k.tenant, WindowMinutes: 30}))
}

// #997 criterion 7 — the down step drops the three tables.
func TestNoticeEmailOutboxMigration_DownDropsTables(t *testing.T) {
	db, schema := outboxSchema(t, false)
	exists := func(table string) bool {
		var ok bool
		require.NoError(t, db.Raw(`SELECT to_regclass(?) IS NOT NULL`, schema+"."+table).Scan(&ok).Error)
		return ok
	}
	tables := []string{"notice_email_outbox", "notice_email_throttle", "notice_email_policies"}
	for _, tbl := range tables {
		require.True(t, exists(tbl), "up creates %s", tbl)
	}
	down, err := os.ReadFile(outboxMigration + ".down.sql")
	require.NoError(t, err)
	require.NoError(t, db.Exec(string(down)).Error)
	for _, tbl := range tables {
		require.False(t, exists(tbl), "down drops %s", tbl)
	}
}
