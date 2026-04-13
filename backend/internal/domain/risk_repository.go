package domain

import (
	"context"

	"github.com/google/uuid"
)

// RiskRepository defines the port for risk data persistence.
// Infrastructure layer implements this interface.
type RiskRepository interface {
	// Create persists a new risk. Returns the created risk with generated ID.
	Create(ctx context.Context, risk *Risk) error

	// GetByID retrieves a risk by ID scoped to an organization.
	GetByID(ctx context.Context, id uuid.UUID, orgID uuid.UUID) (*Risk, error)

	// List retrieves risks with filtering, pagination, and sorting scoped to an organization.
	List(ctx context.Context, orgID uuid.UUID, query RiskQuery) (*PaginatedResult[Risk], error)

	// Update updates an existing risk. Only non-nil fields are updated.
	Update(ctx context.Context, risk *Risk) error

	// Delete soft-deletes a risk by ID scoped to an organization.
	Delete(ctx context.Context, id uuid.UUID, orgID uuid.UUID) error

	// Count returns the total number of risks for an organization.
	Count(ctx context.Context, orgID uuid.UUID) (int64, error)
}

// RiskQuery encapsulates filtering/pagination parameters for listing risks.
type RiskQuery struct {
	// Filtering
	Search   string // searches title and description (ILIKE)
	Status   string // filter by status
	Level    string // filter by severity level
	Owner    string // filter by owner
	Tags     []string
	MinScore *float64
	MaxScore *float64

	// Pagination
	Page  int // 1-indexed
	Limit int // max items per page

	// Sorting
	SortBy    string // field name (default: "created_at")
	SortOrder string // "asc" or "desc" (default: "desc")
}

// PaginatedResult wraps a paginated query result.
type PaginatedResult[T any] struct {
	Data       []T   `json:"data"`
	Total      int64 `json:"total"`
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	TotalPages int   `json:"total_pages"`
}

// NewRiskQuery returns a RiskQuery with sensible defaults.
func NewRiskQuery() RiskQuery {
	return RiskQuery{
		Page:      1,
		Limit:     20,
		SortBy:    "created_at",
		SortOrder: "desc",
	}
}

// Offset calculates the SQL offset from page and limit.
func (q RiskQuery) Offset() int {
	if q.Page < 1 {
		return 0
	}
	return (q.Page - 1) * q.Limit
}

// Sanitize ensures all values are within acceptable bounds.
func (q *RiskQuery) Sanitize() {
	if q.Page < 1 {
		q.Page = 1
	}
	if q.Limit < 1 {
		q.Limit = 20
	}
	if q.Limit > 100 {
		q.Limit = 100
	}
	if q.SortBy == "" {
		q.SortBy = "created_at"
	}
	if q.SortOrder != "asc" {
		q.SortOrder = "desc"
	}
	// Whitelist sortable fields to prevent SQL injection
	allowed := map[string]bool{
		"created_at": true, "updated_at": true, "title": true,
		"score": true, "impact": true, "probability": true,
		"status": true, "level": true, "owner": true,
	}
	if !allowed[q.SortBy] {
		q.SortBy = "created_at"
	}
}
