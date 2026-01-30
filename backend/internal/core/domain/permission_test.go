package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePermission_ValidFormat(t testing.T) {
	tests := []struct {
		input    string
		expected Permission
	}{
		{
			input: "risk:read:own",
			expected: Permission{
				Resource: PermissionResourceRisk,
				Action:   PermissionRead,
				Scope:    PermissionScopeOwn,
			},
		},
		{
			input: "mitigation:create:any",
			expected: Permission{
				Resource: PermissionResourceMitigation,
				Action:   PermissionCreate,
				Scope:    PermissionScopeAny,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t testing.T) {
			perm, err := ParsePermission(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, perm)
		})
	}
}

func TestParsePermission_InvalidFormat(t testing.T) {
	tests := []string{
		"risk:read",
		"read:any",
		"risk:read:own:extra",
		"invalid::format",
	}

	for _, tt := range tests {
		t.Run(tt, func(t testing.T) {
			perm, err := ParsePermission(tt)
			assert.Error(t, err)
			assert.Nil(t, perm)
		})
	}
}

func TestPermissionString(t testing.T) {
	perm := Permission{
		Resource: PermissionResourceRisk,
		Action:   PermissionRead,
		Scope:    PermissionScopeOwn,
	}

	assert.Equal(t, "risk:read:own", perm.String())
}

func TestPermissionMatches_ExactMatch(t testing.T) {
	testCases := []struct {
		name     string
		perm     Permission
		required Permission
		matches  bool
	}{
		{
			name:     "exact match",
			perm:     Permission{Resource: PermissionResourceRisk, Action: PermissionRead, Scope: PermissionScopeOwn},
			required: Permission{Resource: PermissionResourceRisk, Action: PermissionRead, Scope: PermissionScopeOwn},
			matches:  true,
		},
		{
			name:     "no match - different resource",
			perm:     Permission{Resource: PermissionResourceRisk, Action: PermissionRead, Scope: PermissionScopeOwn},
			required: Permission{Resource: PermissionResourceMitigation, Action: PermissionRead, Scope: PermissionScopeOwn},
			matches:  false,
		},
		{
			name:     "no match - different action",
			perm:     Permission{Resource: PermissionResourceRisk, Action: PermissionRead, Scope: PermissionScopeOwn},
			required: Permission{Resource: PermissionResourceRisk, Action: PermissionCreate, Scope: PermissionScopeOwn},
			matches:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t testing.T) {
			result := tc.perm.Matches(tc.required)
			assert.Equal(t, tc.matches, result)
		})
	}
}

func TestPermissionMatches_ResourceWildcard(t testing.T) {
	perm := Permission{Resource: "", Action: PermissionRead, Scope: PermissionScopeOwn}

	testCases := []struct {
		name     string
		required Permission
		matches  bool
	}{
		{
			name:     "wildcard matches risk",
			required: Permission{Resource: PermissionResourceRisk, Action: PermissionRead, Scope: PermissionScopeOwn},
			matches:  true,
		},
		{
			name:     "wildcard matches mitigation",
			required: Permission{Resource: PermissionResourceMitigation, Action: PermissionRead, Scope: PermissionScopeOwn},
			matches:  true,
		},
		{
			name:     "wildcard doesn't match different action",
			required: Permission{Resource: PermissionResourceRisk, Action: PermissionCreate, Scope: PermissionScopeOwn},
			matches:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t testing.T) {
			result := perm.Matches(tc.required)
			assert.Equal(t, tc.matches, result)
		})
	}
}

func TestPermissionMatches_ActionWildcard(t testing.T) {
	perm := Permission{Resource: PermissionResourceRisk, Action: "", Scope: PermissionScopeAny}

	testCases := []struct {
		name     string
		required Permission
		matches  bool
	}{
		{
			name:     "wildcard matches read",
			required: Permission{Resource: PermissionResourceRisk, Action: PermissionRead, Scope: PermissionScopeAny},
			matches:  true,
		},
		{
			name:     "wildcard matches create",
			required: Permission{Resource: PermissionResourceRisk, Action: PermissionCreate, Scope: PermissionScopeAny},
			matches:  true,
		},
		{
			name:     "wildcard doesn't match different resource",
			required: Permission{Resource: PermissionResourceMitigation, Action: PermissionRead, Scope: PermissionScopeAny},
			matches:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t testing.T) {
			result := perm.Matches(tc.required)
			assert.Equal(t, tc.matches, result)
		})
	}
}

