// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package governance

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// =============================================================================
// Chain verification
// =============================================================================

// VerifyAuditChainUseCase replays a tenant's audit trail and reports whether it
// has been altered — and if so, exactly where.
type VerifyAuditChainUseCase struct {
	repo domain.AuditChainRepository
}

func NewVerifyAuditChainUseCase(repo domain.AuditChainRepository) *VerifyAuditChainUseCase {
	return &VerifyAuditChainUseCase{repo: repo}
}

// Execute verifies the whole trail of a tenant. The verification is a pure
// function of the stored rows (domain.VerifyAuditChain), so the same check can
// be run later against an exported file and must give the same verdict.
func (uc *VerifyAuditChainUseCase) Execute(ctx context.Context, tenantID uuid.UUID) (*domain.AuditChainReport, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewValidationError("tenant is required")
	}
	events, err := uc.repo.ListAll(ctx, tenantID, domain.AuditEventFilter{})
	if err != nil {
		return nil, err
	}
	seals, err := uc.repo.ListSeals(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	rep := domain.VerifyAuditChain(tenantID, events, seals)
	return &rep, nil
}

// =============================================================================
// Signed export
// =============================================================================

// ExportFormat is the requested serialisation.
const (
	ExportFormatJSON = "json"
	ExportFormatCSV  = "csv"
)

// AuditExportSignature accompanies an export. It proves the file came from this
// deployment and has not been edited since. It is deliberately separate from the
// per-entry hashes: those already prove internal consistency without any key.
type AuditExportSignature struct {
	Algorithm string `json:"algorithm"`
	Value     string `json:"value,omitempty"`
	KeyID     string `json:"key_id,omitempty"`
	// Reason explains a missing signature instead of quietly shipping an
	// unsigned file that looks signed.
	Reason string `json:"reason,omitempty"`
}

// AuditExport is the JSON export envelope: the entries, the chain verdict at
// export time, and the signature over both.
type AuditExport struct {
	Kind         string                   `json:"kind"`
	Version      int                      `json:"version"`
	TenantID     uuid.UUID                `json:"tenant_id"`
	ExportedAt   time.Time                `json:"exported_at"`
	ExportedBy   string                   `json:"exported_by,omitempty"`
	Filter       map[string]interface{}   `json:"filter,omitempty"`
	Count        int                      `json:"count"`
	Verification *domain.AuditChainReport `json:"verification"`
	Events       []domain.AuditEvent      `json:"events"`
	Signature    *AuditExportSignature    `json:"signature"`
}

// ExportAuditTrailUseCase produces a signed, independently verifiable export.
type ExportAuditTrailUseCase struct {
	repo   domain.AuditChainRepository
	lookup UserLookup
	verify *VerifyAuditChainUseCase
}

func NewExportAuditTrailUseCase(repo domain.AuditChainRepository) *ExportAuditTrailUseCase {
	return &ExportAuditTrailUseCase{repo: repo, verify: NewVerifyAuditChainUseCase(repo)}
}

func (uc *ExportAuditTrailUseCase) WithUserLookup(l UserLookup) *ExportAuditTrailUseCase {
	uc.lookup = l
	return uc
}

// Execute gathers the filtered entries, verifies the full chain, and signs the
// result. The export always carries the verdict — an export of a tampered trail
// says so on its face rather than looking clean.
func (uc *ExportAuditTrailUseCase) Execute(ctx context.Context, tenantID uuid.UUID, actorEmail string, f domain.AuditEventFilter) (*AuditExport, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewValidationError("tenant is required")
	}
	events, err := uc.repo.ListAll(ctx, tenantID, f)
	if err != nil {
		return nil, err
	}
	uc.resolveEmails(ctx, events)

	report, err := uc.verify.Execute(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	exp := &AuditExport{
		Kind:         "openrisk.audit-trail",
		Version:      1,
		TenantID:     tenantID,
		ExportedAt:   time.Now().UTC(),
		ExportedBy:   actorEmail,
		Filter:       filterToMap(f),
		Count:        len(events),
		Verification: report,
		Events:       events,
	}
	exp.Signature = SignExport(exp)
	return exp, nil
}

