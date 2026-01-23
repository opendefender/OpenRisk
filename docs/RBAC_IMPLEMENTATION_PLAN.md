# Phase 5 Priority #5: RBAC & Multi-Tenant Implementation Plan

**Status**: 🟡 **IN PROGRESS - Planning Phase**  
**Date**: January 22, 2026  
**Estimated Completion**: 14-21 days  
**Team**: Backend Engineers + DevSecOps

---

## 📋 Executive Summary

This document defines the comprehensive implementation plan for **Role-Based Access Control (RBAC)** and **Multi-Tenant Support** for OpenRisk. This represents Phase 5 Priority #5, building on the completed performance optimization (Priority #4).

### Key Objectives
- ✅ Implement granular role hierarchy with permission matrix
- ✅ Add multi-tenant isolation at database and application layers
- ✅ Create permission enforcement middleware across all endpoints
- ✅ Build frontend RBAC UI controls and user management dashboard
- ✅ Ensure backward compatibility with existing deployments
- ✅ Provide comprehensive testing and documentation

### Expected Outcomes
- 6+ new domain models
- 4+ middleware implementations
- 8+ API endpoints for RBAC management
- 100% permission coverage on protected endpoints
- Multi-tenant isolation verified
- Full test suite with 40+ tests

---

## 🎯 Sprint Structure

### Sprint 1: Domain Models & Database (5-6 days)
**Goal**: Build the foundation for RBAC and multi-tenant architecture

#### Tasks
1. **Create Enhanced Role Model** (1 day)
   - Extend existing simple role to enterprise-grade model
   - Add role hierarchy (Admin > Manager > Analyst > Viewer)
   - Implement permission matrix (Resource × Action)
   - Add metadata and timestamps

2. **Create Permission Model** (1 day)
   - Define permission resources (Risk, Mitigation, User, Report, etc.)
   - Define permission actions (Create, Read, Update, Delete, Export, etc.)
   - Create permission assignment mapping

3. **Create Tenant Model** (1 day)
   - Tenant isolation structure
   - Tenant-scoped data boundaries
   - Tenant configuration and metadata

4. **Create Role-Permission Mapping** (1 day)
   - Junction table: roles ↔ permissions
   - Predefined role templates (Admin, Manager, Analyst, Viewer)
   - Permission inheritance support

5. **Create Database Migrations** (1 day)
   - Migration 0008: roles table with hierarchy
   - Migration 0009: permissions table
   - Migration 0010: role_permissions junction
   - Migration 0011: tenants table
   - Migration 0012: user_tenants junction
   - Seed default roles and permissions

6. **Update User Model** (1 day)
   - Add tenant_id foreign key
   - Add role_id relationship
   - Add is_active flag for soft deactivation
   - Add metadata (last_login, created_by, etc.)

---

### Sprint 2: Domain Logic & Services (4-5 days)
**Goal**: Implement business logic for RBAC operations

#### Tasks
1. **Create RoleService** (1 day)
   - CRUD operations for roles
   - Permission assignment/revocation
   - Role inheritance management
   - Predefined role creation
   - Role validation and constraints

2. **Create PermissionService** (1 day)
   - Permission registry and lookup
   - Dynamic permission evaluation
   - Permission caching strategy
   - Permission matrix generation

3. **Create TenantService** (1 day)
   - Tenant lifecycle (create, activate, deactivate)
   - Tenant configuration management
   - Tenant metrics and reporting
   - Multi-tenant data isolation

4. **Create PermissionEvaluator** (1 day)
   - Evaluate user permissions
   - Handle role inheritance
   - Handle special cases (owner, creator, admin)
   - Cache permission lookups

5. **Create Unit Tests** (1 day)
   - 40+ unit tests for domain logic
   - Permission matrix verification
   - Role hierarchy validation
   - Tenant isolation verification

---

### Sprint 3: Middleware & Enforcement (4-5 days)
**Goal**: Enforce permissions at API layer

#### Tasks
1. **Create Permission Middleware** (1 day)
   - Extract claims from JWT
   - Evaluate permissions for route
   - Handle permission denials
   - Log permission checks

2. **Create Tenant Middleware** (1 day)
   - Extract tenant context from request
   - Validate tenant ownership
   - Isolate queries to tenant
   - Handle cross-tenant attempts

3. **Create Ownership Middleware** (1 day)
   - Verify resource ownership
   - Handle inherited access (via role)
   - Support cascading permissions
   - Log access attempts

4. **Apply Middleware to Routes** (1 day)
   - Risk endpoints
   - Mitigation endpoints
   - User management endpoints
   - Report endpoints
   - Integration endpoints

5. **Create Integration Tests** (1 day)
   - Test permission enforcement
   - Test tenant isolation
   - Test ownership verification
   - Negative test cases

