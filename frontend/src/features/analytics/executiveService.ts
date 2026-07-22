// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
//
// Typed client for the Executive Dashboard API (spec §11 « Tableau de bord
// exécutif »). Mirrors internal/application/dashboard.ExecutiveDashboard — one
// consolidated, tenant-scoped aggregation. Zero `any`.

import { api } from '../../lib/api';

/** An amount in both currencies (XAF is canonical, USD derived). */
export interface Money {
  xaf: number;
  usd: number;
}

export interface ScoreComponent {
  key: string;
  label: string;
  value: number; // 0..100 (higher = safer)
  weight: number; // effective weight after renormalisation
}

/** Composite posture grade: 0..100 + A..F letter. */
export interface CyberScore {
  score: number;
  grade: string;
  label: string;
  components: ScoreComponent[];
}

export interface FinancialHeadline {
  total_ale: Money;
  total_ale_worst: Money;
  total_risks: number;
  quantified_risks: number;
}

export type KRISeverity = 'ok' | 'warn' | 'critical';

export interface KRI {
  key: string;
  label: string;
  value: number;
  unit: '' | 'days' | '%';
  severity: KRISeverity;
}

export interface ExecRisk {
  id: string;
  title: string;
  score: number;
  probability: number; // 1..5 band for the heatmap
  impact: number; // 1..5 band for the heatmap
  criticality: string;
  status: string;
  lifecycle_phase: string;
  ale: Money;
}

export interface ComplianceCoverage {
  framework_id: string;
  name: string;
  percent: number;
  implemented: number;
  total: number;
}

export interface DistributionSlice {
  criticality: string;
  count: number;
}

export interface MonthlyRiskPoint {
  month: string; // "YYYY-MM"
  avg_score: number;
  critical: number;
  high: number;
  total: number;
}

export interface IncidentTrendPoint {
  month: string; // "YYYY-MM"
  total: number;
  critical: number;
  high: number;
}

/** The single consolidated executive dashboard payload. */
export interface ExecutiveDashboard {
  generated_at: string;
  currency: string;
  xaf_per_usd: number;
  cyber_score: CyberScore;
  financial: FinancialHeadline;
  kris: KRI[];
  top_risks: ExecRisk[];
  risk_trend: MonthlyRiskPoint[];
  risk_distribution: DistributionSlice[];
  compliance: ComplianceCoverage[];
  incident_trend: IncidentTrendPoint[];
}

export const executiveService = {
  /** One consolidated request backing the whole executive board. */
  get: async (): Promise<ExecutiveDashboard> => {
    const res = await api.get<ExecutiveDashboard>('/analytics/executive');
    return res.data;
  },
};
