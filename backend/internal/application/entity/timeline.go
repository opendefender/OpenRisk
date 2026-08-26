// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package entity

import (
	"context"
	"encoding/base64"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// =============================================================================
// Sources
// =============================================================================

// The canonical journal is audit_events. It is the only store keyed by
// (entity_type, entity_id) for every type, the only one carrying an actor and a
// before → after diff, and it is written for every successful mutating request
// by middleware.AuditMutations. §18 forbids standing up a parallel log, so this
// service reads that table and does not own one.
//
// Two things the audit trail structurally cannot see are merged in on top:
//
//   - risk_histories — the score engine recomputes a risk's score from a Redis
//     event, outside any HTTP request. No request, no audit entry. Those score
//     movements are the single most interesting thing that happens to a risk, so
//     a risk timeline without them would be quietly wrong.
//   - an incident's own journal — appended by the incident service as the
//     response is handled.
//
// Merging is why this is a cursor-paginated k-way merge rather than one query.

// EventSource is a supplementary journal for one entity type.
//
// Events MUST return newest-first and MUST be tenant-scoped. `before`, when set,
// is an exclusive upper bound on occurrence time: a source may filter with `<=`
// and let the merge drop the boundary rows, which is what the merge does anyway
// to break ties on equal timestamps.
type EventSource interface {
	Source() TimelineSource
	Events(ctx context.Context, tenantID uuid.UUID, entityID string, before *time.Time, limit int) ([]TimelineEvent, error)
}

// UserLookup resolves actor ids to emails for display. Same shape as the
// governance module's, so the user repository satisfies both.
type UserLookup interface {
	EmailsByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error)
}

// TimelineService reads history. It owns no table.
type TimelineService struct {
	audit  domain.AuditEventRepository
	lookup UserLookup
	extra  map[Type][]EventSource
}

func NewTimelineService(audit domain.AuditEventRepository) *TimelineService {
	return &TimelineService{audit: audit, extra: map[Type][]EventSource{}}
}

// WithUserLookup enriches actors with emails. Optional; without it an event
// shows a shortened actor id rather than a blank.
func (s *TimelineService) WithUserLookup(l UserLookup) *TimelineService {
	s.lookup = l
	return s
}

// WithSource registers a supplementary journal for a type. Optional and additive
// — a type with no extra source simply reads the audit trail.
func (s *TimelineService) WithSource(t Type, src EventSource) *TimelineService {
	if src == nil {
		return s
	}
	s.extra[t] = append(s.extra[t], src)
	return s
}

// =============================================================================
// Audit entity-type aliases
// =============================================================================

// auditTypes maps a drawer type to the entity_type strings the audit trail may
// have recorded for it.
//
// There are two writers and they do not always agree, so a single string would
// silently lose half a type's history:
//
//   - the GORM plugin names a row by its model's AuditEntityType(), so a control
//     is "compliance_control";
//   - the HTTP middleware derives the name from the route when no model-level
//     mutation was observed, so PATCH /compliance/controls/:id is "control".
//
// Both are real rows in production trails. Querying both is not defensive
// programming — it is reading the data that exists.
func auditTypes(t Type) []string {
	switch t {
	case TypeAsset, TypeVendor:
		return []string{"asset"}
	case TypeRisk:
		// Risk is deliberately not Auditable (see governance_auditable.go: the
		// score worker writes it on a hot path), so only the HTTP middleware
		// names it — always "risk".
		return []string{"risk"}
	case TypeVulnerability, TypeFinding:
		return []string{"vulnerability"}
	case TypeControl:
		return []string{"compliance_control", "control"}
	case TypeIncident:
		return []string{"incident"}
	case TypeEvidence:
		return []string{"evidence", "control_evidence"}
	default:
		return nil
	}
}

