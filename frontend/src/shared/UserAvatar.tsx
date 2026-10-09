// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// A person's avatar (#719): their uploaded picture when they have one, their
// initials otherwise. The picture is only ever read from this API — never from
// an address stored by a sign-up form or an identity provider.

import { useAvatarUrl } from '../features/profile/useProfile';

function initialsOf(name: string | undefined, fallback = '?'): string {
  if (!name?.trim()) return fallback;
  const parts = name.trim().split(/\s+/);
  return ((parts[0]?.[0] ?? '') + (parts[1]?.[0] ?? '')).toUpperCase() || fallback;
}

interface UserAvatarProps {
  userId?: string;
  name?: string;
  hasAvatar?: boolean;
  size?: number;
  className?: string;
  fallback?: string;
}

export function UserAvatar({
  userId,
  name,
  hasAvatar = false,
  size = 30,
  className = '',
  fallback,
}: UserAvatarProps) {
  const src = useAvatarUrl(userId, hasAvatar);
  return (
    <div
      // Neutral initials disc from the October 2026 redesign: surface-3 with a
      // hairline, so a column of avatars reads as people, not as accent badges.
      className={`rounded-full flex items-center justify-center font-semibold text-fg-secondary bg-surface-3 border border-border-default shrink-0 overflow-hidden ${className}`}
      style={{
        width: size,
        height: size,
        fontSize: Math.round(size * 0.37),
      }}
      aria-hidden="true"
    >
      {src ? (
        <img src={src} alt="" className="w-full h-full object-cover" />
      ) : (
        initialsOf(name, fallback)
      )}
    </div>
  );
}
