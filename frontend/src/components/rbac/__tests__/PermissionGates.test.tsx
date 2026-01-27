import React from 'react';
import { render, screen } from '@testing-library/react';
import '@testing-library/jest-dom';
import {
  CanAccess,
  CanAccessAll,
  CanAccessAny,
  CanDo,
  AdminOnly,
  IfFeatureEnabled,
  PermissionButton,
} from '../../components/rbac/PermissionGates';
import * as usePermissionsModule from '../../hooks/usePermissions';

// Mock the usePermissions hook
jest.mock('../../hooks/usePermissions');

const mockUsePermissions = usePermissionsModule.usePermissions as jest.MockedFunction<
  typeof usePermissionsModule.usePermissions
>;

describe('PermissionGates Components', () => {
  afterEach(() => {
    jest.clearAllMocks();
  });

  describe('CanAccess', () => {
    it('should render children when user has permission', () => {
      mockUsePermissions.mockReturnValue({
        can: jest.fn().mockReturnValue(true),
        canAll: jest.fn(),
        canAny: jest.fn(),
        canDo: jest.fn(),
        isAdmin: jest.fn(),
        isFeatureEnabled: jest.fn(),
      });

      render(
        <CanAccess permission="users:read">
          <div>Allowed Content</div>
        </CanAccess>
      );

      expect(screen.getByText('Allowed Content')).toBeInTheDocument();
    });

    it('should render fallback when user lacks permission', () => {
      mockUsePermissions.mockReturnValue({
        can: jest.fn().mockReturnValue(false),
        canAll: jest.fn(),
        canAny: jest.fn(),
        canDo: jest.fn(),
        isAdmin: jest.fn(),
        isFeatureEnabled: jest.fn(),
      });

      render(
        <CanAccess permission="admin:delete" fallback={<div>Access Denied</div>}>
          <div>Allowed Content</div>
        </CanAccess>
      );

      expect(screen.getByText('Access Denied')).toBeInTheDocument();
      expect(screen.queryByText('Allowed Content')).not.toBeInTheDocument();
    });

    it('should render null fallback by default', () => {
      mockUsePermissions.mockReturnValue({
        can: jest.fn().mockReturnValue(false),
        canAll: jest.fn(),
        canAny: jest.fn(),
        canDo: jest.fn(),
        isAdmin: jest.fn(),
        isFeatureEnabled: jest.fn(),
      });

      const { container } = render(
        <CanAccess permission="users:delete">
          <div>Allowed Content</div>
        </CanAccess>
      );

      expect(screen.queryByText('Allowed Content')).not.toBeInTheDocument();
    });
  });

  describe('CanAccessAll', () => {
    it('should render children when user has ALL permissions', () => {
      mockUsePermissions.mockReturnValue({
        can: jest.fn(),
        canAll: jest.fn().mockReturnValue(true),
        canAny: jest.fn(),
        canDo: jest.fn(),
        isAdmin: jest.fn(),
        isFeatureEnabled: jest.fn(),
      });

      render(
        <CanAccessAll permissions={['users:read', 'users:write']}>
          <div>Multiple Permissions Content</div>
        </CanAccessAll>
      );

      expect(screen.getByText('Multiple Permissions Content')).toBeInTheDocument();
    });

    it('should render fallback when user lacks ANY required permission', () => {
      mockUsePermissions.mockReturnValue({
        can: jest.fn(),
        canAll: jest.fn().mockReturnValue(false),
        canAny: jest.fn(),
        canDo: jest.fn(),
        isAdmin: jest.fn(),
        isFeatureEnabled: jest.fn(),
      });

      render(
        <CanAccessAll
          permissions={['users:read', 'users:write', 'admin:delete']}
          fallback={<div>Insufficient Permissions</div>}
        >
          <div>Multiple Permissions Content</div>
        </CanAccessAll>
      );

      expect(screen.getByText('Insufficient Permissions')).toBeInTheDocument();
    });
  });

  describe('CanAccessAny', () => {
    it('should render children when user has ANY permission', () => {
      mockUsePermissions.mockReturnValue({
        can: jest.fn(),
        canAll: jest.fn(),
        canAny: jest.fn().mockReturnValue(true),
        canDo: jest.fn(),
        isAdmin: jest.fn(),
        isFeatureEnabled: jest.fn(),
      });

      render(
        <CanAccessAny permissions={['users:read', 'users:write']}>
          <div>Any Permission Content</div>
        </CanAccessAny>
      );

      expect(screen.getByText('Any Permission Content')).toBeInTheDocument();
    });

    it('should render fallback when user lacks ALL permissions', () => {
      mockUsePermissions.mockReturnValue({
        can: jest.fn(),
        canAll: jest.fn(),
        canAny: jest.fn().mockReturnValue(false),
        canDo: jest.fn(),
        isAdmin: jest.fn(),
        isFeatureEnabled: jest.fn(),
      });

      render(
        <CanAccessAny
          permissions={['admin:delete', 'admin:manage']}
          fallback={<div>No Permissions</div>}
        >
          <div>Any Permission Content</div>
        </CanAccessAny>
      );

      expect(screen.getByText('No Permissions')).toBeInTheDocument();
    });
  });

  describe('CanDo', () => {
    it('should render children when user can perform action on resource', () => {
      mockUsePermissions.mockReturnValue({
        can: jest.fn(),
        canAll: jest.fn(),
        canAny: jest.fn(),
        canDo: jest.fn().mockReturnValue(true),
        isAdmin: jest.fn(),
        isFeatureEnabled: jest.fn(),
      });

      render(
        <CanDo action="read" resource="risks">
          <div>Can Read Risks</div>
        </CanDo>
      );

      expect(screen.getByText('Can Read Risks')).toBeInTheDocument();
    });

    it('should render fallback when user cannot perform action', () => {
      mockUsePermissions.mockReturnValue({
        can: jest.fn(),
        canAll: jest.fn(),
        canAny: jest.fn(),
        canDo: jest.fn().mockReturnValue(false),
        isAdmin: jest.fn(),
        isFeatureEnabled: jest.fn(),
      });

      render(
        <CanDo action="delete" resource="users" fallback={<div>Cannot Delete</div>}>
          <div>Can Delete Users</div>
        </CanDo>
      );

      expect(screen.getByText('Cannot Delete')).toBeInTheDocument();
    });
  });

  describe('AdminOnly', () => {
    it('should render children when user is admin', () => {
      mockUsePermissions.mockReturnValue({
        can: jest.fn(),
        canAll: jest.fn(),
        canAny: jest.fn(),
        canDo: jest.fn(),
        isAdmin: jest.fn().mockReturnValue(true),
        isFeatureEnabled: jest.fn(),
      });

      render(
        <AdminOnly>
          <div>Admin Panel</div>
        </AdminOnly>
      );

      expect(screen.getByText('Admin Panel')).toBeInTheDocument();
    });

    it('should render fallback when user is not admin', () => {
      mockUsePermissions.mockReturnValue({
        can: jest.fn(),
        canAll: jest.fn(),
        canAny: jest.fn(),
        canDo: jest.fn(),
        isAdmin: jest.fn().mockReturnValue(false),
        isFeatureEnabled: jest.fn(),
      });

      render(
        <AdminOnly fallback={<div>Admin Access Required</div>}>
          <div>Admin Panel</div>
        </AdminOnly>
      );

      expect(screen.getByText('Admin Access Required')).toBeInTheDocument();
    });
  });

  describe('IfFeatureEnabled', () => {
    it('should render children when feature is enabled', () => {
      mockUsePermissions.mockReturnValue({
        can: jest.fn(),
        canAll: jest.fn(),
        canAny: jest.fn(),
        canDo: jest.fn(),
        isAdmin: jest.fn(),
        isFeatureEnabled: jest.fn().mockReturnValue(true),
      });

      render(
        <IfFeatureEnabled feature="advancedReporting">
          <div>Advanced Reporting Feature</div>
        </IfFeatureEnabled>
      );

      expect(screen.getByText('Advanced Reporting Feature')).toBeInTheDocument();
    });

    it('should render fallback when feature is disabled', () => {
      mockUsePermissions.mockReturnValue({
        can: jest.fn(),
        canAll: jest.fn(),
        canAny: jest.fn(),
        canDo: jest.fn(),
        isAdmin: jest.fn(),
        isFeatureEnabled: jest.fn().mockReturnValue(false),
      });

      render(
        <IfFeatureEnabled feature="advancedReporting" fallback={<div>Feature Unavailable</div>}>
          <div>Advanced Reporting Feature</div>
        </IfFeatureEnabled>
      );

      expect(screen.getByText('Feature Unavailable')).toBeInTheDocument();
    });
  });

  describe('PermissionButton', () => {
    it('should render enabled button when user has permission', () => {
      mockUsePermissions.mockReturnValue({
        can: jest.fn().mockReturnValue(true),
        canAll: jest.fn(),
        canAny: jest.fn(),
        canDo: jest.fn(),
        isAdmin: jest.fn(),
        isFeatureEnabled: jest.fn(),
      });

      render(
        <PermissionButton permission="risks:create">
          Create Risk
        </PermissionButton>
      );

      const button = screen.getByRole('button');
      expect(button).not.toBeDisabled();
    });

    it('should render disabled button when user lacks permission', () => {
      mockUsePermissions.mockReturnValue({
        can: jest.fn().mockReturnValue(false),
        canAll: jest.fn(),
        canAny: jest.fn(),
        canDo: jest.fn(),
        isAdmin: jest.fn(),
        isFeatureEnabled: jest.fn(),
      });

      render(
        <PermissionButton permission="risks:delete" fallbackTooltip="Insufficient permissions">
          Delete Risk
        </PermissionButton>
      );

      const button = screen.getByRole('button');
      expect(button).toBeDisabled();
      expect(button).toHaveAttribute('title', 'Insufficient permissions');
    });

    it('should work with action and resource parameters', () => {
      mockUsePermissions.mockReturnValue({
        can: jest.fn(),
        canAll: jest.fn(),
        canAny: jest.fn(),
        canDo: jest.fn().mockReturnValue(true),
        isAdmin: jest.fn(),
        isFeatureEnabled: jest.fn(),
      });

      render(
        <PermissionButton action="update" resource="risks">
          Update Risk
        </PermissionButton>
      );

      const button = screen.getByRole('button');
      expect(button).not.toBeDisabled();
    });

    it('should respect existing disabled prop', () => {
      mockUsePermissions.mockReturnValue({
        can: jest.fn().mockReturnValue(true),
        canAll: jest.fn(),
        canAny: jest.fn(),
        canDo: jest.fn(),
        isAdmin: jest.fn(),
        isFeatureEnabled: jest.fn(),
      });

      render(
        <PermissionButton permission="risks:create" disabled>
          Create Risk
        </PermissionButton>
      );

      const button = screen.getByRole('button');
      expect(button).toBeDisabled();
    });
  });
});
