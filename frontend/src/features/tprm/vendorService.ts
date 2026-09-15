// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Typed client for the vendor register, the vendor→asset→risk chain and vendor
// assessments (#673; backend #669, #670, #671; ADR 0004).
// Authenticated routes: they go through lib/api.ts and its session.

import { api } from '../../lib/api';
import type { components } from '../../types/openapi.generated';

type Schemas = components['schemas'];

export type VendorRegisterEntry = Schemas['VendorRegisterEntry'];
export type VendorPage = Schemas['VendorPage'];
export type VendorChain = Schemas['VendorChain'];
export type VendorChainLink = Schemas['VendorChainLink'];
export type VendorChainAsset = Schemas['VendorChainAsset'];
export type VendorChainRisk = Schemas['VendorChainRisk'];
export type CreateVendorLinkInput = Schemas['CreateVendorLinkInput'];
export type VendorLinkVerb = CreateVendorLinkInput['verb'];
export type AssetDependency = Schemas['AssetDependency'];
export type VendorAssessment = Schemas['VendorAssessment'];
export type VendorAssessmentItem = Schemas['VendorAssessmentItem'];
export type VendorScoreContribution = Schemas['VendorScoreContribution'];
export type VendorAssessmentDelivery = Schemas['VendorAssessmentDelivery'];
export type SendVendorAssessmentInput = Schemas['SendVendorAssessmentInput'];

export interface VendorListParams {
  search?: string;
  service_criticality?: string;
  limit: number;
  offset: number;
}

const id = (value: string) => encodeURIComponent(value);

export const vendorService = {
  async list(params: VendorListParams): Promise<VendorPage> {
    const { data } = await api.get<VendorPage>('/vendors', { params });
    return data;
  },

  async chain(vendorId: string): Promise<VendorChain> {
    const { data } = await api.get<VendorChain>(`/vendors/${id(vendorId)}/chain`);
    return data;
  },

  async link(vendorId: string, input: CreateVendorLinkInput): Promise<AssetDependency> {
    const { data } = await api.post<AssetDependency>(`/vendors/${id(vendorId)}/assets`, input);
    return data;
  },

  async unlink(vendorId: string, linkId: string): Promise<void> {
    await api.delete(`/vendors/${id(vendorId)}/assets/${id(linkId)}`);
  },

  async listAssessments(vendorId: string): Promise<VendorAssessment[]> {
    const { data } = await api.get<VendorAssessment[]>(`/vendors/${id(vendorId)}/assessments`);
    return data;
  },

  async getAssessment(assessmentId: string): Promise<VendorAssessment> {
    const { data } = await api.get<VendorAssessment>(`/vendor-assessments/${id(assessmentId)}`);
    return data;
  },

  async send(vendorId: string, input: SendVendorAssessmentInput): Promise<VendorAssessmentDelivery> {
    const { data } = await api.post<VendorAssessmentDelivery>(
      `/vendors/${id(vendorId)}/assessments`,
      input,
    );
    return data;
  },

  async revoke(assessmentId: string): Promise<VendorAssessment> {
    const { data } = await api.post<VendorAssessment>(
      `/vendor-assessments/${id(assessmentId)}/revoke`,
    );
    return data;
  },

  async resend(assessmentId: string): Promise<VendorAssessmentDelivery> {
    const { data } = await api.post<VendorAssessmentDelivery>(
      `/vendor-assessments/${id(assessmentId)}/resend`,
    );
    return data;
  },
};
