// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/application/evidence"
	"github.com/opendefender/openrisk/internal/domain"
)

// EvidenceHandler serves the evidence library.
type EvidenceHandler struct {
	svc *evidence.Service
}

func NewEvidenceHandler(svc *evidence.Service) *EvidenceHandler {
	return &EvidenceHandler{svc: svc}
}

// parseUUIDList reads a comma-separated list of ids from a form field or JSON
// body. Silently dropping a malformed id would attach proof to fewer controls
// than the user asked for, without telling them, so an unparseable entry fails
// the whole request.
func parseUUIDList(raw []string) ([]uuid.UUID, error) {
	out := make([]uuid.UUID, 0, len(raw))
	for _, s := range raw {
		for _, part := range strings.Split(s, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			id, err := uuid.Parse(part)
			if err != nil {
				return nil, fmt.Errorf("invalid control id: %q", part)
			}
			out = append(out, id)
		}
	}
	return out, nil
}

// parseDate accepts either a full RFC3339 timestamp or a bare calendar date.
//
// Bare dates are accepted because that is what the fields carry: nobody knows,
// or should be asked, the exact minute a policy was approved.
func parseDate(s string) (*time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return &t, nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return nil, fmt.Errorf("invalid date %q (expected YYYY-MM-DD)", s)
	}
	return &t, nil
}

// Create accepts multipart/form-data (with an optional file) or JSON.
//
// One endpoint for both because from the user's side there is one action —
// "record this proof" — whether the artifact is a PDF they hold or a link to a
// wiki page they do not.
func (h *EvidenceHandler) Create(c *fiber.Ctx) error {
	in := evidence.CreateInput{CollectedBy: userID(c)}

	ct := string(c.Request().Header.ContentType())
	if strings.HasPrefix(ct, fiber.MIMEMultipartForm) {
		in.Title = c.FormValue("title")
		in.Type = c.FormValue("type")
		in.Description = c.FormValue("description")
		in.Source = c.FormValue("source")
		in.SourceDetail = c.FormValue("source_detail")
		in.ExternalURL = c.FormValue("external_url")
		in.Review = c.FormValue("review")

		collectedAt, err := parseDate(c.FormValue("collected_at"))
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": err.Error()})
		}
		in.CollectedAt = collectedAt

		validUntil, err := parseDate(c.FormValue("valid_until"))
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": err.Error()})
		}
		in.ValidUntil = validUntil

		ids, err := parseUUIDList([]string{c.FormValue("control_ids")})
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": err.Error()})
		}
		in.ControlIDs = ids

		if fh, err := c.FormFile("file"); err == nil && fh != nil {
			f, err := fh.Open()
			if err != nil {
				return c.Status(400).JSON(fiber.Map{"error": "failed to read uploaded file"})
			}
			defer f.Close()
			in.Filename = fh.Filename
			in.Content = f
		}
	} else {
		var body struct {
			Title        string   `json:"title"`
			Type         string   `json:"type"`
			Description  string   `json:"description"`
			Source       string   `json:"source"`
			SourceDetail string   `json:"source_detail"`
			ExternalURL  string   `json:"external_url"`
			Review       string   `json:"review"`
			CollectedAt  string   `json:"collected_at"`
			ValidUntil   string   `json:"valid_until"`
			ControlIDs   []string `json:"control_ids"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
		}
		in.Title, in.Type, in.Description = body.Title, body.Type, body.Description
		in.Source, in.SourceDetail, in.ExternalURL = body.Source, body.SourceDetail, body.ExternalURL
		in.Review = body.Review

		collectedAt, err := parseDate(body.CollectedAt)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": err.Error()})
		}
		in.CollectedAt = collectedAt

		validUntil, err := parseDate(body.ValidUntil)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": err.Error()})
		}
		in.ValidUntil = validUntil

		ids, err := parseUUIDList(body.ControlIDs)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": err.Error()})
		}
		in.ControlIDs = ids
	}

	ev, err := h.svc.Create(c.UserContext(), tenantID(c), in)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.Status(201).JSON(ev)
}

// CreateForControl is the per-control upload path
// (POST /compliance/controls/:controlId/evidences).
//
// It exists so the control drawer's "attach a file" keeps its URL and its
// multipart shape, while the artifact it produces lands in the library like any
// other — the same file can then answer a second framework without being
// re-uploaded. It accepts the library's richer fields when they are sent, and
// works without them when they are not.
func (h *EvidenceHandler) CreateForControl(c *fiber.Ctx) error {
	controlID, err := uuid.Parse(c.Params("controlId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid control id"})
	}

	fh, err := c.FormFile("file")
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "file is required"})
	}
	f, err := fh.Open()
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "failed to read uploaded file"})
	}
	defer f.Close()

	collectedAt, err := parseDate(c.FormValue("collected_at"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	validUntil, err := parseDate(c.FormValue("valid_until"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	ev, err := h.svc.Create(c.UserContext(), tenantID(c), evidence.CreateInput{
		Title:       c.FormValue("title"),
		Type:        c.FormValue("type"),
		Description: c.FormValue("description"),
		Filename:    fh.Filename,
		Content:     f,
		CollectedAt: collectedAt,
		ValidUntil:  validUntil,
		ControlIDs:  []uuid.UUID{controlID},
		CollectedBy: userID(c),
	})
	if err != nil {
		return writeAppError(c, err)
	}
	return c.Status(201).JSON(ev)
}

// List returns a filtered page of the library.
func (h *EvidenceHandler) List(c *fiber.Ctx) error {
	f := domain.EvidenceFilter{
		Search: c.Query("q"),
		Limit:  50,
	}
	if v := c.Query("type"); v != "" {
		t, err := domain.ParseEvidenceType(v)
		if err != nil {
			return writeAppError(c, err)
		}
		f.Type = t
	}
	if v := c.Query("review"); v != "" {
		r, err := domain.ParseEvidenceReview(v)
		if err != nil {
			return writeAppError(c, err)
		}
		f.Review = r
	}
	if v := c.Query("control_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid control id"})
		}
		f.ControlID = &id
	}
	if v := c.Query("framework_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid framework id"})
		}
		f.FrameworkID = &id
	}
	if n, err := strconv.Atoi(c.Query("limit")); err == nil && n > 0 && n <= 500 {
		f.Limit = n
	}
	if n, err := strconv.Atoi(c.Query("offset")); err == nil && n > 0 {
		f.Offset = n
	}

	res, err := h.svc.List(c.UserContext(), tenantID(c), f)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(res)
}

func (h *EvidenceHandler) Get(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("evidenceId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid evidence id"})
	}
	ev, err := h.svc.Get(c.UserContext(), tenantID(c), id)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(ev)
}

func (h *EvidenceHandler) Update(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("evidenceId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid evidence id"})
	}

	// Pointers throughout so an absent field means "leave it", not "clear it" —
	// a form that does not render a field must not erase it.
	var body struct {
		Title        *string `json:"title"`
		Type         *string `json:"type"`
		Description  *string `json:"description"`
		ExternalURL  *string `json:"external_url"`
		SourceDetail *string `json:"source_detail"`
		CollectedAt  *string `json:"collected_at"`
		ValidUntil   *string `json:"valid_until"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
	}

	in := evidence.UpdateInput{
		Title: body.Title, Type: body.Type, Description: body.Description,
		ExternalURL: body.ExternalURL, SourceDetail: body.SourceDetail,
	}
	if body.CollectedAt != nil {
		t, err := parseDate(*body.CollectedAt)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": err.Error()})
		}
		in.CollectedAt = t
	}
	if body.ValidUntil != nil {
		// An explicit empty string clears the expiry (the artifact does not go
		// stale); an absent key leaves whatever is there.
		t, err := parseDate(*body.ValidUntil)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": err.Error()})
		}
		in.ValidUntil = &t
	}

	ev, err := h.svc.Update(c.UserContext(), tenantID(c), id, in)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(ev)
}

