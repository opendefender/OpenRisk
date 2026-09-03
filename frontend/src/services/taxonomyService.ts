// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The three classification concepts, kept apart on the client too.
//
//   tags             free text on the risk        → colonne « Étiquettes »
//   categories       controlled, per tenant       → colonne « Catégorie »
//   control_mappings a real compliance control    → colonne « Référentiel »
//
// Only the third may be rendered with a framework badge, because only the third
// IS a framework reference. The register used to fall back to `tags[0]` when the
// (free-text) `frameworks` array was empty, which is how a user's label ended up
// wearing a framework badge.

import { api } from '../lib/api';

export interface RiskCategory {
  id: string;
  name: string;
  slug: string;
  description?: string;
  /** A theme token name, not a hex value — categories stay readable in both themes. */
  color: string;
  sort_order: number;
  active: boolean;
  risk_count?: number;
}

export interface RiskControlMapping {
  id: string;
  risk_id: string;
  framework_id: string;
  /** null = the mapping names a framework but no specific control. */
  control_id?: string | null;
  note?: string;
  framework_name?: string;
  control_code?: string;
  control_name?: string;
}

export interface UnmappedRisk {
  id: string;
  title: string;
  score: number;
  criticality: string;
  lifecycle_state: string;
  created_at: string;
}

export interface ImportedFramework {
  id: string;
  name: string;
  version?: string;
  description?: string;
}

export interface FrameworkControl {
  id: string;
  reference_code: string;
  name: string;
  framework_id: string;
}

/** What the "Référentiel" badge shows. Never a tag, never a free string. */
export function mappingLabel(m: RiskControlMapping): string {
  const fw = m.framework_name ?? '';
  return m.control_code ? `${fw} · ${m.control_code}` : fw;
}

/**
 * Where the badge links. A control-level mapping deep-links to the control; a
 * framework-level one goes to that framework's control list. Never a dead link.
 */
export function mappingHref(m: RiskControlMapping): string {
  return m.control_id
    ? `/compliance/frameworks/${m.framework_id}/controls/${m.control_id}`
    : `/compliance/frameworks/${m.framework_id}/controls`;
}

export const taxonomyService = {
  async listCategories(withCounts = false): Promise<RiskCategory[]> {
    const { data } = await api.get<{ items: RiskCategory[] }>('/risk-categories', {
      params: withCounts ? { with_counts: true } : undefined,
    });
    return data.items ?? [];
  },

  async createCategory(input: Partial<RiskCategory>): Promise<RiskCategory> {
    const { data } = await api.post<RiskCategory>('/risk-categories', input);
    return data;
  },

  async updateCategory(id: string, input: Partial<RiskCategory>): Promise<RiskCategory> {
    const { data } = await api.patch<RiskCategory>(`/risk-categories/${id}`, input);
    return data;
  },

  async deleteCategory(id: string): Promise<void> {
    await api.delete(`/risk-categories/${id}`);
  },

  async listMappings(riskId: string): Promise<RiskControlMapping[]> {
    const { data } = await api.get<{ items: RiskControlMapping[] }>(
      `/risks/${riskId}/control-mappings`,
    );
    return data.items ?? [];
  },

  async createMapping(
    riskId: string,
    input: { framework_id?: string; control_id?: string | null; note?: string },
  ): Promise<RiskControlMapping> {
    const { data } = await api.post<RiskControlMapping>(`/risks/${riskId}/control-mappings`, input);
    return data;
  },

  async deleteMapping(riskId: string, mappingId: string): Promise<void> {
    await api.delete(`/risks/${riskId}/control-mappings/${mappingId}`);
  },

  async listUnmapped(): Promise<UnmappedRisk[]> {
    const { data } = await api.get<{ items: UnmappedRisk[] }>('/risks/unmapped');
    return data.items ?? [];
  },

  /**
   * The frameworks that can actually be mapped to — the tenant's own, carrying
   * at least one control. NEVER a hard-coded list: the previous selector offered
   * ISO27001/CIS/NIST/OWASP as free strings whether or not the tenant had
   * imported anything, so picking one produced a badge pointing at nothing.
   */
  async listImportedFrameworks(): Promise<ImportedFramework[]> {
    const { data } = await api.get<ImportedFramework[]>('/compliance/frameworks', {
      params: { imported: true },
    });
    return Array.isArray(data) ? data : [];
  },

  async listFrameworkControls(frameworkId: string): Promise<FrameworkControl[]> {
    const { data } = await api.get<{ items?: FrameworkControl[] } | FrameworkControl[]>(
      `/compliance/frameworks/${frameworkId}/controls`,
    );
    if (Array.isArray(data)) return data;
    return data.items ?? [];
  },
};
