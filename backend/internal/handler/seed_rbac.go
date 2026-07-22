// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// SeedRBAC provisions the RBAC / multi-tenant tables so the Settings admin tabs
// (Roles, Organizations, Audit log) are fully functional. It is idempotent and
// runs on every startup after AutoMigrate + SeedAdminUser:
//   1. a system permission catalog (resource × action),
//   2. the predefined roles (Admin/Manager/Analyst/Viewer) + their permissions,
//   3. a Tenant mirror of every Organization and a UserTenant for every member.
// The app's request-time authorization still runs off the JWT org_roles/permissions;
// these tables make the admin management screens real rather than decorative.

package handler

import (
	"context"
	"log"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/database"
	"github.com/opendefender/openrisk/internal/service"
)

func SeedRBAC() {
	db := database.DB
	if db == nil {
		return
	}
	ctx := context.Background()

	// 1) permission catalog (resource × action) — assignPermissionsToRole reads this.
	var permCount int64
	db.Model(&domain.PermissionDB{}).Count(&permCount)
	if permCount == 0 {
		resources := []string{"risk", "asset", "mitigation", "compliance", "report", "user", "role", "tenant", "token", "audit", "incident", "dashboard"}
		actions := []string{"read", "create", "update", "delete", "export"}
		for _, r := range resources {
			for _, a := range actions {
				db.Create(&domain.PermissionDB{Resource: r, Action: a, Description: a + " " + r, IsSystem: true})
			}
		}
		log.Printf("SeedRBAC: seeded %d permissions", len(resources)*len(actions))
	}

	// 2) predefined roles (+ role_permissions via assignPermissionsToRole). Idempotent:
	// InitializeDefaultRoles skips roles that already exist.
	roleSvc := service.NewRoleService(db)
	if err := roleSvc.InitializeDefaultRoles(ctx); err != nil {
		log.Printf("SeedRBAC: InitializeDefaultRoles failed: %v", err)
	}

	// 3) mirror every Organization as an RBAC Tenant, and every membership as a UserTenant.
	adminRole, _ := roleSvc.GetRoleByName(ctx, uuid.Nil, "Admin")

	var orgs []domain.Organization
	db.Find(&orgs)
	for _, org := range orgs {
		var existing domain.Tenant
		if err := db.Where("id = ?", org.ID).First(&existing).Error; err == nil {
			continue // already mirrored
		}
		slug := org.Slug
		if slug == "" {
			slug = org.ID.String()
		}
		if err := db.Create(&domain.Tenant{
			ID:       org.ID,
			Name:     org.Name,
			Slug:     slug,
			OwnerID:  org.OwnerID,
			Status:   "active",
			IsActive: true,
		}).Error; err != nil {
			log.Printf("SeedRBAC: create tenant for org %s: %v", org.ID, err)
		}
	}

	if adminRole != nil {
		var members []domain.OrganizationMember
		db.Find(&members)
		for _, m := range members {
			var ut domain.UserTenant
			if err := db.Where("user_id = ? AND tenant_id = ?", m.UserID, m.OrganizationID).First(&ut).Error; err == nil {
				continue
			}
			if err := db.Create(&domain.UserTenant{
				UserID:   m.UserID,
				TenantID: m.OrganizationID,
				RoleID:   adminRole.ID,
			}).Error; err != nil {
				log.Printf("SeedRBAC: create user_tenant for %s: %v", m.UserID, err)
			}
		}
	}

	log.Println("SeedRBAC: RBAC / tenant / audit tables provisioned")
}
