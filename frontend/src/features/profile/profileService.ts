// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Typed client for the self-service profile (#719): GET/PATCH /users/me and
// the avatar routes. Every call acts on the session's own user.

import { api } from '../../lib/api';

export type ThemeModePref = 'light' | 'dark' | 'system';

export interface EffectivePreferences {
  timezone?: string;
  locale?: string;
  date_format?: string;
}

export interface MyProfile {
  id: string;
  email: string;
  username: string;
  full_name: string;
  job_title: string;
  phone: string;
  bio: string;
  /** Own choices; "" = follow the organization. */
  timezone: string;
  locale: string;
  date_format: string;
  theme_mode: ThemeModePref | '';
  has_avatar: boolean;
  avatar_url?: string;
  effective: EffectivePreferences;
  updated_at: string;
}

/** Omitted = unchanged; "" clears (full_name cannot be cleared). */
export interface UserProfilePatch {
  full_name?: string;
  job_title?: string;
  phone?: string;
  bio?: string;
  timezone?: string;
  locale?: string;
  date_format?: string;
  theme_mode?: ThemeModePref | '';
}

export const profileService = {
  async getMe(): Promise<MyProfile> {
    const { data } = await api.get<MyProfile>('/users/me');
    return data;
  },

  async updateMe(patch: UserProfilePatch): Promise<MyProfile> {
    const { data } = await api.patch<MyProfile>('/users/me', patch);
    return data;
  },

  async uploadAvatar(file: File): Promise<MyProfile> {
    const body = new FormData();
    body.append('file', file);
    const { data } = await api.put<MyProfile>('/users/me/avatar', body, {
      headers: { 'Content-Type': 'multipart/form-data' },
    });
    return data;
  },

  async deleteAvatar(): Promise<MyProfile> {
    const { data } = await api.delete<MyProfile>('/users/me/avatar');
    return data;
  },

  /** The avatar travels through the API client so it carries the session in
   *  every deployment, including the split-origin one where an <img> cannot. */
  async getAvatarBlob(userId: string): Promise<Blob> {
    const { data } = await api.get<Blob>(`/users/${userId}/avatar`, { responseType: 'blob' });
    return data;
  },
};
