// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The organization block at the top of the sidebar, and the menu it opens
// (#296). An account belongs to every organization that invited it; this is
// where the user moves between them.
//
// The block always wore a switcher's chevron, but it only opened /settings. It
// now lists the organizations from GET /auth/organizations and switches through
// POST /auth/switch-org — the server re-checks the membership and re-issues the
// session. The organization settings stay one entry away, at the bottom.
//
// After a switch the page is reloaded on the dashboard rather than navigated
// in place. The sidebar, the header and their counters stay mounted across a
// client-side navigation, and an observer that outlived the cache wipe would go
// on showing the previous organization's figures. A reload is the one boundary
// no component survives.

import { useEffect, useId, useRef, useState } from 'react';
import { useNavigate } from 'react-router';
import { useMutation, useQuery } from '@tanstack/react-query';
import { Check, ChevronsUpDown, Settings } from 'lucide-react';

import { cn } from '../../shared/ds';
import { useI18n } from '../../hooks/useI18n';
import { useAuthStore } from '../../hooks/useAuthStore';
import { useRbacCatalog } from '../../features/rbac/useRbac';
import {
  orgSwitchService,
  type MembershipSummary,
} from '../../features/organization/orgSwitchService';
import { OrgPlanLabel } from './OrgPlanLabel';

const MY_ORGANIZATIONS_KEY = ['auth', 'organizations'] as const;

const ORG_ROLE_KEYS: Record<string, string> = {
  root: 'common.orgRole.root',
  admin: 'common.orgRole.admin',
  user: 'common.orgRole.user',
};

/** The enabled menu entries, in order — what the arrow keys move between. */
function menuItems(menu: HTMLElement | null): HTMLElement[] {
  return Array.from(
    menu?.querySelectorAll<HTMLElement>('[role^="menuitem"]:not([disabled])') ?? [],
  );
}

function initials(name: string): string {
  const parts = name.trim().split(/\s+/);
  return ((parts[0]?.[0] ?? '') + (parts[1]?.[0] ?? '')).toUpperCase() || 'OR';
}

interface OrgSwitcherProps {
  /** The active organization's display name. */
  orgName: string;
  /** What happens once the session is in the new organization. Tests replace
   *  the reload. */
  onSwitched?: () => void;
}