func TestPermissionMatches_ScopeWildcard(t testing.T) {
	perm := Permission{Resource: PermissionResourceRisk, Action: PermissionRead, Scope: PermissionScopeAny}

	testCases := []struct {
		name     string
		required Permission
		matches  bool
	}{
		{
			name:     "scope any matches own",
			required: Permission{Resource: PermissionResourceRisk, Action: PermissionRead, Scope: PermissionScopeOwn},
			matches:  true,
		},
		{
			name:     "scope any matches team",
			required: Permission{Resource: PermissionResourceRisk, Action: PermissionRead, Scope: PermissionScopeTeam},
			matches:  true,
		},
		{
			name:     "scope any matches any",
			required: Permission{Resource: PermissionResourceRisk, Action: PermissionRead, Scope: PermissionScopeAny},
			matches:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t testing.T) {
			result := perm.Matches(tc.required)
			assert.Equal(t, tc.matches, result)
		})
	}
}

func TestPermissionMatches_FullWildcard(t testing.T) {
	perm := Permission{Resource: "", Action: "", Scope: PermissionScopeAny}

	testCases := []struct {
		name     string
		required Permission
		matches  bool
	}{
		{
			name:     "full wildcard matches any",
			required: Permission{Resource: PermissionResourceRisk, Action: PermissionDelete, Scope: PermissionScopeOwn},
			matches:  true,
		},
		{
			name:     "full wildcard matches user resource",
			required: Permission{Resource: PermissionResourceUser, Action: PermissionUpdate, Scope: PermissionScopeTeam},
			matches:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t testing.T) {
			result := perm.Matches(tc.required)
			assert.Equal(t, tc.matches, result)
		})
	}
}

func TestPermissionMatrixHasPermission(t testing.T) {
	matrix := &PermissionMatrix{
		Permissions: []Permission{
			{Resource: PermissionResourceRisk, Action: PermissionRead, Scope: PermissionScopeAny},
			{Resource: PermissionResourceMitigation, Action: PermissionCreate, Scope: PermissionScopeOwn},
		},
	}

	testCases := []struct {
		name       string
		permission Permission
		has        bool
	}{
		{
			name:       "has exact permission",
			permission: Permission{Resource: PermissionResourceRisk, Action: PermissionRead, Scope: PermissionScopeAny},
			has:        true,
		},
		{
			name:       "doesn't have permission",
			permission: Permission{Resource: PermissionResourceRisk, Action: PermissionDelete, Scope: PermissionScopeAny},
			has:        false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t testing.T) {
			result := matrix.HasPermission(tc.permission)
			assert.Equal(t, tc.has, result)
		})
	}
}

func TestPermissionMatrixAddPermission(t testing.T) {
	matrix := &PermissionMatrix{}

	perm := Permission{
		Resource: PermissionResourceRisk,
		Action:   PermissionRead,
		Scope:    PermissionScopeOwn,
	}

	err := matrix.AddPermission(perm)
	require.NoError(t, err)
	assert.Equal(t, , len(matrix.Permissions))
	assert.Equal(t, perm, matrix.Permissions[])
}

func TestPermissionMatrixRemovePermission(t testing.T) {
	perm := Permission{
		Resource: PermissionResourceRisk,
		Action:   PermissionRead,
		Scope:    PermissionScopeOwn,
	}

	matrix := &PermissionMatrix{
		Permissions: []Permission{perm},
	}

	err := matrix.RemovePermission(perm)
	require.NoError(t, err)
	assert.Equal(t, , len(matrix.Permissions))
}

