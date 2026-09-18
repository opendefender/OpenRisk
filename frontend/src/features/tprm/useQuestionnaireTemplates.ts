// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// React Query hooks for questionnaire templates (#681).

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  questionnaireTemplateService,
  type QuestionnaireTemplate,
  type QuestionnaireTemplateInput,
} from './questionnaireTemplateService';

export const TEMPLATES_KEY = ['vendor-questionnaire-templates'] as const;

export function useQuestionnaireTemplates(includeArchived: boolean) {
  return useQuery({
    queryKey: [...TEMPLATES_KEY, 'list', includeArchived],
    queryFn: () => questionnaireTemplateService.list(includeArchived),
  });
}

export function useQuestionnaireTemplate(id: string | undefined) {
  return useQuery({
    queryKey: [...TEMPLATES_KEY, 'detail', id],
    queryFn: () => questionnaireTemplateService.get(id ?? ''),
    enabled: Boolean(id),
  });
}

export function useQuestionnaireTemplateMutations() {
  const qc = useQueryClient();
  const invalidate = () => qc.invalidateQueries({ queryKey: TEMPLATES_KEY });

  const create = useMutation({
    mutationFn: (input: QuestionnaireTemplateInput) => questionnaireTemplateService.create(input),
    onSettled: invalidate,
  });

  const update = useMutation({
    mutationFn: ({ id, input }: { id: string; input: QuestionnaireTemplateInput }) =>
      questionnaireTemplateService.update(id, input),
    onSettled: invalidate,
  });

  // Archiving is applied to every cached copy at once (ABSOLUTE RULE 10), and
  // restored verbatim if the server refuses.
  const archive = useMutation({
    mutationFn: (id: string) => questionnaireTemplateService.archive(id),
    onMutate: async (id: string) => {
      await qc.cancelQueries({ queryKey: TEMPLATES_KEY });
      const snapshot = qc.getQueriesData<QuestionnaireTemplate[] | QuestionnaireTemplate>({
        queryKey: TEMPLATES_KEY,
      });
      const archivedAt = new Date().toISOString();
      qc.setQueriesData<QuestionnaireTemplate[] | QuestionnaireTemplate>(
        { queryKey: TEMPLATES_KEY },
        (cached) => {
          if (Array.isArray(cached)) {
            return cached.map((t) => (t.id === id ? { ...t, archived_at: archivedAt } : t));
          }
          if (cached && cached.id === id) return { ...cached, archived_at: archivedAt };
          return cached;
        },
      );
      return { snapshot };
    },
    onError: (_err, _id, context) => {
      for (const [key, value] of context?.snapshot ?? []) {
        qc.setQueryData(key, value);
      }
    },
    onSettled: invalidate,
  });

  return { create, update, archive };
}
