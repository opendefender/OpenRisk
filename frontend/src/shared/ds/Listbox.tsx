// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: Apache-2.0

/**
 * Listbox — the console's select. Picks one VALUE from a list (#935, D-069 C).
 *
 * WHY NOT THE NATIVE <select>   In dark mode the browser draws the open list
 * with OS chrome, which breaks the design on every screen that uses one. This
 * draws its own list and keeps everything the native control gave for free:
 * keyboard, label, error wiring, form submission. `Select` (Field.tsx) stays for
 * the call sites not yet migrated; its API is frozen by #443.
 *
 * THE ARIA PATTERN is the select-only combobox from the APG. The trigger is a
 * button with role="combobox" and keeps DOM focus for the whole interaction; the
 * active option is pointed at with aria-activedescendant, so a screen reader
 * announces it while the user moves. Long lists (more than `searchThreshold`
 * options) add a search field in the popup: focus moves into it, it takes the
 * same keys, and it owns aria-activedescendant instead.
 *
 * KEYBOARD
 *   closed  Enter / Space / Alt+Down / Down / Up open on the current value;
 *           Home / End open on the first / last option; a letter opens and
 *           jumps (or starts the search when there is one).
 *   open    Up / Down move, Home / End jump, letters type ahead, Enter and
 *           Space (outside the search) pick, Alt+Up picks and closes.
 *           Escape closes and KEEPS the value. Tab closes without picking and
 *           lets focus move on, so tabbing through a form never changes data.
 *
 * LAYERING   The list renders in a portal on the top layer (z-tooltip). Not
 * z-popover: hand-rolled overlays in the console go up to z-90 (members invite,
 * found live), and a list opened from a dialog that draws behind the dialog's
 * backdrop is unusable. It is positioned by @floating-ui: `flip` turns it upward near the bottom of the
 * viewport, `size` caps its height to the room left, `hide` drops it when its
 * trigger scrolls out of a scrolling panel. No container can clip it. Escape is
 * registered as a light layer (useEscapeLayer), so inside a modal it closes the
 * list, not the modal.
 */

import {
  useCallback,
  useEffect,
  useId,
  useMemo,
  useRef,
  useState,
  type KeyboardEvent,
  type Ref,
} from 'react';
import {
  FloatingPortal,
  autoUpdate,
  flip,
  hide,
  offset,
  shift,
  size,
  useFloating,
} from '@floating-ui/react';
import { Check, ChevronDown, Loader2, Search } from 'lucide-react';
import { useI18n } from '../../hooks/useI18n';
import { cn } from './cn';
import { controlClasses, useControlWiring } from './fieldContext';
import { useEscapeLayer } from './useDismissableLayer';

export interface ListboxOption<V extends string = string> {
  value: V;
  label: string;
  /** A second, muted line under the label. */
  description?: string;
  disabled?: boolean;
}

export type ListboxSize = 'sm' | 'md';

export interface ListboxProps<V extends string = string> {
  /** The current value. A value matching no option shows the placeholder. */
  value: V | '' | null | undefined;
  onChange: (value: V) => void;
  options: ReadonlyArray<ListboxOption<V>>;
  /** Submits the value with a surrounding <form>, through a hidden input. */
  name?: string;
  disabled?: boolean;
  /** Options are still arriving: spinner on the trigger, a notice in the list. */
  loading?: boolean;
  placeholder?: string;
  /** Overrides the "no option" text shown when `options` is empty. */
  emptyLabel?: string;
  /** Above this many options the list gets a search field. Default 8. */
  searchThreshold?: number;
  size?: ListboxSize;
  id?: string;
  className?: string;
  title?: string;
  'aria-label'?: string;
  'aria-labelledby'?: string;
  'data-testid'?: string;
  ref?: Ref<HTMLButtonElement>;
}

const SEARCH_THRESHOLD = 8;
const TYPEAHEAD_MS = 500;

/** NFD-normalise and strip combining marks, so `regl` matches `Règlement`. */
function fold(s: string): string {
  return s.normalize('NFD').replace(/[̀-ͯ]/g, '').toLowerCase();
}