---

### Sprint 4: Frontend & API (4-5 days)
**Goal**: Create UI and management APIs

#### Tasks
1. **Create User Management API** (1 day)
   - List users with role/tenant info
   - Create user with role assignment
   - Update user role
   - Deactivate user
   - Export user report

2. **Create Role Management API** (1 day)
   - List available roles
   - Create custom role
   - Assign permissions to role
   - Delete role (with safety checks)

3. **Create User Management UI** (1 day)
   - User list with search/filter
   - User creation modal with role selector
   - User edit modal
   - User deactivation dialog
   - User activity log view

4. **Create Role Management UI** (1 day)
   - Role list with permissions
   - Role creation with permission matrix
   - Role editing
   - Permission visualization

5. **Create Permission Matrix Visualization** (1 day)
   - Grid view: Roles × Permissions
   - Visual permission assignment
   - Quick role template selection
   - Permission inheritance display

---

### Sprint 5: Documentation & Testing (3-4 days)
**Goal**: Complete documentation and comprehensive testing

#### Tasks
1. **Create RBAC Documentation** (1 day)
   - Role hierarchy explanation
   - Permission matrix reference
   - API endpoint documentation
   - Best practices guide

2. **Create Multi-Tenant Guide** (1 day)
   - Tenant isolation architecture
   - Data boundary enforcement
   - Multi-tenant deployment guide
   - Troubleshooting guide

3. **Create Test Plan & Execution** (1 day)
   - Permission enforcement tests
   - Tenant isolation tests
   - Performance tests under RBAC
   - Security tests (permission bypass attempts)

4. **Create Deployment Guide** (1 day)
   - Migration execution procedure
   - Backward compatibility notes
   - Rollback procedure
   - Permission assignment workflow

---

## 🏗️ Architecture Overview

### Domain Models

```
User (Enhanced)
├─ id
├─ email
├─ password_hash
├─ tenant_id (FK) ← NEW
├─ role_id (FK) ← NEW (replaces role string)
├─ is_active ← NEW
├─ created_at
├─ updated_at
└─ deleted_at

Role (Enhanced)
├─ id
├─ tenant_id (FK) ← NEW (scoped to tenant)
├─ name
├─ description
├─ level (0-9, hierarchy) ← NEW
├─ is_predefined
├─ permissions[] (many-to-many) ← NEW
├─ created_at
├─ updated_at
└─ metadata (JSON) ← NEW

Permission (NEW)
├─ id
├─ resource (Risk, User, Mitigation, etc.)
├─ action (Create, Read, Update, Delete, Export)
├─ description
├─ is_system (predefined vs custom)
└─ metadata (JSON)

RolePermission (NEW - Junction)
├─ role_id (FK)
├─ permission_id (FK)
└─ created_at

Tenant (NEW)
├─ id
├─ name
├─ slug
├─ owner_id (FK → User)
├─ status (active, suspended, deleted)
├─ metadata (JSON)
├─ created_at
├─ updated_at
└─ deleted_at

UserTenant (NEW - Many-to-Many)
├─ user_id (FK)
├─ tenant_id (FK)
├─ role_id (FK) ← Role scoped to tenant
├─ created_at
└─ updated_at
```

### Permission Matrix

| Resource | Create | Read | Update | Delete | Export | Admin |
|----------|--------|------|--------|--------|--------|-------|
| Risk | Analyst+ | Viewer+ | Analyst+ | Analyst+ | Analyst+ | Admin |
| Mitigation | Analyst+ | Viewer+ | Analyst+ | Analyst+ | Analyst+ | Admin |
| User | Admin | Admin | Admin | Admin | Admin | Admin |
| Report | Analyst+ | Viewer+ | Analyst+ | Analyst+ | Analyst+ | Admin |
| Integration | Admin | Admin | Admin | Admin | Admin | Admin |
| Audit Log | Viewer+ | Viewer+ | - | - | Admin | Admin |

### Role Hierarchy

```
Admin (Level 9)
  └─ All permissions
  └─ Can manage users & roles

Manager (Level 6)
  └─ Full risk management
  └─ Can view reports
  └─ Cannot manage users

Analyst (Level 3)
  └─ Can create/update risks & mitigations
  └─ Can view dashboard
  └─ Cannot delete or manage users

Viewer (Level 0)
  └─ Read-only access
  └─ Can view risks & dashboard
  └─ Cannot create/modify anything
```

---

## 📊 Implementation Tasks Breakdown

