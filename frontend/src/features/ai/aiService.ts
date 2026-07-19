// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
//
// Typed client for the GRC AI assistant (/ai/*). Shapes mirror
// backend/pkg/ai/assistant.go and internal/application/ai. Every endpoint is
// best-effort on the backend (Claude when ANTHROPIC_API_KEY is set, deterministic
// template otherwise), so calls always resolve with a result and a `generated_by`
// provenance field.

import { api } from '../../lib/api';

export type Locale = 'fr' | 'en';

export interface AIStatus {
  llm_enabled: boolean;
  model: string;
}

// --- 1. Treatment plan ------------------------------------------------------

export interface TreatmentPlanAction {
  title: string;
  description: string;
  priority: 'high' | 'medium' | 'low';
}

export interface TreatmentPlan {
  summary: string;
  recommended_strategy: 'mitigate' | 'accept' | 'transfer' | 'avoid';
  actions: TreatmentPlanAction[];
  rationale: string;
}

export interface TreatmentPlanResult {
  plan: TreatmentPlan;
  generated_by: string;
}

// --- 2. Emerging risks ------------------------------------------------------

export interface EmergingRisk {
  title: string;
  description: string;
  category: string;
  severity: 'critical' | 'high' | 'medium' | 'low';
  rationale: string;
  suggested_probability: number; // 0..1
  suggested_impact: number; // 0..10
}

export interface EmergingRisksPayload {
  summary: string;
  risks: EmergingRisk[];
}

export interface EmergingRisksResult {
  result: EmergingRisksPayload;
  generated_by: string;
}

// --- 3. Q&A assistant -------------------------------------------------------

export interface KnowledgeSnippet {
  kind: 'risk' | 'control' | 'vulnerability' | 'framework';
  ref: string;
  title: string;
  detail: string;
}

export interface ChatTurn {
  role: 'user' | 'assistant';
  text: string;
}

export interface AssistantAnswer {
  answer: string;
  sources: string[];
}

export interface AssistantAnswerResult {
  answer: AssistantAnswer;
  generated_by: string;
  retrieved: KnowledgeSnippet[];
}

// --- 4. Audit report --------------------------------------------------------

export interface AuditNarrative {
  executive_summary: string;
  findings: string;
  recommendations: string[];
  conclusion: string;
}

export interface AuditReportResult {
  report: AuditNarrative;
  generated_by: string;
}

// --- 5. Evidence analysis ---------------------------------------------------

export interface EvidenceAssessment {
  verdict: 'satisfies' | 'partial' | 'insufficient' | 'unrelated';
  confidence: number; // 0..1
  rationale: string;
  gaps: string[];
  suggestions: string[];
}

export interface EvidenceAssessmentResult {
  assessment: EvidenceAssessment;
  generated_by: string;
}

export const aiService = {
  async status(): Promise<AIStatus> {
    const res = await api.get<AIStatus>('/ai/status');
    return res.data;
  },

  async ask(question: string, history: ChatTurn[], locale: Locale): Promise<AssistantAnswerResult> {
    const res = await api.post<AssistantAnswerResult>('/ai/assistant/query', {
      question,
      history,
      locale,
    });
    return res.data;
  },

  async detectEmergingRisks(
    input: { source?: string; text: string; context?: string; locale: Locale },
  ): Promise<EmergingRisksResult> {
    const res = await api.post<EmergingRisksResult>('/ai/emerging-risks', input);
    return res.data;
  },

  async treatmentPlan(riskId: string, locale: Locale): Promise<TreatmentPlanResult> {
    const res = await api.post<TreatmentPlanResult>(`/ai/risks/${riskId}/treatment-plan`, { locale });
    return res.data;
  },

  async auditReport(auditId: string, locale: Locale): Promise<AuditReportResult> {
    const res = await api.post<AuditReportResult>(`/ai/audits/${auditId}/report`, { locale });
    return res.data;
  },

  async analyzeEvidence(evidenceId: string, locale: Locale): Promise<EvidenceAssessmentResult> {
    const res = await api.post<EvidenceAssessmentResult>(`/ai/evidence/${evidenceId}/analyze`, { locale });
    return res.data;
  },
};