func (uc *ExportAuditTrailUseCase) resolveEmails(ctx context.Context, events []domain.AuditEvent) {
	if uc.lookup == nil || len(events) == 0 {
		return
	}
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
	emails, err := uc.lookup.EmailsByIDs(ctx, ids)
	if err != nil {
		return
	}
	for i := range events {
		if events[i].ActorID != nil {
			events[i].ActorEmail = emails[*events[i].ActorID]
		}
	}
}

// exportSigningKey reads the deployment's export key. Kept in one place so the
// "unsigned" path is impossible to reach by accident.
func exportSigningKey() []byte {
	for _, k := range []string{"AUDIT_EXPORT_KEY", "AUDIT_SIGNING_KEY"} {
		if v := os.Getenv(k); len(v) >= 16 {
			return []byte(v)
		}
	}
	return nil
}

// SignExport computes the HMAC-SHA256 of the export body (everything except the
// signature field itself). Without a configured key it returns a signature block
// that states plainly why it is empty — never a blank that reads as signed.
func SignExport(exp *AuditExport) *AuditExportSignature {
	key := exportSigningKey()
	if len(key) == 0 {
		return &AuditExportSignature{
			Algorithm: "HMAC-SHA256",
			Reason:    "AUDIT_EXPORT_KEY is not configured on this deployment; the per-entry hash chain still proves the entries have not been altered",
		}
	}
	body := ExportSigningBody(exp)
	mac := hmac.New(sha256.New, key)
	mac.Write(body)
	sum := mac.Sum(nil)
	kid := sha256.Sum256(key)
	return &AuditExportSignature{
		Algorithm: "HMAC-SHA256",
		Value:     hex.EncodeToString(sum),
		KeyID:     hex.EncodeToString(kid[:4]),
	}
}

// ExportSigningBody is the exact byte sequence a signature covers: the export
// with its signature field cleared. Re-deriving it the same way is how a
// recipient re-checks the signature offline.
func ExportSigningBody(exp *AuditExport) []byte {
	clone := *exp
	clone.Signature = nil
	b, err := json.Marshal(clone)
	if err != nil {
		return nil
	}
	return b
}

// VerifyExportSignature re-checks a signed export against the deployment key.
func VerifyExportSignature(exp *AuditExport) bool {
	if exp == nil || exp.Signature == nil || exp.Signature.Value == "" {
		return false
	}
	key := exportSigningKey()
	if len(key) == 0 {
		return false
	}
	mac := hmac.New(sha256.New, key)
	mac.Write(ExportSigningBody(exp))
	want := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(want), []byte(exp.Signature.Value))
}

func filterToMap(f domain.AuditEventFilter) map[string]interface{} {
	m := map[string]interface{}{}
	if f.EntityType != "" {
		m["entity_type"] = f.EntityType
	}
	if f.EntityID != "" {
		m["entity_id"] = f.EntityID
	}
	if f.Action != "" {
		m["action"] = f.Action
	}
	if f.ActorID != nil {
		m["actor_id"] = f.ActorID.String()
	}
	if f.RequestID != "" {
		m["request_id"] = f.RequestID
	}
	if f.Source != "" {
		m["source"] = f.Source
	}
	if f.Search != "" {
		m["search"] = f.Search
	}
	if f.From != nil {
		m["from"] = f.From.UTC()
	}
	if f.To != nil {
		m["to"] = f.To.UTC()
	}
	if len(m) == 0 {
		return nil
	}
	return m
}

// =============================================================================
// Retention
// =============================================================================

// GetRetentionPolicyUseCase reads the tenant's retention window, materialising
// the "keep forever" default rather than returning nothing.
type GetRetentionPolicyUseCase struct {
	repo domain.AuditRetentionRepository
}

func NewGetRetentionPolicyUseCase(repo domain.AuditRetentionRepository) *GetRetentionPolicyUseCase {
	return &GetRetentionPolicyUseCase{repo: repo}
}

func (uc *GetRetentionPolicyUseCase) Execute(ctx context.Context, tenantID uuid.UUID) (*domain.AuditRetentionPolicy, error) {
	p, err := uc.repo.Get(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return &domain.AuditRetentionPolicy{TenantID: tenantID, RetentionDays: 0}, nil
	}
	return p, nil
}

// SetRetentionPolicyUseCase configures the retention window.
type SetRetentionPolicyUseCase struct {
	repo     domain.AuditRetentionRepository
	recorder *AuditRecorder
}

