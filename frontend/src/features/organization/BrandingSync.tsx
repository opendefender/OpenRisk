// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Applies the organization's accent to every member's interface (#718).
//
// Applied when the SERVER value changes — at sign-in, and right after an
// administrator changes it — not on every refetch, so a member who picks a
// different accent on their own device keeps it until the organization's
// choice changes again. An unset accent leaves the device's choice alone.
//
// Renders nothing; mounted once inside the authenticated layout.

import { useEffect, useRef } from 'react';

import { useUIStore } from '../../store/uiStore';
import { isAccentPreset } from '../../shared/accentPresets';
import { useOrganizationBranding } from './useOrganization';

export function BrandingSync() {
  const { data } = useOrganizationBranding();
  const setVariant = useUIStore((s) => s.setVariant);
  const applied = useRef<string | undefined>(undefined);

  useEffect(() => {
    if (!data) return;
    const accent = data.accent;
    if (isAccentPreset(accent) && accent !== applied.current) {
      setVariant(accent);
    }
    applied.current = accent;
  }, [data, setVariant]);

  return null;
}
