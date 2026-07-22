// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial
// This file is part of the OpenRisk Enterprise Edition and is NOT covered by the
// AGPL; it is licensed under the OpenRisk Commercial License (see LICENSE.commercial).

package collectors

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	ldap "github.com/go-ldap/ldap/v3"

	"github.com/opendefender/openrisk/internal/domain"
	scanner "github.com/opendefender/openrisk/internal/scanner"
)

// userAccountControl flags we care about (Active Directory).
const (
	uacAccountDisable    = 0x0002
	uacDontExpirePasswd  = 0x10000
	adPageSize           = 1000
)

// ldapSearcher is the minimal surface of an LDAP connection the AD collector
// needs, so the search/normalisation logic is unit-testable without a live
// directory. *ldap.Conn (wrapped for paging) satisfies it.
type ldapSearcher interface {
	Search(*ldap.SearchRequest) (*ldap.SearchResult, error)
}

// pagingConn adapts *ldap.Conn to ldapSearcher using server-side paging (AD caps
// a single Search at ~1000 rows).
type pagingConn struct{ *ldap.Conn }

func (p pagingConn) Search(req *ldap.SearchRequest) (*ldap.SearchResult, error) {
	return p.Conn.SearchWithPaging(req, adPageSize)
}

// ActiveDirectory is a real go-ldap CloudCollector. It binds to a domain
// controller and enumerates computer objects (as Server/Workstation assets) and
// person objects (as Identity assets), raising hygiene findings for
// end-of-life operating systems and non-expiring passwords.
type ActiveDirectory struct{}

// NewActiveDirectory returns the AD collector.
func NewActiveDirectory() scanner.CloudCollector { return ActiveDirectory{} }

func (ActiveDirectory) Collect(ctx context.Context, cfg scanner.ScanConfig, assets chan<- scanner.AssetDiscovery, findings chan<- scanner.FindingDiscovery, errs chan<- error) {
	conn, err := ldap.DialURL(cfg.Credentials["url"])
	if err != nil {
		errs <- fmt.Errorf("active_directory: dial: %w", err)
		return
	}
	defer conn.Close()
	if err := conn.Bind(cfg.Credentials["bind_dn"], cfg.Credentials["password"]); err != nil {
		errs <- fmt.Errorf("active_directory: bind: %w", err)
		return
	}
	searchAD(ctx, pagingConn{conn}, cfg.Credentials["base_dn"], assets, findings, errs)
}

// searchAD performs the computer + user enumeration against any ldapSearcher.
func searchAD(ctx context.Context, s ldapSearcher, baseDN string, assets chan<- scanner.AssetDiscovery, findings chan<- scanner.FindingDiscovery, errs chan<- error) {
	// Computers.
	compReq := ldap.NewSearchRequest(baseDN, ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		"(objectClass=computer)",
		[]string{"cn", "dNSHostName", "operatingSystem", "operatingSystemVersion", "distinguishedName", "userAccountControl"}, nil)
	if res, err := s.Search(compReq); err != nil {
		errs <- fmt.Errorf("active_directory: search computers: %w", err)
	} else {
		for _, e := range res.Entries {
			if ctx.Err() != nil {
				errs <- ctx.Err()
				return
			}
			emitADComputer(e, assets, findings)
		}
	}

	// Users (persons).
	userReq := ldap.NewSearchRequest(baseDN, ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		"(&(objectClass=user)(objectCategory=person))",
		[]string{"sAMAccountName", "cn", "userPrincipalName", "distinguishedName", "userAccountControl"}, nil)
	if res, err := s.Search(userReq); err != nil {
		errs <- fmt.Errorf("active_directory: search users: %w", err)
	} else {
		for _, e := range res.Entries {
			if ctx.Err() != nil {
				errs <- ctx.Err()
				return
			}
			emitADUser(e, assets, findings)
		}
	}
}

