// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
//
// React Query hooks for the GRC AI assistant.

import { useMutation, useQuery } from '@tanstack/react-query';
import {
  aiService,
  type ChatTurn,
  type Locale,
} from './aiService';

const AI_KEY = ['ai'];

/** Whether a real LLM is configured (drives the "IA active/local" badge). */
export function useAIStatus() {
  return useQuery({
    queryKey: [...AI_KEY, 'status'],
    queryFn: aiService.status,
    staleTime: 5 * 60 * 1000,
  });
}

export function useAskAssistant() {
  return useMutation({
    mutationFn: (vars: { question: string; history: ChatTurn[]; locale: Locale }) =>
      aiService.ask(vars.question, vars.history, vars.locale),
  });
}

export function useDetectEmergingRisks() {
  return useMutation({
    mutationFn: (vars: { source?: string; text: string; context?: string; locale: Locale }) =>
      aiService.detectEmergingRisks(vars),
  });
}

export function useTreatmentPlan() {
  return useMutation({
    mutationFn: (vars: { riskId: string; locale: Locale }) =>
      aiService.treatmentPlan(vars.riskId, vars.locale),
  });
}

export function useAuditReport() {
  return useMutation({
    mutationFn: (vars: { auditId: string; locale: Locale }) =>
      aiService.auditReport(vars.auditId, vars.locale),
  });
}

export function useAnalyzeEvidence() {
  return useMutation({
    mutationFn: (vars: { evidenceId: string; locale: Locale }) =>
      aiService.analyzeEvidence(vars.evidenceId, vars.locale),
  });
}
