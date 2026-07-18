// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
//
// React Query hooks for the Security Automation / SOAR module.

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  automationService,
  type RuleInput,
  type ChannelInput,
  type TestInput,
} from './automationService';

const KEY = ['automation'];

export function useAutomationRules() {
  return useQuery({ queryKey: [...KEY, 'rules'], queryFn: automationService.listRules });
}

export function useAutomationExecutions() {
  return useQuery({
    queryKey: [...KEY, 'executions'],
    queryFn: automationService.listExecutions,
  });
}

export function useSLATrackers() {
  return useQuery({
    queryKey: [...KEY, 'sla'],
    queryFn: automationService.listSLA,
    // SLA countdowns tick — refresh while the tab is open.
    refetchInterval: 30_000,
  });
}

export function useSLAStats() {
  return useQuery({
    queryKey: [...KEY, 'sla', 'stats'],
    queryFn: automationService.slaStats,
    refetchInterval: 30_000,
  });
}

export function useChannelConfig() {
  return useQuery({ queryKey: [...KEY, 'channels'], queryFn: automationService.getChannels });
}

export function useAutomationMutations() {
  const qc = useQueryClient();
  const invalidate = () => qc.invalidateQueries({ queryKey: KEY });

  const createRule = useMutation({
    mutationFn: (input: RuleInput) => automationService.createRule(input),
    onSettled: invalidate,
  });
  const updateRule = useMutation({
    mutationFn: ({ id, input }: { id: string; input: RuleInput }) =>
      automationService.updateRule(id, input),
    onSettled: invalidate,
  });
  const deleteRule = useMutation({
    mutationFn: (id: string) => automationService.deleteRule(id),
    onSettled: invalidate,
  });
  const testRule = useMutation({
    mutationFn: ({ id, input }: { id: string; input: TestInput }) =>
      automationService.testRule(id, input),
    onSettled: invalidate,
  });
  const saveChannels = useMutation({
    mutationFn: (input: ChannelInput) => automationService.saveChannels(input),
    onSettled: invalidate,
  });

  return { createRule, updateRule, deleteRule, testRule, saveChannels };
}
