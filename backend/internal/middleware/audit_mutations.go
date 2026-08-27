// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package middleware

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/audittrail"
)

// ---------------------------------------------------------------------------
// AuditMutations — every state-changing API call lands in the trail.
//
// The GORM plugin covers rows; this covers ACTIONS. Mounted once on the
// authenticated group, it journals every successful POST / PUT / PATCH / DELETE
// with: actor, tenant, action, resource type, resource id, before, after, ip,
// user agent, request id and timestamp. A developer adding a new route gets
// coverage for free — there is nothing to remember, and nothing to forget.
//
// Exactly ONE entry per request: the middleware installs a collector that the
// GORM plugin and the application Recorder write into, then appends a single
// chained entry merging intent with before → after evidence.
// ---------------------------------------------------------------------------

// AuditAppender is the write side of the trail (satisfied by the chained repo).
type AuditAppender interface {
	Append(ctx context.Context, e *domain.AuditEvent) error
}

// auditSkipPrefixes are read-shaped POSTs that would only add noise: they change
// no tenant data, and one of them (search) would otherwise dominate the trail.
var auditSkipPrefixes = []string{
	"/api/v1/auth/refresh",
	"/api/v1/search",
	"/api/v1/notifications/read",
	"/api/v1/metrics",
}

// AuditMutations returns the mutation-journaling middleware. A nil appender
// disables journaling rather than failing requests.
func AuditMutations(appender AuditAppender) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if appender == nil || !isMutation(c.Method()) || skipAudit(c.Path()) {
			return c.Next()
		}
		mw := GetContext(c)
		if mw == nil || mw.OrganizationID == uuid.Nil {
			// No tenant context — journaling it could attribute the action to the
			// wrong tenant, which is worse than not journaling it.
			return c.Next()
		}

		col := audittrail.NewCollector()
		c.SetUserContext(audittrail.WithCollector(c.UserContext(), col))

		err := c.Next()

		status := c.Response().StatusCode()
		if status < 200 || status >= 300 {
			// A refused or failed action changed nothing; the auth audit log
			// already records failed attempts.
			return err
		}

		ev := buildAuditEvent(c, mw, col)
		// Best-effort: a trail write must never fail the business call the user
		// just made successfully.
		_ = appender.Append(c.UserContext(), ev)
		return err
	}
}

func isMutation(method string) bool {
	switch method {
	case fiber.MethodPost, fiber.MethodPut, fiber.MethodPatch, fiber.MethodDelete:
		return true
	}
	return false
}

