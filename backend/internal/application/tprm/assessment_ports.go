// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package tprm

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
	ent "github.com/opendefender/openrisk/pkg/entitlements"
)

// ---------------------------------------------------------------------------
// Ports of the questionnaire and assessment use cases (#670, ADR 0004 D3/D4)
// ---------------------------------------------------------------------------

// AuditSink records an event. Best-effort and nil-safe, as in membership:
// journalling must never be the reason a legitimate action fails.
type AuditSink interface {
	Record(ctx context.Context, ev domain.AuditEvent)
}

// FeatureChecker answers whether a tenant's plan grants a feature. Satisfied by
// the entitlements service.
type FeatureChecker interface {
	Allowed(ctx context.Context, tenant uuid.UUID, f ent.Feature) (bool, ent.Plan, ent.Plan, error)
}

// OrganizationReader resolves the organisation a vendor is told is asking.
type OrganizationReader interface {
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Organization, error)
}

// Throttle is the counter used to debounce the vendor's draft-save audit
// events. Satisfied by the rate-limit stores.
type Throttle interface {
	IsAllowed(key string, maxRequests int, window time.Duration) bool
}

// QuestionnaireMail is everything the vendor mailer needs. QuestionnaireURL
// carries the one-time token.
type QuestionnaireMail struct {
	To               string
	Locale           string
	OrganizationName string
	VendorName       string
	QuestionnaireURL string
	DueAt            time.Time
	Reason           domain.VendorAssessmentTokenReason
}

// QuestionnaireMailer delivers a questionnaire link. It must report honestly
// whether the message went out, never a silent success.
type QuestionnaireMailer interface {
	SendQuestionnaire(ctx context.Context, m QuestionnaireMail) error
}

// TemplateStore persists questionnaire templates.
type TemplateStore interface {
	CreateTemplate(ctx context.Context, t *domain.VendorQuestionnaireTemplate) error
	GetTemplate(ctx context.Context, id, tenantID uuid.UUID) (*domain.VendorQuestionnaireTemplate, error)
	ListTemplates(ctx context.Context, tenantID uuid.UUID, includeArchived bool) ([]domain.VendorQuestionnaireTemplate, error)
	ReplaceTemplate(ctx context.Context, t *domain.VendorQuestionnaireTemplate) (bool, error)
	ArchiveTemplate(ctx context.Context, id, tenantID uuid.UUID, at time.Time) (bool, error)
}

// AssessmentStore persists assessments, their items and their tokens.
type AssessmentStore interface {
	CreateAssessment(ctx context.Context, a *domain.VendorAssessment, tok *domain.VendorAssessmentToken) error
	GetAssessment(ctx context.Context, id, tenantID uuid.UUID) (*domain.VendorAssessment, error)
	ListAssessmentsByVendor(ctx context.Context, tenantID, vendorID uuid.UUID) ([]domain.VendorAssessment, error)
	RevokeAssessment(ctx context.Context, id, tenantID, by uuid.UUID, at time.Time) (bool, error)
	IssueToken(ctx context.Context, tok *domain.VendorAssessmentToken) (bool, error)
	FindTokenByHash(ctx context.Context, hash string) (*domain.VendorAssessmentToken, error)
	SaveAnswers(ctx context.Context, tenantID, assessmentID uuid.UUID, items []domain.VendorAssessmentItem, at time.Time) (bool, error)
	SubmitAssessment(ctx context.Context, tenantID, assessmentID uuid.UUID, items []domain.VendorAssessmentItem, provenance domain.JSONMap, at time.Time) (bool, error)
}

// AssessmentDeps is what every questionnaire and assessment use case is built
// from. One struct, so the thirteen use cases are wired identically and a
// missing dependency is visible in one place.
type AssessmentDeps struct {
	Vendors     domain.VendorRepository
	Templates   TemplateStore
	Assessments AssessmentStore
	// Features is REQUIRED for the public routes and fails closed: with no
	// checker wired, a public link answers 410 rather than serving TPRM to a
	// tenant whose plan was never checked.
	Features FeatureChecker
	Orgs     OrganizationReader  // optional: the organisation name in the view and the mail
	Mailer   QuestionnaireMailer // optional: without it the link is handed to the sender
	Audit    AuditSink           // optional
	Throttle Throttle            // optional: debounces draft-save audit events
	// BaseURL is the public origin of the web app (APP_BASE_URL).
	BaseURL string
	Now     func() time.Time
}

