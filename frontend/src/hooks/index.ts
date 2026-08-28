// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Notification presentation helpers.
//
// The WebSocket notification hook that used to be exported here has been
// removed: it connected to ws://localhost:8080/ws/notifications, an endpoint the
// backend has never had, and read its URL from a Create-React-App environment
// variable in a Vite project. Live delivery now comes from the one realtime
// transport in src/lib/realtime.ts.
export {
  useNotificationAudio,
  checkNotificationSupport,
  vibrateNotification,
  type UseNotificationAudioOptions,
} from './useNotificationAudio';
