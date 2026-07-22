// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Export all notification hooks
export { useNotificationWebSocket, type UseNotificationWebSocketOptions } from './useNotificationWebSocket';
export { useNotificationAudio, checkNotificationSupport, vibrateNotification, type UseNotificationAudioOptions } from './useNotificationAudio';

// Type exports
export type { Notification } from './useNotificationWebSocket';
