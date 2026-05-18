import { useMemo } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { riskService, type Risk, type RiskQueryParams, type CreateRiskInput, type UpdateRiskInput, type BulkRiskActionInput } from '../../services/riskService';

const RISK_LIST_QUERY_KEY = ['risks'];

export function useRisks(params: RiskQueryParams = {}) {
  const queryClient = useQueryClient();

  const query = useQuery({
    queryKey: [...RISK_LIST_QUERY_KEY, params],
    queryFn: () => riskService.listRisks(params),
    keepPreviousData: true,
    staleTime: 1000 * 60 * 1,
    refetchOnWindowFocus: false,
  });

  const createRisk = useMutation({
    mutationFn: (payload: CreateRiskInput) => riskService.createRisk(payload),
    onMutate: async (payload) => {
      await queryClient.cancelQueries({ queryKey: RISK_LIST_QUERY_KEY });
      const previous = queryClient.getQueryData<{ items: Risk[]; total: number }>(RISK_LIST_QUERY_KEY);
      if (previous) {
        const optimisticRisk: Risk = {
          id: `temp-${Date.now()}`,
          title: payload.title,
          description: payload.description,
          score: 0,
          impact: payload.impact,
          probability: payload.probability,
          status: payload.status ?? 'open',
          tags: payload.tags ?? [],
          frameworks: payload.framework ? [payload.framework] : [],
          assets: [],
          source: payload.source ?? 'manual',
          assigned_to: undefined,
          created_by: undefined,
          created_at: new Date().toISOString(),
          updated_at: new Date().toISOString(),
          mitigations: [],
        };
        queryClient.setQueryData(RISK_LIST_QUERY_KEY, {
          items: [optimisticRisk, ...previous.items],
          total: previous.total + 1,
        });
      }
      return { previous };
    },
    onError: (_err, _payload, context) => {
      if (context?.previous) {
        queryClient.setQueryData(RISK_LIST_QUERY_KEY, context.previous);
      }
    },
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: RISK_LIST_QUERY_KEY });
    },
  });

  const updateRisk = useMutation({
    mutationFn: ({ id, payload }: { id: string; payload: UpdateRiskInput }) => riskService.updateRisk(id, payload),
    onMutate: async ({ id, payload }) => {
      await queryClient.cancelQueries({ queryKey: RISK_LIST_QUERY_KEY });
      const previous = queryClient.getQueryData<{ items: Risk[]; total: number }>(RISK_LIST_QUERY_KEY);
      if (previous) {
        queryClient.setQueryData(RISK_LIST_QUERY_KEY, {
          ...previous,
          items: previous.items.map((risk) => (risk.id === id ? { ...risk, ...payload } : risk)),
        });
      }
      return { previous };
    },
    onError: (_err, _vars, context) => {
      if (context?.previous) {
        queryClient.setQueryData(RISK_LIST_QUERY_KEY, context.previous);
      }
    },
    onSettled: () => queryClient.invalidateQueries({ queryKey: RISK_LIST_QUERY_KEY }),
  });

  const deleteRisk = useMutation({
    mutationFn: (id: string) => riskService.deleteRisk(id),
    onMutate: async (id) => {
      await queryClient.cancelQueries({ queryKey: RISK_LIST_QUERY_KEY });
      const previous = queryClient.getQueryData<{ items: Risk[]; total: number }>(RISK_LIST_QUERY_KEY);
      if (previous) {
        queryClient.setQueryData(RISK_LIST_QUERY_KEY, {
          items: previous.items.filter((risk) => risk.id !== id),
          total: Math.max(0, previous.total - 1),
        });
      }
      return { previous };
    },
    onError: (_err, _id, context) => {
      if (context?.previous) {
        queryClient.setQueryData(RISK_LIST_QUERY_KEY, context.previous);
      }
    },
    onSettled: () => queryClient.invalidateQueries({ queryKey: RISK_LIST_QUERY_KEY }),
  });

  const acceptRisk = useMutation({
    mutationFn: ({ id, justification }: { id: string; justification: string }) => riskService.acceptRisk(id, justification),
    onSettled: () => queryClient.invalidateQueries({ queryKey: RISK_LIST_QUERY_KEY }),
  });

  const duplicateRisk = useMutation({
    mutationFn: (id: string) => riskService.duplicateRisk(id),
    onSettled: () => queryClient.invalidateQueries({ queryKey: RISK_LIST_QUERY_KEY }),
  });

  const bulkAction = useMutation({
    mutationFn: (payload: BulkRiskActionInput) => riskService.bulkAction(payload),
    onSettled: () => queryClient.invalidateQueries({ queryKey: RISK_LIST_QUERY_KEY }),
  });

  const data = query.data ?? { items: [], total: 0 };

  return useMemo(
    () => ({
      risks: data.items,
      total: data.total,
      isLoading: query.isLoading,
      error: query.error,
      refetch: query.refetch,
      createRisk,
      updateRisk,
      deleteRisk,
      acceptRisk,
      duplicateRisk,
      bulkAction,
      query,
    }),
    [data, query, createRisk, updateRisk, deleteRisk, acceptRisk, duplicateRisk, bulkAction]
  );
}