// typeForAuditEntity is the inverse: which drawer type an audit row is about, so
// the tenant-wide feed can both authorise and deep-link it. Second return is
// false for entity types with no drawer (automation rules, delegations…).
func typeForAuditEntity(entityType string) (Type, bool) {
	switch entityType {
	case "asset":
		return TypeAsset, true
	case "risk":
		return TypeRisk, true
	case "vulnerability":
		return TypeVulnerability, true
	case "compliance_control", "control":
		return TypeControl, true
	case "incident":
		return TypeIncident, true
	case "evidence", "control_evidence":
		return TypeEvidence, true
	default:
		return "", false
	}
}

// =============================================================================
// Entity timeline
// =============================================================================

const (
	defaultTimelineLimit = 25
	maxTimelineLimit     = 100
)

// ForEntity returns one entity's merged history, newest first.
func (s *TimelineService) ForEntity(ctx context.Context, c Caller, ref Ref, cursor string, f TimelineFilter) (*TimelinePage, error) {
	if !c.Valid() {
		return nil, domain.NewForbiddenError("authentication required")
	}
	limit := clampLimit(f.Limit)
	cur, err := decodeCursor(cursor)
	if err != nil {
		return nil, err
	}

	var (
		candidates []TimelineEvent
		used       []TimelineSource
	)

	// 1. The canonical trail.
	if s.audit != nil {
		events, err := s.auditEvents(ctx, c.TenantID, auditTypes(ref.Type), ref.ID, cur, limit+1, f)
		if err != nil {
			return nil, err
		}
		used = append(used, SourceAudit)
		for _, e := range events {
			candidates = append(candidates, auditToTimeline(e))
		}
	}

	// 2. Supplementary journals. A failure in one of these degrades that source
	// only: the canonical trail is still worth showing, and a timeline that
	// refuses to render because a secondary journal hiccuped is a worse answer
	// than a timeline that is honest about which journals it read.
	var before *time.Time
	if cur != nil {
		t := cur.at
		before = &t
	}
	for _, src := range s.extra[ref.Type] {
		// Over-read for the same boundary reason as the trail above: a source
		// filters on time alone (`<= before`), so rows sharing the cursor's
		// timestamp come back and are then dropped by the merge. These journals
		// are per-entity, so a doubled window covers any realistic collision —
		// one risk does not accumulate a page of history rows in one instant.
		events, err := src.Events(ctx, c.TenantID, ref.ID, before, limit*2+1)
		if err != nil {
			continue
		}
		used = append(used, src.Source())
		candidates = append(candidates, events...)
	}

	page := s.paginate(candidates, cur, limit, f)
	page.Sources = used
	if err := s.resolveActors(ctx, page.Events); err != nil {
		// Display enrichment only — never a reason to fail a read.
		_ = err
	}
	return page, nil
}

// ForTenant returns the tenant-wide activity feed.
//
// It reads only the canonical trail. The supplementary journals are keyed by a
// parent entity (risk_histories has no tenant column of its own — it is a child
// of a risk), so a tenant-wide read of them would mean a join per source for
// events the audit trail already carries for anything done through the API.
//
// Authorisation (§35): an event is visible when the caller may read the type it
// is about. Events about entities with no drawer — automation rules,
// delegations, approval workflows — are governance surface and are shown only to
// a caller holding the audit permission. Filtering happens AFTER the query, so
// the page can come back shorter than the limit; that is correct behaviour, not
// a bug, and the cursor still advances past what was filtered.
func (s *TimelineService) ForTenant(ctx context.Context, c Caller, cursor string, f TimelineFilter) (*TimelinePage, error) {
	if !c.Valid() {
		return nil, domain.NewForbiddenError("authentication required")
	}
	if s.audit == nil {
		return nil, domain.NewValidationError("timeline is not available on this deployment")
	}
	limit := clampLimit(f.Limit)
	cur, err := decodeCursor(cursor)
	if err != nil {
		return nil, err
	}

	// Over-read: the permission filter below removes rows, and a page that comes
	// back empty because the caller could see none of the last 25 rows would look
	// like the end of the feed.
	raw, err := s.auditEvents(ctx, c.TenantID, nil, "", cur, (limit+1)*3, f)
	if err != nil {
		return nil, err
	}

	seeAll := CanReadAudit(c)
	candidates := make([]TimelineEvent, 0, len(raw))
	for _, e := range raw {
		t, known := typeForAuditEntity(e.EntityType)
		if !seeAll {
			if !known || !c.CanRead(t) {
				continue
			}
		}
		ev := auditToTimeline(e)
		if known {
			ev.Target = Ref{Type: t, ID: e.EntityID}
			ev.TargetURL = DeepLink(t, e.EntityID)
		}
		candidates = append(candidates, ev)
	}

	page := s.paginate(candidates, cur, limit, f)
	page.Sources = []TimelineSource{SourceAudit}
	_ = s.resolveActors(ctx, page.Events)
	return page, nil
}

