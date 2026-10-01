// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

/**
 * Matches --dur-fast (src/styles/primitives.css) — the notif panel's exit.
 * Its own module so a test can read the token from disk and assert this
 * literal hasn't drifted from it, without AppHeader.tsx exporting a
 * non-component (which breaks fast refresh).
 */
export const NOTIF_EXIT_MS = 120;
