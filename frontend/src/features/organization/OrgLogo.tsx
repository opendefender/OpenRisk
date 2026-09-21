// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The organization's logo when it uploaded one, its initials otherwise (#718).
// The picture is only ever read from GET /organization/logo — never from a URL
// stored on the organization row.

import { useOrganizationLogoUrl } from './useOrganization';

function initialsOf(name: string): string {
  const parts = name.trim().split(/\s+/);
  return ((parts[0]?.[0] ?? '') + (parts[1]?.[0] ?? '')).toUpperCase() || 'OR';
}

interface OrgLogoProps {
  name: string;
  hasLogo: boolean;
  size: number;
  radius: number;
  className?: string;
}

export function OrgLogo({ name, hasLogo, size, radius, className = '' }: OrgLogoProps) {
  const src = useOrganizationLogoUrl(hasLogo);
  return (
    <div
      className={`flex items-center justify-center font-bold shrink-0 overflow-hidden text-accent-strong ${className}`}
      style={{
        width: size,
        height: size,
        borderRadius: radius,
        fontSize: Math.round(size * 0.38),
        background: src ? 'var(--bg-elevated)' : 'var(--accent-soft)',
      }}
      aria-hidden="true"
      data-testid="org-logo"
    >
      {src ? <img src={src} alt="" className="w-full h-full object-contain" /> : initialsOf(name)}
    </div>
  );
}
