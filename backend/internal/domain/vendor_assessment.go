// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ============================================================================
// VENDOR ASSESSMENTS — TPRM v1 (ADR 0004 D3, D4)
// ============================================================================
//
// A questionnaire template is authored by the tenant. Sending it to a vendor
// creates an assessment whose questions are SNAPSHOTTED into items, so editing
// the template afterwards never rewrites what a vendor was asked, nor a score
// already computed on it. The vendor answers through a public link carrying an
// opaque token; only the token's SHA-256 hash is stored, and every reminder or
// resend issues a new token that supersedes the previous one.
//
// Every table here carries its own tenant_id (ADR 0004 D3): the filter is
// visible in each query text rather than implied by a parent.

// VendorAssessmentStatus is the STORED lifecycle state. Expired is not stored:
// State derives it from the due date, so no job has to flip rows on time.
type VendorAssessmentStatus string

const (
	VendorAssessmentSent       VendorAssessmentStatus = "sent"
	VendorAssessmentInProgress VendorAssessmentStatus = "in_progress"
	VendorAssessmentSubmitted  VendorAssessmentStatus = "submitted"
	VendorAssessmentRevoked    VendorAssessmentStatus = "revoked"
	// VendorAssessmentExpired is derived by State, never persisted.
	VendorAssessmentExpired VendorAssessmentStatus = "expired"
)

const (
	// VendorAssessmentGrace is how long after its due date an assessment still
	// accepts answers, and a submitted one can still be read through its link.
	VendorAssessmentGrace = 7 * 24 * time.Hour
	// VendorAssessmentSource is the canonical-envelope source of every
	// assessment (ADR 0004 D3).
	VendorAssessmentSource = "vendor_questionnaire"
	// VendorSelfAttestedConfidence is ADR 0004 D3a: the vendor answered about
	// themselves and nobody verified it. Every v1 assessment carries it.
	VendorSelfAttestedConfidence = 0.50

	MaxVendorQuestions         = 200
	MaxVendorTemplateNameLen   = 200
	MaxVendorQuestionTextLen   = 2000
	MaxVendorAnswerTextLen     = 5000
	MaxVendorAnswerCommentLen  = 2000
	MaxVendorQuestionOptions   = 20
	MaxVendorQuestionWeight    = 10.0
	vendorQuestionWeightFactor = 10 // one decimal, numeric(4,1)
)

// VendorAnswerType is how a question is answered. A yes/no question is a choice
// with two options: for some questions "No" is the favourable answer, so the
// points are the author's to set.
type VendorAnswerType string

const (
	VendorAnswerChoice VendorAnswerType = "choice"
	VendorAnswerText   VendorAnswerType = "text"
)

// VendorQuestionnaireLanguages are the languages a questionnaire may be written
// in and a vendor contact may be mailed in: the two the product ships.
var VendorQuestionnaireLanguages = []string{"fr", "en"}

