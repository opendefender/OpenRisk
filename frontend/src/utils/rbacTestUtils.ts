/**
 * RBAC Testing Utilities
 * Helper functions for testing RBAC components and logic
 */

import { ROLE_TEMPLATES } from '../config/rbacConfig';
import type { User } from '../store/authStore';
import type { RoleTemplate } from './roleTemplateUtils';

/**
 * Create mock user for testing
 */
export const createMockUser = (overrides?: Partial<User>): User => {
  return {
    id: 'user-123',
    email: 'test@example.com',
    name: 'Test User',
    role: 'Analyst',
    permissions: ['dashboards:read', 'dashboards:create', 'audit-logs:read'],
    ...overrides,
  };
};

/**
 * Create mock admin user
 */
export const createMockAdminUser = (overrides?: Partial<User>): User => {
  const adminTemplate = ROLE_TEMPLATES.ADMIN as RoleTemplate;
  return {
    id: 'admin-123',
    email: 'admin@example.com',
    name: 'Admin User',
    role: 'Administrator',
    permissions: adminTemplate.permissions,
    ...overrides,
  };
};

/**
 * Create mock viewer user
 */
export const createMockViewerUser = (overrides?: Partial<User>): User => {
  const viewerTemplate = ROLE_TEMPLATES.VIEWER as RoleTemplate;
  return {
    id: 'viewer-123',
    email: 'viewer@example.com',
    name: 'Viewer User',
    role: 'Viewer',
    permissions: viewerTemplate.permissions,
    ...overrides,
  };
};

/**
 * Create users with different roles for testing
 */
export const createUsersByRoles = (roles?: string[]): User[] => {
  const rolesArray = roles || ['Viewer', 'Analyst', 'Manager', 'Administrator'];
  return rolesArray.map((role, index) => {
    const template = Object.values(ROLE_TEMPLATES).find((t) => (t as RoleTemplate).name === role) as RoleTemplate;
    return {
      id: `user-${index}`,
      email: `${role.toLowerCase()}@example.com`,
      name: `${role} User`,
      role,
      permissions: template?.permissions || [],
    };
  });
};

/**
 * Generate random permission
 */
export const generateRandomPermission = (): string => {
  const resources = ['users', 'roles', 'tenants', 'dashboards', 'audit-logs'];
  const actions = ['read', 'create', 'update', 'delete', 'manage'];
  const resource = resources[Math.floor(Math.random() * resources.length)];
  const action = actions[Math.floor(Math.random() * actions.length)];
  return `${resource}:${action}`;
};

/**
 * Generate multiple random permissions
 */
export const generateRandomPermissions = (count: number = 5): string[] => {
  const permissions = new Set<string>();
  while (permissions.size < count) {
    permissions.add(generateRandomPermission());
  }
  return Array.from(permissions);
};

/**
 * Mock audit log entry
 */
export interface MockAuditEntry {
  id: string;
  userId: string;
  action: string;
  permission: string;
  allowed: boolean;
  timestamp: Date;
}

/**
 * Create mock audit log entry
 */
export const createMockAuditEntry = (overrides?: Partial<MockAuditEntry>): MockAuditEntry => {
  return {
    id: `audit-${Date.now()}`,
    userId: 'user-123',
    action: 'check',
    permission: generateRandomPermission(),
    allowed: Math.random() > 0.1, // 90% success rate
    timestamp: new Date(),
    ...overrides,
  };
};

/**
 * Create mock audit log entries
 */
export const createMockAuditLog = (count: number = 10): MockAuditEntry[] => {
  return Array.from({ length: count }, () => createMockAuditEntry());
};

/**
 * Test permission matching
 */
export const testPermissionMatching = (
  userPermissions: string[],
  requiredPermission: string
): { allowed: boolean; matchingPermission?: string } => {
  const match = userPermissions.find((perm) => {
    // Exact match
    if (perm === requiredPermission) return true;
    // Wildcard matches
    if (perm === '*') return true;
    const [userRes, userAct] = perm.split(':');
    const [reqRes, reqAct] = requiredPermission.split(':');
    if (userAct === '*' && userRes === reqRes) return true;
    if (userRes === '*' && userAct === reqAct) return true;
    return false;
  });

  return {
    allowed: !!match,
    matchingPermission: match,
  };
};