func (d AssessmentDeps) now() time.Time {
	if d.Now != nil {
		return d.Now().UTC()
	}
	return time.Now().UTC()
}

func (d AssessmentDeps) record(ctx context.Context, tenantID, actorID uuid.UUID, action domain.AuditAction, entityType, entityID, summary string, after domain.JSONMap) {
	if d.Audit == nil || tenantID == uuid.Nil {
		return
	}
	var actor *uuid.UUID
	if actorID != uuid.Nil {
		a := actorID
		actor = &a
	}
	d.Audit.Record(ctx, domain.AuditEvent{
		TenantID:   tenantID,
		ActorID:    actor,
		Action:     action,
		EntityType: entityType,
		EntityID:   entityID,
		Summary:    summary,
		After:      after,
	})
}

// questionnaireURL puts the token in the URL FRAGMENT (ADR 0004 D4): a fragment
// is not sent to the server when the page loads, so the token never lands in an
// access log, a proxy log or a Referer header.
func (d AssessmentDeps) questionnaireURL(token string) string {
	return strings.TrimRight(strings.TrimSpace(d.BaseURL), "/") + "/vendor-questionnaire#" + token
}

func (d AssessmentDeps) organizationName(ctx context.Context, tenantID uuid.UUID) string {
	if d.Orgs == nil {
		return ""
	}
	org, err := d.Orgs.GetByID(ctx, tenantID)
	if err != nil || org == nil {
		return ""
	}
	return org.Name
}

func (d AssessmentDeps) vendorName(ctx context.Context, a *domain.VendorAssessment) string {
	if d.Vendors == nil {
		return ""
	}
	v, err := d.Vendors.GetVendor(ctx, a.VendorAssetID, a.TenantID)
	if err != nil || v == nil {
		return ""
	}
	return v.Name
}

func conflict(message string) error {
	return &domain.AppError{Err: domain.ErrConflict, Message: message, Code: http.StatusConflict}
}

// ---------------------------------------------------------------------------
// Delivery — the membership invitation's pattern, applied to a vendor link
// ---------------------------------------------------------------------------

// DeliveryStatus says whether the questionnaire email went out.
type DeliveryStatus string

const (
	DeliverySent        DeliveryStatus = "sent"
	DeliveryUnavailable DeliveryStatus = "unavailable"
	DeliveryFailed      DeliveryStatus = "failed"
)

// AssessmentDelivery is the result of sending or resending a questionnaire.
type AssessmentDelivery struct {
	Assessment     *domain.VendorAssessment `json:"assessment"`
	Delivery       DeliveryStatus           `json:"delivery"`
	DeliveryDetail string                   `json:"delivery_detail,omitempty"`
	// QuestionnaireURL carries the one-time token and is returned ONLY when the
	// email did not go out. When mail works, the vendor is the only one who
	// receives the link, and the sender has no reason to hold a credential that
	// writes somebody else's answers. When mail does not work, withholding it
	// would leave a questionnaire nobody can reach — so the sender gets it once,
	// to deliver by hand, and no listing ever returns it again.
	QuestionnaireURL string `json:"questionnaire_url,omitempty"`
}

func (d AssessmentDeps) deliver(ctx context.Context, a *domain.VendorAssessment, token string, reason domain.VendorAssessmentTokenReason) *AssessmentDelivery {
	url := d.questionnaireURL(token)
	out := &AssessmentDelivery{Assessment: a}

	if d.Mailer == nil {
		out.Delivery = DeliveryUnavailable
		out.DeliveryDetail = "No email transport is configured on this deployment — share the link below with the vendor yourself."
		out.QuestionnaireURL = url
		return out
	}

	mail := QuestionnaireMail{
		To:               a.ContactEmail,
		Locale:           a.ContactLanguage,
		OrganizationName: d.organizationName(ctx, a.TenantID),
		VendorName:       d.vendorName(ctx, a),
		QuestionnaireURL: url,
		DueAt:            a.DueAt,
		Reason:           reason,
	}
	if err := d.Mailer.SendQuestionnaire(ctx, mail); err != nil {
		out.Delivery = DeliveryFailed
		out.DeliveryDetail = "The questionnaire was created but the email could not be sent — share the link below with the vendor yourself."
		out.QuestionnaireURL = url
		return out
	}
	out.Delivery = DeliverySent
	return out
}

