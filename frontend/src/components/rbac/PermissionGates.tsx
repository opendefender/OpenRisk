import React from 'react';
import { usePermissions } from '../../hooks/usePermissions';
import type { PermissionAction, PermissionResource } from '../../utils/rbacHelpers';

/
  Component wrapper that shows children only if user has required permission
 /
export const CanAccess: React.FC<{
  permission: string;
  children: React.ReactNode;
  fallback?: React.ReactNode;
}> = ({ permission, children, fallback = null }) => {
  const permissions = usePermissions();

  if (permissions.can(permission)) {
    return <>{children}</>;
  }

  return <>{fallback}</>;
};

/
  Show children if user has ALL required permissions
 /
export const CanAccessAll: React.FC<{
  permissions: string[];
  children: React.ReactNode;
  fallback?: React.ReactNode;
}> = ({ permissions: requiredPerms, children, fallback = null }) => {
  const permissions = usePermissions();

  if (permissions.canAll(requiredPerms)) {
    return <>{children}</>;
  }

  return <>{fallback}</>;
};

/
  Show children if user has ANY of the required permissions
 /
export const CanAccessAny: React.FC<{
  permissions: string[];
  children: React.ReactNode;
  fallback?: React.ReactNode;
}> = ({ permissions: requiredPerms, children, fallback = null }) => {
  const permissions = usePermissions();

  if (permissions.canAny(requiredPerms)) {
    return <>{children}</>;
  }

  return <>{fallback}</>;
};

/
  Show children if user can perform specific resource action
 /
export const CanDo: React.FC<{
  action: PermissionAction;
  resource: PermissionResource;
  children: React.ReactNode;
  fallback?: React.ReactNode;
}> = ({ action, resource, children, fallback = null }) => {
  const permissions = usePermissions();

  if (permissions.canDo(action, resource)) {
    return <>{children}</>;
  }

  return <>{fallback}</>;
};

/
  Show children only if user is admin
 /
export const AdminOnly: React.FC<{
  children: React.ReactNode;
  fallback?: React.ReactNode;
}> = ({ children, fallback = null }) => {
  const permissions = usePermissions();

  if (permissions.isAdmin()) {
    return <>{children}</>;
  }

  return <>{fallback}</>;
};

/
  Show children only if feature is enabled for user
 /
export const IfFeatureEnabled: React.FC<{
  feature: string;
  children: React.ReactNode;
  fallback?: React.ReactNode;
}> = ({ feature, children, fallback = null }) => {
  const permissions = usePermissions();

  if (permissions.isFeatureEnabled(feature)) {
    return <>{children}</>;
  }

  return <>{fallback}</>;
};

/
  Button that disables based on permissions
 /
export const PermissionButton: React.FC<
  React.ButtonHTMLAttributes<HTMLButtonElement> & {
    permission?: string;
    action?: PermissionAction;
    resource?: PermissionResource;
    fallbackTooltip?: string;
  }
> = ({ permission, action, resource, fallbackTooltip, disabled, ...props }) => {
  const permissions = usePermissions();

  let hasAccess = true;

  if (permission) {
    hasAccess = permissions.can(permission);
  } else if (action && resource) {
    hasAccess = permissions.canDo(action, resource);
  }

  return (
    <button
      {...props}
      disabled={disabled || !hasAccess}
      title={!hasAccess ? fallbackTooltip : undefined}
    />
  );
};

export default {
  CanAccess,
  CanAccessAll,
  CanAccessAny,
  CanDo,
  AdminOnly,
  IfFeatureEnabled,
  PermissionButton,
};