// auditEvents runs the trail query. entityTypes empty means "any type".
//
// The repository's upper bound is inclusive (created_at <= To), while a cursor
// means "strictly after the row I last showed you". The rows in between — those
// sharing the cursor's exact timestamp and already emitted — sort to the HEAD of
// the result and are dropped by the merge. If they were simply dropped out of a
// limit-sized page, they would eat the page: twenty events written in the same
// millisecond by one import would return a page of nothing and a paginator that
// stops early, silently hiding the rest of the history.
//
// So when a cursor is present this walks the source with a growing offset until
// it has collected a full page of rows that really are past the cursor, or the
// source runs out. Batches are disjoint (the offset advances by what came back),
// so nothing is counted twice. auditScanBudget bounds the walk: a pathological
// run of identical timestamps longer than that returns a short page rather than
// scanning forever, and the cursor still advances.
func (s *TimelineService) auditEvents(ctx context.Context, tenantID uuid.UUID, entityTypes []string, entityID string, cur *cursor, limit int, f TimelineFilter) ([]domain.AuditEvent, error) {
	filter := domain.AuditEventFilter{
		EntityTypes: entityTypes,
		EntityID:    entityID,
		Action:      f.Kind,
		ActorID:     f.ActorID,
		From:        f.Since,
		Limit:       limit,
	}
	if f.Until != nil {
		filter.To = f.Until
	}
	if cur == nil {
		events, _, err := s.audit.List(ctx, tenantID, filter)
		return events, err
	}

	at := cur.at
	if filter.To == nil || at.Before(*filter.To) {
		filter.To = &at
	}

	var (
		kept    []domain.AuditEvent
		offset  int
		scanned int
	)
	for scanned < auditScanBudget {
		filter.Offset = offset
		batch, _, err := s.audit.List(ctx, tenantID, filter)
		if err != nil {
			return nil, err
		}
		if len(batch) == 0 {
			break
		}
		scanned += len(batch)
		for _, e := range batch {
			if beforeCursorAt(e.CreatedAt, e.ID.String(), cur) {
				kept = append(kept, e)
			}
		}
		if len(kept) >= limit {
			break
		}
		if len(batch) < filter.Limit {
			// The source is exhausted.
			break
		}
		offset += len(batch)
	}
	return kept, nil
}

// auditScanBudget caps how many trail rows one page may walk past. It only ever
// binds when more rows than this share the cursor's exact timestamp.
const auditScanBudget = 2000

