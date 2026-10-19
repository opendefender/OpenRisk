// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

/**
 * The top-level namespaces of `locales/fr.json` that ship in the entry bundle
 * (#858). They are the ones the app shell (sidebar, header, org switcher,
 * action center badge, the risk counts) reads before any screen has loaded.
 * Every other namespace loads with the first lazy screen, in parallel with its
 * code (see `lazyScreen`).
 *
 * The build enforces this list: a module in the entry graph that names a key
 * outside these namespaces fails `vite build` (vite.config.ts,
 * `openrisk:shell-catalog`), because that key would render blank on first
 * paint.
 */
export const SHELL_NAMESPACES = ['common', 'risks', 'actionCenter', 'orgSwitcher'] as const;