const TABBABLE = [
  'a[href]',
  'button:not([disabled])',
  'input:not([disabled]):not([type="hidden"])',
  'select:not([disabled])',
  'textarea:not([disabled])',
  '[tabindex]:not([tabindex="-1"])',
].join(',');

/** The tabbable element next to `from`, within its dialog if it sits in one. */
function neighbourOf(from: HTMLElement, delta: 1 | -1): HTMLElement {
  const scope = from.closest<HTMLElement>('[role="dialog"]') ?? document.body;
  const all = Array.from(scope.querySelectorAll<HTMLElement>(TABBABLE)).filter(
    (el) => el === from || (el.tabIndex >= 0 && !el.closest('[hidden],[inert]')),
  );
  const at = all.indexOf(from);
  return all[at + delta] ?? from;
}

/** Substring match on the folded label; an empty query keeps every option. */
function filterOptions<V extends string>(
  options: ReadonlyArray<ListboxOption<V>>,
  text: string,
): ReadonlyArray<ListboxOption<V>> {
  const q = fold(text.trim());
  return q ? options.filter((o) => fold(o.label).includes(q)) : options;
}

function isPrintable(event: KeyboardEvent): boolean {
  return event.key.length === 1 && !event.ctrlKey && !event.metaKey && !event.altKey;
}