// paginate merges, orders and cuts. This is the k-way merge that makes a
// multi-source timeline pageable: every source is asked for one more row than
// the page needs, the union is ordered by (occurred_at, id) descending, and the
// cursor names the last row emitted. A source that had nothing to add simply
// contributes nothing to the union.
func (s *TimelineService) paginate(candidates []TimelineEvent, cur *cursor, limit int, f TimelineFilter) *TimelinePage {
	// Never nil: a client that has to guard for both nil and [] will eventually
	// forget one.
	filtered := make([]TimelineEvent, 0, len(candidates))
	for _, e := range candidates {
		if cur != nil && !beforeCursor(e, cur) {
			continue
		}
		if f.Kind != "" && !strings.EqualFold(e.Kind, f.Kind) {
			continue
		}
		if f.Since != nil && e.OccurredAt.Before(*f.Since) {
			continue
		}
		if f.Until != nil && e.OccurredAt.After(*f.Until) {
			continue
		}
		filtered = append(filtered, e)
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		if !filtered[i].OccurredAt.Equal(filtered[j].OccurredAt) {
			return filtered[i].OccurredAt.After(filtered[j].OccurredAt)
		}
		// Deterministic tie-break, so the same page is the same page twice.
		return filtered[i].ID > filtered[j].ID
	})

	page := &TimelinePage{Events: []TimelineEvent{}}
	if len(filtered) > limit {
		page.Events = filtered[:limit]
		last := page.Events[len(page.Events)-1]
		page.NextCursor = encodeCursor(cursor{at: last.OccurredAt, id: last.ID})
	} else {
		page.Events = filtered
	}
	return page
}

// resolveActors fills actor emails in one lookup for the whole page.
func (s *TimelineService) resolveActors(ctx context.Context, events []TimelineEvent) error {
	if s.lookup == nil || len(events) == 0 {
		return nil
	}
	idset := map[uuid.UUID]struct{}{}
	for _, e := range events {
		if e.Actor == nil || e.Actor.ID == "" {
			continue
		}
		if id, err := uuid.Parse(e.Actor.ID); err == nil && id != uuid.Nil {
			idset[id] = struct{}{}
		}
	}
	if len(idset) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, 0, len(idset))
	for id := range idset {
		ids = append(ids, id)
	}
	emails, err := s.lookup.EmailsByIDs(ctx, ids)
	if err != nil {
		return err
	}
	for i := range events {
		if events[i].Actor == nil {
			continue
		}
		id, err := uuid.Parse(events[i].Actor.ID)
		if err != nil {
			continue
		}
		if email := emails[id]; email != "" {
			events[i].Actor.Email = email
			events[i].Actor.Label = email
		}
	}
	return nil
}

// =============================================================================
// Audit trail (raw)
// =============================================================================

// AuditPage is a page of raw audit records — before/after included.
type AuditPage struct {
	Events []domain.AuditEvent `json:"events"`
	Total  int64               `json:"total"`
	Limit  int                 `json:"limit"`
	Offset int                 `json:"offset"`
}

// AuditForEntity returns the raw trail for one entity. Offset paging here rather
// than a cursor: this is a single-source query where a total matters (an auditor
// asks "how many changes"), and it is the same paging the governance screen
// already uses.
func (s *TimelineService) AuditForEntity(ctx context.Context, c Caller, ref Ref, limit, offset int) (*AuditPage, error) {
	if !CanReadAudit(c) {
		return nil, domain.NewForbiddenError("missing permission " + AuditPermission)
	}
	if s.audit == nil {
		return nil, domain.NewValidationError("the audit trail is not available on this deployment")
	}
	if limit <= 0 || limit > maxTimelineLimit {
		limit = defaultTimelineLimit
	}
	if offset < 0 {
		offset = 0
	}
	events, total, err := s.audit.List(ctx, c.TenantID, domain.AuditEventFilter{
		EntityTypes: auditTypes(ref.Type),
		EntityID:    ref.ID,
		Limit:       limit,
		Offset:      offset,
	})
	if err != nil {
		return nil, err
	}
	if s.lookup != nil {
		s.resolveAuditActors(ctx, events)
	}
	return &AuditPage{Events: events, Total: total, Limit: limit, Offset: offset}, nil
}