export function OrgSwitcher({
  orgName,
  onSwitched = () => window.location.assign('/'),
}: OrgSwitcherProps) {
  const { t, locale } = useI18n();
  const navigate = useNavigate();
  const currentOrgId = useAuthStore((s) => s.user?.tenant_id);
  const switchOrganization = useAuthStore((s) => s.switchOrganization);
  const [open, setOpen] = useState(false);
  const menuId = useId();
  const rootRef = useRef<HTMLDivElement>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const menuRef = useRef<HTMLDivElement>(null);

  const orgs = useQuery({
    queryKey: MY_ORGANIZATIONS_KEY,
    queryFn: () => orgSwitchService.listMine(),
    enabled: open,
    staleTime: 60_000,
  });
  const { data: catalog } = useRbacCatalog();

  const doSwitch = useMutation({
    mutationFn: (org: MembershipSummary) => switchOrganization(org.organization_id, org.name),
    onSuccess: () => onSwitched(),
  });

  const close = (returnFocus: boolean) => {
    setOpen(false);
    if (returnFocus) triggerRef.current?.focus();
  };

  // A click anywhere outside closes the menu. A listener rather than a
  // full-screen backdrop: the sidebar is translated on mobile, which would size
  // a fixed backdrop to the sidebar instead of the viewport.
  useEffect(() => {
    if (!open) return;
    const onPointerDown = (e: PointerEvent) => {
      if (!rootRef.current?.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener('pointerdown', onPointerDown);
    return () => document.removeEventListener('pointerdown', onPointerDown);
  }, [open]);

  // Opening moves focus into the menu, onto the current organization when the
  // list is there, so the arrow keys start from where the user is.
  useEffect(() => {
    if (!open || !menuRef.current) return;
    const items = menuItems(menuRef.current);
    const current = items.find((el) => el.getAttribute('aria-checked') === 'true');
    (current ?? items[0])?.focus();
  }, [open, orgs.data]);

  const onMenuKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Escape') {
      e.preventDefault();
      close(true);
      return;
    }
    if (e.key === 'Tab') {
      close(false);
      return;
    }
    const items = menuItems(menuRef.current);
    if (items.length === 0) return;
    const at = items.indexOf(document.activeElement as HTMLElement);
    let next = -1;
    if (e.key === 'ArrowDown') next = (at + 1) % items.length;
    else if (e.key === 'ArrowUp') next = (at - 1 + items.length) % items.length;
    else if (e.key === 'Home') next = 0;
    else if (e.key === 'End') next = items.length - 1;
    if (next >= 0) {
      e.preventDefault();
      items[next]?.focus();
    }
  };

  const roleLabel = (org: MembershipSummary): string => {
    if (org.business_role) {
      const preset = catalog?.business_roles.find((r) => r.key === org.business_role);
      if (preset) return locale === 'fr' ? preset.label_fr : preset.label_en;
    }
    const key = ORG_ROLE_KEYS[org.role];
    return key ? t(key) : '';
  };

  const pick = (org: MembershipSummary) => {
    if (doSwitch.isPending) return;
    if (org.organization_id === currentOrgId) {
      close(true);
      return;
    }
    doSwitch.mutate(org);
  };

  const list = orgs.data ?? [];

  return (
    <div ref={rootRef} className="relative">
      <button
        ref={triggerRef}
        type="button"
        aria-haspopup="menu"
        aria-expanded={open}
        aria-controls={open ? menuId : undefined}
        aria-label={t('orgSwitcher.trigger', { name: orgName })}
        onClick={() => {
          doSwitch.reset();
          setOpen((v) => !v);
        }}
        className="w-full flex items-center gap-2.5 px-2 py-1.5 rounded-[9px] hover:bg-hover focus-visible:bg-hover outline-none focus-visible:ring-2 focus-visible:ring-accent transition-colors"
      >
        <div
          className="w-[26px] h-[26px] rounded-[7px] flex items-center justify-center text-[11px] font-bold shrink-0 text-accent-strong"
          style={{ background: 'var(--accent-soft)' }}
          aria-hidden="true"
        >
          {initials(orgName)}
        </div>
        <div className="min-w-0 flex-1 text-left">
          <div className="text-[12.5px] font-semibold leading-tight text-ink truncate">
            {orgName}
          </div>
          <OrgPlanLabel />
        </div>
        <ChevronsUpDown size={13} className="text-ink-muted shrink-0" aria-hidden="true" />
      </button>

      {open && (
        <div
          ref={menuRef}
          id={menuId}
          role="menu"
          aria-label={t('orgSwitcher.title')}
          onKeyDown={onMenuKeyDown}
          className="absolute left-0 right-0 top-[calc(100%+4px)] z-60 rounded-[12px] overflow-hidden shadow-card-lg"
          style={{
            background: 'var(--bg-elevated)',
            border: '1px solid var(--border)',
            animation: 'or-scalein .14s cubic-bezier(.2,.8,.2,1)',
          }}
        >
          <div className="px-3 pt-2.5 pb-1.5 text-[10.5px] tracking-[0.08em] uppercase font-semibold text-ink-muted">
            {t('orgSwitcher.title')}
          </div>

          <div className="max-h-[50vh] overflow-y-auto">
            {orgs.isLoading && (
              <div aria-busy="true" aria-label={t('orgSwitcher.loading')} className="px-3 pb-2">
                {[0, 1].map((i) => (
                  <div key={i} className="flex items-center gap-2.5 py-2">
                    <div className="w-[22px] h-[22px] rounded-[6px] or-skeleton" />
                    <div className="flex-1 space-y-1.5">
                      <div className="h-[9px] w-3/4 rounded or-skeleton" />
                      <div className="h-[8px] w-1/3 rounded or-skeleton" />
                    </div>
                  </div>
                ))}
              </div>
            )}

            {orgs.isError && (
              <div className="px-3 pb-2.5 text-[12px] text-ink-soft">
                <p role="alert">{t('orgSwitcher.loadError')}</p>
                <button
                  type="button"
                  role="menuitem"
                  onClick={() => void orgs.refetch()}
                  className="mt-1.5 text-[12px] font-semibold text-accent hover:underline outline-none focus-visible:ring-2 focus-visible:ring-accent rounded"
                >
                  {t('orgSwitcher.retry')}
                </button>
              </div>
            )}

            {list.map((org) => {
              const current = org.organization_id === currentOrgId;
              const pending =
                doSwitch.isPending && doSwitch.variables?.organization_id === org.organization_id;
              return (
                <button
                  key={org.organization_id}
                  type="button"
                  role="menuitemradio"
                  aria-checked={current}
                  disabled={doSwitch.isPending && !pending}
                  onClick={() => pick(org)}
                  className={cn(
                    'w-full flex items-center gap-2.5 px-3 py-2 text-left transition-colors outline-none',
                    'hover:bg-hover focus-visible:bg-hover disabled:opacity-50',
                  )}
                >
                  <div
                    className="w-[22px] h-[22px] rounded-[6px] flex items-center justify-center text-[10px] font-bold shrink-0 text-accent-strong"
                    style={{ background: 'var(--accent-soft)' }}
                    aria-hidden="true"
                  >
                    {initials(org.name)}
                  </div>
                  <div className="min-w-0 flex-1">
                    <div className="text-[12.5px] font-semibold text-ink truncate">{org.name}</div>
                    <div className="text-[10.5px] text-ink-soft truncate">
                      {pending ? t('orgSwitcher.switching') : roleLabel(org)}
                    </div>
                  </div>
                  {current && (
                    <Check
                      size={14}
                      className="text-accent shrink-0"
                      aria-label={t('orgSwitcher.current')}
                    />
                  )}
                </button>
              );
            })}

            {orgs.isSuccess && list.length <= 1 && (
              <p className="px-3 pb-2 pt-1 text-[11px] leading-snug text-ink-muted">
                {t('orgSwitcher.onlyOne')}
              </p>
            )}
          </div>

          {doSwitch.isError && (
            <p
              role="alert"
              className="px-3 py-2 text-[11.5px] leading-snug border-t border-border"
              style={{ color: 'var(--critical)' }}
            >
              {t('orgSwitcher.switchError', { name: orgName })}
            </p>
          )}

          <button
            type="button"
            role="menuitem"
            onClick={() => {
              close(false);
              navigate('/settings');
            }}
            className="w-full flex items-center gap-2.5 px-3 py-2.5 text-left text-[12.5px] font-medium text-ink border-t border-border hover:bg-hover focus-visible:bg-hover outline-none transition-colors"
          >
            <Settings size={15} strokeWidth={1.8} aria-hidden="true" />
            {t('orgSwitcher.settings')}
          </button>
        </div>
      )}
    </div>
  );
}
