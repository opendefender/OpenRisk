// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import { lazy } from 'react';

import { loadScreenCatalogs } from './catalog';

/**
 * `React.lazy` that also waits for the screen catalogue (#858). The entry
 * bundle carries only the words the app shell reads; a lazy component gets the
 * rest of the default language before it first renders, fetched in parallel
 * with its code, so it never shows a raw key. Same signature as `lazy`.
 */
export const lazyScreen: typeof lazy = (load) =>
  lazy(() => Promise.all([load(), loadScreenCatalogs()]).then(([module]) => module));
