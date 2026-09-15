// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Typed client for questionnaire templates (#681, backend #670, ADR 0004 D3).
// Authenticated routes: they go through lib/api.ts and its session.

import { api } from '../../lib/api';
import type { components } from '../../types/openapi.generated';

export type QuestionnaireTemplate = components['schemas']['VendorQuestionnaireTemplate'];
export type QuestionnaireTemplateInput = components['schemas']['VendorQuestionnaireTemplateInput'];
export type QuestionInput = components['schemas']['VendorQuestionnaireQuestionInput'];
export type QuestionOption = components['schemas']['VendorQuestionOption'];

const BASE = '/vendor-questionnaire-templates';

export const questionnaireTemplateService = {
  async list(includeArchived: boolean): Promise<QuestionnaireTemplate[]> {
    const { data } = await api.get<QuestionnaireTemplate[]>(BASE, {
      params: includeArchived ? { include_archived: true } : undefined,
    });
    return data;
  },

  async get(id: string): Promise<QuestionnaireTemplate> {
    const { data } = await api.get<QuestionnaireTemplate>(`${BASE}/${encodeURIComponent(id)}`);
    return data;
  },

  async create(input: QuestionnaireTemplateInput): Promise<QuestionnaireTemplate> {
    const { data } = await api.post<QuestionnaireTemplate>(BASE, input);
    return data;
  },

  async update(id: string, input: QuestionnaireTemplateInput): Promise<QuestionnaireTemplate> {
    const { data } = await api.put<QuestionnaireTemplate>(`${BASE}/${encodeURIComponent(id)}`, input);
    return data;
  },

  async archive(id: string): Promise<QuestionnaireTemplate> {
    const { data } = await api.post<QuestionnaireTemplate>(
      `${BASE}/${encodeURIComponent(id)}/archive`,
    );
    return data;
  },
};
