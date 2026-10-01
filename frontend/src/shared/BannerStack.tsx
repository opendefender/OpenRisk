// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// #751 phase 4 — the shell's app-level banner stack.
//
// Before this, App.tsx rendered <DemoBanner /> then <OfflineBanner /> as two
// independent, hand-placed divs in plain document flow — no shared container,
// no governed order, "stacking" only in the sense that mount order happened
// to put one above the other. This file gives that order a name.
//
// Order is fixed, not a `priority` prop: Demo is permanent (never dismissible,
// never disappears once a tenant is seeded from fixtures) and sits on top;
// Offline comes and goes with the connection, so it sits closest to the
// header it pushes. At most two banners exist today, both always fully
// readable — no collapsing to "N alerts" (that has no case yet: a third
// app-level banner would need one, and building it unexercised was explicitly
// dropped by the art direction for this phase).
import type { ReactNode } from 'react';

export function BannerStack({ children }: { children: ReactNode }) {
  return <div className="flex flex-col shrink-0">{children}</div>;
}