func emitADComputer(e *ldap.Entry, assets chan<- scanner.AssetDiscovery, findings chan<- scanner.FindingDiscovery) {
	dn := e.GetAttributeValue("distinguishedName")
	if dn == "" {
		dn = e.DN
	}
	os := e.GetAttributeValue("operatingSystem")
	name := e.GetAttributeValue("cn")
	host := e.GetAttributeValue("dNSHostName")

	typ := domain.AssetTypeWorkstation
	if strings.Contains(strings.ToLower(os), "server") {
		typ = domain.AssetTypeServer
	}
	a := scanner.AssetDiscovery{
		ExternalID:  dn,
		Name:        name,
		Type:        typ,
		CPE:         adOSCPE(os),
		Tags:        []string{"active_directory", "domain-joined"},
		RawMetadata: map[string]any{"operating_system": os, "os_version": e.GetAttributeValue("operatingSystemVersion")},
	}
	if host != "" {
		a.Hostname = ptr(host)
	}
	if os != "" {
		a.OS = ptr(os)
		if v := e.GetAttributeValue("operatingSystemVersion"); v != "" {
			a.OSVersion = ptr(v)
		}
	}
	assets <- a

	if eol, label := eolWindows(os); eol {
		findings <- scanner.FindingDiscovery{
			Title:           "End-of-life operating system",
			Description:     fmt.Sprintf("Host %q runs %s, which is past its end of support and no longer receives security updates.", name, label),
			Severity:        scanner.SeverityHigh,
			Evidence:        "operatingSystem=" + os,
			RemediationHint: "Upgrade or decommission this host; isolate it from the network until then.",
			Source:          "active_directory",
			AssetExternalID: dn,
			AffectedCPE:     adOSCPE(os),
		}
	}
}

func emitADUser(e *ldap.Entry, assets chan<- scanner.AssetDiscovery, findings chan<- scanner.FindingDiscovery) {
	dn := e.GetAttributeValue("distinguishedName")
	if dn == "" {
		dn = e.DN
	}
	sam := e.GetAttributeValue("sAMAccountName")
	name := e.GetAttributeValue("cn")
	if name == "" {
		name = sam
	}
	uac, _ := strconv.Atoi(e.GetAttributeValue("userAccountControl"))
	tags := []string{"active_directory", "identity"}
	if uac&uacAccountDisable != 0 {
		tags = append(tags, "disabled")
	}
	assets <- scanner.AssetDiscovery{
		ExternalID:  dn,
		Name:        name,
		Type:        domain.AssetTypeIdentity,
		Tags:        tags,
		RawMetadata: map[string]any{"sam_account_name": sam, "upn": e.GetAttributeValue("userPrincipalName"), "uac": uac},
	}
	// Hygiene: enabled account whose password never expires.
	if uac&uacAccountDisable == 0 && uac&uacDontExpirePasswd != 0 {
		findings <- scanner.FindingDiscovery{
			Title:           "Account password set to never expire",
			Description:     fmt.Sprintf("Account %q (%s) has DONT_EXPIRE_PASSWORD set.", name, sam),
			Severity:        scanner.SeverityLow,
			Evidence:        "userAccountControl includes DONT_EXPIRE_PASSWD (0x10000)",
			RemediationHint: "Remove the never-expire flag or enforce a rotation policy for this account.",
			Source:          "active_directory",
			AssetExternalID: dn,
		}
	}
}

// eolWindows reports whether an operatingSystem string names a Windows release
// past end of support, with a human label.
func eolWindows(os string) (bool, string) {
	l := strings.ToLower(os)
	for _, m := range []struct{ needle, label string }{
		{"windows xp", "Windows XP"},
		{"windows vista", "Windows Vista"},
		{"windows 7", "Windows 7"},
		{"windows 8", "Windows 8"},
		{"server 2003", "Windows Server 2003"},
		{"server 2008", "Windows Server 2008"},
		{"server 2012", "Windows Server 2012"},
	} {
		if strings.Contains(l, m.needle) {
			return true, m.label
		}
	}
	return false, ""
}

func adOSCPE(os string) []string {
	l := strings.ToLower(os)
	switch {
	case strings.Contains(l, "server"):
		return []string{"cpe:2.3:o:microsoft:windows_server"}
	case strings.Contains(l, "windows"):
		return []string{"cpe:2.3:o:microsoft:windows"}
	default:
		return nil
	}
}
