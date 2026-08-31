// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// <UserPicker> — the ONE way anything is assigned to anyone.
//
// It is a single component on purpose. Assignment used to be a free-text field
// here, a jsonb array there and nothing at all elsewhere, so "who is this for?"
// had four different answers and none of them was searchable. One picker, one
// list, one payload shape.
//
// The list it offers comes from GET /ownership/assignable — the SAME endpoint
// the API validates writes against. A picker that offers someone the server
// would refuse is a lie with a dropdown around it.
//
// Usable from a modal, a drawer, a row menu, a bulk bar and a detail page: it
// renders as a plain trigger button + a floating panel, takes its width from its
// container, and never assumes it is inside a form.

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { Check, Loader2, Search, UserX, Users, X } from 'lucide-react';
import {
  ownershipService,
  ROLE_LABELS,
  type Assignee,
  type AssignableGroup,
  type OwnershipRole,
} from '../services/ownershipService';
import { useUIStore } from '../store/uiStore';

interface UserPickerProps {
  /** Currently selected user id, or null when the slot is empty. */
  value: string | null;
  /** null = unassign. Called only when the selection really changes. */
  onChange: (userId: string | null) => void;
  /** Which slot this picker fills — drives the label and the empty-state copy. */
  role?: OwnershipRole;
  /**
   * Permission the assignee should hold (e.g. "risks:update"). Members without
   * it are still listed but marked, so the absence is explainable rather than
   * mysterious.
   */
  permission?: string;
  /** Hide members who cannot act, instead of marking them. */
  onlyCapable?: boolean;
  disabled?: boolean;
  /** Shown when nothing is selected. Defaults to the role label. */
  placeholder?: string;
  /** Email the server already resolved, so the trigger reads a name before the list loads. */
  currentEmail?: string;
  /** Compact trigger for row menus and table cells. */
  size?: 'sm' | 'md';
  className?: string;
}

/** Deterministic avatar tint — same input, same colour, across every screen. */
function tintFor(id: string): string {
  const tints = ['var(--accent)', 'var(--high)', 'var(--medium)', 'var(--low)', 'var(--critical)'];
  let hash = 0;
  for (let i = 0; i < id.length; i += 1) hash = (hash * 31 + id.charCodeAt(i)) >>> 0;
  return tints[hash % tints.length];
}

function initialsFrom(email: string): string {
  const name = email.split('@')[0] ?? '';
  const parts = name.split(/[._-]+/).filter(Boolean);
  if (parts.length >= 2) return (parts[0][0] + parts[1][0]).toUpperCase();
  return name.slice(0, 2).toUpperCase() || '?';
}

function AvatarDot({ id, initials, size = 24 }: { id: string; initials: string; size?: number }) {
  return (
    <span
      className="inline-flex items-center justify-center rounded-full font-semibold text-text-primary shrink-0"
      style={{ width: size, height: size, fontSize: size * 0.42, background: tintFor(id) }}
      aria-hidden
    >
      {initials}
    </span>
  );
}

