/
  Role Template Utilities
  Functions for creating, modifying, and managing roles from templates
 /

import { ROLE_TEMPLATES, type PermissionAction, type PermissionResource } from '../config/rbacConfig';

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

/
  Get template by name
 /
export const getTemplateByName = (name: string): RoleTemplate | null => {
  const key = name.toUpperCase();
  const template = ROLE_TEMPLATES[key as keyof typeof ROLE_TEMPLATES];
  return template ? (template as RoleTemplate) : null;
};

/
  Get all available templates
 /
export const getAllTemplates = (): RoleTemplate[] => {
  return Object.values(ROLE_TEMPLATES) as RoleTemplate[];
};

/
  Get template by level
 /
export const getTemplateByLevel = (level: number): RoleTemplate | null => {
  const template = Object.values(ROLE_TEMPLATES).find(
    (t) => (t as RoleTemplate).level === level
  );
  return template ? (template as RoleTemplate) : null;
};

/
  Create custom role from template
 /
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
    description: Custom role based on ${templateName},
    permissions: uniquePermissions,
    features: template.features,
    parentTemplate: templateName,
    customPermissions: additionalPermissions,
    excludedPermissions,
  };
};

/
  Compare two role templates
 /
export const compareTemplates = (
  template: RoleTemplate,
  template: RoleTemplate
): {
  commonPermissions: string[];
  onlyInTemplate: string[];
  onlyInTemplate: string[];
  allPermissions: string[];
} => {
  const perms = new Set(template.permissions);
  const perms = new Set(template.permissions);
  const allPerms = new Set([...template.permissions, ...template.permissions]);

  return {
    commonPermissions: template.permissions.filter((p) => perms.has(p)),
    onlyInTemplate: template.permissions.filter((p) => !perms.has(p)),
    onlyInTemplate: template.permissions.filter((p) => !perms.has(p)),
    allPermissions: Array.from(allPerms),
  };
};

/
  Get permission coverage (what % of permissions does role have)
 /
export const getPermissionCoverage = (
  rolePermissions: string[],
  allAvailablePermissions: string[]
): number => {
  if (allAvailablePermissions.length === ) return ;
  const coverage = rolePermissions.filter((p) =>
    allAvailablePermissions.includes(p)
  ).length;
  return Math.round((coverage / allAvailablePermissions.length)  );
};

/
  Check if role meets minimum permission requirements
 /
export const roleHasMinimumPermissions = (
  rolePermissions: string[],
  minimumRequired: string[]
): boolean => {
  return minimumRequired.every((perm) => rolePermissions.includes(perm));
};

/
  Get recommended template based on use case
 /
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

/
  Get role hierarchy level name
 /
export const getRoleLevelName = (level: number): string => {
  const names: Record<number, string> = {
    : 'Viewer',
    : 'Analyst',
    : 'Manager',
    : 'Administrator',
  };
  return names[level] || Custom Level ${level};
};

/
  Validate custom role
 /
export const validateCustomRole = (role: Partial<CustomRole>): {
  valid: boolean;
  errors: string[];
} => {
  const errors: string[] = [];

  if (!role.name || role.name.trim().length === ) {
    errors.push('Role name is required');
  }

  if (role.name && role.name.length > ) {
    errors.push('Role name must be less than  characters');
  }

  if (role.level === undefined) {
    errors.push('Role level is required');
  } else if (role.level <  || role.level > ) {
    errors.push('Role level must be between  and ');
  }

  if (!role.permissions || role.permissions.length === ) {
    errors.push('At least one permission is required');
  }

  if (role.description && role.description.length > ) {
    errors.push('Role description must be less than  characters');
  }

  return {
    valid: errors.length === ,
    errors,
  };
};

/
  Clone template with modifications
 /
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

/
  Merge multiple templates
 /
export const mergeTemplates = (
  templates: RoleTemplate[],
  options?: {
    preferHigherLevel?: boolean;
    combineFeatures?: boolean;
  }
): RoleTemplate => {
  if (templates.length === ) {
    return getTemplateByName('VIEWER')!;
  }

  if (templates.length === ) {
    return cloneTemplate(templates[]);
  }

  const allPermissions = Array.from(
    new Set(templates.flatMap((t) => t.permissions))
  );
  const allFeatures = options?.combineFeatures
    ? Array.from(new Set(templates.flatMap((t) => t.features)))
    : templates[].features;
  const maxLevel = options?.preferHigherLevel
    ? Math.max(...templates.map((t) => t.level))
    : templates[].level;

  return {
    name: Merged Role (${templates.length}),
    level: maxLevel,
    description: Merged role from ${templates.length} templates,
    permissions: allPermissions,
    features: allFeatures,
  };
};

/
  Export template as JSON
 /
export const exportTemplate = (template: RoleTemplate): string => {
  return JSON.stringify(template, null, );
};

/
  Import template from JSON
 /
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
