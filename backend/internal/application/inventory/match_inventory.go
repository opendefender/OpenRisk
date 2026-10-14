// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package inventory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	vulnapp "github.com/opendefender/openrisk/internal/application/vulnerability"
	"github.com/opendefender/openrisk/internal/domain"
)

// PackageQuery is one exact package version, for OSV.
type PackageQuery struct {
	Ecosystem string
	Name      string
	Version   string
}

// Advisory is one known flaw affecting a looked-up version.
type Advisory struct {
	ID       string
	CVE      string // empty when the advisory has no CVE alias
	Summary  string
	Details  string
	CVSS     float64
	Severity string // critical|high|medium|low, or "" (the ingest derives it from CVSS)
	Fixed    string
}

// PackageLookup asks OSV about Linux packages. A failure is an error, never
// an empty answer.
type PackageLookup interface {
	Lookup(ctx context.Context, qs []PackageQuery) (map[PackageQuery][]Advisory, error)
}

// ProgramLookup asks NVD about one Windows program. resolved is false when no
// NVD product matches it strictly: the program is then not checkable.
type ProgramLookup interface {
	Lookup(ctx context.Context, publisher, name, version string) (advs []Advisory, resolved bool, err error)
}

// FindingWriter is the vulnerability ingest.
type FindingWriter interface {
	Execute(ctx context.Context, tenantID uuid.UUID, in vulnapp.IngestInput) (*vulnapp.IngestResult, error)
}

// FindingCloser closes a host's findings that the latest match no longer reports.
type FindingCloser interface {
	CloseAbsent(ctx context.Context, tenantID uuid.UUID, source domain.VulnSource, prefix string, keep []string, now time.Time) (int64, error)
}

// LookupPolicy says whether the tenant allows online lookups (D-071: on by
// default, a per-tenant switch). Nil policy: allowed.
type LookupPolicy interface {
	OnlineLookupEnabled(ctx context.Context, tenantID uuid.UUID) bool
}

// Match error codes the console words.
const (
	MatchErrUnreachable = "source_unreachable"
	MatchErrDisabled    = "lookup_disabled"
)

// Lookup methods, kept on each finding's raw_data so the console can say how
// sure the match is (an NVD CPE match is to be confirmed).
const (
	LookupOSV    = "osv"
	LookupNVDCPE = "nvd_cpe"
)

// rawKeys are the raw fields persisted on each finding's raw_data.
var rawKeys = []string{"package", "installed_version", "fixed_version", "ecosystem", "lookup", "advisory_id"}

var cveID = regexp.MustCompile(`^CVE-\d{4}-\d{4,}$`)

// MatchInventoryUseCase checks one stored inventory for known vulnerabilities,
// writes what it finds to the register and closes what is fixed.
type MatchInventoryUseCase struct {
	repo     domain.HostInventoryRepository
	osv      PackageLookup
	nvd      ProgramLookup
	findings FindingWriter
	closer   FindingCloser
	policy   LookupPolicy
	now      func() time.Time
}

// NewMatchInventoryUseCase builds the use case. nvd may be nil (Windows
// programs are then not checkable).
func NewMatchInventoryUseCase(r domain.HostInventoryRepository, osv PackageLookup, nvd ProgramLookup, w FindingWriter, c FindingCloser) *MatchInventoryUseCase {
	return &MatchInventoryUseCase{repo: r, osv: osv, nvd: nvd, findings: w, closer: c, now: time.Now}
}

// WithPolicy wires the per-tenant online-lookup switch.
func (uc *MatchInventoryUseCase) WithPolicy(p LookupPolicy) *MatchInventoryUseCase {
	uc.policy = p
	return uc
}

// MatchResult says what a match did.
type MatchResult struct {
	Findings    int `json:"findings"`
	Closed      int `json:"closed"`
	Uncheckable int `json:"uncheckable"`
}

// MatchInventory implements the Matcher the store use case calls.
func (uc *MatchInventoryUseCase) MatchInventory(ctx context.Context, tenantID, inventoryID uuid.UUID) error {
	_, err := uc.Execute(ctx, tenantID, inventoryID)
	return err
}

