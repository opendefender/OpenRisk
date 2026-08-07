// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

package reportjob

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

/* ---------------- doubles ---------------- */

type memRepo struct {
	jobs      map[uuid.UUID]*domain.ReportJob
	createErr error
}

func newMemRepo() *memRepo { return &memRepo{jobs: map[uuid.UUID]*domain.ReportJob{}} }

func (m *memRepo) Create(_ context.Context, j *domain.ReportJob) error {
	if m.createErr != nil {
		return m.createErr
	}
	if j.ID == uuid.Nil {
		j.ID = uuid.New()
	}
	cp := *j
	m.jobs[j.ID] = &cp
	return nil
}

func (m *memRepo) GetByID(_ context.Context, tenantID, id uuid.UUID) (*domain.ReportJob, error) {
	j, ok := m.jobs[id]
	if !ok || j.TenantID != tenantID {
		return nil, domain.NewNotFoundError("report job", id.String())
	}
	cp := *j
	return &cp, nil
}

func (m *memRepo) List(_ context.Context, tenantID uuid.UUID, _ int) ([]domain.ReportJob, error) {
	var out []domain.ReportJob
	for _, j := range m.jobs {
		if j.TenantID == tenantID {
			out = append(out, *j)
		}
	}
	return out, nil
}

func (m *memRepo) Update(_ context.Context, j *domain.ReportJob) error {
	cur, ok := m.jobs[j.ID]
	if !ok || cur.TenantID != j.TenantID {
		return domain.NewNotFoundError("report job", j.ID.String())
	}
	cp := *j
	m.jobs[j.ID] = &cp
	return nil
}

type fakeGen struct {
	kind    domain.ReportKind
	res     GeneratedReport
	err     error
	gotArgs map[string]any
}

func (f *fakeGen) Kind() domain.ReportKind { return f.kind }
func (f *fakeGen) Generate(_ context.Context, _, _ uuid.UUID, params map[string]any) (GeneratedReport, error) {
	f.gotArgs = params
	return f.res, f.err
}

func okGen() *fakeGen {
	return &fakeGen{
		kind: domain.ReportKindComplianceFramework,
		res: GeneratedReport{
			Title: "ISO/IEC 27001 2022", Filename: "r.pdf",
			ContentType: "application/pdf", Bytes: []byte("%PDF-1.4 fake"),
		},
	}
}

/* ---------------- tests ---------------- */

