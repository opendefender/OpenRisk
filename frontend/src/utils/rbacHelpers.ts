/**
 * Advanced RBAC Permission Utilities
 * Provides permission checking and role-based UI rendering helpers
 */

export type PermissionAction = 'read' | 'create' | 'update' | 'delete' | 'manage' | 'all';
export type PermissionResource = 'users' | 'roles' | 'tenants' | 'reports' | 'audit' | 'connector' | 'assets' | 'incidents' | 'risks';

/**
 * Check if a permission string matches a pattern
 * Supports wildcards: resource:action or resource:* or *:action or *
 */
export const matchesPermissionPattern = (
  userPermission: string,
  requiredPermission: string
): boolean => {
  // Exact match
  if (userPermission === requiredPermission) return true;

  // Full wildcard
  if (userPermission === '*') return true;

  const [userRes, userAct] = userPermission.split(':');
  const [reqRes, reqAct] = requiredPermission.split(':');

  // Resource wildcard: resource:*
  if (userAct === '*' && userRes === reqRes) return true;

  // Action wildcard: *:action
  if (userRes === '*' && userAct === reqAct) return true;

  return false;
};

/**
 * Check if user has a specific permission
 */
export const hasPermission = (
  userPermissions: string[],
  requiredPermission: string
): boolean => {
  return userPermissions.some(perm =>
    matchesPermissionPattern(perm, requiredPermission)
  );
};

/**
 * Check if user has ALL required permissions
 */
export const hasAllPermissions = (
  userPermissions: string[],
  requiredPermissions: string[]
): boolean => {
  return requiredPermissions.every(perm =>
    hasPermission(userPermissions, perm)
  );
};

/**
 * Check if user has ANY of the required permissions
 */
export const hasAnyPermission = (
  userPermissions: string[],
  requiredPermissions: string[]
): boolean => {
  return requiredPermissions.some(perm =>
    hasPermission(userPermissions, perm)
  );
};

/**
 * Get all actions available for a resource
 */
export const getResourceActions = (resource: PermissionResource): PermissionAction[] => {
  const actions: Record<PermissionResource, PermissionAction[]> = {
    users: ['read', 'create', 'update', 'delete', 'manage'],
    roles: ['read', 'create', 'update', 'delete', 'manage'],
    tenants: ['read', 'create', 'update', 'delete', 'manage'],
    reports: ['read', 'create', 'update', 'delete'],
    audit: ['read', 'manage'],
    connector: ['read', 'create', 'update', 'delete'],
    assets: ['read', 'create', 'update', 'delete'],
    incidents: ['read', 'create', 'update', 'delete'],
    risks: ['read', 'create', 'update', 'delete', 'manage'],
  };

  return actions[resource] || ['read'];
};

/**
 * Format permission for display
 */
export const formatPermission = (permission: string): string => {
  const [resource, action] = permission.split(':');

  if (resource === '*' && action === '*') return 'Full Access';
  if (resource === '*') return `${action} Everything`;
  if (action === '*') return `All ${resource} Actions`;

  return `${capitalize(action)} ${capitalize(resource)}`;
};

/**
 * Helper to capitalize strings
 */
const capitalize = (str: string): string => {
  return str.charAt(0).toUpperCase() + str.slice(1).toLowerCase();
};

/**
 * Get role hierarchy level
 */
export const getRoleLevel = (roleLevel: number): { name: string; description: string; color: string } => {
  const levels = {
    0: { name: 'Viewer', description: 'Read-only access', color: 'zinc' },
    3: { name: 'Analyst', description: 'Can create and analyze', color: 'blue' },
    6: { name: 'Manager', description: 'Can manage resources', color: 'purple' },
    9: { name: 'Admin', description: 'Full access', color: 'red' },
  };

  return levels[roleLevel as keyof typeof levels] || levels[0];
};

/**
 * Permission-based feature flag
 */
export const isFeatureEnabled = (
  userPermissions: string[],
  feature: string
): boolean => {
  const featurePermissionMap: Record<string, string[]> = {
    'role-management': ['roles:manage'],
    'tenant-management': ['tenants:manage'],
    'user-management': ['users:manage'],
    'advanced-reports': ['reports:create', 'reports:manage'],
    'audit-logs': ['audit:read'],
    'api-tokens': ['users:manage'],
  };

  const requiredPerms = featurePermissionMap[feature] || [];
  return hasAnyPermission(userPermissions, requiredPerms);
};

/**
 * Get available actions for current user on a resource
 */
export const getAvailableActions = (
  userPermissions: string[],
  resource: PermissionResource
): PermissionAction[] => {
  const allActions = getResourceActions(resource);
  return allActions.filter(action =>
    hasPermission(userPermissions, `${resource}:${action}`)
  );
};

/**
 * Predefined permission sets for roles
 */
export const rolePermissionSets = {
  viewer: [
    'users:read',
    'roles:read',
    'tenants:read',
    'reports:read',
    'audit:read',
    'assets:read',
    'incidents:read',
    'risks:read',
  ] as const,
} as const;

// Define other role sets after viewer is available
const viewerPerms = rolePermissionSets.viewer;
const analystPerms = [
  ...viewerPerms,
  'risks:create',
  'risks:update',
  'incidents:create',
  'reports:create',
  'connector:read',
] as const;

const managerPerms = [
  ...analystPerms,
  'users:create',
  'users:update',
  'roles:read',
  'roles:create',
  'tenants:read',
  'assets:create',
  'assets:update',
] as const;

const adminPerms = ['*'] as const;

// Re-export with complete sets
export const rolePermissionSetsComplete = {
  viewer: viewerPerms,
  analyst: analystPerms,
  manager: managerPerms,
  admin: adminPerms,
};

/**
 * Check if permission is a protected admin-only permission
 */
export const isProtectedPermission = (permission: string): boolean => {
  const protectedPerms = [
    'roles:manage',
    'permissions:manage',
    'tenants:manage',
    'settings:manage',
    'audit-logs:manage',
    'api-keys:manage',
  ];
  return protectedPerms.includes(permission) || protectedPerms.some(p => permission.includes(p));
};

/**
 * Build a permission string from resource and action
 */
export const buildPermissionString = (
  resource: PermissionResource,
  action: PermissionAction
): string => {
  return `${resource}:${action}`;
};

/**
 * Parse permission string into resource and action
 */
export const parsePermission = (permission: string): { resource: string; action: string } => {
  const [resource, action] = permission.split(':');
  return { resource: resource || '*', action: action || '*' };
};

export default {
  matchesPermissionPattern,
  hasPermission,
  hasAllPermissions,
  hasAnyPermission,
  getResourceActions,
  formatPermission,
  getRoleLevel,
  isFeatureEnabled,
  getAvailableActions,
  rolePermissionSets: rolePermissionSetsComplete,
  isProtectedPermission,
  buildPermissionString,
  parsePermission,
};