// IsVendorQuestionnaireLanguage reports whether l is one of them.
func IsVendorQuestionnaireLanguage(l string) bool {
	for _, v := range VendorQuestionnaireLanguages {
		if v == l {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// jsonb value types
// ---------------------------------------------------------------------------

// VendorQuestionOption is one answer a choice question offers. Points is in
// [0, 1], where 1 is the fully favourable answer (ADR 0004 D3).
type VendorQuestionOption struct {
	Value  string  `json:"value"`
	Label  string  `json:"label"`
	Points float64 `json:"points"`
}

// VendorQuestionOptions is a question's option list, stored as jsonb.
type VendorQuestionOptions []VendorQuestionOption

func (o VendorQuestionOptions) Value() (driver.Value, error) {
	if o == nil {
		return nil, nil
	}
	return jsonbValue(o)
}

func (o *VendorQuestionOptions) Scan(value interface{}) error { return jsonbScan(value, o) }

// VendorScoreContribution is one question's share of a vendor score
// (ADR 0004 D5). The contributions of an assessment sum to its score.
type VendorScoreContribution struct {
	ItemID       uuid.UUID `json:"item_id"`
	Weight       float64   `json:"weight"`
	Points       float64   `json:"points"`
	NA           bool      `json:"na"`
	Contribution float64   `json:"contribution"`
}

// VendorScoreBreakdown is the stored arithmetic of a vendor score, as jsonb.
type VendorScoreBreakdown []VendorScoreContribution

func (b VendorScoreBreakdown) Value() (driver.Value, error) {
	if b == nil {
		return nil, nil
	}
	return jsonbValue(b)
}

func (b *VendorScoreBreakdown) Scan(value interface{}) error { return jsonbScan(value, b) }

// VendorAssessmentScoring is what the scoring step stores on submission (#671,
// ADR 0004 D5). Score and Tier are nil when nothing was scorable.
type VendorAssessmentScoring struct {
	Score     *float64
	Tier      *string
	Breakdown VendorScoreBreakdown
	Version   string
}

func jsonbValue(v interface{}) (driver.Value, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return string(raw), nil
}

func jsonbScan(value interface{}, dst interface{}) error {
	var raw []byte
	switch v := value.(type) {
	case nil:
		return nil
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return fmt.Errorf("jsonb: unsupported scan type %T", value)
	}
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	return json.Unmarshal(raw, dst)
}

// ---------------------------------------------------------------------------
// Templates
// ---------------------------------------------------------------------------

// VendorQuestionnaireTemplate is a reusable questionnaire the tenant authors.
type VendorQuestionnaireTemplate struct {
	ID          uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	TenantID    uuid.UUID  `gorm:"type:uuid;not null;index" json:"tenant_id"`
	Name        string     `gorm:"type:text;not null" json:"name"`
	Description string     `gorm:"type:text" json:"description"`
	Language    string     `gorm:"type:varchar(8);not null" json:"language"`
	Version     int        `gorm:"not null;default:1" json:"version"`
	ArchivedAt  *time.Time `json:"archived_at"`
	CreatedBy   uuid.UUID  `gorm:"type:uuid" json:"created_by"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`

	// Questions is loaded alongside the template; it is its own table.
	Questions []VendorQuestionnaireQuestion `gorm:"-" json:"questions"`
}

func (VendorQuestionnaireTemplate) TableName() string { return "vendor_questionnaire_templates" }

// Validate checks the template's own fields.
func (t VendorQuestionnaireTemplate) Validate() error {
	name := strings.TrimSpace(t.Name)
	if name == "" {
		return NewValidationError("template name is required")
	}
	if len(name) > MaxVendorTemplateNameLen {
		return NewValidationError("template name is too long")
	}
	if !IsVendorQuestionnaireLanguage(t.Language) {
		return NewValidationError("language must be fr or en")
	}
	return nil
}

// VendorQuestionnaireQuestion is one question of a template's current version.
type VendorQuestionnaireQuestion struct {
	ID         uuid.UUID             `gorm:"type:uuid;primaryKey" json:"id"`
	TenantID   uuid.UUID             `gorm:"type:uuid;not null;index" json:"tenant_id"`
	TemplateID uuid.UUID             `gorm:"type:uuid;not null;index" json:"template_id"`
	Position   int                   `gorm:"not null" json:"position"`
	Text       string                `gorm:"type:text;not null" json:"text"`
	Help       string                `gorm:"type:text" json:"help"`
	AnswerType VendorAnswerType      `gorm:"type:varchar(16);not null" json:"answer_type"`
	Options    VendorQuestionOptions `gorm:"type:jsonb" json:"options"`
	Weight     float64               `gorm:"type:numeric(4,1);not null;default:0" json:"weight"`
	Required   bool                  `gorm:"not null;default:false" json:"required"`
	NAAllowed  bool                  `gorm:"column:na_allowed;not null;default:false" json:"na_allowed"`
	// ControlRef is an optional citation, written by a person. It is never
	// generated: an empty field is honest, a fabricated mapping is not.
	ControlRef string `gorm:"type:text" json:"control_ref"`
}

func (VendorQuestionnaireQuestion) TableName() string { return "vendor_questionnaire_questions" }

// Normalize applies what a question's type implies: a text answer never scores
// (weight 0, no options), and weights keep one decimal.
func (q *VendorQuestionnaireQuestion) Normalize() {
	q.Text = strings.TrimSpace(q.Text)
	q.Help = strings.TrimSpace(q.Help)
	q.ControlRef = strings.TrimSpace(q.ControlRef)
	if q.AnswerType == VendorAnswerText {
		q.Weight = 0
		q.Options = nil
		return
	}
	q.Weight = math.Round(q.Weight*vendorQuestionWeightFactor) / vendorQuestionWeightFactor
	for i := range q.Options {
		q.Options[i].Value = strings.TrimSpace(q.Options[i].Value)
		q.Options[i].Label = strings.TrimSpace(q.Options[i].Label)
	}
}

// Validate checks one question. Call Normalize first.
func (q VendorQuestionnaireQuestion) Validate() error {
	if q.Text == "" {
		return NewValidationError("question text is required")
	}
	if len(q.Text) > MaxVendorQuestionTextLen {
		return NewValidationError("question text is too long")
	}
	switch q.AnswerType {
	case VendorAnswerText:
		return nil
	case VendorAnswerChoice:
	default:
		return NewValidationError("answer_type must be choice or text")
	}
	if q.Weight < 0 || q.Weight > MaxVendorQuestionWeight {
		return NewValidationError("weight must be between 0 and 10")
	}
	if len(q.Options) < 2 {
		return NewValidationError("a choice question needs at least two options")
	}
	if len(q.Options) > MaxVendorQuestionOptions {
		return NewValidationError("a choice question has too many options")
	}
	seen := make(map[string]struct{}, len(q.Options))
	for _, o := range q.Options {
		if o.Value == "" || o.Label == "" {
			return NewValidationError("every option needs a value and a label")
		}
		if _, dup := seen[o.Value]; dup {
			return NewValidationError("option values must be unique within a question")
		}
		seen[o.Value] = struct{}{}
		if o.Points < 0 || o.Points > 1 {
			return NewValidationError("option points must be between 0 and 1")
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Assessments
// ---------------------------------------------------------------------------

// VendorAssessment is one questionnaire sent to one vendor — the canonical
// Assessment (ADR 0004 D3), carrying the #541 envelope.
type VendorAssessment struct {
	ID              uuid.UUID              `gorm:"type:uuid;primaryKey" json:"id"`
	TenantID        uuid.UUID              `gorm:"type:uuid;not null;index" json:"tenant_id"`
	VendorAssetID   uuid.UUID              `gorm:"type:uuid;not null;index" json:"vendor_asset_id"`
	TemplateID      uuid.UUID              `gorm:"type:uuid;not null;index" json:"template_id"`
	TemplateVersion int                    `gorm:"not null" json:"template_version"`
	Status          VendorAssessmentStatus `gorm:"type:varchar(16);not null;index" json:"status"`
	OwnerUserID     uuid.UUID              `gorm:"type:uuid;not null;index" json:"owner_user_id"`
	ContactEmail    string                 `gorm:"type:text;not null" json:"contact_email"`
	ContactLanguage string                 `gorm:"type:varchar(8);not null" json:"contact_language"`
	DueAt           time.Time              `gorm:"not null;index" json:"due_at"`
	SentBy          uuid.UUID              `gorm:"type:uuid;not null" json:"sent_by"`
	SentAt          time.Time              `gorm:"not null" json:"sent_at"`
	SubmittedAt     *time.Time             `json:"submitted_at"`
	RevokedAt       *time.Time             `json:"revoked_at"`
	RevokedBy       *uuid.UUID             `gorm:"type:uuid" json:"revoked_by"`

	// Score columns are written on submission by the scoring step (#671).
	Score          *float64             `gorm:"type:numeric(5,2)" json:"score"`
	Tier           *string              `gorm:"type:varchar(16)" json:"tier"`
	ScoreBreakdown VendorScoreBreakdown `gorm:"type:jsonb" json:"score_breakdown"`
	ScoringVersion *string              `gorm:"type:varchar(32)" json:"scoring_version"`

	// Reminder stamps, one per offset (ADR 0004 D6, #672).
	ReminderD7SentAt *time.Time `gorm:"column:reminder_d7_sent_at" json:"reminder_d7_sent_at"`
	ReminderD3SentAt *time.Time `gorm:"column:reminder_d3_sent_at" json:"reminder_d3_sent_at"`
	ReminderD1SentAt *time.Time `gorm:"column:reminder_d1_sent_at" json:"reminder_d1_sent_at"`

	// Canonical envelope (#541, ADR 0004 D3/D3a).
	Source        string     `gorm:"type:varchar(32);not null" json:"source"`
	SourceID      string     `gorm:"type:text;not null" json:"source_id"`
	SourceVersion string     `gorm:"type:text;not null" json:"source_version"`
	ObservedAt    *time.Time `json:"observed_at"`
	Confidence    float64    `gorm:"type:numeric(3,2);not null" json:"confidence"`
	Provenance    JSONMap    `gorm:"type:jsonb" json:"provenance"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Items is loaded alongside the assessment; it is its own table.
	Items []VendorAssessmentItem `gorm:"-" json:"items,omitempty"`
}

func (VendorAssessment) TableName() string { return "vendor_assessments" }

// GraceEndsAt is when the assessment stops accepting answers and stops being
// readable through its link.
func (a *VendorAssessment) GraceEndsAt() time.Time { return a.DueAt.Add(VendorAssessmentGrace) }

// State is the effective status at now: the stored one, except that an open
// assessment past its grace period reads as expired. Submitted and revoked are
// terminal and never expire.
func (a *VendorAssessment) State(now time.Time) VendorAssessmentStatus {
	switch a.Status {
	case VendorAssessmentSubmitted, VendorAssessmentRevoked:
		return a.Status
	}
	if now.After(a.GraceEndsAt()) {
		return VendorAssessmentExpired
	}
	return a.Status
}

// AcceptsAnswers reports whether a vendor may still write at now.
func (a *VendorAssessment) AcceptsAnswers(now time.Time) bool {
	s := a.State(now)
	return s == VendorAssessmentSent || s == VendorAssessmentInProgress
}

// MissingRequired returns the positions of required items with no answer, so a
// refused submission can say which questions to complete.
func (a *VendorAssessment) MissingRequired() []int {
	var missing []int
	for _, it := range a.Items {
		if it.Required && !it.Answered() {
			missing = append(missing, it.Position)
		}
	}
	return missing
}

// NewVendorAssessmentInput is what sending a questionnaire needs.
type NewVendorAssessmentInput struct {
	TenantID        uuid.UUID
	VendorAssetID   uuid.UUID
	OwnerUserID     uuid.UUID
	SentBy          uuid.UUID
	Template        VendorQuestionnaireTemplate
	ContactEmail    string
	ContactLanguage string
	DueAt           time.Time
}

// NewVendorAssessment builds an assessment with its questions snapshotted into
// items, and the first token. The plaintext token is returned exactly once.
func NewVendorAssessment(in NewVendorAssessmentInput, now time.Time) (*VendorAssessment, *VendorAssessmentToken, string, error) {
	if in.Template.ArchivedAt != nil {
		return nil, nil, "", NewValidationError("an archived template cannot be sent")
	}
	if len(in.Template.Questions) == 0 {
		return nil, nil, "", NewValidationError("the template has no question")
	}
	email := NormalizeEmail(in.ContactEmail)
	if email == "" || !strings.Contains(email, "@") || strings.ContainsAny(email, " \r\n") {
		return nil, nil, "", NewValidationError("a valid contact_email is required")
	}
	if !IsVendorQuestionnaireLanguage(in.ContactLanguage) {
		return nil, nil, "", NewValidationError("contact_language must be fr or en")
	}
	if !in.DueAt.After(now) {
		return nil, nil, "", NewValidationError("due_at must be in the future")
	}

	a := &VendorAssessment{
		ID:              uuid.New(),
		TenantID:        in.TenantID,
		VendorAssetID:   in.VendorAssetID,
		TemplateID:      in.Template.ID,
		TemplateVersion: in.Template.Version,
		Status:          VendorAssessmentSent,
		OwnerUserID:     in.OwnerUserID,
		ContactEmail:    email,
		ContactLanguage: in.ContactLanguage,
		DueAt:           in.DueAt.UTC(),
		SentBy:          in.SentBy,
		SentAt:          now,
		Source:          VendorAssessmentSource,
		SourceVersion:   strconv.Itoa(in.Template.Version),
		Confidence:      VendorSelfAttestedConfidence,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	a.SourceID = a.ID.String()
	a.Provenance = JSONMap{
		"channel":          "public_link",
		"template_id":      in.Template.ID.String(),
		"template_version": in.Template.Version,
	}

	questions := append([]VendorQuestionnaireQuestion(nil), in.Template.Questions...)
	sort.SliceStable(questions, func(i, j int) bool { return questions[i].Position < questions[j].Position })
	a.Items = make([]VendorAssessmentItem, 0, len(questions))
	for i, q := range questions {
		var opts VendorQuestionOptions
		if q.Options != nil {
			opts = append(VendorQuestionOptions(nil), q.Options...)
		}
		a.Items = append(a.Items, VendorAssessmentItem{
			ID:           uuid.New(),
			TenantID:     in.TenantID,
			AssessmentID: a.ID,
			Position:     i + 1,
			Text:         q.Text,
			Help:         q.Help,
			AnswerType:   q.AnswerType,
			Options:      opts,
			Weight:       q.Weight,
			Required:     q.Required,
			NAAllowed:    q.NAAllowed,
			ControlRef:   q.ControlRef,
		})
	}

	tok, plaintext, err := NewVendorAssessmentToken(in.TenantID, a.ID, VendorTokenReasonSend, now)
	if err != nil {
		return nil, nil, "", err
	}
	return a, tok, plaintext, nil
}

// ---------------------------------------------------------------------------
// Items
// ---------------------------------------------------------------------------

// VendorAssessmentItem is a question snapshotted at send, with its answer.
type VendorAssessmentItem struct {
	ID            uuid.UUID             `gorm:"type:uuid;primaryKey" json:"id"`
	TenantID      uuid.UUID             `gorm:"type:uuid;not null;index" json:"tenant_id"`
	AssessmentID  uuid.UUID             `gorm:"type:uuid;not null;index" json:"assessment_id"`
	Position      int                   `gorm:"not null" json:"position"`
	Text          string                `gorm:"type:text;not null" json:"text"`
	Help          string                `gorm:"type:text" json:"help"`
	AnswerType    VendorAnswerType      `gorm:"type:varchar(16);not null" json:"answer_type"`
	Options       VendorQuestionOptions `gorm:"type:jsonb" json:"options"`
	Weight        float64               `gorm:"type:numeric(4,1);not null;default:0" json:"weight"`
	Required      bool                  `gorm:"not null;default:false" json:"required"`
	NAAllowed     bool                  `gorm:"column:na_allowed;not null;default:false" json:"na_allowed"`
	ControlRef    string                `gorm:"type:text" json:"control_ref"`
	AnswerValue   *string               `gorm:"type:text" json:"answer_value"`
	AnswerNA      bool                  `gorm:"column:answer_na;not null;default:false" json:"answer_na"`
	AnswerComment string                `gorm:"type:text" json:"answer_comment"`
	AnsweredAt    *time.Time            `json:"answered_at"`
}

func (VendorAssessmentItem) TableName() string { return "vendor_assessment_items" }

// Answered reports whether the item carries an answer: N/A counts, an empty
// value does not.
func (it VendorAssessmentItem) Answered() bool {
	if it.AnswerNA {
		return true
	}
	return it.AnswerValue != nil && strings.TrimSpace(*it.AnswerValue) != ""
}

// VendorAnswerInput is one answer as the vendor sends it.
type VendorAnswerInput struct {
	ItemID  uuid.UUID `json:"item_id"`
	Value   *string   `json:"answer_value"`
	NA      bool      `json:"answer_na"`
	Comment string    `json:"answer_comment"`
}

// ApplyAnswer validates and records an answer on the item. A nil or empty value
// without N/A clears the answer: a draft may be emptied before submission.
func (it *VendorAssessmentItem) ApplyAnswer(in VendorAnswerInput, now time.Time) error {
	comment := strings.TrimSpace(in.Comment)
	if len(comment) > MaxVendorAnswerCommentLen {
		return NewValidationError(fmt.Sprintf("question %d: comment is too long", it.Position))
	}

	if in.NA {
		if !it.NAAllowed {
			return NewValidationError(fmt.Sprintf("question %d does not accept not applicable", it.Position))
		}
		it.AnswerNA = true
		it.AnswerValue = nil
		it.AnswerComment = comment
		it.AnsweredAt = &now
		return nil
	}

	value := ""
	if in.Value != nil {
		value = strings.TrimSpace(*in.Value)
	}
	if value == "" {
		it.AnswerNA = false
		it.AnswerValue = nil
		it.AnswerComment = comment
		it.AnsweredAt = nil
		return nil
	}

	switch it.AnswerType {
	case VendorAnswerChoice:
		valid := false
		for _, o := range it.Options {
			if o.Value == value {
				valid = true
				break
			}
		}
		if !valid {
			return NewValidationError(fmt.Sprintf("question %d: not one of the offered answers", it.Position))
		}
	case VendorAnswerText:
		if len(value) > MaxVendorAnswerTextLen {
			return NewValidationError(fmt.Sprintf("question %d: answer is too long", it.Position))
		}
	default:
		return NewValidationError(fmt.Sprintf("question %d: unknown answer type", it.Position))
	}

	it.AnswerNA = false
	it.AnswerValue = &value
	it.AnswerComment = comment
	it.AnsweredAt = &now
	return nil
}

// ---------------------------------------------------------------------------
// Tokens (ADR 0004 D4)
// ---------------------------------------------------------------------------

// VendorAssessmentTokenReason records why a token was issued.
type VendorAssessmentTokenReason string

const (
	VendorTokenReasonSend     VendorAssessmentTokenReason = "send"
	VendorTokenReasonReminder VendorAssessmentTokenReason = "reminder"
	VendorTokenReasonResend   VendorAssessmentTokenReason = "resend"
)

// VendorAssessmentToken is one issued link. Only its hash is stored. At most
// one token per assessment is active (superseded_at IS NULL): a partial unique
// index enforces it (migration 0062), and the repository supersedes before it
// inserts.
type VendorAssessmentToken struct {
	ID           uuid.UUID                   `gorm:"type:uuid;primaryKey" json:"id"`
	TenantID     uuid.UUID                   `gorm:"type:uuid;not null;index" json:"-"`
	AssessmentID uuid.UUID                   `gorm:"type:uuid;not null;index" json:"assessment_id"`
	TokenHash    string                      `gorm:"type:varchar(64);uniqueIndex;not null" json:"-"`
	Reason       VendorAssessmentTokenReason `gorm:"type:varchar(16);not null" json:"reason"`
	IssuedAt     time.Time                   `gorm:"not null" json:"issued_at"`
	SupersededAt *time.Time                  `json:"superseded_at"`
}

func (VendorAssessmentToken) TableName() string { return "vendor_assessment_tokens" }

// NewVendorAssessmentToken mints a token with the invitation construction:
// 32 bytes from crypto/rand, base64url, SHA-256 hex stored (ADR 0004 D4).
func NewVendorAssessmentToken(tenantID, assessmentID uuid.UUID, reason VendorAssessmentTokenReason, now time.Time) (*VendorAssessmentToken, string, error) {
	token, hash, err := NewInvitationToken()
	if err != nil {
		return nil, "", NewInternalError("could not mint an assessment token")
	}
	return &VendorAssessmentToken{
		ID:           uuid.New(),
		TenantID:     tenantID,
		AssessmentID: assessmentID,
		TokenHash:    hash,
		Reason:       reason,
		IssuedAt:     now,
	}, token, nil
}

// HashVendorAssessmentToken is the transform between a presented token and the
// stored hash. It is the invitation's, on purpose: one construction to review.
func HashVendorAssessmentToken(token string) string { return HashInvitationToken(token) }

// ---------------------------------------------------------------------------
// The public view — what a vendor may see, and nothing more (ADR 0004 D4)
// ---------------------------------------------------------------------------

// VendorPublicOption is an option WITHOUT its points: a vendor who can see the
// weights can answer to the score.
type VendorPublicOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// VendorPublicItem is an item as the vendor sees it: no weight, no points, no
// control reference, no tenant id.
type VendorPublicItem struct {
	ID            uuid.UUID            `json:"id"`
	Position      int                  `json:"position"`
	Text          string               `json:"text"`
	Help          string               `json:"help"`
	AnswerType    VendorAnswerType     `json:"answer_type"`
	Options       []VendorPublicOption `json:"options"`
	Required      bool                 `json:"required"`
	NAAllowed     bool                 `json:"na_allowed"`
	AnswerValue   *string              `json:"answer_value"`
	AnswerNA      bool                 `json:"answer_na"`
	AnswerComment string               `json:"answer_comment"`
}

// VendorAssessmentPublicView is the whole public payload.
type VendorAssessmentPublicView struct {
	OrganizationName string                 `json:"organization_name"`
	VendorName       string                 `json:"vendor_name"`
	Language         string                 `json:"language"`
	DueAt            time.Time              `json:"due_at"`
	Status           VendorAssessmentStatus `json:"status"`
	ReadOnly         bool                   `json:"read_only"`
	Items            []VendorPublicItem     `json:"items"`
}

// PublicView projects the assessment onto what its link may disclose.
func (a *VendorAssessment) PublicView(organizationName, vendorName string, now time.Time) VendorAssessmentPublicView {
	view := VendorAssessmentPublicView{
		OrganizationName: organizationName,
		VendorName:       vendorName,
		Language:         a.ContactLanguage,
		DueAt:            a.DueAt,
		Status:           a.State(now),
		ReadOnly:         !a.AcceptsAnswers(now),
		Items:            make([]VendorPublicItem, 0, len(a.Items)),
	}
	for _, it := range a.Items {
		opts := make([]VendorPublicOption, 0, len(it.Options))
		for _, o := range it.Options {
			opts = append(opts, VendorPublicOption{Value: o.Value, Label: o.Label})
		}
		view.Items = append(view.Items, VendorPublicItem{
			ID:            it.ID,
			Position:      it.Position,
			Text:          it.Text,
			Help:          it.Help,
			AnswerType:    it.AnswerType,
			Options:       opts,
			Required:      it.Required,
			NAAllowed:     it.NAAllowed,
			AnswerValue:   it.AnswerValue,
			AnswerNA:      it.AnswerNA,
			AnswerComment: it.AnswerComment,
		})
	}
	return view
}
