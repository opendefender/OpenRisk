/**
 * Dashboard Data Types
 * Type definitions for all dashboard API responses
 */

export interface DashboardMetrics {
  average_risk_score: number;
  trending_up_percent: number;
  overdue_count: number;
  sla_compliance_rate: number;
  total_risks: number;
  active_risks: number;
  mitigation_rate: number;
  updated_at: string;
}

export interface RiskTrendDataPoint {
  date: string;
  score: number;
  count: number;
  new_risks: number;
  mitigated: number;
}

export interface RiskSeverityDistribution {
  critical: number;
  high: number;
  medium: number;
  low: number;
}

export interface MitigationStatus {
  completed: number;
  in_progress: number;
  not_started: number;
  overdue: number;
}

export interface TopRisk {
  id: string;
  name: string;
  score: number;
  severity: string;
  status: string;
  trend_percent: number;
  last_updated: string;
  assigned_team?: string;
  mitigation_count: number;
}

export interface MitigationProgress {
  id: string;
  name: string;
  status: string;
  progress: number;
  due_date: string;
  owner?: string;
  risk_id: string;
  risk_name: string;
  cost: number;
  last_updated: string;
  days_remaining: number;
}

export interface CompleteDashboardAnalytics {
  metrics: DashboardMetrics;
  risk_trends: RiskTrendDataPoint[];
  severity_distribution: RiskSeverityDistribution;
  mitigation_status: MitigationStatus;
  top_risks: TopRisk[];
  mitigation_progress: MitigationProgress[];
  generated_at: string;
}

export interface ApiResponse<T> {
  data?: T;
  error?: string;
  count?: number;
  top_risks?: TopRisk[];
  mitigations?: MitigationProgress[];
  trends?: RiskTrendDataPoint[];
  metrics?: DashboardMetrics;
}
