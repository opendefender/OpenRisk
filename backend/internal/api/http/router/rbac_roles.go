// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package router

import (
	handlers "github.com/opendefender/openrisk/internal/handler"
)

// RegisterRBACRoleRoutes mounts /rbac/roles exactly as main.go did: an
// admin-only group, no route-level guard.
func RegisterRBACRoleRoutes(protected Mounter, h *handlers.RBACRoleHandler) {
	rbacRoles := protected.Group("/rbac/roles", Role("admin"))
	rbacRoles.Get("", SessionOnly, h.ListRoles)
	rbacRoles.Post("", SessionOnly, h.CreateRole)
	rbacRoles.Get("/:role_id", SessionOnly, h.GetRole)
	rbacRoles.Patch("/:role_id", SessionOnly, h.UpdateRole)
	rbacRoles.Delete("/:role_id", SessionOnly, h.DeleteRole)
	rbacRoles.Get("/:role_id/permissions", SessionOnly, h.GetRolePermissions)
	rbacRoles.Post("/:role_id/permissions", SessionOnly, h.AssignPermissionToRole)
	rbacRoles.Delete("/:role_id/permissions", SessionOnly, h.RemovePermissionFromRole)
}