// ---------------------------------------------------------------------------
// Public access — ADR 0004 D4's status contract, in one place
// ---------------------------------------------------------------------------

// maxPresentedTokenLen bounds what is hashed. A real token is 43 characters.
const maxPresentedTokenLen = 128

// questionnaireNotFound is the ONE answer for every token that names nothing
// usable: missing, malformed, unknown. Identical in status and body, so the
// public routes cannot be used to test guesses.
func questionnaireNotFound() error {
	return domain.NewNotFoundError("questionnaire", "link")
}

type publicAccess struct {
	assessment *domain.VendorAssessment
	token      *domain.VendorAssessmentToken
}

// resolveToken applies the status contract. The tenant comes from the token
// row and from nowhere else: no header, body, cookie or session is read.
func (d AssessmentDeps) resolveToken(ctx context.Context, presented string, write bool) (*publicAccess, error) {
	presented = strings.TrimSpace(presented)
	if presented == "" || len(presented) > maxPresentedTokenLen {
		return nil, questionnaireNotFound()
	}
	if d.Assessments == nil {
		return nil, questionnaireNotFound()
	}

	tok, err := d.Assessments.FindTokenByHash(ctx, domain.HashVendorAssessmentToken(presented))
	if err != nil {
		return nil, domain.NewInternalError(err.Error())
	}
	// Belt and braces over the hash lookup, in constant time.
	if tok == nil || !domain.InvitationTokenMatches(presented, tok.TokenHash) {
		return nil, questionnaireNotFound()
	}
	// A superseded link was held legitimately: tell its holder what to do.
	if tok.SupersededAt != nil {
		return nil, domain.NewGoneError("a newer link to this questionnaire was sent to the same address — use the most recent email")
	}

	a, err := d.Assessments.GetAssessment(ctx, tok.AssessmentID, tok.TenantID)
	if err != nil {
		return nil, domain.NewInternalError(err.Error())
	}
	if a == nil || a.TenantID != tok.TenantID {
		return nil, questionnaireNotFound()
	}

	if d.Features == nil {
		return nil, domain.NewGoneError("this questionnaire is no longer available")
	}
	allowed, _, _, err := d.Features.Allowed(ctx, tok.TenantID, ent.FeatVendorRisk)
	if err != nil {
		return nil, domain.NewInternalError(err.Error())
	}
	if !allowed {
		// Generic on purpose: the vendor is not told about the sender's plan.
		return nil, domain.NewGoneError("this questionnaire is no longer available")
	}

	now := d.now()
	if a.Status == domain.VendorAssessmentRevoked {
		return nil, domain.NewGoneError("this questionnaire was withdrawn by the organisation that sent it")
	}
	if now.After(a.GraceEndsAt()) {
		return nil, domain.NewGoneError("this questionnaire has expired — ask the organisation that sent it for a new link")
	}
	if write && !a.AcceptsAnswers(now) {
		return nil, conflict("this questionnaire was already submitted and can no longer be changed")
	}
	return &publicAccess{assessment: a, token: tok}, nil
}

func (d AssessmentDeps) publicView(ctx context.Context, a *domain.VendorAssessment) *domain.VendorAssessmentPublicView {
	view := a.PublicView(d.organizationName(ctx, a.TenantID), d.vendorName(ctx, a), d.now())
	return &view
}

// vendorContactActor names the system actor of a vendor-side audit event: the
// contact holds no account, so there is no user id to record.
func vendorContactActor(a *domain.VendorAssessment) string {
	return "vendor_contact:" + a.ID.String()
}
