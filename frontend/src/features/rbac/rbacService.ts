// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Typed client for the RBAC catalogue (/rbac/business-roles): the permission
// vocabulary and the least-privilege presets that a member can be assigned.
// Shapes mirror backend/internal/domain/business_roles.go.
//
// Listing members and assigning their roles used to live here too. They moved
// to features/organization (W0-04) along with the endpoints, so there is one
// place that answers "who has access here" instead of two.

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

export const rbacService = {
  /** The permission catalog + business-role presets (any authenticated member). */
  async getCatalog(): Promise<RBACCatalog> {
    const { data } = await api.get<RBACCatalog>('/rbac/business-roles');
    return data;
  },
};
