/**
 * Role Template Utilities
 * Functions for creating, modifying, and managing roles from templates
 */

import { ROLE_TEMPLATES } from '../config/rbacConfig';

export interface RoleTemplate {
  name: string;
  level: number;
  description: string;
  permissions: string[];
  features: string[];
}

export interface CustomRole extends RoleTemplate {
  parentTemplate?: string;
  customPermissions: string[]; // Permissions added beyond template
  excludedPermissions: string[]; // Permissions removed from template
}

/**
 * Get template by name
 */
export const getTemplateByName = (name: string): RoleTemplate | null => {
  const key = name.toUpperCase();
  const template = ROLE_TEMPLATES[key as keyof typeof ROLE_TEMPLATES];
  if (template) {
    return {
      ...template,
      permissions: [...template.permissions],
      features: [...template.features],
    };
  }
  return null;
};

/**
 * Get all available templates
 */
export const getAllTemplates = (): RoleTemplate[] => {
  return Object.values(ROLE_TEMPLATES).map(t => ({
    ...t,
    permissions: [...t.permissions],
    features: [...t.features],
  }));
};

/**
 * Get template by level
 */
export const getTemplateByLevel = (level: number): RoleTemplate | null => {
  const template = Object.values(ROLE_TEMPLATES).find(
    (t) => t.level === level
  );
  if (template) {
    return {
      ...template,
      permissions: [...template.permissions],
      features: [...template.features],
    };
  }
  return null;
};

/**
 * Create custom role from template
 */
export const createCustomRoleFromTemplate = (
  templateName: string,
  customName: string,
  customLevel?: number,
  additionalPermissions: string[] = [],
  excludedPermissions: string[] = []
): CustomRole | null => {
  const template = getTemplateByName(templateName);
  if (!template) return null;

  const finalPermissions = [
    ...template.permissions.filter((p) => !excludedPermissions.includes(p)),
    ...additionalPermissions,
  ];

  // Remove duplicates
  const uniquePermissions = Array.from(new Set(finalPermissions));

  return {
    name: customName,
    level: customLevel ?? template.level,
    description: `Custom role based on ${templateName}`,
    permissions: uniquePermissions,
    features: template.features,
    parentTemplate: templateName,
    customPermissions: additionalPermissions,
    excludedPermissions,
  };
};

/**
 * Compare two role templates
 */
export const compareTemplates = (
  template1: RoleTemplate,
  template2: RoleTemplate
): {
  commonPermissions: string[];
  onlyInTemplate1: string[];
  onlyInTemplate2: string[];
  allPermissions: string[];
} => {
  const perms1 = new Set(template1.permissions);
  const perms2 = new Set(template2.permissions);
  const allPerms = new Set([...template1.permissions, ...template2.permissions]);

  return {
    commonPermissions: template1.permissions.filter((p) => perms2.has(p)),
    onlyInTemplate1: template1.permissions.filter((p) => !perms2.has(p)),
    onlyInTemplate2: template2.permissions.filter((p) => !perms1.has(p)),
    allPermissions: Array.from(allPerms),
  };
};

/**
 * Get permission coverage (what % of permissions does role have)
 */
export const getPermissionCoverage = (
  rolePermissions: string[],
  allAvailablePermissions: string[]
): number => {
  if (allAvailablePermissions.length === 0) return 0;
  const coverage = rolePermissions.filter((p) =>
    allAvailablePermissions.includes(p)
  ).length;
  return Math.round((coverage / allAvailablePermissions.length) * 100);
};

/**
 * Check if role meets minimum permission requirements
 */
export const roleHasMinimumPermissions = (
  rolePermissions: string[],
  minimumRequired: string[]
): boolean => {
  return minimumRequired.every((perm) => rolePermissions.includes(perm));
};

/**
 * Get recommended template based on use case
 */
