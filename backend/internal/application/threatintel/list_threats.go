// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package threatintel

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/pkg/cti"
)

// maxThreats bounds the list. The response says when it was cut.
const maxThreats = 200

// ThreatAsset is one asset a threat touches, and how.
type ThreatAsset struct {
	ID          *uuid.UUID `json:"id"` // nil for a scanned host not matched to an asset
	Name        string     `json:"name"`
	Criticality string     `json:"criticality"`
	Via         []string   `json:"via"` // scan, cpe
}

// Threat is one catalogue entry that touches the tenant's assets.
type Threat struct {
	CVEID       string                `json:"cve_id"`
	Title       string                `json:"title"` // the scanner's title when one found it, else empty
	Description string                `json:"description"`
	Severity    string                `json:"severity"`
	CVSS        float64               `json:"cvss"`
	EPSS        float64               `json:"epss"`
	KEV         bool                  `json:"kev"`
	KEVDueDate  *time.Time            `json:"kev_due_date,omitempty"`
	PublishedAt *time.Time            `json:"published_at"`
	Techniques  []string              `json:"techniques"`
	Tactics     []string              `json:"tactics"`
	Assets      []ThreatAsset         `json:"assets"`
	Relevance   int                   `json:"relevance"`
	Factors     []domain.ThreatFactor `json:"factors"`
	// Sources names where each fact came from: nvd, cisa_kev, mitre_attack,
	// and the scanners (nessus, qualys, …) that reported it.
	Sources     []string   `json:"sources"`
	DismissedAt *time.Time `json:"dismissed_at"`
}

// ThreatList is the response of GET /cti/threats.
type ThreatList struct {
	Threats []Threat `json:"threats"`
	// AssetsWithCPEs says how many assets could be matched by CPE at all, so an
	// empty list can tell "nothing touches you" from "nothing to match on".
	AssetsWithCPEs int  `json:"assets_with_cpes"`
	Truncated      bool `json:"truncated"`
}

// ListThreatsUseCase builds the tenant's threat list.
type ListThreatsUseCase struct {
	tenant    TenantSide
	catalogue Catalogue
}

// NewListThreatsUseCase builds the use case.
func NewListThreatsUseCase(t TenantSide, c Catalogue) *ListThreatsUseCase {
	return &ListThreatsUseCase{tenant: t, catalogue: c}
}

type threatAcc struct {
	vuln     cti.CTIVulnerability
	title    string
	epss     float64
	scanned  bool
	assets   map[string]*ThreatAsset // keyed by asset id, or by name when unmatched
	order    []string
	scanners map[string]bool
}

func (a *threatAcc) touch(key string, id *uuid.UUID, name string, crit domain.AssetCriticality, via string) {
	ta, ok := a.assets[key]
	if !ok {
		c := strings.ToUpper(string(crit))
		if c == "" {
			c = string(domain.CriticalityMedium)
		}
		ta = &ThreatAsset{ID: id, Name: name, Criticality: c}
		a.assets[key] = ta
		a.order = append(a.order, key)
	}
	for _, v := range ta.Via {
		if v == via {
			return
		}
	}
	ta.Via = append(ta.Via, via)
}

