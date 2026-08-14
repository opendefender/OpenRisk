// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { evidenceService } from '../../services/evidenceService';
import { OVERVIEW_QUERY_KEY } from '../compliance/useCompliance';
import type {
  EvidenceFilter,
  CreateEvidenceInput,
  UpdateEvidenceInput,
} from '../../types/evidence';

export const EVIDENCE_QUERY_KEY = ['evidence'];
const MISSING_QUERY_KEY = ['evidence', 'missing'];

export function useEvidenceLibrary(filter: EvidenceFilter = {}) {
  return useQuery({
    queryKey: [...EVIDENCE_QUERY_KEY, 'list', filter],
    queryFn: () => evidenceService.list(filter),
  });
}

export function useEvidence(id: string | undefined) {
  return useQuery({
    queryKey: [...EVIDENCE_QUERY_KEY, 'one', id],
    queryFn: () => evidenceService.get(id!),
    enabled: Boolean(id),
  });
}

/** The "what proof am I missing" worklist, per framework or across all of them. */
export function useMissingEvidence(frameworkId?: string) {
  return useQuery({
    queryKey: [...MISSING_QUERY_KEY, frameworkId ?? 'all'],
    queryFn: () => evidenceService.missing(frameworkId),
  });
}

/**
 * Every evidence mutation invalidates the compliance overview as well as the
 * library.
 *
 * Not defensive: an artifact expiring or being rejected changes how many
 * controls count as covered, which changes the coverage percentage on the
 * Compliance grid. Leaving that stale is how a register shows a number the
 * evidence no longer supports.
 */
function useEvidenceMutation<TVars, TData>(fn: (vars: TVars) => Promise<TData>) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: fn,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: EVIDENCE_QUERY_KEY });
      queryClient.invalidateQueries({ queryKey: MISSING_QUERY_KEY });
      queryClient.invalidateQueries({ queryKey: OVERVIEW_QUERY_KEY });
      queryClient.invalidateQueries({ queryKey: ['compliance'] });
    },
  });
}

export function useCreateEvidence() {
  return useEvidenceMutation((input: CreateEvidenceInput) => evidenceService.create(input));
}

export function useUpdateEvidence() {
  return useEvidenceMutation(({ id, input }: { id: string; input: UpdateEvidenceInput }) =>
    evidenceService.update(id, input),
  );
}

export function useLinkEvidence() {
  return useEvidenceMutation(({ id, controlIds, note }: { id: string; controlIds: string[]; note?: string }) =>
    evidenceService.link(id, controlIds, note),
  );
}

export function useUnlinkEvidence() {
  return useEvidenceMutation(({ id, controlId }: { id: string; controlId: string }) =>
    evidenceService.unlink(id, controlId),
  );
}

export function useReviewEvidence() {
  return useEvidenceMutation(
    ({ id, review, note }: { id: string; review: 'accepted' | 'rejected' | 'pending'; note?: string }) =>
      evidenceService.review(id, review, note),
  );
}

export function useDeleteEvidence() {
  return useEvidenceMutation((id: string) => evidenceService.remove(id));
}
