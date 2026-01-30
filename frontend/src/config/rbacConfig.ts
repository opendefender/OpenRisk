/
  Centralized RBAC Configuration
  Defines all permissions, resources, and role templates
 /

export type PermissionAction = 'create' | 'read' | 'update' | 'delete' | 'execute' | 'manage';
export type PermissionResource =
  | 'roles'
  | 'users'
  | 'permissions'
  | 'tenants'
  | 'audit-logs'
  | 'settings'
  | 'dashboards'
  | 'integrations'
  | 'api-keys';

/
  Resource-Action combinations that form valid permissions
 /
export const RBAC_RESOURCES = {
  ROLES: 'roles',
  USERS: 'users',
  PERMISSIONS: 'permissions',
  TENANTS: 'tenants',
  AUDIT_LOGS: 'audit-logs',
  SETTINGS: 'settings',
  DASHBOARDS: 'dashboards',
  INTEGRATIONS: 'integrations',
  API_KEYS: 'api-keys',
} as const;

export const RBAC_ACTIONS = {
  CREATE: 'create',
  READ: 'read',
  UPDATE: 'update',
  DELETE: 'delete',
  EXECUTE: 'execute',
  MANAGE: 'manage',
} as const;

/
  Feature flags configuration
 /
export const FEATURES = {
  ROLE_MANAGEMENT: 'role-management',
  TENANT_MANAGEMENT: 'tenant-management',
  USER_MANAGEMENT: 'user-management',
  AUDIT_LOGS: 'audit-logs',
  API_KEYS: 'api-keys',
  ADVANCED_ANALYTICS: 'advanced-analytics',
  CUSTOM_ROLES: 'custom-roles',
  BULK_OPERATIONS: 'bulk-operations',
  ROLE_TEMPLATES: 'role-templates',
  PERMISSION_ANALYTICS: 'permission-analytics',
} as const;

/
  Standard role permission templates
  Maps role names to their default permission sets
 /
export const ROLE_TEMPLATES = {
  VIEWER: {
    name: 'Viewer',
    level: ,
    description: 'Read-only access to dashboards and reports',
    permissions: [
      'dashboards:read',
      'audit-logs:read',
    ],
    features: ['audit-logs'],
  },
  ANALYST: {
    name: 'Analyst',
    level: ,
    description: 'Can create and manage dashboards and reports',
    permissions: [
      'dashboards:create',
      'dashboards:read',
      'dashboards:update',
      'dashboards:delete',
      'audit-logs:read',
      'integrations:read',
    ],
    features: ['audit-logs', 'role-templates'],
  },
  MANAGER: {
    name: 'Manager',
    level: ,
    description: 'Can manage users, dashboards, and team settings',
    permissions: [
      'users:create',
      'users:read',
      'users:update',
      'users:delete',
      'dashboards:manage',
      'settings:read',
      'settings:update',
      'audit-logs:read',
      'integrations:read',
      'integrations:update',
    ],
    features: ['audit-logs', 'user-management', 'role-templates', 'bulk-operations'],
  },
  ADMIN: {
    name: 'Administrator',
    level: ,
    description: 'Full system access including role and tenant management',
    permissions: [
      'roles:manage',
      'users:manage',
      'permissions:manage',
      'tenants:manage',
      'settings:manage',
      'audit-logs:manage',
      'dashboards:manage',
      'integrations:manage',
      'api-keys:manage',
    ],
    features: [
      'role-management',
      'tenant-management',
      'user-management',
      'audit-logs',
      'api-keys',
      'advanced-analytics',
      'custom-roles',
      'bulk-operations',
      'role-templates',
      'permission-analytics',
    ],
  },
} as const;

/
  Permission requirement levels for common operations
  Useful for determining minimum role needed
 /
export const PERMISSION_REQUIREMENTS = {
  VIEW_DASHBOARDS: ['dashboards:read'],
  CREATE_DASHBOARD: ['dashboards:create'],
  MANAGE_DASHBOARDS: ['dashboards:manage'],
  
  VIEW_USERS: ['users:read'],
  MANAGE_USERS: ['users:create', 'users:update', 'users:delete'],
  
  VIEW_ROLES: ['roles:read'],
  MANAGE_ROLES: ['roles:manage', 'permissions:manage'],
  
  MANAGE_TENANTS: ['tenants:manage'],
  
  VIEW_AUDIT_LOGS: ['audit-logs:read'],
  MANAGE_SETTINGS: ['settings:manage'],
  MANAGE_INTEGRATIONS: ['integrations:manage'],
  MANAGE_API_KEYS: ['api-keys:manage'],
} as const;

/
  Admin-only permissions that should never be granted to non-admin roles
 /
export const PROTECTED_PERMISSIONS = [
  'roles:manage',
  'permissions:manage',
  'tenants:manage',
  'settings:manage',
  'audit-logs:manage',
  'api-keys:manage',
] as const;

/
  Helper to build permission string
 /
export const buildPermission = (resource: PermissionResource, action: PermissionAction): string => {
  return ${resource}:${action};
};

/
  Helper to get all permissions for a role template
 /
export const getRolePermissions = (roleLevel: number | string) => {
  const roleKey = typeof roleLevel === 'string' 
    ? roleLevel.toUpperCase() 
    : Object.entries(ROLE_TEMPLATES).find(([, template]) => template.level === roleLevel)?.[];
  
  if (!roleKey || !(roleKey in ROLE_TEMPLATES)) {
    return [];
  }
  
  return ROLE_TEMPLATES[roleKey as keyof typeof ROLE_TEMPLATES].permissions;
};

/
  Helper to get features for a role
 /
export const getRoleFeatures = (roleLevel: number | string) => {
  const roleKey = typeof roleLevel === 'string' 
    ? roleLevel.toUpperCase() 
    : Object.entries(ROLE_TEMPLATES).find(([, template]) => template.level === roleLevel)?.[];
  
  if (!roleKey || !(roleKey in ROLE_TEMPLATES)) {
    return [];
  }
  
  return ROLE_TEMPLATES[roleKey as keyof typeof ROLE_TEMPLATES].features;
};

export default {
  RBAC_RESOURCES,
  RBAC_ACTIONS,
  FEATURES,
  ROLE_TEMPLATES,
  PERMISSION_REQUIREMENTS,
  PROTECTED_PERMISSIONS,
  buildPermission,
  getRolePermissions,
  getRoleFeatures,
};