func skipAudit(path string) bool {
	for _, p := range auditSkipPrefixes {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

// buildAuditEvent merges what the request said it did (route + method) with what
// the layers below observed (collector).
func buildAuditEvent(c *fiber.Ctx, mw *RequestContext, col *audittrail.Collector) *domain.AuditEvent {
	resType, resID, verb := resourceFromRoute(c)
	action := actionFor(c.Method(), verb)

	var actorID *uuid.UUID
	if mw.UserID != uuid.Nil {
		id := mw.UserID
		actorID = &id
	}

	ev := &domain.AuditEvent{
		ID:         uuid.New(),
		TenantID:   mw.OrganizationID,
		ActorID:    actorID,
		Action:     action,
		EntityType: resType,
		EntityID:   resID,
		IPAddress:  c.IP(),
		UserAgent:  c.Get("User-Agent"),
		RequestID:  requestID(c),
		Method:     c.Method(),
		Path:       c.Path(),
		StatusCode: c.Response().StatusCode(),
		Source:     domain.AuditSourceHTTP,
	}

	if m, ok := col.Primary(); ok {
		if m.EntityType != "" {
			ev.EntityType = m.EntityType
		}
		if m.EntityID != "" {
			ev.EntityID = m.EntityID
		}
		// The observed action beats the HTTP verb's guess: POST /…/bulk-archive
		// really is an update, and the row layer knows that while the method
		// does not.
		if m.Action != "" {
			ev.Action = m.Action
		}
		ev.Before = m.Before
		ev.After = m.After
		ev.ChangedFields = m.Changed
		ev.Summary = m.Summary
	}
	// A collection POST names no id in its route, and for a model that is not
	// Auditable the row layer never observes one either — so a creation was
	// journalled without saying WHAT was created. Every risk create in the trail
	// carried an empty entity_id, which meant a risk's own history could never
	// show its creation: the entity-scoped query filters on that column.
	//
	// The created resource's id is in the response the handler just wrote, so
	// read it from there. Bounded and guarded: only when nothing better is
	// known, only on a successful create, only for a JSON body small enough
	// that parsing it is free next to the request that produced it.
	if ev.EntityID == "" && ev.Action == domain.AuditActionCreate {
		if id := createdIDFromResponse(c); id != "" {
			ev.EntityID = id
		}
	}
	if ev.Summary == "" {
		ev.Summary = defaultSummary(action, ev.EntityType, ev.EntityID, verb)
	}
	// When a request touched several rows, say so rather than pretending the
	// primary one is the whole story.
	if n := col.Len(); n > 1 {
		ev.Summary += " (" + itoa(n) + " records affected)"
	}
	return ev
}

// requestID returns the caller's correlation id, minting one when absent so
// every entry can be tied back to a single request even without a gateway.
func requestID(c *fiber.Ctx) string {
	for _, h := range []string{"X-Request-ID", "X-Correlation-ID", "X-Request-Id"} {
		if v := strings.TrimSpace(c.Get(h)); v != "" {
			return v
		}
	}
	if v, ok := c.Locals("requestid").(string); ok && v != "" {
		return v
	}
	return uuid.NewString()
}

// resourceFromRoute derives (resource_type, resource_id, verb) from the matched
// route. `/api/v1/risks/:id/transition` with id=abc yields ("risk", "abc",
// "transition"); `/api/v1/automation/rules` yields ("automation_rule", "", "").
func resourceFromRoute(c *fiber.Ctx) (resType, resID, verb string) {
	pattern := c.Path()
	if r := c.Route(); r != nil && r.Path != "" {
		pattern = r.Path
	}
	segs := splitPath(pattern)
	// Drop the API prefix.
	for len(segs) > 0 && (segs[0] == "api" || isVersion(segs[0])) {
		segs = segs[1:]
	}

	var collections []string
	for i, s := range segs {
		if strings.HasPrefix(s, ":") || strings.HasPrefix(s, "+") || strings.HasPrefix(s, "*") {
			name := strings.TrimLeft(s, ":+*")
			name = strings.TrimSuffix(name, "?")
			if v := c.Params(name); v != "" {
				resID = v
			}
			// The collection right before the param names the resource.
			if i > 0 && len(collections) > 0 {
				resType = collections[len(collections)-1]
			}
			verb = ""
			continue
		}
		collections = append(collections, s)
		if resID != "" {
			// A static segment after an id is the action verb (…/:id/approve).
			verb = s
		}
	}
	if resType == "" && len(collections) > 0 {
		resType = collections[len(collections)-1]
		if verb != "" && resType == verb {
			resType = ""
		}
	}
	if resType == "" && len(collections) > 0 {
		resType = collections[0]
	}
	resType = normaliseResource(collections, resType)
	return resType, resID, verb
}

func splitPath(p string) []string {
	raw := strings.Split(strings.Trim(p, "/"), "/")
	out := make([]string, 0, len(raw))
	for _, s := range raw {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func isVersion(s string) bool {
	return len(s) >= 2 && s[0] == 'v' && s[1] >= '0' && s[1] <= '9'
}

// normaliseResource turns a URL collection into a stable singular entity name,
// prefixing the module when the bare name would be ambiguous ("rules",
// "workflows", "channels" exist in more than one module).
func normaliseResource(collections []string, last string) string {
	if last == "" {
		return "unknown"
	}
	name := singular(last)
	ambiguous := map[string]bool{
		"rule": true, "workflow": true, "channel": true, "execution": true,
		"template": true, "config": true, "action": true, "event": true, "member": true,
	}
	if ambiguous[name] && len(collections) > 1 {
		for i := len(collections) - 2; i >= 0; i-- {
			if p := singular(collections[i]); p != "" && p != name {
				return p + "_" + name
			}
		}
	}
	return name
}

// singular strips the common plural suffixes used by the API's collections.
func singular(s string) string {
	s = strings.ReplaceAll(s, "-", "_")
	switch {
	case strings.HasSuffix(s, "ies") && len(s) > 3:
		return s[:len(s)-3] + "y"
	case strings.HasSuffix(s, "sses"):
		return s[:len(s)-2]
	case strings.HasSuffix(s, "ss"):
		return s
	case strings.HasSuffix(s, "s") && len(s) > 1:
		return s[:len(s)-1]
	}
	return s
}

// actionFor maps an HTTP method + trailing verb onto the trail's action verbs.
// Unknown verbs fall back to the method's natural verb, so the filter list stays
// finite while the verb itself survives in the summary.
func actionFor(method, verb string) domain.AuditAction {
	switch strings.ToLower(verb) {
	case "approve":
		return domain.AuditActionApprove
	case "reject":
		return domain.AuditActionReject
	case "revoke":
		return domain.AuditActionRevoke
	case "delegate":
		return domain.AuditActionDelegate
	case "submit":
		return domain.AuditActionSubmit
	case "export":
		return domain.AuditActionExport
	}
	switch method {
	case fiber.MethodDelete:
		return domain.AuditActionDelete
	case fiber.MethodPut, fiber.MethodPatch:
		return domain.AuditActionUpdate
	default:
		return domain.AuditActionCreate
	}
}

func defaultSummary(action domain.AuditAction, resType, resID, verb string) string {
	var b strings.Builder
	if verb != "" {
		b.WriteString(verb)
	} else {
		b.WriteString(string(action))
	}
	b.WriteString(" ")
	b.WriteString(resType)
	if resID != "" {
		b.WriteString(" ")
		b.WriteString(resID)
	}
	return b.String()
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}

// maxAuditBodyScan bounds the response body the trail will parse looking for a
// created id. A create response is a single record; anything larger is a list or
// an export, and neither names one new entity.
const maxAuditBodyScan = 64 << 10

// createdIDFromResponse reads the id of the record a successful create just
// returned.
//
// It decodes into a map rather than a struct so it works for every module's
// response shape without this middleware knowing any of them, and it accepts
// only a string or a number: an object under "id" is a nested relation, not this
// record's identity. Anything it cannot read leaves the id empty, which is
// exactly the behaviour that existed before.
func createdIDFromResponse(c *fiber.Ctx) string {
	if !strings.Contains(strings.ToLower(string(c.Response().Header.ContentType())), "json") {
		return ""
	}
	body := c.Response().Body()
	if len(body) == 0 || len(body) > maxAuditBodyScan {
		return ""
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}
	switch v := payload["id"].(type) {
	case string:
		return v
	case float64:
		// Sequential integer ids (incidents) arrive as JSON numbers.
		return strconv.FormatInt(int64(v), 10)
	default:
		return ""
	}
}
