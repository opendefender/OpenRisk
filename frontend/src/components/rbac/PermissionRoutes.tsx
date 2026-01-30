import React from 'react';
import { useAuthStore } from '../store/authStore';
import { usePermissions } from '../hooks/usePermissions';
import type { PermissionAction, PermissionResource } from '../utils/rbacHelpers';

/
  Route guard component for protecting routes based on authentication
 /
export const ProtectedRoute: React.FC<{
  children: React.ReactNode;
  fallback?: React.ReactNode;
  requiredRole?: string;
  requiredLevel?: number;
}> = ({ children, fallback, requiredRole, requiredLevel }) => {
  const { user, isAuthenticated } = useAuthStore();

  if (!isAuthenticated || !user) {
    return <>{fallback || <div>Please log in to continue</div>}</>;
  }

  if (requiredRole && user.role !== requiredRole) {
    return <>{fallback || <div>You don't have the required role</div>}</>;
  }

  if (requiredLevel !== undefined) {
    const userLevel = user.role === 'Administrator' ?  : 
                      user.role === 'Manager' ?  : 
                      user.role === 'Analyst' ?  : ;
    
    if (userLevel < requiredLevel) {
      return <>{fallback || <div>Insufficient permissions</div>}</>;
    }
  }

  return <>{children}</>;
};

/
  Permission-based route guard
 /
export const PermissionRoute: React.FC<{
  children: React.ReactNode;
  permission?: string;
  permissions?: string[];
  requireAll?: boolean;
  fallback?: React.ReactNode;
  action?: PermissionAction;
  resource?: PermissionResource;
}> = ({
  children,
  permission,
  permissions = [],
  requireAll = true,
  fallback,
  action,
  resource,
}) => {
  const perms = usePermissions();
  const { isAuthenticated } = useAuthStore();

  if (!isAuthenticated) {
    return <>{fallback || <div>Please log in to continue</div>}</>;
  }

  let hasAccess = true;

  if (action && resource) {
    hasAccess = perms.canDo(action, resource);
  } else if (permission) {
    hasAccess = perms.can(permission);
  } else if (permissions.length > ) {
    hasAccess = requireAll
      ? perms.canAll(permissions)
      : perms.canAny(permissions);
  }

  if (!hasAccess) {
    return <>{fallback || <div>You don't have permission to access this page</div>}</>;
  }

  return <>{children}</>;
};

/
  Admin-only route guard
 /
export const AdminRoute: React.FC<{
  children: React.ReactNode;
  fallback?: React.ReactNode;
}> = ({ children, fallback }) => {
  const { isAdmin } = usePermissions();
  const { isAuthenticated } = useAuthStore();

  if (!isAuthenticated) {
    return <>{fallback || <div>Please log in to continue</div>}</>;
  }

  if (!isAdmin()) {
    return <>{fallback || <div>Admin access required</div>}</>;
  }

  return <>{children}</>;
};

/
  Feature flag route guard
 /
export const FeatureRoute: React.FC<{
  children: React.ReactNode;
  feature: string;
  fallback?: React.ReactNode;
}> = ({ children, feature, fallback }) => {
  const { isFeatureEnabled } = usePermissions();
  const { isAuthenticated } = useAuthStore();

  if (!isAuthenticated) {
    return <>{fallback || <div>Please log in to continue</div>}</>;
  }

  if (!isFeatureEnabled(feature)) {
    return <>{fallback || <div>This feature is not available for your account</div>}</>;
  }

  return <>{children}</>;
};

export default {
  ProtectedRoute,
  PermissionRoute,
  AdminRoute,
  FeatureRoute,
};
