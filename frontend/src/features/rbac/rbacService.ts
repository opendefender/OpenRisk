// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Typed client for the RBAC business-role API (/rbac/business-roles, /rbac/members).
// Shapes mirror backend/internal/domain/business_roles.go and
// internal/application/rbac. This is the runtime RBAC that actually drives
// authorization (organization_members → JWT permissions), not the legacy
// /rbac/roles management tables.

import { api } from '../../lib/api';

export interface PermissionDef {
  key: string;
  group: string;
  label_fr: string;
  label_en: string;
}

export interface BusinessRole {
  key: string;
  label_fr: string;
  label_en: string;
  description_fr: string;
  description_en: string;
  permissions: string[];
  default_landing: string;
}

export interface RBACCatalog {
  permissions: PermissionDef[];
  business_roles: BusinessRole[];
}

export interface MemberView {
  user_id: string;
  email: string;
  full_name: string;
  org_role: 'root' | 'admin' | 'user';
  business_role?: string;
  is_active: boolean;
  permissions: string[];
}

export interface AssignBusinessRoleInput {
  business_role: string; // preset key, or "" to clear
  member_role?: 'admin' | 'user'; // optional org-role change
}

export const rbacService = {
  /** The permission catalog + business-role presets (any authenticated member). */
  async getCatalog(): Promise<RBACCatalog> {
    const { data } = await api.get<RBACCatalog>('/rbac/business-roles');
    return data;
  },

  /** The tenant's members with their org role, business role and resolved access. */
  async listMembers(): Promise<MemberView[]> {
    const { data } = await api.get<{ members: MemberView[] }>('/rbac/members');
    return data.members ?? [];
  },

  /** Assign (or clear) a member's business role; optionally change the org role. */
  async assignBusinessRole(userId: string, input: AssignBusinessRoleInput): Promise<MemberView> {
    const { data } = await api.put<MemberView>(`/rbac/members/${userId}/business-role`, input);
    return data;
  },
};
