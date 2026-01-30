 RBAC Frontend Components Documentation

 Overview

This document provides comprehensive guidance on using the RBAC (Role-Based Access Control) components, hooks, and utilities in the frontend application.

 Core RBAC System

 Permissions

Permissions follow the pattern: resource:action

Resources:
- roles - Role management
- users - User management
- tenants - Tenant management
- permissions - Permission management
- audit-logs - Audit log access
- settings - System settings
- dashboards - Dashboard management
- integrations - Integration management
- api-keys - API key management

Actions:
- read - View/retrieve
- create - Create new
- update - Modify existing
- delete - Remove
- manage - Full control (typically implies all actions)
- execute - Run/execute

 Wildcard Support

Permissions support flexible wildcard matching:
-  - All permissions
- resource: - All actions on a resource
- :action - Action on any resource

Example:
typescript
// User with "roles:" can read, create, update, and delete roles
// User with ":read" can read any resource
// User with "" can do anything


 Usage Patterns

 . Permission Hook (usePermissions)

The most powerful way to check permissions in components:

typescript
import { usePermissions } from '../hooks/usePermissions';

export const MyComponent = () => {
  const perms = usePermissions();

  // Check single permission
  if (perms.can('users:create')) {
    // Render create user button
  }

  // Check multiple permissions (all required)
  if (perms.canAll(['users:read', 'users:update'])) {
    // Render user edit form
  }

  // Check multiple permissions (any)
  if (perms.canAny(['tenants:manage', 'tenants:create'])) {
    // Render tenant creation
  }

  // Check resource-action pair
  if (perms.canDo('delete', 'users')) {
    // Render delete button
  }

  // Check if feature is enabled
  if (perms.isFeatureEnabled('role-templates')) {
    // Show role template selection
  }

  // Check admin status
  if (perms.isAdmin()) {
    // Show admin-only features
  }

  // Get available actions for resource
  const actions = perms.availableActions('roles');
  // Returns: ['read', 'create', 'update', 'delete', 'manage']

  return (
    <div>
      {perms.can('users:read') && <UsersList />}
      {perms.isAdmin() && <AdminPanel />}
    </div>
  );
};


 . Permission Gate Components

Declarative conditional rendering with component wrappers:

typescript
import {
  CanAccess,
  CanAccessAll,
  CanAccessAny,
  CanDo,
  AdminOnly,
  IfFeatureEnabled,
} from '../components/rbac/PermissionGates';

// Single permission
<CanAccess permission="users:create">
  <CreateUserButton />
  <CreateUserModal />
</CanAccess>

// All permissions required
<CanAccessAll permissions={['users:read', 'users:update']}>
  <EditUserForm />
</CanAccessAll>

// Any permission required
<CanAccessAny permissions={['tenants:create', 'tenants:manage']}>
  <CreateTenantButton />
</CanAccessAny>

// Resource-action check
<CanDo action="delete" resource="users">
  <DeleteButton />
</CanDo>

// Admin only
<AdminOnly>
  <AdvancedSettings />
</AdminOnly>

// Feature enabled
<IfFeatureEnabled feature="role-templates">
  <RoleTemplateSelector />
</IfFeatureEnabled>

