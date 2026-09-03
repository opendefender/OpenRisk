// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import { api } from '../../lib/api';

/** The tenant's vulnerability→risk rule. Mirrors domain.VulnRiskRule. */
export interface VulnRiskRule {
  id?: string;
  enabled: boolean;
  min_cvss: number;
  require_internet_exposure: boolean;
  min_asset_criticality: '' | 'LOW' | 'MEDIUM' | 'HIGH' | 'CRITICAL';
  require_kev: boolean;
  require_asset: boolean;
  notify_on_create: boolean;
  updated_at?: string;
}

export interface RulePreviewSample {
  vulnerability_id: string;
  title: string;
  cve_id?: string;
  asset_name?: string;
  reason: string;
}

export interface RulePreview {
  would_create: number;
  already_linked: number;
  evaluated: number;
  samples: RulePreviewSample[];
  top_rejections: Record<string, number>;
}

/** A machine-proposed DRAFT risk awaiting review. */
export interface DraftRisk {
  id: string;
  name?: string;
  title?: string;
  description?: string;
  criticality?: string;
  score?: number;
  source?: string;
  source_cve_id?: string | null;
  source_vulnerability_id?: string | null;
  source_rule_reason?: string;
  created_at?: string;
}

export interface DraftRiskList {
  items: DraftRisk[];
  total: number;
  page: number;
  limit: number;
}

export interface BulkReviewResult {
  accepted: string[] | null;
  dismissed: string[] | null;
  failed?: Record<string, string>;
}

export const riskRuleService = {
  get: async (): Promise<VulnRiskRule> => {
    const { data } = await api.get<VulnRiskRule>('/attack-surface/risk-rule');
    return data;
  },

  update: async (rule: VulnRiskRule): Promise<VulnRiskRule> => {
    const { data } = await api.put<VulnRiskRule>('/attack-surface/risk-rule', rule);
    return data;
  },

  /** What the rule WOULD do against the current register. Nothing is created. */
  preview: async (rule?: VulnRiskRule): Promise<RulePreview> => {
    const { data } = await api.post<RulePreview>('/attack-surface/risk-rule/preview', rule ?? {});
    return data;
  },

  listDrafts: async (): Promise<DraftRiskList> => {
    const { data } = await api.get<DraftRiskList>('/attack-surface/draft-risks', {
      params: { limit: 100 },
    });
    return data;
  },

  reviewDrafts: async (
    riskIds: string[],
    decision: 'accept' | 'dismiss',
  ): Promise<BulkReviewResult> => {
    const { data } = await api.post<BulkReviewResult>('/attack-surface/draft-risks/review', {
      risk_ids: riskIds,
      decision,
    });
    return data;
  },
};