func TestCreate_Success(t *testing.T) {
	repo, gen := newMemRepo(), okGen()
	uc := New(repo, gen)
	tenant, user := uuid.New(), uuid.New()

	job, err := uc.Create(context.Background(), tenant, user, CreateInput{
		Kind:   domain.ReportKindComplianceFramework,
		Params: map[string]any{"framework_id": "abc", "locale": "en"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if job.Status != domain.ReportJobSucceeded {
		t.Fatalf("status = %q, want succeeded", job.Status)
	}
	if job.SizeBytes != len(gen.res.Bytes) {
		t.Errorf("size = %d, want %d", job.SizeBytes, len(gen.res.Bytes))
	}
	if job.Title != gen.res.Title || job.Filename != gen.res.Filename {
		t.Errorf("title/filename not carried onto the job: %q / %q", job.Title, job.Filename)
	}
	if job.CompletedAt == nil {
		t.Error("CompletedAt not set on a finished job")
	}
	if job.TenantID != tenant {
		t.Errorf("tenant = %v, want %v", job.TenantID, tenant)
	}
	if gen.gotArgs["locale"] != "en" {
		t.Errorf("params not forwarded to the generator: %v", gen.gotArgs)
	}

	// The artifact must be readable back, so a re-download serves the document
	// that was generated rather than a fresh render.
	stored, err := uc.Get(context.Background(), tenant, job.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(stored.Artifact) != string(gen.res.Bytes) {
		t.Error("artifact was not persisted on the job")
	}
}

// A generation failure must produce a readable FAILED job, not an error. The
// user needs somewhere to land that explains what happened; returning an error
// would leave them on the screen they were bouncing between.
func TestCreate_GeneratorFailure_YieldsFailedJobNotError(t *testing.T) {
	repo := newMemRepo()
	gen := okGen()
	gen.err = domain.NewNotFoundError("framework", "nope")
	uc := New(repo, gen)
	tenant := uuid.New()

	job, err := uc.Create(context.Background(), tenant, uuid.New(), CreateInput{
		Kind:   domain.ReportKindComplianceFramework,
		Params: map[string]any{"framework_id": "nope"},
	})
	if err != nil {
		t.Fatalf("expected a failed job, got error: %v", err)
	}
	if job.Status != domain.ReportJobFailed {
		t.Fatalf("status = %q, want failed", job.Status)
	}
	if job.Error == "" {
		t.Error("failed job carries no reason for the user")
	}
}

// A non-domain error must not reach the client verbatim.
func TestCreate_OpaqueGeneratorError_IsNotEchoed(t *testing.T) {
	gen := okGen()
	gen.err = errors.New("pq: connection refused on 10.0.0.4:5432")
	uc := New(newMemRepo(), gen)

	job, err := uc.Create(context.Background(), uuid.New(), uuid.New(), CreateInput{
		Kind: domain.ReportKindComplianceFramework,
	})
	if err != nil {
		t.Fatalf("expected a failed job, got error: %v", err)
	}
	if job.Error != "report generation failed" {
		t.Errorf("internal error leaked to the client: %q", job.Error)
	}
}

func TestCreate_UnknownKind(t *testing.T) {
	uc := New(newMemRepo(), okGen())
	_, err := uc.Create(context.Background(), uuid.New(), uuid.New(), CreateInput{Kind: "astrology"})
	if err == nil {
		t.Fatal("expected a validation error for an unknown kind")
	}
}

func TestCreate_NoGeneratorConfigured(t *testing.T) {
	uc := New(newMemRepo()) // no generators
	_, err := uc.Create(context.Background(), uuid.New(), uuid.New(), CreateInput{
		Kind: domain.ReportKindComplianceFramework,
	})
	if err == nil {
		t.Fatal("expected a validation error when no generator is registered")
	}
}

func TestCreate_NilTenantRejected(t *testing.T) {
	uc := New(newMemRepo(), okGen())
	_, err := uc.Create(context.Background(), uuid.Nil, uuid.New(), CreateInput{
		Kind: domain.ReportKindComplianceFramework,
	})
	if err == nil {
		t.Fatal("expected a nil tenant to be refused (RULE #2)")
	}
}

// RULE #2: another tenant's job id must read as absent.
func TestGet_CrossTenant_IsNotFound(t *testing.T) {
	repo, gen := newMemRepo(), okGen()
	uc := New(repo, gen)
	owner := uuid.New()

	job, err := uc.Create(context.Background(), owner, uuid.New(), CreateInput{
		Kind: domain.ReportKindComplianceFramework,
	})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	if _, err := uc.Get(context.Background(), uuid.New(), job.ID); err == nil {
		t.Fatal("a job was readable from another tenant")
	}
}

func TestList_ScopedToTenant(t *testing.T) {
	repo, gen := newMemRepo(), okGen()
	uc := New(repo, gen)
	a, b := uuid.New(), uuid.New()

	for _, tenant := range []uuid.UUID{a, a, b} {
		if _, err := uc.Create(context.Background(), tenant, uuid.New(), CreateInput{
			Kind: domain.ReportKindComplianceFramework,
		}); err != nil {
			t.Fatalf("setup: %v", err)
		}
	}

	got, err := uc.List(context.Background(), a, 25)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2 (tenant b's job leaked)", len(got))
	}
}

func TestReportFilename(t *testing.T) {
	for _, tc := range []struct{ name, version, want string }{
		{"ISO/IEC 27001", "2022", "compliance-report-iso-iec-27001-2022.pdf"},
		{"", "", "compliance-report.pdf"},
		{"NIST CSF", "", "compliance-report-nist-csf.pdf"},
	} {
		if got := ReportFilename(tc.name, tc.version); got != tc.want {
			t.Errorf("ReportFilename(%q,%q) = %q, want %q", tc.name, tc.version, got, tc.want)
		}
	}
}