### Backend Tasks
- [ ] Create role domain model with hierarchy
- [ ] Create permission domain model
- [ ] Create tenant domain model
- [ ] Create database migrations (6 migrations)
- [ ] Create RoleService with 12+ methods
- [ ] Create PermissionService with 8+ methods
- [ ] Create TenantService with 10+ methods
- [ ] Create PermissionEvaluator logic
- [ ] Create permission middleware
- [ ] Create tenant middleware
- [ ] Create ownership middleware
- [ ] Create 8 RBAC management endpoints
- [ ] Create 40+ unit tests
- [ ] Create 20+ integration tests
- [ ] Update 15+ existing endpoints with RBAC enforcement

### Frontend Tasks
- [ ] Create User Management page with full CRUD
- [ ] Create Role Management page
- [ ] Create Permission Matrix visualization
- [ ] Create role selector in user creation
- [ ] Add RBAC checks to UI (hide/disable features)
- [ ] Create audit log page
- [ ] Create 15+ React components for RBAC

### DevOps/QA Tasks
- [ ] Test permission enforcement
- [ ] Test tenant isolation
- [ ] Performance test RBAC evaluation
- [ ] Security audit of permission logic
- [ ] Create staging deployment guide
- [ ] Create monitoring for permission denials

---

## 🔒 Security Considerations

### Permission Denial Protection
- ✅ Evaluate permissions on every protected endpoint
- ✅ Log all permission denials for security audit
- ✅ Rate limit permission checks to prevent brute force
- ✅ Use consistent permission evaluation logic

### Tenant Isolation
- ✅ Filter queries by tenant_id on all reads
- ✅ Validate ownership on all writes
- ✅ Prevent cross-tenant data access
- ✅ Validate tenant ownership in middleware

### Privilege Escalation Prevention
- ✅ Only admins can assign roles
- ✅ Cannot assign higher-level role than own
- ✅ Audit all role changes
- ✅ Restrict permission modifications

### Token Security
- ✅ Include tenant_id in JWT claims
- ✅ Include role_id in JWT claims
- ✅ Include permission_hash for quick checks
- ✅ Validate claims on every request

---

## 📈 Performance Targets

| Metric | Target | Strategy |
|--------|--------|----------|
| Permission Check Latency | < 5ms | Cache permissions in JWT |
| Role Lookup | < 10ms | Redis cache with TTL |
| Tenant Query Filter | < 2ms | Index on tenant_id |
| Permission Matrix Load | < 50ms | Lazy load on demand |
| RBAC Middleware | < 1ms | Fast path for common roles |

---

## 🚀 Deployment Strategy

### Phase 1: Database & Models (Days 1-2)
1. Create new migrations (non-breaking)
2. Seed default roles and permissions
3. Migrate existing users to roles

### Phase 2: Services & Logic (Days 3-4)
1. Deploy new services (backward compatible)
2. Add permission enforcement gradually
3. Monitor permission denials

### Phase 3: Middleware & Enforcement (Days 5-6)
1. Apply middleware to protected routes
2. Validate permission enforcement
3. Monitor for issues

### Phase 4: Frontend & Management (Days 7-8)
1. Deploy management UI
2. Train users on new features
3. Gather feedback

### Phase 5: Migration & Cutover (Days 9-10)
1. Migrate all existing roles
2. Verify all permissions working
3. Document mapping

---

## 📋 Definition of Done

✅ All domain models created and tested  
✅ Database migrations created and versioned  
✅ All RBAC services implemented  
✅ Permission middleware enforced on all protected routes  
✅ Tenant middleware enforces isolation  
✅ Frontend UI for role management complete  
✅ 60+ unit and integration tests passing  
✅ Documentation complete and peer-reviewed  
✅ Security audit passed  
✅ Performance targets met  
✅ Backward compatibility maintained  
✅ Deployment procedure tested  

---

## 📚 Related Documents

- [ADVANCED_PERMISSIONS.md](ADVANCED_PERMISSIONS.md) - Permission system architecture
- [LOCAL_DEVELOPMENT.md](LOCAL_DEVELOPMENT.md) - Development setup
- [STAGING_VALIDATION_CHECKLIST.md](../STAGING_VALIDATION_CHECKLIST.md) - Deployment procedure

---

## 🔗 Acceptance Criteria

### Functional
- [ ] Users can be assigned roles (Admin, Manager, Analyst, Viewer)
- [ ] Permissions are enforced on all protected endpoints
- [ ] Users cannot access resources outside their tenant
- [ ] Role permissions can be customized
- [ ] Permission changes take effect immediately (cached)

### Non-Functional
- [ ] Permission checks complete in < 5ms
- [ ] No performance degradation vs current system
- [ ] 99.9% availability during permission lookups
- [ ] All permission denials logged

### Testing
- [ ] 100% coverage of permission logic
- [ ] All role hierarchy tested
- [ ] Cross-tenant access prevented in tests
- [ ] Privilege escalation attempts fail safely

---

**Next Step**: Begin Sprint 1 - Domain Models & Database implementation