export function UserPicker({
  value,
  onChange,
  role = 'assignee',
  permission,
  onlyCapable = false,
  disabled = false,
  placeholder,
  currentEmail,
  size = 'md',
  className = '',
}: UserPickerProps) {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const roleLabel = ROLE_LABELS[role];

  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState('');
  const [users, setUsers] = useState<Assignee[]>([]);
  const [groups, setGroups] = useState<AssignableGroup[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(false);
  const [activeIndex, setActiveIndex] = useState(0);

  const triggerRef = useRef<HTMLButtonElement>(null);
  const panelRef = useRef<HTMLDivElement>(null);
  const [anchor, setAnchor] = useState<{ top: number; left: number; width: number } | null>(null);

  // Fetch on open and on every debounced keystroke. Cancellable, so a fast
  // typist does not race an older response into the list.
  useEffect(() => {
    if (!open) return;
    const controller = new AbortController();
    const timer = window.setTimeout(async () => {
      setLoading(true);
      setError(false);
      try {
        const res = await ownershipService.listAssignable(
          { q: query || undefined, permission, only_capable: onlyCapable || undefined },
          controller.signal,
        );
        setUsers(res.users ?? []);
        setGroups(res.groups ?? []);
        setActiveIndex(0);
      } catch {
        if (!controller.signal.aborted) setError(true);
      } finally {
        if (!controller.signal.aborted) setLoading(false);
      }
    }, query ? 180 : 0);
    return () => {
      controller.abort();
      window.clearTimeout(timer);
    };
  }, [open, query, permission, onlyCapable]);

  // Position the floating panel against the trigger. Portalled so a picker
  // inside a drawer or a table row is never clipped by an overflow container —
  // the failure mode that made the old row menus unusable on the last row.
  const place = useCallback(() => {
    const el = triggerRef.current;
    if (!el) return;
    const r = el.getBoundingClientRect();
    const width = Math.max(r.width, 288);
    const spaceBelow = window.innerHeight - r.bottom;
    const panelHeight = 340;
    const top = spaceBelow < panelHeight && r.top > panelHeight ? r.top - panelHeight - 6 : r.bottom + 6;
    const left = Math.min(Math.max(8, r.left), window.innerWidth - width - 8);
    setAnchor({ top, left, width });
  }, []);

  useEffect(() => {
    if (!open) return;
    place();
    window.addEventListener('resize', place);
    window.addEventListener('scroll', place, true);
    return () => {
      window.removeEventListener('resize', place);
      window.removeEventListener('scroll', place, true);
    };
  }, [open, place]);

  useEffect(() => {
    if (!open) return;
    const onDown = (e: MouseEvent) => {
      const t = e.target as Node;
      if (panelRef.current?.contains(t) || triggerRef.current?.contains(t)) return;
      setOpen(false);
    };
    document.addEventListener('mousedown', onDown);
    return () => document.removeEventListener('mousedown', onDown);
  }, [open]);

  const selected = useMemo(() => users.find((u) => u.user_id === value), [users, value]);
  const triggerLabel = selected?.full_name || selected?.email || currentEmail || '';
  const triggerInitials = selected?.initials || (currentEmail ? initialsFrom(currentEmail) : '');

  const commit = (userId: string | null) => {
    if (userId !== value) onChange(userId);
    setOpen(false);
    setQuery('');
  };

  const onKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Escape') {
      setOpen(false);
      return;
    }
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      setActiveIndex((i) => Math.min(i + 1, users.length - 1));
    }
    if (e.key === 'ArrowUp') {
      e.preventDefault();
      setActiveIndex((i) => Math.max(i - 1, 0));
    }
    if (e.key === 'Enter' && users[activeIndex]) {
      e.preventDefault();
      commit(users[activeIndex].user_id);
    }
  };

  const pad = size === 'sm' ? 'px-2.5 py-1.5 text-[12px]' : 'px-3 py-2.5 text-sm';

  return (
    <>
      <button
        ref={triggerRef}
        type="button"
        disabled={disabled}
        onClick={() => setOpen((o) => !o)}
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-label={`${roleLabel[lang === 'fr' ? 'fr' : 'en']}${triggerLabel ? ` : ${triggerLabel}` : ''}`}
        className={`flex w-full items-center gap-2 rounded-2xl border border-border bg-elevated ${pad} text-left text-ink transition hover:border-accent disabled:cursor-not-allowed disabled:opacity-60 ${className}`}
      >
        {value && triggerInitials ? (
          <AvatarDot id={value} initials={triggerInitials} size={size === 'sm' ? 20 : 24} />
        ) : (
          <span className="inline-flex h-6 w-6 items-center justify-center rounded-full border border-dashed border-border text-ink-muted shrink-0">
            <Users size={12} />
          </span>
        )}
        <span className={`truncate ${value ? '' : 'text-ink-muted'}`}>
          {triggerLabel || placeholder || tr(`Choisir un ${roleLabel.fr.toLowerCase()}`, `Pick ${roleLabel.en.toLowerCase()}`)}
        </span>
      </button>

      {open && anchor
        ? createPortal(
            <div
              ref={panelRef}
              role="listbox"
              onKeyDown={onKeyDown}
              className="fixed z-70 overflow-hidden rounded-3xl border border-border bg-elevated shadow-2xl"
              style={{ top: anchor.top, left: anchor.left, width: anchor.width }}
            >
              <div className="flex items-center gap-2 border-b border-border px-3 py-2.5">
                <Search size={14} className="text-ink-muted shrink-0" />
                <input
                  autoFocus
                  value={query}
                  onChange={(e) => setQuery(e.target.value)}
                  placeholder={tr('Rechercher un membre…', 'Search a member…')}
                  className="w-full bg-transparent text-sm text-ink outline-none placeholder:text-ink-muted"
                />
                {loading ? <Loader2 size={14} className="animate-spin text-ink-muted" /> : null}
                <button
                  type="button"
                  onClick={() => setOpen(false)}
                  aria-label={tr('Fermer', 'Close')}
                  className="text-ink-muted hover:text-ink"
                >
                  <X size={14} />
                </button>
              </div>

              <div className="max-h-[264px] overflow-y-auto py-1">
                {/* Unassigning is a first-class choice, not a hidden gesture. */}
                {value ? (
                  <button
                    type="button"
                    onClick={() => commit(null)}
                    className="flex w-full items-center gap-2 px-3 py-2 text-left text-[13px] text-ink-soft hover:bg-app"
                  >
                    <UserX size={14} />
                    {tr('Retirer l’affectation', 'Unassign')}
                  </button>
                ) : null}

                {error ? (
                  <p className="px-3 py-4 text-[13px] text-ink-muted">
                    {tr(
                      'Impossible de charger les membres. Réessayez, ou contactez un administrateur.',
                      'Could not load members. Try again, or contact an administrator.',
                    )}
                  </p>
                ) : null}

                {!error && !loading && users.length === 0 ? (
                  <p className="px-3 py-4 text-[13px] text-ink-muted">
                    {query
                      ? tr('Aucun membre ne correspond.', 'No member matches.')
                      : tr(
                          "Aucun membre assignable. Invitez quelqu'un depuis Paramètres › Membres.",
                          'No assignable member. Invite someone from Settings › Members.',
                        )}
                  </p>
                ) : null}

                {users.map((u, i) => {
                  const isSelected = u.user_id === value;
                  return (
                    <button
                      key={u.user_id}
                      type="button"
                      role="option"
                      aria-selected={isSelected}
                      onMouseEnter={() => setActiveIndex(i)}
                      onClick={() => commit(u.user_id)}
                      className={`flex w-full items-center gap-2.5 px-3 py-2 text-left transition ${
                        i === activeIndex ? 'bg-app' : ''
                      }`}
                    >
                      <AvatarDot id={u.user_id} initials={u.initials} />
                      <span className="min-w-0 flex-1">
                        <span className="block truncate text-[13px] text-ink">{u.full_name || u.email}</span>
                        <span className="block truncate text-[11px] text-ink-muted">
                          {u.business_role_label ? `${u.business_role_label} · ` : ''}
                          {u.email}
                        </span>
                      </span>
                      {/* Marked, not hidden: "why can't I assign them?" needs a
                          visible answer, and hiding people just moves the
                          question to a support ticket. */}
                      {permission && !u.can_act ? (
                        <span
                          className="rounded-full px-1.5 py-0.5 text-[10px] font-semibold"
                          style={{ background: 'color-mix(in srgb, var(--medium) 16%, transparent)', color: 'var(--medium)' }}
                          title={tr(
                            `Ce membre n'a pas la permission ${permission}.`,
                            `This member lacks the ${permission} permission.`,
                          )}
                        >
                          {tr('sans droit', 'no access')}
                        </span>
                      ) : null}
                      {isSelected ? <Check size={14} style={{ color: 'var(--accent-500)' }} /> : null}
                    </button>
                  );
                })}

                {/* Role buckets are informational: the model assigns to a person,
                    so picking "the RSSIs" would have to invent one. Showing the
                    bucket helps a user find the right person inside it. */}
                {groups.length > 0 && !query ? (
                  <div className="border-t border-border px-3 py-2">
                    <p className="mb-1.5 text-[10px] font-semibold uppercase tracking-[0.16em] text-ink-muted">
                      {tr('Rôles', 'Roles')}
                    </p>
                    <div className="flex flex-wrap gap-1.5">
                      {groups.map((g) => (
                        <button
                          key={g.key}
                          type="button"
                          onClick={() => setQuery('')}
                          title={tr(`${g.count} membre(s)`, `${g.count} member(s)`)}
                          className="rounded-full border border-border px-2 py-0.5 text-[11px] text-ink-soft hover:border-accent"
                        >
                          {g.label} · {g.count}
                        </button>
                      ))}
                    </div>
                  </div>
                ) : null}
              </div>
            </div>,
            document.body,
          )
        : null}
    </>
  );
}

