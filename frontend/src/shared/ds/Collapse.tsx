// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: Apache-2.0

/**
 * Collapse — the panel half of a disclosure: a section that opens and closes
 * under a trigger the caller owns (a row, a "3 decisions" link, a heading).
 *
 * The trigger stays with the caller because it is never the same twice: a run
 * row carries a status, a duration and a replay button beside the toggle, and a
 * primitive that owned the trigger would have to own all of that too. What the
 * caller must add to its button is two attributes:
 *
 *   <button aria-expanded={open} aria-controls={id} onClick={toggle}>…</button>
 *   <Collapse id={id} open={open}>…</Collapse>
 *
 * MOTION    Height animates through grid-template-rows 0fr → 1fr, on --dur-slow
 *           and --ease-out, the same both ways (#751): opening and closing a
 *           section is one reversible motion. No measuring, so content that
 *           changes height while open just reflows. Reduced motion removes the
 *           transition (global rule in index.css); the section still opens.
 *
 * A11Y      Closed content is `inert` and aria-hidden: it stays in the DOM so
 *           the close can animate, but it is neither focusable nor read out.
 *           Content is not mounted until the first open, so a list of fifty
 *           collapsed rows does not render fifty hidden panels.
 */

import { useState, type ReactNode } from 'react';

export interface CollapseProps {
  /** The id the trigger's aria-controls points at. */
  id: string;
  open: boolean;
  children: ReactNode;
  className?: string;
}

export function Collapse({ id, open, children, className }: CollapseProps) {
  // Mount on first open, then keep: unmounting on close would cut the close
  // animation short. Adjusted during render, not in an effect, so the content
  // and the 1fr row land in the same commit and the opening animates.
  const [mounted, setMounted] = useState(open);
  if (open && !mounted) setMounted(true);

  return (
    <div
      id={id}
      data-state={open ? 'open' : 'closed'}
      aria-hidden={!open}
      inert={!open}
      className="grid transition-[grid-template-rows] duration-slow ease-out"
      style={{ gridTemplateRows: open ? '1fr' : '0fr' }}
    >
      {/* Two layers: the outer one shrinks to 0 and clips, the inner one takes
          the caller's spacing. Padding on the clipping layer would keep it
          that many pixels tall when closed. */}
      <div className="min-h-0 overflow-hidden">
        <div className={className}>{mounted && children}</div>
      </div>
    </div>
  );
}