export function Listbox<V extends string = string>({
  value,
  onChange,
  options,
  name,
  disabled = false,
  loading = false,
  placeholder,
  emptyLabel,
  searchThreshold = SEARCH_THRESHOLD,
  size: controlSize = 'md',
  id,
  className,
  title,
  'aria-label': ariaLabel,
  'aria-labelledby': ariaLabelledBy,
  'data-testid': testId,
  ref,
}: ListboxProps<V>) {
  const { t } = useI18n();
  const { id: triggerId, status, aria } = useControlWiring(id);
  const listId = useId();
  const optionIdBase = useId();

  const [open, setOpen] = useState(false);
  const [activeIndex, setActiveIndex] = useState(-1);
  const [query, setQuery] = useState('');

  const triggerRef = useRef<HTMLButtonElement | null>(null);
  const searchRef = useRef<HTMLInputElement | null>(null);
  const typeahead = useRef({ buffer: '', at: 0 });

  const searchable = options.length > searchThreshold;
  const isDisabled = disabled || Boolean(aria.disabled);

  const filterBy = (text: string) => filterOptions(options, text);

  const visible = useMemo(
    () => (searchable ? filterOptions(options, query) : options),
    [options, query, searchable],
  );

  const selected = options.find((o) => o.value === value);
  const selectedIndex = visible.findIndex((o) => o.value === value);

  const {
    refs: { setReference, setFloating },
    floatingStyles,
    middlewareData,
  } = useFloating({
    open,
    placement: 'bottom-start',
    whileElementsMounted: autoUpdate,
    middleware: [
      offset(4),
      flip({ padding: 8 }),
      shift({ padding: 8 }),
      size({
        padding: 8,
        apply({ availableHeight, rects, elements }) {
          // Written only on change: autoUpdate observes this element, and a
          // style write inside its own resize callback is a ResizeObserver loop.
          const { style } = elements.floating;
          const minWidth = `${Math.round(rects.reference.width)}px`;
          const maxHeight = `${Math.round(Math.max(140, Math.min(availableHeight, 320)))}px`;
          if (style.minWidth !== minWidth) style.minWidth = minWidth;
          if (style.maxHeight !== maxHeight) style.maxHeight = maxHeight;
        },
      }),
      hide(),
    ],
  });

  const setTriggerRef = (node: HTMLButtonElement | null) => {
    triggerRef.current = node;
    setReference(node);
    if (typeof ref === 'function') ref(node);
    else if (ref) ref.current = node;
  };

  /* ---------------------------------------------------------- navigation -- */

  const enabledIndices = (list: ReadonlyArray<ListboxOption<V>>) =>
    list.flatMap((o, i) => (o.disabled ? [] : [i]));

  const firstEnabled = (list: ReadonlyArray<ListboxOption<V>>) => enabledIndices(list)[0] ?? -1;
  const lastEnabled = (list: ReadonlyArray<ListboxOption<V>>) => enabledIndices(list).at(-1) ?? -1;

  function step(from: number, delta: 1 | -1): number {
    const enabled = enabledIndices(visible);
    if (enabled.length === 0) return -1;
    if (from < 0) return delta > 0 ? enabled[0] : enabled[enabled.length - 1];
    // No wrap: a list that loops hides where it ends, which matters at 40 items.
    const next =
      delta > 0 ? enabled.find((i) => i > from) : [...enabled].reverse().find((i) => i < from);
    return next ?? from;
  }

  function openAt(index: number, seed = '') {
    if (isDisabled) return;
    setQuery(seed);
    setOpen(true);
    setActiveIndex(seed ? firstEnabled(filterBy(seed)) : index);
  }

  function close(refocus: boolean) {
    setOpen(false);
    setQuery('');
    setActiveIndex(-1);
    if (refocus) triggerRef.current?.focus({ preventScroll: true });
  }

  function pick(index: number) {
    const option = visible[index];
    // While loading the list shows a notice, not options: nothing to pick.
    if (loading || !option || option.disabled) return;
    close(true);
    if (option.value !== value) onChange(option.value);
  }

  /** Type-ahead on the labels: repeating one letter cycles through its options. */
  function typeAhead(char: string, from: number): number {
    const now = Date.now();
    const state = typeahead.current;
    state.buffer = now - state.at > TYPEAHEAD_MS ? char : state.buffer + char;
    state.at = now;
    const buffer = fold(state.buffer);
    const repeated = buffer.split('').every((c) => c === buffer[0]);
    const needle = repeated ? buffer[0] : buffer;
    const start = repeated ? from + 1 : Math.max(from, 0);
    const order = [...visible.keys()].map((k) => (k + start) % Math.max(visible.length, 1));
    const hit = order.find(
      (i) => !visible[i].disabled && fold(visible[i].label).startsWith(needle),
    );
    return hit ?? from;
  }

  const initialIndex = () => (selectedIndex >= 0 ? selectedIndex : firstEnabled(visible));

  useEscapeLayer(open, () => close(true));

  /* An outside press closes without moving focus: the user is going somewhere. */
  const floatingNode = useRef<HTMLDivElement | null>(null);
  useEffect(() => {
    if (!open) return;
    const onPointerDown = (event: PointerEvent) => {
      const target = event.target as Node;
      if (triggerRef.current?.contains(target) || floatingNode.current?.contains(target)) return;
      close(false);
    };
    document.addEventListener('pointerdown', onPointerDown, true);
    return () => document.removeEventListener('pointerdown', onPointerDown, true);
  }, [open]);

  /* The search field takes focus as it mounts. A ref callback, not an effect:
     FloatingPortal creates its node a tick after the list opens, so an effect
     on `open` runs before the field exists. */
  const attachSearch = useCallback((node: HTMLInputElement | null) => {
    searchRef.current = node;
    node?.focus({ preventScroll: true });
  }, []);

  /* Keep the active option in view while the keyboard walks a long list. */
  useEffect(() => {
    if (!open || activeIndex < 0) return;
    document
      .getElementById(`${optionIdBase}-${activeIndex}`)
      ?.scrollIntoView?.({ block: 'nearest' });
  }, [open, activeIndex, optionIdBase]);

  /* ------------------------------------------------------------- keyboard -- */

  function onClosedKey(event: KeyboardEvent<HTMLButtonElement>) {
    const { key } = event;
    if (key === 'Enter' || key === ' ' || key === 'ArrowDown' || key === 'ArrowUp') {
      // Alt+Down is the platform's "open" chord; plain arrows open too, as the
      // native control does on Windows.
      event.preventDefault();
      openAt(initialIndex());
    } else if (key === 'Home') {
      event.preventDefault();
      openAt(firstEnabled(options));
    } else if (key === 'End') {
      event.preventDefault();
      openAt(lastEnabled(options));
    } else if (isPrintable(event)) {
      event.preventDefault();
      if (searchable) openAt(-1, event.key);
      else openAt(typeAhead(event.key, selectedIndex));
    }
  }

  /** Shared by the trigger (no search) and the search field (search). */
  function onOpenKey(event: KeyboardEvent<HTMLElement>, inSearch: boolean) {
    const { key, altKey } = event;
    if (key === 'ArrowDown') {
      event.preventDefault();
      setActiveIndex((i) => step(i, 1));
    } else if (key === 'ArrowUp') {
      event.preventDefault();
      if (altKey) pick(activeIndex);
      else setActiveIndex((i) => step(i, -1));
    } else if (key === 'Home') {
      event.preventDefault();
      setActiveIndex(firstEnabled(visible));
    } else if (key === 'End') {
      event.preventDefault();
      setActiveIndex(lastEnabled(visible));
    } else if (key === 'Enter') {
      event.preventDefault();
      pick(activeIndex);
    } else if (key === 'Tab') {
      // Leave without picking. From the trigger the browser's own Tab is right.
      // The search field lives in a portal at the end of the body, so there the
      // move is ours: to the control before or after the trigger in the page.
      if (inSearch) {
        event.preventDefault();
        const trigger = triggerRef.current;
        close(false);
        if (trigger) neighbourOf(trigger, event.shiftKey ? -1 : 1).focus({ preventScroll: true });
      } else close(false);
    } else if (!inSearch && key === ' ') {
      event.preventDefault();
      if (typeahead.current.buffer && Date.now() - typeahead.current.at < TYPEAHEAD_MS) {
        setActiveIndex((i) => typeAhead(' ', i));
      } else pick(activeIndex);
    } else if (!inSearch && isPrintable(event)) {
      event.preventDefault();
      setActiveIndex((i) => typeAhead(event.key, i));
    }
  }

  /* --------------------------------------------------------------- render -- */

  const activeId = open && activeIndex >= 0 ? `${optionIdBase}-${activeIndex}` : undefined;
  const isEmpty = !loading && options.length === 0;
  const noMatch = !loading && options.length > 0 && visible.length === 0;
  const hidden = Boolean(middlewareData.hide?.referenceHidden);

  return (
    <>
      <button
        ref={setTriggerRef}
        id={triggerId}
        type="button"
        role="combobox"
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-controls={open ? listId : undefined}
        aria-activedescendant={searchable ? undefined : activeId}
        aria-label={ariaLabel}
        aria-labelledby={ariaLabelledBy}
        aria-busy={loading || undefined}
        aria-describedby={aria['aria-describedby']}
        aria-invalid={aria['aria-invalid']}
        aria-required={aria['aria-required']}
        disabled={isDisabled}
        title={title}
        data-testid={testId}
        data-state={open ? 'open' : 'closed'}
        onClick={() => (open ? close(true) : openAt(initialIndex()))}
        onKeyDown={(e) => (open ? onOpenKey(e, false) : onClosedKey(e))}
        className={cn(
          controlClasses(
            status,
            controlSize === 'sm' ? 'h-(--control-h-sm) text-xs' : 'h-(--control-h-md) text-sm',
          ),
          // A button matches :read-only; it must not look like a read-only input.
          'read-only:bg-surface-1 read-only:text-fg-primary',
          'inline-flex items-center gap-2 text-left',
          open && 'border-accent',
          className,
        )}
      >
        <span className={cn('min-w-0 flex-1 truncate', !selected && 'text-fg-muted')}>
          {selected?.label ?? placeholder ?? t('common.listbox.placeholder', 'Choisir…')}
        </span>
        {loading ? (
          <Loader2
            size={14}
            aria-hidden="true"
            className="shrink-0 text-fg-muted motion-safe:animate-spin"
          />
        ) : (
          <ChevronDown
            size={14}
            aria-hidden="true"
            className={cn(
              'shrink-0 text-fg-muted transition-transform duration-fast',
              open && 'rotate-180',
            )}
          />
        )}
      </button>

      {name !== undefined && <input type="hidden" name={name} value={value ?? ''} />}

      {open && (
        <FloatingPortal>
          <div
            ref={(node) => {
              floatingNode.current = node;
              setFloating(node);
            }}
            style={floatingStyles}
            data-hidden={hidden || undefined}
            data-testid={testId ? `${testId}-popup` : undefined}
            className={cn(
              'z-tooltip flex max-w-[min(28rem,calc(100vw-16px))] flex-col overflow-hidden',
              'rounded-md border border-default bg-surface-2 shadow-overlay',
              'motion-safe:animate-or-fadein',
              // Its trigger scrolled out of a scrolling panel: don't float over the rest.
              'data-hidden:invisible',
            )}
          >
            {searchable && (
              /* The focus mark is the row's bottom edge, in the focus-ring colour:
                 the global outline would be clipped by the popup's rounded,
                 overflow-hidden frame. */
              <div className="flex items-center gap-2 border-b border-subtle px-3 focus-within:shadow-[inset_0_-2px_0_var(--focus-ring-color)]">
                <Search size={14} aria-hidden="true" className="shrink-0 text-fg-muted" />
                <input
                  ref={attachSearch}
                  type="text"
                  role="combobox"
                  aria-expanded
                  aria-controls={listId}
                  aria-autocomplete="list"
                  aria-activedescendant={activeId}
                  aria-label={t('common.listbox.search', 'Rechercher dans la liste')}
                  autoComplete="off"
                  spellCheck={false}
                  value={query}
                  placeholder={t('common.listbox.searchPlaceholder', 'Rechercher…')}
                  onChange={(e) => {
                    setQuery(e.target.value);
                    setActiveIndex(firstEnabled(filterBy(e.target.value)));
                  }}
                  onKeyDown={(e) => onOpenKey(e, true)}
                  style={{ outline: 'none' }}
                  className="h-(--control-h-md) min-w-0 flex-1 bg-transparent text-sm text-fg-primary placeholder:text-fg-muted"
                />
              </div>
            )}

            {loading || isEmpty || noMatch ? (
              <p id={listId} role="status" className="px-3 py-2.5 text-sm text-fg-muted">
                {loading
                  ? t('common.listbox.loading', 'Chargement…')
                  : isEmpty
                    ? (emptyLabel ?? t('common.listbox.empty', 'Aucune option'))
                    : t('common.listbox.noMatch', 'Aucun résultat')}
              </p>
            ) : (
              <ul
                id={listId}
                role="listbox"
                aria-labelledby={ariaLabelledBy ?? (ariaLabel ? undefined : triggerId)}
                aria-label={ariaLabelledBy ? undefined : ariaLabel}
                className="min-h-0 overflow-y-auto overscroll-contain py-1"
              >
                {visible.map((option, index) => {
                  const isSelected = option.value === value;
                  const isActive = index === activeIndex;
                  return (
                    <li
                      key={option.value}
                      id={`${optionIdBase}-${index}`}
                      role="option"
                      aria-selected={isSelected}
                      aria-disabled={option.disabled || undefined}
                      data-active={isActive || undefined}
                      onPointerMove={() => !option.disabled && setActiveIndex(index)}
                      onMouseDown={(e) => e.preventDefault() /* keep focus where it is */}
                      onClick={() => pick(index)}
                      className={cn(
                        'flex cursor-default items-start gap-2 px-3 py-1.5 text-sm text-fg-primary',
                        controlSize === 'sm' && 'text-xs',
                        isActive && 'bg-surface-3',
                        option.disabled && 'opacity-55',
                      )}
                    >
                      <Check
                        size={14}
                        aria-hidden="true"
                        className={cn('mt-0.5 shrink-0 text-accent', !isSelected && 'invisible')}
                      />
                      <span className="min-w-0">
                        <span className="block truncate">{option.label}</span>
                        {option.description && (
                          <span className="block text-2xs text-fg-muted">{option.description}</span>
                        )}
                      </span>
                    </li>
                  );
                })}
              </ul>
            )}
          </div>
        </FloatingPortal>
      )}
    </>
  );
}
