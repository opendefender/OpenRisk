 Phase  Priority : RBAC & Multi-Tenant Implementation Plan

Status:  IN PROGRESS - Planning Phase  
Date: January ,   
Estimated Completion: - days  
Team: Backend Engineers + DevSecOps

---

  Executive Summary

This document defines the comprehensive implementation plan for Role-Based Access Control (RBAC) and Multi-Tenant Support for OpenRisk. This represents Phase  Priority , building on the completed performance optimization (Priority ).

 Key Objectives
-  Implement granular role hierarchy with permission matrix
-  Add multi-tenant isolation at database and application layers
-  Create permission enforcement middleware across all endpoints
-  Build frontend RBAC UI controls and user management dashboard
-  Ensure backward compatibility with existing deployments
-  Provide comprehensive testing and documentation

 Expected Outcomes
- + new domain models
- + middleware implementations
- + API endpoints for RBAC management
- % permission coverage on protected endpoints
- Multi-tenant isolation verified
- Full test suite with + tests

---

  Sprint Structure

 Sprint : Domain Models & Database (- days)
Goal: Build the foundation for RBAC and multi-tenant architecture

 Tasks
. Create Enhanced Role Model ( day)
   - Extend existing simple role to enterprise-grade model
   - Add role hierarchy (Admin > Manager > Analyst > Viewer)
   - Implement permission matrix (Resource × Action)
   - Add metadata and timestamps

. Create Permission Model ( day)
   - Define permission resources (Risk, Mitigation, User, Report, etc.)
   - Define permission actions (Create, Read, Update, Delete, Export, etc.)
   - Create permission assignment mapping

. Create Tenant Model ( day)
   - Tenant isolation structure
   - Tenant-scoped data boundaries
   - Tenant configuration and metadata

. Create Role-Permission Mapping ( day)
   - Junction table: roles ↔ permissions
   - Predefined role templates (Admin, Manager, Analyst, Viewer)
   - Permission inheritance support

. Create Database Migrations ( day)
   - Migration : roles table with hierarchy
   - Migration : permissions table
   - Migration : role_permissions junction
   - Migration : tenants table
   - Migration : user_tenants junction
   - Seed default roles and permissions

. Update User Model ( day)
   - Add tenant_id foreign key
   - Add role_id relationship
   - Add is_active flag for soft deactivation
   - Add metadata (last_login, created_by, etc.)

---

 Sprint : Domain Logic & Services (- days)
Goal: Implement business logic for RBAC operations

 Tasks
. Create RoleService ( day)
   - CRUD operations for roles
   - Permission assignment/revocation
   - Role inheritance management
   - Predefined role creation
   - Role validation and constraints

. Create PermissionService ( day)
   - Permission registry and lookup
   - Dynamic permission evaluation
   - Permission caching strategy
   - Permission matrix generation

. Create TenantService ( day)
   - Tenant lifecycle (create, activate, deactivate)
   - Tenant configuration management
   - Tenant metrics and reporting
   - Multi-tenant data isolation

. Create PermissionEvaluator ( day)
   - Evaluate user permissions
   - Handle role inheritance
   - Handle special cases (owner, creator, admin)
   - Cache permission lookups

. Create Unit Tests ( day)
   - + unit tests for domain logic
   - Permission matrix verification
   - Role hierarchy validation
   - Tenant isolation verification

---

 Sprint : Middleware & Enforcement (- days)
Goal: Enforce permissions at API layer

 Tasks
. Create Permission Middleware ( day)
   - Extract claims from JWT
   - Evaluate permissions for route
   - Handle permission denials
   - Log permission checks

. Create Tenant Middleware ( day)
   - Extract tenant context from request
   - Validate tenant ownership
   - Isolate queries to tenant
   - Handle cross-tenant attempts

. Create Ownership Middleware ( day)
   - Verify resource ownership
   - Handle inherited access (via role)
   - Support cascading permissions
   - Log access attempts

. Apply Middleware to Routes ( day)
   - Risk endpoints
   - Mitigation endpoints
   - User management endpoints
   - Report endpoints
   - Integration endpoints

. Create Integration Tests ( day)
   - Test permission enforcement
   - Test tenant isolation
   - Test ownership verification
   - Negative test cases

---

 Sprint : Frontend & API (- days)
