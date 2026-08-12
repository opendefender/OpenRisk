// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial
//
// React Query hooks for the Security Automation / SOAR module.

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  automationService,
  type RuleInput,
  type ChannelInput,
  type TestInput,
  type DryRunInput,
  type NotifyChannel,
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

export function useChannelCatalogue() {
  return useQuery({
    queryKey: [...KEY, 'channels', 'catalogue'],
    queryFn: automationService.channelCatalogue,
  });
}

// The live state indicator. Polled rather than pushed: the payload is small and
// a rule's health changes on the order of minutes, so a socket would cost more
// than it saves.
export function useAutomationState() {
  return useQuery({
    queryKey: [...KEY, 'state'],
    queryFn: automationService.getState,
    refetchInterval: 15_000,
  });
}

export function useAutomationTemplates() {
  return useQuery({
    queryKey: [...KEY, 'templates'],
    queryFn: automationService.listTemplates,
    staleTime: 60 * 60 * 1000, // a shipped catalogue does not change at runtime
  });
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
  // A dry run changes nothing, so it deliberately does NOT invalidate anything:
  // re-fetching the rule list after a test that touched no data would only make
  // the UI flicker.
  const dryRun = useMutation({
    mutationFn: ({ id, input, signal }: { id: string; input: DryRunInput; signal?: AbortSignal }) =>
      automationService.dryRun(id, input, signal),
  });
  const runRule = useMutation({
    mutationFn: ({ id, input }: { id: string; input: TestInput }) =>
      automationService.runRule(id, input),
    onSettled: invalidate,
  });
  const enableRule = useMutation({
    mutationFn: (id: string) => automationService.enableRule(id),
    onSettled: invalidate,
  });
  const suspendRule = useMutation({
    mutationFn: ({ id, reason }: { id: string; reason: string }) =>
      automationService.suspendRule(id, reason),
    onSettled: invalidate,
  });
  const replayExecution = useMutation({
    mutationFn: (id: string) => automationService.replayExecution(id),
    onSettled: invalidate,
  });
  const adoptTemplate = useMutation({
    mutationFn: ({ key, name }: { key: string; name?: string }) =>
      automationService.adoptTemplate(key, name),
    onSettled: invalidate,
  });
  const testChannel = useMutation({
    mutationFn: (channel: NotifyChannel) => automationService.testChannel(channel),
    onSettled: invalidate,
  });
  const saveChannels = useMutation({
    mutationFn: (input: ChannelInput) => automationService.saveChannels(input),
    onSettled: invalidate,
  });

  return {
    createRule,
    updateRule,
    deleteRule,
    dryRun,
    runRule,
    enableRule,
    suspendRule,
    replayExecution,
    adoptTemplate,
    testChannel,
    saveChannels,
  };
}
