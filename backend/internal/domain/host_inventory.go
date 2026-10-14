// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// =============================================================================
// A host's software inventory (#910, spec docs/910_AGENT_PACKAGE_VULN_SCAN.md)
// =============================================================================
//
// The OpenRisk agent reads, with osquery, the packages installed on the host it
// runs on. The backend keeps the latest snapshot per host and checks those
// exact versions online: OSV for Linux distributions, NVD (CPE) for Windows
// programs. A host is attached to an asset when one matches; an inventory never
// creates an asset.

// Package sources, as osquery names its tables.
const (
	PackageSourceDeb      = "deb"
	PackageSourceRPM      = "rpm"
	PackageSourceAPK      = "apk"
	PackageSourceWindows  = "programs"
	maxPackageField       = 256
	MaxInventoryPackages  = 20000
	InventoryErrNoOsquery = "osquery_missing"
)

// HostInventory is the latest snapshot of one host's software, per tenant.
type HostInventory struct {
	ID       uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	TenantID uuid.UUID  `gorm:"type:uuid;not null;uniqueIndex:ux_host_inventory_host" json:"-"`
	AgentID  *uuid.UUID `gorm:"type:uuid" json:"agent_id"`
	// HostKey is the normalised hostname, the identity of the host for the
	// tenant (see HostKey).
	HostKey  string     `gorm:"size:255;not null;uniqueIndex:ux_host_inventory_host" json:"host_key"`
	Hostname string     `gorm:"size:255" json:"hostname"`
	AssetID  *uuid.UUID `gorm:"type:uuid;index" json:"asset_id"`
	Platform string     `gorm:"size:32" json:"platform"`
	OS       string     `gorm:"size:128" json:"os"`
	// OSVersion is osquery's os_version.version, e.g. "22.04.3 LTS (Jammy Jellyfish)".
	OSVersion string `gorm:"size:128" json:"os_version"`
	// Ecosystem is the OSV ecosystem of the host's distribution packages, empty
	// when OSV has none for it (Windows, RHEL, SUSE).
	Ecosystem        string     `gorm:"size:64" json:"ecosystem"`
	PackageCount     int        `json:"package_count"`
	UncheckableCount int        `json:"uncheckable_count"`
	CollectedAt      time.Time  `json:"collected_at"`
	MatchedAt        *time.Time `json:"matched_at"`
	// MatchError is a code the console words (osquery_missing, source_unreachable,
	// lookup_disabled), empty after a clean match.
	MatchError string    `gorm:"size:64" json:"match_error"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// TableName pins the table name.
func (HostInventory) TableName() string { return "host_inventories" }

// HostPackage is one installed package of an inventory.
type HostPackage struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey" json:"-"`
	TenantID    uuid.UUID `gorm:"type:uuid;not null;index" json:"-"`
	InventoryID uuid.UUID `gorm:"type:uuid;not null;index" json:"-"`
	Name        string    `gorm:"size:256;not null" json:"name"`
	Version     string    `gorm:"size:256" json:"version"`
	Source      string    `gorm:"size:16" json:"source"`
	Publisher   string    `gorm:"size:256" json:"publisher,omitempty"`
	Arch        string    `gorm:"size:32" json:"arch,omitempty"`
}

// TableName pins the table name.
func (HostPackage) TableName() string { return "host_packages" }

// HostPackageInput is a package as the agent reports it.
type HostPackageInput struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	Source    string `json:"source"`
	Publisher string `json:"publisher,omitempty"`
	Arch      string `json:"arch,omitempty"`
}

// HostKey normalises a hostname into the host's identity: lower case, first
// label only, so "SRV-Core-01.corp.local." and "srv-core-01" are one host.
func HostKey(hostname string) string {
	h := strings.ToLower(strings.TrimSpace(hostname))
	h = strings.TrimSuffix(h, ".")
	if i := strings.IndexByte(h, '.'); i > 0 {
		h = h[:i]
	}
	return h
}

// NormalisePackages trims, caps and de-duplicates what the agent sent. A
// package with no name is dropped; nothing else is guessed.
func NormalisePackages(in []HostPackageInput) []HostPackageInput {
	seen := make(map[string]bool, len(in))
	out := make([]HostPackageInput, 0, len(in))
	for _, p := range in {
		n := HostPackageInput{
			Name:      capField(p.Name),
			Version:   capField(p.Version),
			Source:    strings.ToLower(capField(p.Source)),
			Publisher: capField(p.Publisher),
			Arch:      capField(p.Arch),
		}
		if n.Name == "" {
			continue
		}
		k := n.Source + "\x00" + n.Name + "\x00" + n.Version + "\x00" + n.Arch
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, n)
	}
	return out
}

func capField(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > maxPackageField {
		s = s[:maxPackageField]
	}
	return s
}

var leadingVersion = regexp.MustCompile(`^(\d+)(?:\.(\d+))?`)

// OSVEcosystem maps a host's platform (osquery os_version.platform), its
// version string and a package source to the OSV ecosystem name, or "" when
// OSV has none: Windows programs (matched through NVD instead), RHEL (OSV
// wants the package's repository CPE, which osquery does not report) and SUSE
// (OSV wants the exact PRETTY_NAME). Formats per the OSV schema.
func OSVEcosystem(platform, version, source string) string {
	m := leadingVersion.FindStringSubmatch(strings.TrimSpace(version))
	if m == nil {
		return ""
	}
	major, minor := m[1], m[2]
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case "debian":
		if source == PackageSourceDeb {
			return "Debian:" + major
		}
	case "ubuntu":
		if source == PackageSourceDeb && minor != "" {
			eco := "Ubuntu:" + major + "." + minor
			if strings.Contains(strings.ToUpper(version), "LTS") {
				eco += ":LTS"
			}
			return eco
		}
	case "alpine":
		if source == PackageSourceAPK && minor != "" {
			return "Alpine:v" + major + "." + minor
		}
	case "rocky":
		if source == PackageSourceRPM {
			return "Rocky Linux:" + major
		}
	case "almalinux":
		if source == PackageSourceRPM {
			return "AlmaLinux:" + major
		}
	}
	return ""
}

// HostInventoryRepository stores inventories. Every method but TenantsWithInventories
// is scoped to tenantID.
type HostInventoryRepository interface {
	// Replace upserts the host's inventory on (tenant, host key) and replaces
	// its packages, in one transaction. inv.ID is set to the stored row's id.
	Replace(ctx context.Context, inv *HostInventory, pkgs []HostPackageInput) error
	Get(ctx context.Context, tenantID, id uuid.UUID) (*HostInventory, error)
	List(ctx context.Context, tenantID uuid.UUID) ([]HostInventory, error)
	ListByAsset(ctx context.Context, tenantID, assetID uuid.UUID) ([]HostInventory, error)
	Packages(ctx context.Context, tenantID, inventoryID uuid.UUID) ([]HostPackage, error)
	MarkMatched(ctx context.Context, tenantID, id uuid.UUID, at time.Time, errCode string, uncheckable int) error
	// TenantsWithInventories lists the tenants that have at least one
	// inventory, for the daily re-match. It reads no tenant data.
	TenantsWithInventories(ctx context.Context) ([]uuid.UUID, error)
}
