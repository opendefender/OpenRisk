/**
 * Mitigation Module Types
 * Strict TypeScript definitions for mitigation plans, sub-actions, and related data.
 */

export type MitigationStatus = 'TODO' | 'IN_PROGRESS' | 'REVIEW' | 'DONE';
export type SubActionStatus = 'TODO' | 'IN_PROGRESS' | 'DONE';
export type CompletedSource = 'manual' | 'scanner';

export interface SubAction {
  id: string;
  mitigation_id: string;
  title: string;
  description?: string;
  status: SubActionStatus;
  completed_at?: string;
  completed_by?: string;
  completed_source: CompletedSource;
  depends_on?: string[];
  order: number;
  evidence_ids?: string[];
  created_at: string;
  updated_at: string;
  scan_job_id?: string;
  scanner_details?: {
    scan_id: string;
    detected_at: string;
    asset_id?: string;
  };
}

export interface Evidence {
  id: string;
  title: string;
  description?: string;
  file_url?: string;
  file_name?: string;
  file_size?: number;
  mime_type?: string;
  created_at: string;
  created_by: string;
  sub_action_id?: string;
}

export interface TimelineEvent {
  id: string;
  mitigation_id: string;
  type: 'created' | 'status_changed' | 'assigned' | 'auto_completed' | 'reverted' | 'evidence_added' | 'comment_added';
  actor_id?: string;
  actor_name?: string;
  description: string;
  payload?: Record<string, any>;
  timestamp: string;
}

export interface Mitigation {
  id: string;
  risk_id: string;
  title: string;
  description?: string;
  status: MitigationStatus;
  priority: 'low' | 'medium' | 'high' | 'critical';
  assigned_to?: string;
  assigned_to_user?: {
    id: string;
    name: string;
    avatar?: string;
    email?: string;
  };
  due_date: string;
  estimated_effort_days?: number;
  cost_estimate?: number;
  progress_percentage: number;
  risk_title?: string;
  risk_score?: number;
  sub_actions: SubAction[];
  auto_detected_count: number;
  completed_at?: string;
  created_by?: string;
  created_at: string;
  updated_at: string;
  last_edited_by?: string;
  last_edited_at?: string;
  editing_lock?: {
    user_id: string;
    user_name: string;
    locked_at: string;
  };
}

export interface MitigationListResponse {
  items: Mitigation[];
  total: number;
  page?: number;
  per_page?: number;
}

export interface MitigationQueryParams {
  q?: string;
  risk_id?: string;
  status?: MitigationStatus;
  priority?: 'low' | 'medium' | 'high' | 'critical';
  assigned_to?: string;
  due_date_from?: string;
  due_date_to?: string;
  overdue_only?: boolean;
  page?: number;
  per_page?: number;
  sort_by?: 'title' | 'due_date' | 'progress' | 'priority' | 'created_at';
  sort_dir?: 'asc' | 'desc';
}

export interface CreateMitigationInput {
  risk_id: string;
  title: string;
  description?: string;
  due_date: string;
  priority?: 'low' | 'medium' | 'high' | 'critical';
  assigned_to?: string;
  estimated_effort_days?: number;
  cost_estimate?: number;
}

export interface UpdateMitigationInput {
  title?: string;
  description?: string;
  status?: MitigationStatus;
  priority?: 'low' | 'medium' | 'high' | 'critical';
  assigned_to?: string;
  due_date?: string;
  estimated_effort_days?: number;
  cost_estimate?: number;
}

export interface CreateSubActionInput {
  mitigation_id: string;
  title: string;
  description?: string;
  depends_on?: string[];
}

export interface UpdateSubActionInput {
  title?: string;
  description?: string;
  status?: SubActionStatus;
  depends_on?: string[];
  order?: number;
}

export interface RevertSubActionInput {
  sub_action_id: string;
  reason?: string;
}

export interface UpdateSubActionStatusInput {
  status: SubActionStatus;
  evidence_ids?: string[];
}

export interface BulkMitigationActionInput {
  action: 'change_status' | 'assign_to' | 'change_priority' | 'delete';
  mitigation_ids: string[];
  payload?: {
    status?: MitigationStatus;
    assigned_to?: string;
    priority?: 'low' | 'medium' | 'high' | 'critical';
  };
}

export interface RescanInput {
  sub_action_id: string;
  asset_id?: string;
}

export interface MitigationFilters {
  q?: string;
  risk_id?: string;
  status?: MitigationStatus;
  priority?: 'low' | 'medium' | 'high' | 'critical';
  assigned_to?: string;
  due_date_from?: string;
  due_date_to?: string;
  overdue_only?: boolean;
  source?: CompletedSource;
}

export interface MitigationUIState {
  selectedMitigationId?: string | null;
  isDrawerOpen: boolean;
  activeTab: 'overview' | 'sub_actions' | 'evidence' | 'timeline' | 'ai_suggestions';
  filters: MitigationFilters;
  viewMode: 'kanban' | 'table' | 'gantt';
  selectedIds: string[];
}