Goal: Create UI and management APIs

 Tasks
. Create User Management API ( day)
   - List users with role/tenant info
   - Create user with role assignment
   - Update user role
   - Deactivate user
   - Export user report

. Create Role Management API ( day)
   - List available roles
   - Create custom role
   - Assign permissions to role
   - Delete role (with safety checks)

. Create User Management UI ( day)
   - User list with search/filter
   - User creation modal with role selector
   - User edit modal
   - User deactivation dialog
   - User activity log view

. Create Role Management UI ( day)
   - Role list with permissions
   - Role creation with permission matrix
   - Role editing
   - Permission visualization

. Create Permission Matrix Visualization ( day)
   - Grid view: Roles × Permissions
   - Visual permission assignment
   - Quick role template selection
   - Permission inheritance display

---

 Sprint : Documentation & Testing (- days)
Goal: Complete documentation and comprehensive testing

 Tasks
. Create RBAC Documentation ( day)
   - Role hierarchy explanation
   - Permission matrix reference
   - API endpoint documentation
   - Best practices guide

. Create Multi-Tenant Guide ( day)
   - Tenant isolation architecture
   - Data boundary enforcement
   - Multi-tenant deployment guide
   - Troubleshooting guide

. Create Test Plan & Execution ( day)
   - Permission enforcement tests
   - Tenant isolation tests
   - Performance tests under RBAC
   - Security tests (permission bypass attempts)

. Create Deployment Guide ( day)
   - Migration execution procedure
   - Backward compatibility notes
   - Rollback procedure
   - Permission assignment workflow

---

  Architecture Overview

 Domain Models


User (Enhanced)
 id
 email
 password_hash
 tenant_id (FK) ← NEW
 role_id (FK) ← NEW (replaces role string)
 is_active ← NEW
 created_at
 updated_at
 deleted_at

Role (Enhanced)
 id
 tenant_id (FK) ← NEW (scoped to tenant)
 name
 description
 level (-, hierarchy) ← NEW
 is_predefined
 permissions[] (many-to-many) ← NEW
 created_at
 updated_at
 metadata (JSON) ← NEW

Permission (NEW)
 id
 resource (Risk, User, Mitigation, etc.)
 action (Create, Read, Update, Delete, Export)
 description
 is_system (predefined vs custom)
 metadata (JSON)

RolePermission (NEW - Junction)
 role_id (FK)
 permission_id (FK)
 created_at

Tenant (NEW)
 id
 name
 slug
 owner_id (FK → User)
 status (active, suspended, deleted)
 metadata (JSON)
 created_at
 updated_at
 deleted_at

UserTenant (NEW - Many-to-Many)
 user_id (FK)
 tenant_id (FK)
 role_id (FK) ← Role scoped to tenant
 created_at
 updated_at


 Permission Matrix

| Resource | Create | Read | Update | Delete | Export | Admin |
|----------|--------|------|--------|--------|--------|-------|
| Risk | Analyst+ | Viewer+ | Analyst+ | Analyst+ | Analyst+ | Admin |
| Mitigation | Analyst+ | Viewer+ | Analyst+ | Analyst+ | Analyst+ | Admin |
| User | Admin | Admin | Admin | Admin | Admin | Admin |
| Report | Analyst+ | Viewer+ | Analyst+ | Analyst+ | Analyst+ | Admin |
| Integration | Admin | Admin | Admin | Admin | Admin | Admin |
| Audit Log | Viewer+ | Viewer+ | - | - | Admin | Admin |

 Role Hierarchy


Admin (Level )
   All permissions
   Can manage users & roles

Manager (Level )
   Full risk management
   Can view reports
   Cannot manage users

Analyst (Level )
   Can create/update risks & mitigations
   Can view dashboard
   Cannot delete or manage users

Viewer (Level )
   Read-only access
   Can view risks & dashboard
   Cannot create/modify anything


---

  Implementation Tasks Breakdown

 Backend Tasks
