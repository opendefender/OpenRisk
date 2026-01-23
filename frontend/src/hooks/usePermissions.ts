import { useMemo } from 'react';
import { useAuthStore } from './useAuthStore';
import {
  hasPermission,
  hasAllPermissions,
  hasAnyPermission,
  getAvailableActions,
  isFeatureEnabled,
  getRoleLevel,
} from '../utils/rbacHelpers';
import type { PermissionAction, PermissionResource } from '../utils/rbacHelpers';

/**
 * Hook for permission-based access control
 * Provides methods to check user permissions
 */
export const usePermissions = () => {
  const user = useAuthStore((state) => state.user);
  const userPermissions = user?.permissions || [];

  return useMemo(
    () => ({
      /**
       * Check if user has a specific permission
       */
      can: (permission: string) => hasPermission(userPermissions, permission),

      /**
       * Check if user has ALL required permissions
       */
      canAll: (permissions: string[]) => hasAllPermissions(userPermissions, permissions),

      /**
       * Check if user has ANY of the required permissions
       */
      canAny: (permissions: string[]) => hasAnyPermission(userPermissions, permissions),

      /**
       * Check specific resource + action permission
       */
      canDo: (action: PermissionAction, resource: PermissionResource) =>
        hasPermission(userPermissions, `${resource}:${action}`),

      /**
       * Get available actions for a resource
       */
      availableActions: (resource: PermissionResource) =>
        getAvailableActions(userPermissions, resource),

      /**
       * Check if a feature is enabled
       */
      isFeatureEnabled: (feature: string) =>
        isFeatureEnabled(userPermissions, feature),

      /**
       * Get user's role level info
       */
      roleLevel: user?.role_level ? getRoleLevel(user.role_level) : null,

      /**
       * Get all user permissions
       */
      permissions: userPermissions,

      /**
       * Check if user is admin
       */
      isAdmin: () => hasPermission(userPermissions, '*'),
    }),
    [userPermissions]
  );
};

export default usePermissions;
