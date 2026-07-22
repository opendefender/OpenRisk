// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// React Query hooks for the Governance module.

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  governanceService,
  type AuditFilter,
  type CreateDelegationInput,
  type WorkflowInput,
  type SubmitApprovalInput,
  type DecideApprovalInput,
} from './governanceService';

const KEY = ['governance'];

export function useAuditEvents(filter: AuditFilter) {
  return useQuery({
    queryKey: [...KEY, 'audit', filter],
    queryFn: () => governanceService.listAuditEvents(filter),
  });
}

export function useDelegations() {
  return useQuery({ queryKey: [...KEY, 'delegations'], queryFn: governanceService.listDelegations });
}

export function useEffectivePermissions(delegateId?: string) {
  return useQuery({
    queryKey: [...KEY, 'effective', delegateId ?? 'me'],
    queryFn: () => governanceService.effectivePermissions(delegateId),
  });
}

export function useWorkflows() {
  return useQuery({ queryKey: [...KEY, 'workflows'], queryFn: governanceService.listWorkflows });
}

export function useApprovals(params: { status?: string; mine?: boolean } = {}) {
  return useQuery({
    queryKey: [...KEY, 'approvals', params],
    queryFn: () => governanceService.listApprovals(params),
    // The inbox changes as colleagues submit/decide — keep it fresh.
    refetchInterval: 30_000,
  });
}

export function useGovernanceMutations() {
  const qc = useQueryClient();
  const invalidate = () => qc.invalidateQueries({ queryKey: KEY });

  const createDelegation = useMutation({
    mutationFn: (input: CreateDelegationInput) => governanceService.createDelegation(input),
    onSettled: invalidate,
  });
  const revokeDelegation = useMutation({
    mutationFn: (id: string) => governanceService.revokeDelegation(id),
    onSettled: invalidate,
  });
  const createWorkflow = useMutation({
    mutationFn: (input: WorkflowInput) => governanceService.createWorkflow(input),
    onSettled: invalidate,
  });
  const deleteWorkflow = useMutation({
    mutationFn: (id: string) => governanceService.deleteWorkflow(id),
    onSettled: invalidate,
  });
  const submitApproval = useMutation({
    mutationFn: (input: SubmitApprovalInput) => governanceService.submitApproval(input),
    onSettled: invalidate,
  });
  const decideApproval = useMutation({
    mutationFn: ({ id, input }: { id: string; input: DecideApprovalInput }) =>
      governanceService.decideApproval(id, input),
    onSettled: invalidate,
  });
  const cancelApproval = useMutation({
    mutationFn: (id: string) => governanceService.cancelApproval(id),
    onSettled: invalidate,
  });

  return {
    createDelegation, revokeDelegation,
    createWorkflow, deleteWorkflow,
    submitApproval, decideApproval, cancelApproval,
  };
}