- [ ] Create role domain model with hierarchy
- [ ] Create permission domain model
- [ ] Create tenant domain model
- [ ] Create database migrations ( migrations)
- [ ] Create RoleService with + methods
- [ ] Create PermissionService with + methods
- [ ] Create TenantService with + methods
- [ ] Create PermissionEvaluator logic
- [ ] Create permission middleware
- [ ] Create tenant middleware
- [ ] Create ownership middleware
- [ ] Create  RBAC management endpoints
- [ ] Create + unit tests
- [ ] Create + integration tests
- [ ] Update + existing endpoints with RBAC enforcement

 Frontend Tasks
- [ ] Create User Management page with full CRUD
- [ ] Create Role Management page
- [ ] Create Permission Matrix visualization
- [ ] Create role selector in user creation
- [ ] Add RBAC checks to UI (hide/disable features)
- [ ] Create audit log page
- [ ] Create + React components for RBAC

 DevOps/QA Tasks
- [ ] Test permission enforcement
- [ ] Test tenant isolation
- [ ] Performance test RBAC evaluation
- [ ] Security audit of permission logic
- [ ] Create staging deployment guide
- [ ] Create monitoring for permission denials

---

  Security Considerations

 Permission Denial Protection
-  Evaluate permissions on every protected endpoint
-  Log all permission denials for security audit
-  Rate limit permission checks to prevent brute force
-  Use consistent permission evaluation logic

 Tenant Isolation
-  Filter queries by tenant_id on all reads
-  Validate ownership on all writes
-  Prevent cross-tenant data access
-  Validate tenant ownership in middleware

 Privilege Escalation Prevention
-  Only admins can assign roles
-  Cannot assign higher-level role than own
-  Audit all role changes
-  Restrict permission modifications

 Token Security
-  Include tenant_id in JWT claims
-  Include role_id in JWT claims
-  Include permission_hash for quick checks
-  Validate claims on every request

---

  Performance Targets

| Metric | Target | Strategy |
|--------|--------|----------|
| Permission Check Latency | < ms | Cache permissions in JWT |
| Role Lookup | < ms | Redis cache with TTL |
| Tenant Query Filter | < ms | Index on tenant_id |
| Permission Matrix Load | < ms | Lazy load on demand |
| RBAC Middleware | < ms | Fast path for common roles |

---

  Deployment Strategy

 Phase : Database & Models (Days -)
. Create new migrations (non-breaking)
. Seed default roles and permissions
. Migrate existing users to roles

 Phase : Services & Logic (Days -)
. Deploy new services (backward compatible)
. Add permission enforcement gradually
. Monitor permission denials

 Phase : Middleware & Enforcement (Days -)
. Apply middleware to protected routes
. Validate permission enforcement
. Monitor for issues

 Phase : Frontend & Management (Days -)
. Deploy management UI
. Train users on new features
. Gather feedback

 Phase : Migration & Cutover (Days -)
. Migrate all existing roles
. Verify all permissions working
. Document mapping

---

  Definition of Done

 All domain models created and tested  
 Database migrations created and versioned  
 All RBAC services implemented  
 Permission middleware enforced on all protected routes  
 Tenant middleware enforces isolation  
 Frontend UI for role management complete  
 + unit and integration tests passing  
 Documentation complete and peer-reviewed  
 Security audit passed  
 Performance targets met  
 Backward compatibility maintained  
 Deployment procedure tested  

---

  Related Documents

- [ADVANCED_PERMISSIONS.md](ADVANCED_PERMISSIONS.md) - Permission system architecture
- [LOCAL_DEVELOPMENT.md](LOCAL_DEVELOPMENT.md) - Development setup
- [STAGING_VALIDATION_CHECKLIST.md](../STAGING_VALIDATION_CHECKLIST.md) - Deployment procedure

---

  Acceptance Criteria

 Functional
- [ ] Users can be assigned roles (Admin, Manager, Analyst, Viewer)
- [ ] Permissions are enforced on all protected endpoints
- [ ] Users cannot access resources outside their tenant
- [ ] Role permissions can be customized
- [ ] Permission changes take effect immediately (cached)

 Non-Functional
- [ ] Permission checks complete in < ms
- [ ] No performance degradation vs current system
- [ ] .% availability during permission lookups
- [ ] All permission denials logged

 Testing
- [ ] % coverage of permission logic
- [ ] All role hierarchy tested
- [ ] Cross-tenant access prevented in tests
- [ ] Privilege escalation attempts fail safely

---

Next Step: Begin Sprint  - Domain Models & Database implementation