// Review records the human verdict on an artifact.
func (h *EvidenceHandler) Review(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("evidenceId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid evidence id"})
	}
	var body struct {
		Review string `json:"review"`
		Note   string `json:"note"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
	}
	ev, err := h.svc.Review(c.UserContext(), tenantID(c), id, userID(c), body.Review, body.Note)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(ev)
}

func (h *EvidenceHandler) Delete(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("evidenceId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid evidence id"})
	}
	if err := h.svc.Delete(c.UserContext(), tenantID(c), id); err != nil {
		return writeAppError(c, err)
	}
	return c.SendStatus(204)
}

// Link attaches an existing artifact to one or more controls — reuse without
// re-upload.
func (h *EvidenceHandler) Link(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("evidenceId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid evidence id"})
	}
	var body struct {
		ControlIDs []string `json:"control_ids"`
		Note       string   `json:"note"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
	}
	ids, err := parseUUIDList(body.ControlIDs)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	ev, err := h.svc.Link(c.UserContext(), tenantID(c), id, ids, body.Note, userID(c))
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(ev)
}

func (h *EvidenceHandler) Unlink(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("evidenceId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid evidence id"})
	}
	controlID, err := uuid.Parse(c.Params("controlId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid control id"})
	}
	if err := h.svc.Unlink(c.UserContext(), tenantID(c), id, controlID); err != nil {
		return writeAppError(c, err)
	}
	return c.SendStatus(204)
}

// Download streams the artifact's bytes.
func (h *EvidenceHandler) Download(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("evidenceId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid evidence id"})
	}
	ev, content, err := h.svc.Download(c.UserContext(), tenantID(c), id)
	if err != nil {
		return writeAppError(c, err)
	}
	// No defer content.Close(): SendStream hands the reader to fasthttp, which
	// reads and closes it after this handler returns. Closing here races the write.
	name := ev.Filename
	if name == "" {
		name = ev.Title
	}
	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="%s"`, name))
	c.Set(fiber.HeaderContentType, "application/octet-stream")
	return c.SendStream(content)
}

// ListByControl returns the artifacts attached to one control.
func (h *EvidenceHandler) ListByControl(c *fiber.Ctx) error {
	controlID, err := uuid.Parse(c.Params("controlId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid control id"})
	}
	items, err := h.svc.ListByControl(c.UserContext(), tenantID(c), controlID)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(items)
}

// Missing serves the "what proof am I missing" worklist, for one framework or
// across all of them.
func (h *EvidenceHandler) Missing(c *fiber.Ctx) error {
	var frameworkID uuid.UUID
	if v := c.Query("framework_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid framework id"})
		}
		frameworkID = id
	}
	cov, err := h.svc.MissingEvidence(c.UserContext(), tenantID(c), frameworkID)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(fiber.Map{"frameworks": cov})
}
