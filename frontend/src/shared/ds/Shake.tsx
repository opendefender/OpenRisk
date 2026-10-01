// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: Apache-2.0

/**
 * Shakes its children once whenever `errorKey` changes to a new truthy value.
 * Promoted from `features/auth/fields.tsx` (#751 phase 2) so `Field` can host
 * it for every form, not only the auth screens.
 *
 * Implemented by using errorKey as the React `key`: a changed key remounts the
 * wrapper, and a fresh mount restarts the CSS animation (`--animate-or-shake`,
 * `or-shake` in index.css). That is why callers pass an incrementing nonce
 * rather than the message itself — a SECOND failure with identical wording
 * still has to shake, or someone retyping the same wrong value gets no
 * feedback at all and concludes the control is dead. It must fire once per
 * failed SUBMIT: bump the nonce from the submit handler's failure branch only,
 * never from onChange/onBlur, or every keystroke before the user has even
 * submitted would shake the field.
 *
 * Deriving the animation from the key rather than from an effect keeps this
 * render-pure: no state, no effect, nothing to get out of step.
 *
 * The shake is decoration layered on top of the message and the invalid
 * border; it never auto-reverts on its own timer, because the border and
 * message follow real validity (WCAG 3.3.1) — the shake plays once and then
 * gets out of the way. Under `prefers-reduced-motion` the global CSS rule
 * strips the animation and the other two still carry the meaning.
 */

import type { ReactNode } from 'react';

export function Shake({ errorKey, children }: { errorKey?: string | number; children: ReactNode }) {
  return (
    <div
      key={String(errorKey ?? '')}
      className={errorKey ? 'motion-safe:animate-or-shake' : undefined}
    >
      {children}
    </div>
  );
}
