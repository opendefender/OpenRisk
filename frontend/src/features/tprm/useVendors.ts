// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// React Query hooks for the vendor register, chain and assessments (#673).
//
// Adding and removing a vendor link is optimistic: the chain changes at once and
// is restored exactly as it was if the server refuses. Revoking an assessment is
// optimistic the same way.

import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { assetService } from '../../services/assetService';
import {
  vendorService,
  type CreateVendorLinkInput,
  type SendVendorAssessmentInput,
  type VendorAssessment,
  type VendorChain,
  type VendorChainAsset,
  type VendorListParams,
} from './vendorService';

export const VENDORS_KEY = ['vendors'] as const;
const registerKey = (params: VendorListParams) => [...VENDORS_KEY, 'register', params] as const;
const chainKey = (vendorId: string) => [...VENDORS_KEY, 'chain', vendorId] as const;
const assessmentsKey = (vendorId: string) => [...VENDORS_KEY, 'assessments', vendorId] as const;
const assessmentKey = (assessmentId: string) => ['vendor-assessments', assessmentId] as const;

/** The id an optimistic link carries until the server answers. */
export const PENDING_LINK_PREFIX = 'pending:';

export function useVendorRegister(params: VendorListParams) {
  return useQuery({
    queryKey: registerKey(params),
    queryFn: () => vendorService.list(params),
    // Paging and filtering keep the previous page on screen instead of
    // flashing the skeleton on every keystroke.
    placeholderData: keepPreviousData,
  });
}

export function useVendorChain(vendorId: string | undefined) {
  return useQuery({
    queryKey: chainKey(vendorId ?? ''),
    queryFn: () => vendorService.chain(vendorId ?? ''),
    enabled: Boolean(vendorId),
  });
}

/** The vendor's own asset record: its attributes (contact email, legal name). */
export function useVendorRecord(vendorId: string | undefined) {
  return useQuery({
    queryKey: ['assets', 'record', vendorId ?? ''],
    queryFn: () => assetService.getAsset(vendorId ?? ''),
    enabled: Boolean(vendorId),
  });
}

export function useVendorLinkMutations(vendorId: string) {
  const qc = useQueryClient();
  const key = chainKey(vendorId);
  const settle = () => {
    void qc.invalidateQueries({ queryKey: key });
    // The register's linked_assets count moved too.
    void qc.invalidateQueries({ queryKey: [...VENDORS_KEY, 'register'] });
  };

  const link = useMutation({
    mutationFn: ({ input }: { input: CreateVendorLinkInput; asset: VendorChainAsset }) =>
      vendorService.link(vendorId, input),
    onMutate: async ({ input, asset }) => {
      await qc.cancelQueries({ queryKey: key });
      const previous = qc.getQueryData<VendorChain>(key);
      qc.setQueryData<VendorChain>(key, (old) =>
        old
          ? {
              ...old,
              links: [
                ...old.links,
                {
                  link_id: `${PENDING_LINK_PREFIX}${asset.id}:${input.verb}`,
                  verb: input.verb,
                  asset,
                  risks: [],
                },
              ],
            }
          : old,
      );
      return { previous };
    },
    onError: (_err, _vars, ctx) => {
      if (ctx?.previous) qc.setQueryData(key, ctx.previous);
    },
    onSettled: settle,
  });

  const unlink = useMutation({
    mutationFn: (linkId: string) => vendorService.unlink(vendorId, linkId),
    onMutate: async (linkId) => {
      await qc.cancelQueries({ queryKey: key });
      const previous = qc.getQueryData<VendorChain>(key);
      qc.setQueryData<VendorChain>(key, (old) =>
        old ? { ...old, links: old.links.filter((l) => l.link_id !== linkId) } : old,
      );
      return { previous };
    },
    onError: (_err, _vars, ctx) => {
      if (ctx?.previous) qc.setQueryData(key, ctx.previous);
    },
    onSettled: settle,
  });

  return { link, unlink };
}

export function useVendorAssessments(vendorId: string | undefined) {
  return useQuery({
    queryKey: assessmentsKey(vendorId ?? ''),
    queryFn: () => vendorService.listAssessments(vendorId ?? ''),
    enabled: Boolean(vendorId),
  });
}

export function useVendorAssessment(assessmentId: string | undefined) {
  return useQuery({
    queryKey: assessmentKey(assessmentId ?? ''),
    queryFn: () => vendorService.getAssessment(assessmentId ?? ''),
    enabled: Boolean(assessmentId),
  });
}

export function useVendorAssessmentMutations(vendorId: string) {
  const qc = useQueryClient();
  const listKey = assessmentsKey(vendorId);
  const settle = () => {
    void qc.invalidateQueries({ queryKey: listKey });
    void qc.invalidateQueries({ queryKey: [...VENDORS_KEY, 'register'] });
  };

  const send = useMutation({
    mutationFn: (input: SendVendorAssessmentInput) => vendorService.send(vendorId, input),
    onSettled: settle,
  });

  const revoke = useMutation({
    mutationFn: (assessmentId: string) => vendorService.revoke(assessmentId),
    onMutate: async (assessmentId) => {
      await qc.cancelQueries({ queryKey: listKey });
      const previous = qc.getQueryData<VendorAssessment[]>(listKey);
      qc.setQueryData<VendorAssessment[]>(listKey, (old) =>
        old?.map((a) => (a.id === assessmentId ? { ...a, status: 'revoked' } : a)),
      );
      return { previous };
    },
    onError: (_err, _vars, ctx) => {
      if (ctx?.previous) qc.setQueryData(listKey, ctx.previous);
    },
    onSettled: (_data, _err, assessmentId) => {
      settle();
      void qc.invalidateQueries({ queryKey: assessmentKey(assessmentId) });
    },
  });

  const resend = useMutation({
    mutationFn: (assessmentId: string) => vendorService.resend(assessmentId),
    onSettled: (_data, _err, assessmentId) => {
      settle();
      void qc.invalidateQueries({ queryKey: assessmentKey(assessmentId) });
    },
  });

  return { send, revoke, resend };
}
