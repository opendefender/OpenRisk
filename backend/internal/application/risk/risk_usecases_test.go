package risk

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

// --- Mock Repository ---

type mockRiskRepo struct {
	risks  map[uuid.UUID]*domain.Risk
	orgIdx map[uuid.UUID][]uuid.UUID // orgID -> riskIDs
	err    error                     // injectable error
}

func newMockRiskRepo() *mockRiskRepo {
	return &mockRiskRepo{
		risks:  make(map[uuid.UUID]*domain.Risk),
		orgIdx: make(map[uuid.UUID][]uuid.UUID),
	}
}

func (m *mockRiskRepo) Create(ctx context.Context, risk *domain.Risk) error {
	if m.err != nil {
		return m.err
	}
	m.risks[risk.ID] = risk
	m.orgIdx[risk.OrganizationID] = append(m.orgIdx[risk.OrganizationID], risk.ID)
	return nil
}

func (m *mockRiskRepo) GetByID(ctx context.Context, id uuid.UUID, orgID uuid.UUID) (*domain.Risk, error) {
	if m.err != nil {
		return nil, m.err
	}
	risk, ok := m.risks[id]
	if !ok || risk.OrganizationID != orgID {
		return nil, nil
	}
	return risk, nil
}

func (m *mockRiskRepo) List(ctx context.Context, orgID uuid.UUID, query domain.RiskQuery) (*domain.PaginatedResult[domain.Risk], error) {
	if m.err != nil {
		return nil, m.err
	}
	var results []domain.Risk
	for _, id := range m.orgIdx[orgID] {
		results = append(results, *m.risks[id])
	}
	return &domain.PaginatedResult[domain.Risk]{
		Data:  results,
		Total: int64(len(results)),
		Page:  query.Page,
		Limit: query.Limit,
	}, nil
}

func (m *mockRiskRepo) Update(ctx context.Context, risk *domain.Risk) error {
	if m.err != nil {
		return m.err
	}
	m.risks[risk.ID] = risk
	return nil
}

func (m *mockRiskRepo) Delete(ctx context.Context, id uuid.UUID, orgID uuid.UUID) error {
	if m.err != nil {
		return m.err
	}
	delete(m.risks, id)
	return nil
}

func (m *mockRiskRepo) Count(ctx context.Context, orgID uuid.UUID) (int64, error) {
	return int64(len(m.orgIdx[orgID])), m.err
}

// --- CreateRisk Tests ---

func TestCreateRisk_Success(t *testing.T) {
	repo := newMockRiskRepo()
	uc := NewCreateRiskUseCase(repo)

	orgID := uuid.New()
	input := CreateRiskInput{
		Title:       "SQL Injection in login form",
		Description: "The login endpoint is vulnerable to SQLi",
		Impact:      4,
		Probability: 3,
		Owner:       "analyst@openrisk.io",
	}

	risk, err := uc.Execute(context.Background(), orgID, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if risk.Title != input.Title {
		t.Errorf("expected title %q, got %q", input.Title, risk.Title)
	}
	if risk.OrganizationID != orgID {
		t.Error("expected risk to be scoped to organization")
	}
	if risk.Score != 12.0 { // 4 * 3
		t.Errorf("expected score 12.0, got %.2f", risk.Score)
	}
	if risk.Status != domain.StatusDraft {
		t.Errorf("expected status DRAFT, got %s", risk.Status)
	}
}

func TestCreateRisk_ValidationError(t *testing.T) {
	repo := newMockRiskRepo()
	uc := NewCreateRiskUseCase(repo)

	tests := []struct {
		name  string
		input CreateRiskInput
	}{
		{
			name:  "empty title",
			input: CreateRiskInput{Title: "", Impact: 3, Probability: 2},
		},
		{
			name:  "impact too low",
			input: CreateRiskInput{Title: "test", Impact: 0, Probability: 2},
		},
		{
			name:  "impact too high",
			input: CreateRiskInput{Title: "test", Impact: 6, Probability: 2},
		},
		{
			name:  "probability too low",
			input: CreateRiskInput{Title: "test", Impact: 3, Probability: 0},
		},
		{
			name:  "probability too high",
			input: CreateRiskInput{Title: "test", Impact: 3, Probability: 6},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := uc.Execute(context.Background(), uuid.New(), tt.input)
			if err == nil {
				t.Fatal("expected validation error, got nil")
			}
			if !errors.Is(err, domain.ErrValidation) {
				t.Errorf("expected ErrValidation, got %v", err)
			}
		})
	}
}