// Execute matches the inventory. A lookup failure records source_unreachable
// and closes nothing: a failed lookup is not a fixed host.
func (uc *MatchInventoryUseCase) Execute(ctx context.Context, tenantID, inventoryID uuid.UUID) (*MatchResult, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewUnauthorizedError("no tenant")
	}
	inv, err := uc.repo.Get(ctx, tenantID, inventoryID)
	if err != nil {
		return nil, domain.NewInternalError("could not read the inventory: " + err.Error())
	}
	if inv == nil {
		return nil, domain.NewNotFoundError("host inventory", inventoryID)
	}
	if uc.policy != nil && !uc.policy.OnlineLookupEnabled(ctx, tenantID) {
		_ = uc.repo.MarkMatched(ctx, tenantID, inv.ID, uc.now(), MatchErrDisabled, inv.PackageCount)
		return &MatchResult{Uncheckable: inv.PackageCount}, nil
	}
	pkgs, err := uc.repo.Packages(ctx, tenantID, inv.ID)
	if err != nil {
		return nil, domain.NewInternalError("could not read the packages: " + err.Error())
	}

	res := &MatchResult{}
	var raws []map[string]any
	keep := map[string]bool{}
	add := func(p domain.HostPackage, eco string, a Advisory, lookup string) {
		key := a.CVE
		if key == "" {
			key = a.ID
		}
		ext := FindingID(inv.HostKey, eco, p.Name, key)
		if keep[ext] {
			return // two advisories for the same CVE on the same package
		}
		keep[ext] = true
		title := a.Summary
		if title == "" && a.CVE == "" {
			title = a.ID // the ingest titles CVE-only findings with the CVE
		}
		raws = append(raws, map[string]any{
			"cve": a.CVE, "title": title, "description": a.Details, "cvss": a.CVSS,
			"severity": a.Severity, "external_id": ext,
			// The correlator attaches the finding through the hostname, like any
			// scanner finding; DefaultAssetID below pins it when known.
			"hostname": inv.Hostname, "asset": inv.Hostname,
			"package": p.Name, "installed_version": p.Version, "fixed_version": a.Fixed,
			"ecosystem": eco, "lookup": lookup, "advisory_id": a.ID,
		})
	}

	// Linux packages through OSV.
	var queries []PackageQuery
	byQuery := map[PackageQuery][]domain.HostPackage{}
	for _, p := range pkgs {
		if p.Source == domain.PackageSourceWindows {
			continue
		}
		eco := domain.OSVEcosystem(inv.Platform, inv.OSVersion, p.Source)
		if eco == "" || p.Version == "" {
			res.Uncheckable++
			continue
		}
		q := PackageQuery{Ecosystem: eco, Name: p.Name, Version: p.Version}
		if _, seen := byQuery[q]; !seen {
			queries = append(queries, q)
		}
		byQuery[q] = append(byQuery[q], p)
	}
	if len(queries) > 0 {
		found, err := uc.osv.Lookup(ctx, queries)
		if err != nil {
			return uc.unreachable(ctx, tenantID, inv, err)
		}
		for q, advs := range found {
			for _, p := range byQuery[q] {
				for _, a := range advs {
					add(p, q.Ecosystem, a, LookupOSV)
				}
			}
		}
	}

	// Windows programs through NVD. One program NVD refuses is not the host's
	// failure: it counts as not checkable and the others are still checked.
	// But then nothing is closed, since a refused lookup is not a fix.
	lookupFailed := false
	asked := 0
	for _, p := range pkgs {
		if p.Source != domain.PackageSourceWindows {
			continue
		}
		if uc.nvd == nil || p.Version == "" || asked >= MaxProgramLookups {
			res.Uncheckable++
			continue
		}
		asked++
		advs, resolved, err := uc.nvd.Lookup(ctx, p.Publisher, p.Name, p.Version)
		if err != nil {
			if ctx.Err() != nil {
				return uc.unreachable(ctx, tenantID, inv, err)
			}
			lookupFailed = true
			res.Uncheckable++
			continue
		}
		if !resolved {
			res.Uncheckable++
			continue
		}
		for _, a := range advs {
			add(p, "windows", a, LookupNVDCPE)
		}
	}

	if len(raws) > 0 {
		out, err := uc.findings.Execute(ctx, tenantID, vulnapp.IngestInput{
			Source: domain.VulnSourceAgent, FromAgent: true, Findings: raws,
			DefaultAssetID: inv.AssetID, KeepRawKeys: rawKeys, ReopenRemediated: true,
		})
		if err != nil {
			return nil, err
		}
		res.Findings = out.Created + out.Updated
	}
	now := uc.now()
	code := ""
	if lookupFailed {
		code = MatchErrUnreachable
	} else {
		keepList := make([]string, 0, len(keep))
		for k := range keep {
			keepList = append(keepList, k)
		}
		closed, err := uc.closer.CloseAbsent(ctx, tenantID, domain.VulnSourceAgent, inv.HostKey+"|", keepList, now)
		if err != nil {
			return nil, domain.NewInternalError("could not close fixed findings: " + err.Error())
		}
		res.Closed = int(closed)
	}
	if err := uc.repo.MarkMatched(ctx, tenantID, inv.ID, now, code, res.Uncheckable); err != nil {
		return nil, domain.NewInternalError("could not record the match: " + err.Error())
	}
	return res, nil
}

func (uc *MatchInventoryUseCase) unreachable(ctx context.Context, tenantID uuid.UUID, inv *domain.HostInventory, cause error) (*MatchResult, error) {
	_ = uc.repo.MarkMatched(ctx, tenantID, inv.ID, uc.now(), MatchErrUnreachable, inv.UncheckableCount)
	return nil, domain.NewInternalError("vulnerability lookup failed: " + cause.Error())
}

// MaxProgramLookups caps the NVD lookups of one match: NVD allows 5 requests
// per 30 s without a key, so a host with thousands of programs would hold the
// shared budget for hours (security review of #910). The rest count as not
// checkable and are tried again on the next match, cache first.
const MaxProgramLookups = 300

// FindingID is a finding's external id: the host key, then a fixed-length
// hash of (ecosystem, package, CVE or advisory), so it always fits the 255
// column whatever the package name, and the host prefix stays exact.
func FindingID(hostKey, ecosystem, name, key string) string {
	sum := sha256.Sum256([]byte(ecosystem + "\x00" + name + "\x00" + key))
	return hostKey + "|" + hex.EncodeToString(sum[:16])
}

// FirstCVE picks the CVE among an advisory's id and aliases, or "".
func FirstCVE(id string, aliases []string) string {
	if cveID.MatchString(strings.ToUpper(id)) {
		return strings.ToUpper(id)
	}
	for _, a := range aliases {
		if cveID.MatchString(strings.ToUpper(a)) {
			return strings.ToUpper(a)
		}
	}
	return ""
}
