// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package crosstenant

// Assets, custom fields, saved views, categories and teams.
var assetProbes = []Probe{
	list("/api/v1/assets", KindAsset),
	get("/api/v1/assets/:id", KindAsset),
	patch("/api/v1/assets/:id", `{"name":"changed by attacker"}`, KindAsset),
	del("/api/v1/assets/:id", KindAsset),
	get("/api/v1/assets/:id/analysis", KindAsset),
	get("/api/v1/assets/:id/history", KindAsset),
	allow(get("/api/v1/assets/:id/inventories", KindAsset),
		"answers 200 with an empty list for any asset id, foreign or random; the query filters tenant_id and asset_id, B's rows are not returned", "RR-02", 200),
	agg("/api/v1/assets/exposure"),
	agg("/api/v1/assets/statistics"),

	list("/api/v1/asset-dependencies", KindAssetDep),
	post("/api/v1/asset-dependencies", `{"source_asset_id":"{{asset}}","target_asset_id":"{{assetpeer}}"}`, KindAsset, KindAssetPeer),
	del("/api/v1/asset-dependencies/:id", KindAssetDep),

	list("/api/v1/custom-fields", KindCustomField),
	get("/api/v1/custom-fields/:id", KindCustomField),
	patch("/api/v1/custom-fields/:id", `{"display_name":"changed by attacker"}`, KindCustomField),
	del("/api/v1/custom-fields/:id", KindCustomField),
	list("/api/v1/custom-fields/scope/:scope", "=risk", KindCustomField),

	list("/api/v1/saved-views", KindSavedView),
	patch("/api/v1/saved-views/:id", `{"name":"changed by attacker"}`, KindSavedView),
	del("/api/v1/saved-views/:id", KindSavedView),

	list("/api/v1/risk-categories", KindCategory),
	patch("/api/v1/risk-categories/:id", `{"name":"changed by attacker"}`, KindCategory),
	del("/api/v1/risk-categories/:id", KindCategory),

	list("/api/v1/teams", KindTeam),
	get("/api/v1/teams/:id", KindTeam),
	patch("/api/v1/teams/:id", `{"name":"changed by attacker"}`, KindTeam),
	del("/api/v1/teams/:id", KindTeam),
	post("/api/v1/teams/:id/members/:userId", `{}`, KindTeam, KindUser),
	del("/api/v1/teams/:id/members/:userId", KindTeam, KindUser),
}