func TestCreateRisk_RepositoryError(t *testing.T) {
	repo := newMockRiskRepo()
	repo.err = errors.New("db connection lost")
	uc := NewCreateRiskUseCase(repo)

	_, err := uc.Execute(context.Background(), uuid.New(), CreateRiskInput{
		Title: "test", Impact: 3, Probability: 2,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- GetRisk Tests ---

func TestGetRisk_Success(t *testing.T) {
	repo := newMockRiskRepo()
	orgID := uuid.New()
	riskID := uuid.New()

	repo.risks[riskID] = &domain.Risk{ID: riskID, OrganizationID: orgID, Title: "Test Risk"}
	repo.orgIdx[orgID] = []uuid.UUID{riskID}

	uc := NewGetRiskUseCase(repo)
	risk, err := uc.Execute(context.Background(), orgID, riskID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if risk.Title != "Test Risk" {
		t.Errorf("expected 'Test Risk', got %q", risk.Title)
	}
}

func TestGetRisk_NotFound(t *testing.T) {
	repo := newMockRiskRepo()
	uc := NewGetRiskUseCase(repo)

	_, err := uc.Execute(context.Background(), uuid.New(), uuid.New())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestGetRisk_WrongTenant(t *testing.T) {
	repo := newMockRiskRepo()
	orgA := uuid.New()
	orgB := uuid.New()
	riskID := uuid.New()

	// Risk belongs to orgA
	repo.risks[riskID] = &domain.Risk{ID: riskID, OrganizationID: orgA, Title: "OrgA Risk"}

	uc := NewGetRiskUseCase(repo)
	// Try to access from orgB → should not find
	_, err := uc.Execute(context.Background(), orgB, riskID)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound for cross-tenant access, got %v", err)
	}
}

// --- ListRisks Tests ---

func TestListRisks_Success(t *testing.T) {
	repo := newMockRiskRepo()
	orgID := uuid.New()

	for i := 0; i < 5; i++ {
		id := uuid.New()
		repo.risks[id] = &domain.Risk{ID: id, OrganizationID: orgID}
		repo.orgIdx[orgID] = append(repo.orgIdx[orgID], id)
	}

	uc := NewListRisksUseCase(repo)
	result, err := uc.Execute(context.Background(), orgID, domain.NewRiskQuery())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Total != 5 {
		t.Errorf("expected 5 risks, got %d", result.Total)
	}
}

func TestListRisks_EmptyOrg(t *testing.T) {
	repo := newMockRiskRepo()
	uc := NewListRisksUseCase(repo)

	result, err := uc.Execute(context.Background(), uuid.New(), domain.NewRiskQuery())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Total != 0 {
		t.Errorf("expected 0 risks, got %d", result.Total)
	}
}

// --- UpdateRisk Tests ---

func TestUpdateRisk_Success(t *testing.T) {
	repo := newMockRiskRepo()
	orgID := uuid.New()
	riskID := uuid.New()

	repo.risks[riskID] = &domain.Risk{
		ID: riskID, OrganizationID: orgID,
		Title: "Old Title", Impact: 2, Probability: 2,
	}
	repo.orgIdx[orgID] = []uuid.UUID{riskID}

	uc := NewUpdateRiskUseCase(repo)
	newTitle := "Updated Title"
	newImpact := 4
	risk, err := uc.Execute(context.Background(), orgID, riskID, UpdateRiskInput{
		Title:  &newTitle,
		Impact: &newImpact,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if risk.Title != "Updated Title" {
		t.Errorf("expected 'Updated Title', got %q", risk.Title)
	}
	if risk.Score != 8.0 { // 4 * 2
		t.Errorf("expected score 8.0, got %.2f", risk.Score)
	}
}

func TestUpdateRisk_NotFound(t *testing.T) {
	repo := newMockRiskRepo()
	uc := NewUpdateRiskUseCase(repo)

	title := "test"
	_, err := uc.Execute(context.Background(), uuid.New(), uuid.New(), UpdateRiskInput{Title: &title})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestUpdateRisk_ValidationError(t *testing.T) {
	repo := newMockRiskRepo()
	orgID := uuid.New()
	riskID := uuid.New()
	repo.risks[riskID] = &domain.Risk{
		ID: riskID, OrganizationID: orgID,
		Title: "Test", Impact: 2, Probability: 2,
	}

	uc := NewUpdateRiskUseCase(repo)

	badImpact := 10
	_, err := uc.Execute(context.Background(), orgID, riskID, UpdateRiskInput{Impact: &badImpact})
	if !errors.Is(err, domain.ErrValidation) {
		t.Errorf("expected ErrValidation, got %v", err)
	}
}

// --- DeleteRisk Tests ---

func TestDeleteRisk_Success(t *testing.T) {
	repo := newMockRiskRepo()
	orgID := uuid.New()
	riskID := uuid.New()
	repo.risks[riskID] = &domain.Risk{ID: riskID, OrganizationID: orgID}

	uc := NewDeleteRiskUseCase(repo)
	err := uc.Execute(context.Background(), orgID, riskID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := repo.risks[riskID]; ok {
		t.Error("expected risk to be deleted from store")
	}
}

func TestDeleteRisk_NotFound(t *testing.T) {
	repo := newMockRiskRepo()
	uc := NewDeleteRiskUseCase(repo)

	err := uc.Execute(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestDeleteRisk_WrongTenant(t *testing.T) {
	repo := newMockRiskRepo()
	orgA := uuid.New()
	orgB := uuid.New()
	riskID := uuid.New()
	repo.risks[riskID] = &domain.Risk{ID: riskID, OrganizationID: orgA}

	uc := NewDeleteRiskUseCase(repo)
	err := uc.Execute(context.Background(), orgB, riskID)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound for cross-tenant delete, got %v", err)
	}
}
