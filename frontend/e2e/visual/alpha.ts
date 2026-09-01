// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * Stylesheet-only harness for e2e/visual/alpha-modifiers.spec.ts.
 *
 * It renders nothing. The spec injects one element per opacity-modifier class
 * and reads what the browser computed, so all this file has to do is put the
 * real stylesheet on the page — the same one the app loads, not a rebuilt
 * subset, because #427 was a bug in what the stylesheet CONTAINED.
 */

import '../../src/index.css';

document.getElementById('alpha-root')!.setAttribute('data-ready', 'true');