export const getRecommendedTemplate = (useCase: string): RoleTemplate | null => {
  const useCaseLower = useCase.toLowerCase();

  if (
    useCaseLower.includes('read') ||
    useCaseLower.includes('view') ||
    useCaseLower.includes('viewer')
  ) {
    return getTemplateByName('VIEWER');
  }

  if (
    useCaseLower.includes('analyst') ||
    useCaseLower.includes('analyst') ||
    useCaseLower.includes('create dashboard')
  ) {
    return getTemplateByName('ANALYST');
  }

  if (
    useCaseLower.includes('manage') ||
    useCaseLower.includes('manager') ||
    useCaseLower.includes('lead')
  ) {
    return getTemplateByName('MANAGER');
  }

  if (
    useCaseLower.includes('admin') ||
    useCaseLower.includes('administrator') ||
    useCaseLower.includes('full')
  ) {
    return getTemplateByName('ADMIN');
  }

  return null;
};

/**
 * Get role hierarchy level name
 */
export const getRoleLevelName = (level: number): string => {
  const names: Record<number, string> = {
    0: 'Viewer',
    3: 'Analyst',
    6: 'Manager',
    9: 'Administrator',
  };
  return names[level] || `Custom Level ${level}`;
};

/**
 * Validate custom role
 */
export const validateCustomRole = (role: Partial<CustomRole>): {
  valid: boolean;
  errors: string[];
} => {
  const errors: string[] = [];

  if (!role.name || role.name.trim().length === 0) {
    errors.push('Role name is required');
  }

  if (role.name && role.name.length > 50) {
    errors.push('Role name must be less than 50 characters');
  }

  if (role.level === undefined) {
    errors.push('Role level is required');
  } else if (role.level < 0 || role.level > 9) {
    errors.push('Role level must be between 0 and 9');
  }

  if (!role.permissions || role.permissions.length === 0) {
    errors.push('At least one permission is required');
  }

  if (role.description && role.description.length > 500) {
    errors.push('Role description must be less than 500 characters');
  }

  return {
    valid: errors.length === 0,
    errors,
  };
};

/**
 * Clone template with modifications
 */
export const cloneTemplate = (
  template: RoleTemplate,
  overrides: Partial<RoleTemplate> = {}
): RoleTemplate => {
  return {
    name: overrides.name ?? template.name,
    level: overrides.level ?? template.level,
    description: overrides.description ?? template.description,
    permissions: overrides.permissions ?? [...template.permissions],
    features: overrides.features ?? [...template.features],
  };
};

/**
 * Merge multiple templates
 */
export const mergeTemplates = (
  templates: RoleTemplate[],
  options?: {
    preferHigherLevel?: boolean;
    combineFeatures?: boolean;
  }
): RoleTemplate => {
  if (templates.length === 0) {
    return getTemplateByName('VIEWER')!;
  }

  if (templates.length === 1) {
    return cloneTemplate(templates[0]);
  }

  const allPermissions = Array.from(
    new Set(templates.flatMap((t) => t.permissions))
  );
  const allFeatures = options?.combineFeatures
    ? Array.from(new Set(templates.flatMap((t) => t.features)))
    : templates[0].features;
  const maxLevel = options?.preferHigherLevel
    ? Math.max(...templates.map((t) => t.level))
    : templates[0].level;

  return {
    name: `Merged Role (${templates.length})`,
    level: maxLevel,
    description: `Merged role from ${templates.length} templates`,
    permissions: allPermissions,
    features: allFeatures,
  };
};

/**
 * Export template as JSON
 */
export const exportTemplate = (template: RoleTemplate): string => {
  return JSON.stringify(template, null, 2);
};

/**
 * Import template from JSON
 */
export const importTemplate = (json: string): RoleTemplate | null => {
  try {
    const parsed = JSON.parse(json);
    const validation = validateCustomRole(parsed);
    return validation.valid ? parsed : null;
  } catch {
    return null;
  }
};

export default {
  getTemplateByName,
  getAllTemplates,
  getTemplateByLevel,
  createCustomRoleFromTemplate,
  compareTemplates,
  getPermissionCoverage,
  roleHasMinimumPermissions,
  getRecommendedTemplate,
  getRoleLevelName,
  validateCustomRole,
  cloneTemplate,
  mergeTemplates,
  exportTemplate,
  importTemplate,
};
