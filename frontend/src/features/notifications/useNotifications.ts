// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// React Query bindings for the notification bell.

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { notificationService, type NotificationPreferencePatch } from './notificationService';

const NOTIF_KEY = ['notifications'];
const UNREAD_KEY = ['notifications', 'unread-count'];

export function useNotifications(limit = 20) {
  const { data, isLoading, isError } = useQuery({
    queryKey: [...NOTIF_KEY, limit],
    queryFn: () => notificationService.list(limit),
    // Deliberately no placeholderData: an invented notification is worse than a
    // brief skeleton, and a stale placeholder would let the bell claim activity
    // on a tenant that has none.
    refetchInterval: 60_000,
  });
  return { notifications: data ?? [], isLoading, isError };
}

export function useUnreadCount() {
  const { data } = useQuery({
    queryKey: UNREAD_KEY,
    queryFn: () => notificationService.unreadCount(),
    refetchInterval: 60_000,
  });
  // 0 until the server says otherwise. The bell's unread dot used to be a static
  // element in the markup, so it was lit on a tenant with no notifications at all.
  return data ?? 0;
}

export function useNotificationActions() {
  const qc = useQueryClient();
  const invalidate = () => {
    qc.invalidateQueries({ queryKey: NOTIF_KEY });
  };
  const markRead = useMutation({
    mutationFn: (id: string) => notificationService.markRead(id),
    onSuccess: invalidate,
  });
  const markAllRead = useMutation({
    mutationFn: () => notificationService.markAllRead(),
    onSuccess: invalidate,
  });
  return { markRead, markAllRead };
}

/* -------------------------------------------------------------------------- */
/* Preferences (W0-05 / D2)                                                    */
/*                                                                             */
/* Settings › Notifications wrote these to localStorage while the real API sat  */
/* unused, so the switches changed nothing and carried across a change of user. */
/* They are now server state: per user, per tenant, and read by the backend     */
/* before it sends (domain.NotificationPreference.Allows).                      */
/* -------------------------------------------------------------------------- */

export const NOTIF_PREFS_KEY = ['notifications', 'preferences'];

export function useNotificationPreferences() {
  const { data, isLoading, isError, refetch } = useQuery({
    queryKey: NOTIF_PREFS_KEY,
    queryFn: () => notificationService.getPreferences(),
    // No placeholder and no default object: rendering a switch position before
    // the server has said what it is would be the same lie in a smaller frame.
    // The screen shows a skeleton until it knows.
    staleTime: 60_000,
  });
  return { prefs: data, isLoading, isError, refetch };
}

export function useUpdateNotificationPreferences() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (patch: NotificationPreferencePatch) =>
      notificationService.updatePreferences(patch),
    // The response IS the stored row, so seed the cache with it rather than
    // refetching: what renders afterwards is what the server holds, not what
    // the client hoped it would hold.
    onSuccess: (saved) => qc.setQueryData(NOTIF_PREFS_KEY, saved),
  });
}