/**
 * The three slots as one block. Used by the risk drawer, the mitigation drawer
 * and the create modals so the wording and the order never diverge.
 */
export function OwnershipFields({
  value,
  onChange,
  permission,
  disabled,
}: {
  value: { owner_id?: string | null; assignee_id?: string | null; reviewer_id?: string | null;
    owner_email?: string; assignee_email?: string; reviewer_email?: string };
  onChange: (role: OwnershipRole, userId: string | null) => void;
  permission?: string;
  disabled?: boolean;
}) {
  const lang = useUIStore((s) => s.lang);
  const roles: OwnershipRole[] = ['owner', 'assignee', 'reviewer'];
  const ids: Record<OwnershipRole, string | null> = {
    owner: value.owner_id ?? null,
    assignee: value.assignee_id ?? null,
    reviewer: value.reviewer_id ?? null,
  };
  const emails: Record<OwnershipRole, string> = {
    owner: value.owner_email ?? '',
    assignee: value.assignee_email ?? '',
    reviewer: value.reviewer_email ?? '',
  };

  return (
    <div className="grid gap-3 sm:grid-cols-3">
      {roles.map((role) => {
        const meta = ROLE_LABELS[role];
        return (
          <div key={role} className="space-y-1.5">
            <label className="block text-[10px] font-semibold uppercase tracking-[0.16em] text-ink-muted">
              {lang === 'fr' ? meta.fr : meta.en}
            </label>
            <UserPicker
              role={role}
              value={ids[role]}
              currentEmail={emails[role]}
              onChange={(id) => onChange(role, id)}
              permission={permission}
              disabled={disabled}
              size="sm"
            />
            <p className="text-[11px] text-ink-muted">{lang === 'fr' ? meta.hint_fr : meta.hint_en}</p>
          </div>
        );
      })}
    </div>
  );
}
