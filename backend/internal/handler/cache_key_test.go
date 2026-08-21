// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

package handler

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/middleware"
	"github.com/opendefender/openrisk/pkg/cache"
)

// keyFor drives one key builder through a real fiber request, optionally with a
// resolved tenant, and returns what it produced.
func keyFor(t *testing.T, build cache.KeyFunc, target string, tenant uuid.UUID, params map[string]string) (string, bool) {
	t.Helper()
	app := fiber.New()
	var (
		key string
		ok  bool
	)
	app.Get(target, func(c *fiber.Ctx) error {
		if tenant != uuid.Nil {
			middleware.SetContext(c, &middleware.RequestContext{OrganizationID: tenant, UserID: uuid.New()})
		}
		key, ok = build(c)
		return c.SendStatus(fiber.StatusOK)
	})

	path := strings.Replace(target, ":id", "11111111-1111-1111-1111-111111111111", 1)
	if len(params) > 0 {
		q := make([]string, 0, len(params))
		for k, v := range params {
			q = append(q, k+"="+v)
		}
		path += "?" + strings.Join(q, "&")
	}
	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, path, nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	return key, ok
}

// Every builder registered on a cached route, with the path shape it is
// registered under.
var cacheKeyBuilders = []struct {
	name   string
	build  cache.KeyFunc
	target string
}{
	{"dashboard stats", dashboardStatsCacheKey, "/stats"},
	{"dashboard matrix", dashboardMatrixCacheKey, "/stats/risk-matrix"},
	{"dashboard timeline", dashboardTimelineCacheKey, "/stats/trends"},
	{"risk list", riskListCacheKey, "/risks"},
	{"risk search", riskSearchCacheKey, "/risks/search"},
	{"risk by id", riskByIDCacheKey, "/risks/:id"},
	{"connector list", connectorListCacheKey, "/connectors"},
	{"connector by id", connectorByIDCacheKey, "/connectors/:id"},
	{"marketplace app", marketplaceAppByIDCacheKey, "/marketplace/apps/:id"},
}

// THE test of this file. Two tenants asking the same question must never land on
// the same key.
//
// Before W0-06 every one of these builders failed this: `dashboard:stats:month`,
// `dashboard:matrix:all` and `risk:list:page:1:sev::status:` are the same string
// for everybody. It was not a live disclosure only because WrapWithCache returns
// the handler unchanged — a property invisible from the call sites in main.go,
// and one implementation away from serving one organisation's dashboard to
// another.
func TestCacheKeys_NeverCollideAcrossTenants(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	for _, tc := range cacheKeyBuilders {
		t.Run(tc.name, func(t *testing.T) {
			keyA, okA := keyFor(t, tc.build, tc.target, a, nil)
			keyB, okB := keyFor(t, tc.build, tc.target, b, nil)
			require.True(t, okA)
			require.True(t, okB)
			assert.NotEqual(t, keyA, keyB, "two tenants must not share a cache key")
			assert.Contains(t, keyA, a.String(), "the key must carry the tenant it belongs to")
			assert.NotContains(t, keyA, b.String())
		})
	}
}

// No tenant, no key. False is the correct answer for an unresolved request, and
// the KeyFunc contract turns it into "do not cache" rather than "cache under a
// key everyone shares".
func TestCacheKeys_RefuseWithoutATenant(t *testing.T) {
	for _, tc := range cacheKeyBuilders {
		t.Run(tc.name, func(t *testing.T) {
			key, ok := keyFor(t, tc.build, tc.target, uuid.Nil, nil)
			assert.False(t, ok, "an unresolved tenant must yield no key")
			assert.Empty(t, key)
		})
	}
}

// The period is part of the answer, so it must be part of the key: two windows
// over the same register are two different responses, and serving one for the
// other is the cache equivalent of ignoring the filter.
func TestCacheKeys_DashboardStatsVariesWithThePeriod(t *testing.T) {
	tenant := uuid.New()
	all, _ := keyFor(t, dashboardStatsCacheKey, "/stats", tenant, map[string]string{"period": "all"})
	week, _ := keyFor(t, dashboardStatsCacheKey, "/stats", tenant, map[string]string{"period": "7d"})
	custom, _ := keyFor(t, dashboardStatsCacheKey, "/stats", tenant,
		map[string]string{"from": "2026-08-01", "to": "2026-09-01"})

	assert.NotEqual(t, all, week)
	assert.NotEqual(t, all, custom)
	assert.NotEqual(t, week, custom)
}

// Same tenant, same question, same key — or the cache never hits and the whole
// mechanism is a slow no-op.
func TestCacheKeys_AreStableForTheSameRequest(t *testing.T) {
	tenant := uuid.New()
	for _, tc := range cacheKeyBuilders {
		first, _ := keyFor(t, tc.build, tc.target, tenant, nil)
		second, _ := keyFor(t, tc.build, tc.target, tenant, nil)
		assert.Equal(t, first, second, "%s: identical requests must produce identical keys", tc.name)
	}
}