// Execute lists the threats to the tenant, most relevant first.
func (uc *ListThreatsUseCase) Execute(ctx context.Context, tenantID uuid.UUID) (*ThreatList, error) {
	assets, err := uc.tenant.AssetsWithCPEs(ctx, tenantID)
	if err != nil {
		return nil, domain.NewInternalError("could not read the assets' CPEs: " + err.Error())
	}
	findings, err := uc.tenant.FindingsWithCVE(ctx, tenantID)
	if err != nil {
		return nil, domain.NewInternalError("could not read the findings: " + err.Error())
	}
	dismissed, err := uc.tenant.Dismissals(ctx, tenantID)
	if err != nil {
		return nil, domain.NewInternalError("could not read the dismissals: " + err.Error())
	}

	acc := map[string]*threatAcc{}
	get := func(v cti.CTIVulnerability) *threatAcc {
		a, ok := acc[v.CVEID]
		if !ok {
			a = &threatAcc{vuln: v, assets: map[string]*ThreatAsset{}, scanners: map[string]bool{}}
			acc[v.CVEID] = a
		}
		return a
	}

	// CPE matches: the same rule as the CTI matcher, an exact intersection of
	// the asset's CPEs with the CVE's affected CPEs.
	var cpes []string
	for _, a := range assets {
		cpes = append(cpes, a.CPEs...)
	}
	if len(cpes) > 0 {
		vulns, err := uc.catalogue.ByCPEs(ctx, uniq(cpes))
		if err != nil {
			return nil, domain.NewInternalError("could not match CPEs: " + err.Error())
		}
		for _, v := range vulns {
			affected := map[string]bool{}
			for _, c := range v.AffectedCPE {
				affected[c] = true
			}
			for _, a := range assets {
				if intersects(a.CPEs, affected) {
					id := a.ID
					get(v).touch(id.String(), &id, a.Name, a.Criticality, domain.ThreatViaCPE)
				}
			}
		}
	}

	// Scanner findings: the CVE was seen on the tenant's hosts.
	if len(findings) > 0 {
		ids := make([]string, 0, len(findings))
		for _, f := range findings {
			ids = append(ids, strings.ToUpper(f.CVEID))
		}
		vulns, err := uc.catalogue.ByCVEs(ctx, uniq(ids))
		if err != nil {
			return nil, domain.NewInternalError("could not read the catalogue: " + err.Error())
		}
		byID := make(map[string]cti.CTIVulnerability, len(vulns))
		for _, v := range vulns {
			byID[strings.ToUpper(v.CVEID)] = v
		}
		for _, f := range findings {
			v, ok := byID[strings.ToUpper(f.CVEID)]
			if !ok {
				// Not in the catalogue: the vulnerability queue already lists
				// it, and there is no intelligence to add here.
				continue
			}
			a := get(v)
			key := "name:" + f.AssetName
			if f.AssetID != nil {
				key = f.AssetID.String()
			}
			a.touch(key, f.AssetID, f.AssetName, f.Criticality, domain.ThreatViaScan)
			a.scanned = true
			if f.EPSS > a.epss {
				a.epss = f.EPSS
			}
			if a.title == "" {
				a.title = f.Title
			}
			if f.Source != "" {
				a.scanners[string(f.Source)] = true
			}
		}
	}

	out := make([]Threat, 0, len(acc))
	for _, a := range acc {
		out = append(out, a.threat(dismissed))
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Relevance != out[j].Relevance {
			return out[i].Relevance > out[j].Relevance
		}
		return out[i].CVEID > out[j].CVEID
	})
	res := &ThreatList{Threats: out, AssetsWithCPEs: len(assets)}
	if len(out) > maxThreats {
		res.Threats, res.Truncated = out[:maxThreats], true
	}
	return res, nil
}

func (a *threatAcc) threat(dismissed map[string]time.Time) Threat {
	v := a.vuln
	assets := make([]ThreatAsset, 0, len(a.order))
	crits := make([]domain.AssetCriticality, 0, len(a.order))
	for _, k := range a.order {
		ta := *a.assets[k]
		assets = append(assets, ta)
		crits = append(crits, domain.AssetCriticality(ta.Criticality))
	}
	// Most critical asset first.
	sort.SliceStable(assets, func(i, j int) bool {
		return critRank(assets[i].Criticality) > critRank(assets[j].Criticality)
	})
	rel, factors := domain.ThreatRelevance(domain.ThreatRelevanceInput{
		Criticalities: crits, KEV: v.CISAKnown, EPSS: a.epss, CVSS: v.CVSSV3, Scanned: a.scanned,
	})

	var sources []string
	if v.CVSSV3 > 0 || !v.PublishedAt.IsZero() {
		sources = append(sources, "nvd")
	}
	if v.CISAKnown {
		sources = append(sources, "cisa_kev")
	}
	if len(v.MitreTechniques) > 0 {
		sources = append(sources, "mitre_attack")
	}
	scanners := make([]string, 0, len(a.scanners))
	for s := range a.scanners {
		scanners = append(scanners, s)
	}
	sort.Strings(scanners)
	sources = append(sources, scanners...)

	t := Threat{
		CVEID:       v.CVEID,
		Title:       a.title,
		Description: v.Description,
		Severity:    strings.ToLower(v.Severity),
		CVSS:        v.CVSSV3,
		EPSS:        a.epss,
		KEV:         v.CISAKnown,
		KEVDueDate:  v.CISADueDate,
		Techniques:  nonNil(v.MitreTechniques),
		Tactics:     nonNil(v.MitreTactics),
		Assets:      assets,
		Relevance:   rel,
		Factors:     factors,
		Sources:     sources,
	}
	if !v.PublishedAt.IsZero() {
		p := v.PublishedAt
		t.PublishedAt = &p
	}
	if at, ok := dismissed[strings.ToUpper(v.CVEID)]; ok {
		t.DismissedAt = &at
	}
	return t
}

func critRank(c string) int {
	switch c {
	case "CRITICAL":
		return 4
	case "HIGH":
		return 3
	case "MEDIUM":
		return 2
	}
	return 1
}

func intersects(cpes []string, set map[string]bool) bool {
	for _, c := range cpes {
		if set[c] {
			return true
		}
	}
	return false
}

func uniq(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