func (s *TimelineService) resolveAuditActors(ctx context.Context, events []domain.AuditEvent) {
	idset := map[uuid.UUID]struct{}{}
	for _, e := range events {
		if e.ActorID != nil && *e.ActorID != uuid.Nil {
			idset[*e.ActorID] = struct{}{}
		}
	}
	if len(idset) == 0 {
		return
	}
	ids := make([]uuid.UUID, 0, len(idset))
	for id := range idset {
		ids = append(ids, id)
	}
	emails, err := s.lookup.EmailsByIDs(ctx, ids)
	if err != nil {
		return
	}
	for i := range events {
		if events[i].ActorID != nil {
			events[i].ActorEmail = emails[*events[i].ActorID]
		}
	}
}

// =============================================================================
// Conversion
// =============================================================================

// auditToTimeline projects an audit record into a user-facing event.
//
// Before/After are deliberately dropped (§23). The timeline says a field moved;
// it does not carry the snapshot, because the snapshot may hold values a
// timeline reader is not cleared for. Changed field NAMES are kept — they say
// what moved without saying what it moved to.
func auditToTimeline(e domain.AuditEvent) TimelineEvent {
	ev := TimelineEvent{
		ID:         e.ID.String(),
		Kind:       string(e.Action),
		OccurredAt: e.CreatedAt,
		Summary:    e.Summary,
		Source:     SourceAudit,
		Target:     Ref{ID: e.EntityID},
	}
	if t, ok := typeForAuditEntity(e.EntityType); ok {
		ev.Target.Type = t
		ev.TargetURL = DeepLink(t, e.EntityID)
	}
	if e.ActorID != nil && *e.ActorID != uuid.Nil {
		ev.Actor = &Actor{ID: e.ActorID.String(), Email: e.ActorEmail, Label: e.ActorEmail}
	}
	for _, field := range e.ChangedFields {
		ev.Changes = append(ev.Changes, Change{Field: field})
	}
	if ev.Summary == "" {
		ev.Summary = fmt.Sprintf("%s %s", strings.Title(string(e.Action)), e.EntityType) //nolint:staticcheck // ASCII verbs only
	}
	return ev
}

// =============================================================================
// Cursor
// =============================================================================

// cursor is the (occurrence time, id) pair naming the last row a page emitted.
// The id is part of it because several events can share a timestamp — an import
// that creates twenty rows in the same millisecond — and a time-only cursor
// would either skip or repeat them.
type cursor struct {
	at time.Time
	id string
}

func encodeCursor(c cursor) string {
	raw := strconv.FormatInt(c.at.UTC().UnixNano(), 10) + "|" + c.id
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeCursor(s string) (*cursor, error) {
	if strings.TrimSpace(s) == "" {
		return nil, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, domain.NewValidationError("invalid timeline cursor")
	}
	parts := strings.SplitN(string(raw), "|", 2)
	if len(parts) != 2 {
		return nil, domain.NewValidationError("invalid timeline cursor")
	}
	nanos, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return nil, domain.NewValidationError("invalid timeline cursor")
	}
	return &cursor{at: time.Unix(0, nanos).UTC(), id: parts[1]}, nil
}

// beforeCursor reports whether an event sorts strictly after the cursor row in a
// newest-first listing — i.e. whether it belongs on a LATER page.
func beforeCursor(e TimelineEvent, c *cursor) bool {
	return beforeCursorAt(e.OccurredAt, e.ID, c)
}

// beforeCursorAt is the same comparison over the raw (time, id) pair, so the
// audit walk and the merge cannot disagree about where a page ends.
func beforeCursorAt(occurredAt time.Time, id string, c *cursor) bool {
	if occurredAt.Before(c.at) {
		return true
	}
	if occurredAt.Equal(c.at) {
		return id < c.id
	}
	return false
}

func clampLimit(n int) int {
	if n <= 0 {
		return defaultTimelineLimit
	}
	if n > maxTimelineLimit {
		return maxTimelineLimit
	}
	return n
}