func TestStandardPermissions_AdminPermissions(t testing.T) {
	matrix := &PermissionMatrix{Permissions: AdminPermissions}

	// Admin should have all permissions (full wildcard)
	testPerms := []Permission{
		{Resource: PermissionResourceRisk, Action: PermissionDelete, Scope: PermissionScopeAny},
		{Resource: PermissionResourceUser, Action: PermissionCreate, Scope: PermissionScopeAny},
		{Resource: PermissionResourceAuditLog, Action: PermissionRead, Scope: PermissionScopeTeam},
	}

	for _, perm := range testPerms {
		assert.True(t, matrix.HasPermission(perm), "admin should have permission %s", perm.String())
	}
}

func TestStandardPermissions_AnalystPermissions(t testing.T) {
	matrix := &PermissionMatrix{Permissions: AnalystPermissions}

	// Analyst should have risk CRUD
	assert.True(t, matrix.HasPermission(Permission{Resource: PermissionResourceRisk, Action: PermissionRead, Scope: PermissionScopeAny}))
	assert.True(t, matrix.HasPermission(Permission{Resource: PermissionResourceRisk, Action: PermissionCreate, Scope: PermissionScopeAny}))
	assert.True(t, matrix.HasPermission(Permission{Resource: PermissionResourceRisk, Action: PermissionUpdate, Scope: PermissionScopeAny}))

	// Analyst can only delete own
	assert.True(t, matrix.HasPermission(Permission{Resource: PermissionResourceRisk, Action: PermissionDelete, Scope: PermissionScopeOwn}))
	assert.False(t, matrix.HasPermission(Permission{Resource: PermissionResourceRisk, Action: PermissionDelete, Scope: PermissionScopeAny}))
}

func TestStandardPermissions_ViewerPermissions(t testing.T) {
	matrix := &PermissionMatrix{Permissions: ViewerPermissions}

	// Viewer can read
	assert.True(t, matrix.HasPermission(Permission{Resource: PermissionResourceRisk, Action: PermissionRead, Scope: PermissionScopeAny}))

	// Viewer cannot create, update, or delete
	assert.False(t, matrix.HasPermission(Permission{Resource: PermissionResourceRisk, Action: PermissionCreate, Scope: PermissionScopeAny}))
	assert.False(t, matrix.HasPermission(Permission{Resource: PermissionResourceRisk, Action: PermissionUpdate, Scope: PermissionScopeAny}))
	assert.False(t, matrix.HasPermission(Permission{Resource: PermissionResourceRisk, Action: PermissionDelete, Scope: PermissionScopeAny}))
}

func TestAnalystCanCreateRisks(t testing.T) {
	matrix := &PermissionMatrix{Permissions: AnalystPermissions}

	assert.True(t, matrix.HasPermission(Permission{
		Resource: PermissionResourceRisk,
		Action:   PermissionCreate,
		Scope:    PermissionScopeAny,
	}))
}

func TestViewerCannotDeleteRisks(t testing.T) {
	matrix := &PermissionMatrix{Permissions: ViewerPermissions}

	assert.False(t, matrix.HasPermission(Permission{
		Resource: PermissionResourceRisk,
		Action:   PermissionDelete,
		Scope:    PermissionScopeAny,
	}))
}

func TestAdminCanDoAnything(t testing.T) {
	matrix := &PermissionMatrix{Permissions: AdminPermissions}

	testCases := []Permission{
		{Resource: PermissionResourceRisk, Action: PermissionDelete, Scope: PermissionScopeAny},
		{Resource: PermissionResourceUser, Action: PermissionCreate, Scope: PermissionScopeOwn},
		{Resource: PermissionResourceAuditLog, Action: PermissionRead, Scope: PermissionScopeTeam},
		{Resource: PermissionResourceIntegration, Action: PermissionUpdate, Scope: PermissionScopeAny},
	}

	for _, tc := range testCases {
		assert.True(t, matrix.HasPermission(tc), "admin should have permission %s", tc.String())
	}
}