func NewSetRetentionPolicyUseCase(repo domain.AuditRetentionRepository) *SetRetentionPolicyUseCase {
	return &SetRetentionPolicyUseCase{repo: repo}
}
func (uc *SetRetentionPolicyUseCase) WithRecorder(r *AuditRecorder) *SetRetentionPolicyUseCase {
	uc.recorder = r
	return uc
}

func (uc *SetRetentionPolicyUseCase) Execute(ctx context.Context, tenantID, actorID uuid.UUID, days int) (*domain.AuditRetentionPolicy, error) {
	existing, err := uc.repo.Get(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	p := &domain.AuditRetentionPolicy{TenantID: tenantID, RetentionDays: days}
	if existing != nil {
		p.ID = existing.ID
		p.LastPrunedAt = existing.LastPrunedAt
	}
	if actorID != uuid.Nil {
		a := actorID
		p.UpdatedBy = &a
	}
	if err := p.Validate(); err != nil {
		return nil, err
	}
	if err := uc.repo.Upsert(ctx, p); err != nil {
		return nil, err
	}
	if uc.recorder != nil {
		actor := actorID
		before := domain.JSONMap{}
		if existing != nil {
			before["retention_days"] = existing.RetentionDays
		}
		uc.recorder.Record(ctx, domain.AuditEvent{
			TenantID:   tenantID,
			ActorID:    &actor,
			Action:     domain.AuditActionUpdate,
			EntityType: "audit_retention_policy",
			EntityID:   tenantID.String(),
			Summary:    "audit retention set to " + retentionLabel(days),
			Before:     before,
			After:      domain.JSONMap{"retention_days": days},
		})
	}
	return p, nil
}

func retentionLabel(days int) string {
	if days <= 0 {
		return "keep forever"
	}
	return itoa(days) + " days"
}

// PruneAuditTrailUseCase applies a tenant's retention window, sealing whatever
// it removes so the surviving chain stays verifiable across the gap.
type PruneAuditTrailUseCase struct {
	chain     domain.AuditChainRepository
	retention domain.AuditRetentionRepository
}

func NewPruneAuditTrailUseCase(chain domain.AuditChainRepository, retention domain.AuditRetentionRepository) *PruneAuditTrailUseCase {
	return &PruneAuditTrailUseCase{chain: chain, retention: retention}
}

// PruneResult reports what a sweep removed for one tenant.
type PruneResult struct {
	TenantID uuid.UUID              `json:"tenant_id"`
	Pruned   int64                  `json:"pruned"`
	Seal     *domain.AuditChainSeal `json:"seal,omitempty"`
}

// ExecuteForTenant prunes one tenant using its configured window. A tenant with
// no window (keep forever) is a no-op.
func (uc *PruneAuditTrailUseCase) ExecuteForTenant(ctx context.Context, tenantID uuid.UUID, now time.Time) (*PruneResult, error) {
	p, err := uc.retention.Get(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if p == nil || p.RetentionDays <= 0 {
		return &PruneResult{TenantID: tenantID}, nil
	}
	return uc.prune(ctx, *p, now)
}

// ExecuteAll runs the sweep across every tenant with a finite window.
func (uc *PruneAuditTrailUseCase) ExecuteAll(ctx context.Context, now time.Time) ([]PruneResult, error) {
	policies, err := uc.retention.ListWithRetention(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]PruneResult, 0, len(policies))
	for _, p := range policies {
		res, err := uc.prune(ctx, p, now)
		if err != nil {
			// One tenant's failure must not stop the sweep for the others.
			continue
		}
		out = append(out, *res)
	}
	return out, nil
}

func (uc *PruneAuditTrailUseCase) prune(ctx context.Context, p domain.AuditRetentionPolicy, now time.Time) (*PruneResult, error) {
	cutoff := now.UTC().AddDate(0, 0, -p.RetentionDays)
	seal, err := uc.chain.Prune(ctx, p.TenantID, cutoff)
	if err != nil {
		return nil, err
	}
	res := &PruneResult{TenantID: p.TenantID, Seal: seal}
	if seal != nil {
		res.Pruned = seal.PrunedCount
	}
	stamp := now.UTC()
	p.LastPrunedAt = &stamp
	_ = uc.retention.Upsert(ctx, &p)
	return res, nil
}
