// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1

package collectors

import (
	"context"
	"strings"
	"testing"

	ldap "github.com/go-ldap/ldap/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
	scanner "github.com/opendefender/openrisk/internal/scanner"
)

// fakeLDAP returns canned computer/user entries based on the search filter.
type fakeLDAP struct{}

func (fakeLDAP) Search(req *ldap.SearchRequest) (*ldap.SearchResult, error) {
	if strings.Contains(req.Filter, "objectClass=computer") {
		return &ldap.SearchResult{Entries: []*ldap.Entry{
			ldap.NewEntry("CN=SRV01,DC=corp", map[string][]string{
				"cn": {"SRV01"}, "dNSHostName": {"srv01.corp.local"},
				"operatingSystem": {"Windows Server 2008 R2 Standard"},
				"distinguishedName": {"CN=SRV01,DC=corp"}, "userAccountControl": {"4096"},
			}),
		}}, nil
	}
	return &ldap.SearchResult{Entries: []*ldap.Entry{
		ldap.NewEntry("CN=jdoe,DC=corp", map[string][]string{
			"cn": {"John Doe"}, "sAMAccountName": {"jdoe"}, "userPrincipalName": {"jdoe@corp.local"},
			"distinguishedName": {"CN=jdoe,DC=corp"}, "userAccountControl": {"66048"}, // NORMAL + DONT_EXPIRE_PASSWD
		}),
	}}, nil
}

func TestActiveDirectorySearch(t *testing.T) {
	assets := make(chan scanner.AssetDiscovery, 8)
	findings := make(chan scanner.FindingDiscovery, 8)
	errs := make(chan error, 8)

	searchAD(context.Background(), fakeLDAP{}, "DC=corp", assets, findings, errs)
	close(assets)
	close(findings)
	close(errs)

	var gotAssets []scanner.AssetDiscovery
	for a := range assets {
		gotAssets = append(gotAssets, a)
	}
	require.Len(t, gotAssets, 2)
	assert.Equal(t, domain.AssetTypeServer, gotAssets[0].Type, "Windows Server → Server")
	assert.Equal(t, "srv01.corp.local", *gotAssets[0].Hostname)
	assert.Equal(t, domain.AssetTypeIdentity, gotAssets[1].Type, "person → Identity")

	var gotFindings []scanner.FindingDiscovery
	for f := range findings {
		gotFindings = append(gotFindings, f)
	}
	require.Len(t, gotFindings, 2)
	assert.Equal(t, scanner.SeverityHigh, gotFindings[0].Severity, "EOL Server 2008")
	assert.Equal(t, scanner.SeverityLow, gotFindings[1].Severity, "never-expiring password")

	for e := range errs {
		t.Fatalf("unexpected error: %v", e)
	}
}