// With fallback
<CanAccess 
  permission="users:read"
  fallback={<div>You don't have access to view users</div>}
>
  <UsersList />
</CanAccess>


 . Permission Button Component

Button that automatically disables based on permissions:

typescript
import { PermissionButton } from '../components/rbac/PermissionGates';

// Check single permission
<PermissionButton
  permission="users:create"
  onClick={handleCreate}
  fallbackTooltip="You don't have permission to create users"
>
  Create User
</PermissionButton>

// Check resource-action pair
<PermissionButton
  action="delete"
  resource="users"
  onClick={handleDelete}
  fallbackTooltip="You can't delete users"
>
  Delete
</PermissionButton>


 . RBAC Utility Functions

Direct permission checking utilities:

typescript
import {
  hasPermission,
  hasAllPermissions,
  hasAnyPermission,
  matchesPermissionPattern,
  isProtectedPermission,
  buildPermissionString,
  parsePermission,
  getRoleLevel,
  isFeatureEnabled,
} from '../utils/rbacHelpers';

// Basic permission checks
const userPerms = ['users:read', 'users:create'];

hasPermission(userPerms, 'users:read'); // true
hasPermission(userPerms, 'users:delete'); // false

hasAllPermissions(userPerms, ['users:read', 'users:create']); // true
hasAllPermissions(userPerms, ['users:read', 'users:delete']); // false

hasAnyPermission(userPerms, ['users:delete', 'users:read']); // true

// Wildcard matching
matchesPermissionPattern('users:', 'users:delete'); // true
matchesPermissionPattern(':read', 'users:read'); // true
matchesPermissionPattern('', 'anything'); // true

// Protected permissions (admin-only)
isProtectedPermission('roles:manage'); // true
isProtectedPermission('users:read'); // false

// Permission string operations
buildPermissionString('users', 'create'); // "users:create"
parsePermission('users:delete'); // { resource: 'users', action: 'delete' }

// Role information
getRoleLevel('Administrator'); // { name: 'Administrator', level:  }

// Feature flags
isFeatureEnabled('role-management', ['role-management', 'user-management']); // true


 Role Templates

 Standard Roles

Four built-in role templates with permissions:

Viewer (Level )
- Read-only access to dashboards and audit logs
- No modification capabilities
- Features: audit-logs

Analyst (Level )
- Can create and manage dashboards
- Can view integrations and audit logs
- No user or tenant management
- Features: audit-logs, role-templates

Manager (Level )
- Can manage users and dashboards
- Can update settings and integrations
- Cannot manage roles or tenants
- Features: audit-logs, user-management, role-templates, bulk-operations

Administrator (Level )
- Full system access
- Can manage all resources
- All features enabled
- Features: all

 Using Role Templates

typescript
import { ROLE_TEMPLATES, FEATURES } from '../config/rbacConfig';

// Get template for specific role
const adminRole = ROLE_TEMPLATES.ADMIN;
console.log(adminRole.permissions); // All permissions
console.log(adminRole.features); // All features enabled

// Build custom role from template
function createCustomRole() {
  const managerTemplate = ROLE_TEMPLATES.MANAGER;
  const customPermissions = [
    ...managerTemplate.permissions,
    'api-keys:read', // Add API key viewing
  ];
  
  return {
    name: 'Senior Manager',
    level: ,
    permissions: customPermissions,
  };
}


 Advanced Patterns

 Conditional Rendering Based on Permissions

typescript
export const AdminDashboard = () => {
  const { can, isAdmin } = usePermissions();

  return (
    <div>
      <h>Admin Dashboard</h>
      
      {isAdmin() ? (
        <AdminPanel />
      ) : (
        <AccessDenied />
      )}

      <Section title="Role Management">
        <CanAccess permission="roles:manage">
          <RoleManagementPanel />
        </CanAccess>
      </Section>

      <Section title="Tenant Management">
        <CanAccess permission="tenants:manage">
          <TenantManagementPanel />
        </CanAccess>
      </Section>

      <Section title="User Management">
        <CanAccessAll permissions={['users:read', 'users:update']}>
          <UserManagementPanel />
        </CanAccessAll>
      </Section>

      <Section title="Audit Logs">
        <CanAccess permission="audit-logs:read">
          <AuditLogViewer />
        </CanAccess>
      </Section>
    </div>
  );
};


 Dynamic UI Enablement

typescript
export const UserForm = ({ userId }: { userId: string }) => {
  const { canDo, can } = usePermissions();
  const isEditMode = !!userId;
  const canEdit = isEditMode && canDo('update', 'users');
  const canDelete = isEditMode && canDo('delete', 'users');
  const canCreate = !isEditMode && can('users:create');
  const canView = can('users:read');

  if (!canView && !canEdit && !canCreate) {
    return <AccessDenied />;
  }

  return (
    <form>
      <FormFields disabled={!canEdit && isEditMode} />
      
      <PermissionButton
        permission="users:create"
        disabled={isEditMode}
        onClick={handleCreate}
      >
        Create User
      </PermissionButton>

      <PermissionButton
        action="update"
        resource="users"
        disabled={!isEditMode}
        onClick={handleUpdate}
      >
        Update User
      </PermissionButton>

      <PermissionButton
        action="delete"
        resource="users"
        disabled={!isEditMode}
        onClick={handleDelete}
        fallbackTooltip="You don't have permission to delete users"
      >
        Delete User
      </PermissionButton>
    </form>
  );
};


 Feature-Gated Components

typescript
export const Dashboard = () => {
  return (
    <div>
      <BaseMetrics />
      
      <IfFeatureEnabled feature="advanced-analytics">
        <AdvancedAnalyticsSection />
      </IfFeatureEnabled>

      <IfFeatureEnabled feature="permission-analytics">
        <PermissionAnalyticsPanel />
      </IfFeatureEnabled>

      <IfFeatureEnabled feature="custom-roles">
        <CustomRoleBuilderSection />
      </IfFeatureEnabled>

      <IfFeatureEnabled feature="bulk-operations">
        <BulkOperationsPanel />
      </IfFeatureEnabled>
    </div>
  );
};


 Best Practices

. Use the Permission Hook for Logic
   - Prefer usePermissions() hook for component logic
   - More flexible than gate components
   - Better performance with memoization

. Use Gate Components for UI
   - Use <CanAccess> and similar for conditional rendering
   - Cleaner, more declarative code
   - Better for non-technical developers to understand

. Check Permissions Early
   - Check at page level with <ProtectedRoute>
   - Also check at component level for granularity
   - Use fallback UI to explain access denial

. Don't Rely on Frontend Checks Alone
   - Always validate permissions on the backend
   - Frontend checks are for UX only
   - Assume all data could be accessed if backend isn't secure

. Document Permission Requirements
   - Add comments explaining which permissions are needed
   - Keep permission requirements centralized
   - Use PERMISSION_REQUIREMENTS config

. Use Appropriate Granularity
   - Don't make permissions too granular (hard to manage)
   - Don't make permissions too broad (security risk)
   - Follow established pattern: resource:action

. Test Permission Combinations
   - Test with different role levels
   - Verify cascading permissions work correctly
   - Test wildcard matching in all scenarios

 API Integration

The RBAC system integrates with the backend API:

Get User with Permissions

GET /auth/me
Response: { user: { id, email, role, permissions: ['users:read', ...] } }


List Roles

GET /rbac/roles
Response: { roles: [{ id, name, level, permissions: [...] }] }


Create Role

POST /rbac/roles
Body: { name, level, permissions: [...] }


Manage Role Permissions

PUT /rbac/roles/:roleId
Body: { permissions: [...] }


Check Permission

GET /rbac/check-permission?permission=users:create
Response: { allowed: true/false }


All permission checks happen in the useAuthStore and are passed from the backend during login.

 Troubleshooting

 Permissions Not Working
- Check that user's permissions are loaded in useAuthStore
- Verify permission format: resource:action
- Check wildcard matching logic
- Ensure role level is correctly mapped

 Components Not Rendering
- Verify permission string matches user's actual permissions
- Check for typos in permission names
- Test with admin user to confirm logic works
- Check browser console for errors

 Performance Issues
- usePermissions hook is memoized but check if custom hooks are
- Avoid creating new permission arrays in render methods
- Cache permission checks in variables
- Minimize re-renders of permission-dependent components

 Migration Guide

 From Previous System
If upgrading from a simpler permission system:

Old:
typescript
if (user.isAdmin) { ... }


New:
typescript
const { isAdmin } = usePermissions();
if (isAdmin()) { ... }


Old:
typescript
{user.role === 'admin' && <AdminPanel />}


New:
typescript
<AdminOnly><AdminPanel /></AdminOnly>


 Files Reference

- Hooks: src/hooks/usePermissions.ts
- Utilities: src/utils/rbacHelpers.ts
- Components: src/components/rbac/PermissionGates.tsx
- Config: src/config/rbacConfig.ts
- Pages: src/pages/RoleManagement.tsx, src/pages/TenantManagement.tsx
- Settings: src/features/settings/RBACTab.tsx