/**
 * Generate test scenarios
 */
export interface TestScenario {
  name: string;
  user: User;
  permission: string;
  expectedResult: boolean;
}

/**
 * Create test scenarios for RBAC
 */
export const createTestScenarios = (): TestScenario[] => {
  const adminUser = createMockAdminUser();
  const viewerUser = createMockViewerUser();
  const analystUser = createMockUser();

  return [
    {
      name: 'Admin can access anything',
      user: adminUser,
      permission: 'roles:manage',
      expectedResult: true,
    },
    {
      name: 'Admin can access anything (wildcard)',
      user: adminUser,
      permission: 'random:permission',
      expectedResult: true,
    },
    {
      name: 'Viewer cannot manage roles',
      user: viewerUser,
      permission: 'roles:manage',
      expectedResult: false,
    },
    {
      name: 'Viewer can read dashboards',
      user: viewerUser,
      permission: 'dashboards:read',
      expectedResult: false, // Viewer template may not include this
    },
    {
      name: 'Analyst can read dashboards',
      user: analystUser,
      permission: 'dashboards:read',
      expectedResult: true,
    },
    {
      name: 'Analyst cannot manage tenants',
      user: analystUser,
      permission: 'tenants:manage',
      expectedResult: false,
    },
  ];
};

/**
 * Run test scenarios
 */
export const runTestScenarios = (
  scenarios: TestScenario[],
  checkPermissionFn: (user: User, permission: string) => boolean
): { name: string; passed: boolean; expected: boolean; actual: boolean }[] => {
  return scenarios.map((scenario) => {
    const actual = checkPermissionFn(scenario.user, scenario.permission);
    return {
      name: scenario.name,
      passed: actual === scenario.expectedResult,
      expected: scenario.expectedResult,
      actual,
    };
  });
};

/**
 * Generate coverage report for role template
 */
export interface CoverageReport {
  roleName: string;
  totalPermissions: number;
  coveredByTemplate: number;
  coverage: number;
  missingPermissions: string[];
}

export const generateRoleCoverageReport = (
  template: RoleTemplate,
  allPossiblePermissions: string[]
): CoverageReport => {
  const covered = allPossiblePermissions.filter((p) =>
    template.permissions.includes(p)
  );
  const missing = allPossiblePermissions.filter(
    (p) => !template.permissions.includes(p)
  );

  return {
    roleName: template.name,
    totalPermissions: allPossiblePermissions.length,
    coveredByTemplate: covered.length,
    coverage: (covered.length / allPossiblePermissions.length) * 100,
    missingPermissions: missing,
  };
};

/**
 * Mock API responses for testing
 */
export const mockApiResponses = {
  /**
   * Mock successful permission check
   */
  checkPermissionSuccess: (permission: string) => ({
    permission,
    allowed: true,
    timestamp: new Date(),
  }),

  /**
   * Mock failed permission check
   */
  checkPermissionFailed: (permission: string, reason?: string) => ({
    permission,
    allowed: false,
    reason: reason || 'Insufficient permissions',
    timestamp: new Date(),
  }),

  /**
   * Mock role list response
   */
  roleListResponse: (roles: RoleTemplate[] = []) => ({
    roles: roles.length > 0 ? roles : Object.values(ROLE_TEMPLATES),
    total: roles.length || Object.keys(ROLE_TEMPLATES).length,
  }),

  /**
   * Mock audit log response
   */
  auditLogResponse: (entries: MockAuditEntry[] = []) => ({
    entries: entries.length > 0 ? entries : createMockAuditLog(),
    total: entries.length || 10,
    page: 1,
    pageSize: 50,
  }),
};

/**
 * Test utilities summary
 */
export const testUtilsSummary = {
  users: {
    createMockUser,
    createMockAdminUser,
    createMockViewerUser,
    createUsersByRoles,
  },
  permissions: {
    generateRandomPermission,
    generateRandomPermissions,
    testPermissionMatching,
  },
  audit: {
    createMockAuditEntry,
    createMockAuditLog,
  },
  scenarios: {
    createTestScenarios,
    runTestScenarios,
  },
  coverage: {
    generateRoleCoverageReport,
  },
  api: mockApiResponses,
};

export default testUtilsSummary;
