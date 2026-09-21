// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// React Query hooks for the self-service profile (#719).

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { profileService, type MyProfile, type UserProfilePatch } from './profileService';
import { ACTIVATION_QUERY_KEY } from '../onboarding/useActivation';

export const MY_PROFILE_KEY = ['profile', 'me'] as const;
export const avatarKey = (userId: string) => ['profile', 'avatar', userId] as const;

export function useMyProfile(enabled = true) {
  return useQuery({
    queryKey: MY_PROFILE_KEY,
    queryFn: () => profileService.getMe(),
    enabled,
  });
}

/** Optimistic: the edited values show at once and are put back on refusal. */
export function useUpdateMyProfile() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (patch: UserProfilePatch) => profileService.updateMe(patch),
    onMutate: async (patch) => {
      await qc.cancelQueries({ queryKey: MY_PROFILE_KEY });
      const previous = qc.getQueryData<MyProfile>(MY_PROFILE_KEY);
      if (previous) {
        const changes = Object.fromEntries(
          Object.entries(patch).filter(([, v]) => v !== undefined),
        );
        qc.setQueryData<MyProfile>(MY_PROFILE_KEY, { ...previous, ...changes });
      }
      return { previous };
    },
    onError: (_err, _patch, ctx) => {
      if (ctx?.previous) qc.setQueryData(MY_PROFILE_KEY, ctx.previous);
    },
    onSuccess: (profile) => {
      qc.setQueryData(MY_PROFILE_KEY, profile);
      // Saving a name ticks the "complete your profile" checklist step.
      void qc.invalidateQueries({ queryKey: ACTIVATION_QUERY_KEY });
    },
  });
}

function useAvatarMutation<TVars>(fn: (vars: TVars) => Promise<MyProfile>) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: fn,
    onSuccess: (profile) => {
      qc.setQueryData(MY_PROFILE_KEY, profile);
      void qc.invalidateQueries({ queryKey: avatarKey(profile.id) });
    },
  });
}

export function useUploadAvatar() {
  return useAvatarMutation((file: File) => profileService.uploadAvatar(file));
}

export function useDeleteAvatar() {
  return useAvatarMutation(() => profileService.deleteAvatar());
}

function blobToDataUrl(blob: Blob): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(String(reader.result));
    reader.onerror = () => reject(reader.error ?? new Error('avatar unreadable'));
    reader.readAsDataURL(blob);
  });
}

/**
 * A user's avatar as a data URL (at most 1 MB), or null while loading, on error
 * or without an avatar. A data URL rather than an object URL: nothing to revoke,
 * so a cached avatar survives remounts and StrictMode's double effects.
 */
export function useAvatarUrl(userId: string | undefined, hasAvatar: boolean): string | null {
  const { data } = useQuery({
    queryKey: avatarKey(userId ?? ''),
    queryFn: async () => blobToDataUrl(await profileService.getAvatarBlob(userId as string)),
    enabled: Boolean(userId) && hasAvatar,
    staleTime: 5 * 60_000,
    retry: false,
  });
  return hasAvatar ? (data ?? null) : null;
}
