// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import { api } from '../lib/api';

// Mirror of the backend open-core matrix. The frontend NEVER grants — it only
// greys and explains from this snapshot; the backend enforces on write.
export type PlanKey = 'free' | 'pro' | 'business' | 'enterprise';
export type RegionKey = 'eu' | 'africa';

export interface FeatureEntitlement {
  enabled: boolean;
  level: string;
  required_plan: PlanKey;
}

export interface LimitEntitlement {
  limit: number; // -1 = unlimited
  used: number;
}

export interface Price {
  amount: number;
  currency: string; // EUR | XAF
  period: string; // month
  custom: boolean; // Enterprise = quote
}

export interface TrialInfo {
  active: boolean;
  ends_at?: string;
  days_left: number;
}

export interface Entitlements {
  plan: PlanKey;
  region: RegionKey;
  features: Record<string, FeatureEntitlement>;
  limits: Record<string, LimitEntitlement>;
  trial?: TrialInfo;
  prices: Record<string, Price>;
  trial_days: number;
}

export interface Subscription {
  id: string;
  organization_id: string;
  plan: PlanKey;
  status: 'trialing' | 'active' | 'past_due' | 'canceled' | 'incomplete';
  region: RegionKey;
  provider?: string;
  trial_ends_at?: string;
  current_period_end?: string;
  cancel_at_period_end: boolean;
  canceled_at?: string;
}

export interface Invoice {
  id: string;
  number: string;
  amount_cents: number;
  currency: string;
  status: string;
  hosted_url?: string;
  period_start?: string;
  period_end?: string;
  created_at: string;
}

export interface BillingState {
  subscription: Subscription | null;
  invoices: Invoice[];
  configured_providers: string[];
  trial_days: number;
}

export interface CheckoutSession {
  provider: string;
  url: string;
  reference: string;
}

export const entitlementService = {
  get: () => api.get<Entitlements>('/entitlements').then((r) => r.data),

  getBilling: () => api.get<BillingState>('/billing').then((r) => r.data),
  startTrial: (plan: PlanKey, region: RegionKey) =>
    api.post<Subscription>('/billing/trial', { plan, region }).then((r) => r.data),
  checkout: (plan: PlanKey, region: RegionKey, provider?: string) =>
    api.post<CheckoutSession>('/billing/checkout', { plan, region, provider }).then((r) => r.data),
  changePlan: (plan: PlanKey, region: RegionKey) =>
    api.post<Subscription>('/billing/change-plan', { plan, region }).then((r) => r.data),
  cancel: () => api.post<Subscription>('/billing/cancel', {}).then((r) => r.data),
};

// Telemetry (instance-level, opt-in) ----------------------------------------
export interface TelemetryState {
  enabled: boolean;
  consent: boolean;
  env_forced_off: boolean;
  instance_id: string;
  schema_url?: string;
  version?: string;
  last_sent_at?: string | null;
}

export const telemetryService = {
  get: () => api.get<TelemetryState>('/telemetry').then((r) => r.data),
  set: (enabled: boolean) => api.put<TelemetryState>('/telemetry', { enabled }).then((r) => r.data),
};

// Danger zone (org deletion) -------------------------------------------------
export interface DeletionState {
  pending: boolean;
  request?: {
    id: string;
    status: string;
    scheduled_purge_at: string;
    confirmed_name: string;
    export_path?: string;
  };
  days_remaining?: number;
  scheduled_purge_at?: string;
}

export const orgDeletionService = {
  get: () => api.get<DeletionState>('/organization/deletion').then((r) => r.data),
  request: (confirm_name: string, mfa_code: string, reason: string) =>
    api.post('/organization/deletion', { confirm_name, mfa_code, reason }).then((r) => r.data),
  cancel: () => api.post('/organization/deletion/cancel', {}).then((r) => r.data),
  exportUrl: '/api/v1/organization/export',
};
