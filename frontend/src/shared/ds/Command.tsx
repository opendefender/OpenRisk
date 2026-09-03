// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: Apache-2.0

/**
 * Command — a filtering list of actions, driven from a text input.
 *
 * The ⌘K surface, reduced to the part that is not about OpenRisk. This is
 * deliberately NOT a dialog: it is the input and the list, so the same primitive
 * serves the palette (inside a `Modal`), an inline entity picker, and an
 * "assign to…" list in a drawer. Wrapping it in its own overlay would have made
 * it usable in exactly one of those three.
 *
 * COMBOBOX, NOT MENU — the distinction that decides the keyboard contract.
 * `Menu` moves FOCUS between its items with a roving tab stop, because focus is
 * not needed anywhere else. Here focus must stay in the input, or the user
 * cannot keep typing to narrow the list. So the list is a `listbox`, the input
 * owns focus for the whole interaction, and the active option is pointed at with
 * `aria-activedescendant`. Using Menu's pattern here is the classic bug: the
 * first arrow key moves focus out of the input and the next keystroke goes
 * nowhere.
 *
 * MATCHING IGNORES ACCENTS, which is not a nicety in this product. The console
 * ships in French, and the actions a user searches for are "Règlement",
 * "Contrôles", "Créer un risque". A user typing `regl` on a keyboard where the
 * accent is a dead key gets nothing from a naive `includes`, concludes the item
 * does not exist, and navigates by hand instead. Normalising to NFD and dropping
 * the combining marks makes `regl` find `Règlement` and costs one string pass.
 *
 * FILTERING IS SUBSTRING, NOT FUZZY. A fuzzy match ranks `Delete tenant` highly
 * for the query `dt`, and a command list is the one place a surprising top hit
 * is dangerous — the user presses Enter on what they assume they asked for.
 */

import {
  useEffect,
  useId,
  useMemo,
  useRef,
  useState,
  type KeyboardEvent,
  type ReactNode,
} from 'react';
import { type LucideIcon } from 'lucide-react';
import { cn } from './cn';

export interface CommandItem {
  /** Stable across filtering; used for the option id and as the React key. */
  id: string;
  label: string;
  onSelect: () => void;
  icon?: LucideIcon;
  /** Heading this item is listed under. Items with no group are listed first. */
  group?: string;
  /**
   * Extra terms that should match. The point is synonyms and the OTHER language:
   * a French user searching "risque" should find "New risk" in an English
   * session, and vice versa, without the label itself being duplicated.
   */
  keywords?: readonly string[];
  /** Rendered right-aligned, e.g. "⌘N". Display only; binding it is the caller's. */
  shortcut?: string;
  disabled?: boolean;
}

export interface CommandProps {
  items: ReadonlyArray<CommandItem>;
  placeholder?: string;
  /** Shown when the query matches nothing. */
  emptyMessage?: ReactNode;
  /** Accessible name for the list. */
  label: string;
  /** Caps the rendered rows. The list is a shortcut, not a browsable index. */
  maxVisible?: number;
  autoFocus?: boolean;
  className?: string;
}

/** NFD-normalise and strip combining marks, so `règlement` matches `reglement`. */
function fold(s: string): string {
  return s
    .normalize('NFD')
    .replace(/[\u0300-\u036f]/g, '')
    .toLowerCase();
}

function matches(item: CommandItem, folded: string): boolean {
  if (!folded) return true;
  if (fold(item.label).includes(folded)) return true;
  return (item.keywords ?? []).some((k) => fold(k).includes(folded));
}

export function Command({
  items,
  placeholder = 'Type a command or search…',
  emptyMessage = 'No results',
  label,
  maxVisible = 50,
  autoFocus,
  className,
}: CommandProps) {
  const [query, setQuery] = useState('');
  const [requestedIndex, setActiveIndex] = useState(0);
  const listId = useId();
  const inputId = useId();
  const listRef = useRef<HTMLDivElement>(null);

  const visible = useMemo(() => {
    const folded = fold(query.trim());
    return items.filter((i) => matches(i, folded)).slice(0, maxVisible);
  }, [items, query, maxVisible]);

  /* Grouped for rendering, but the flat `visible` order stays the source of
     truth for the keyboard: the active index must mean the same thing to the
     arrow keys and to the rendered rows, and computing it twice is how the two
     drift apart. Insertion order of a Map preserves first-seen group order. */
  const groups = useMemo(() => {
    const m = new Map<string, CommandItem[]>();
    for (const item of visible) {
      const key = item.group ?? '';
      const bucket = m.get(key);
      if (bucket) bucket.push(item);
      else m.set(key, [item]);
    }
    return [...m.entries()];
  }, [visible]);

  /* Clamped on READ rather than corrected in an effect. Re-narrowing the list —
     or a caller swapping `items` — can leave the requested index past the end,
     and an effect that fixed it up would render once with an out-of-range
     highlight and again without it. Typing resets the request to 0 at the event
     that causes it (the input's onChange), which is where that belongs. */
  const activeIndex = visible.length === 0 ? 0 : Math.min(requestedIndex, visible.length - 1);

  /* Keeps the active row in view when arrowing past the fold. `block: 'nearest'`
     rather than centring, so the list does not lurch on every keystroke. */
  useEffect(() => {
    const node = listRef.current?.querySelector<HTMLElement>('[data-active="true"]');
    node?.scrollIntoView({ block: 'nearest' });
  }, [activeIndex]);

  function step(delta: number) {
    if (visible.length === 0) return;
    /* Wraps, because a list this short is faster to reach backwards from the
       top than to arrow to the bottom of. */
    setActiveIndex((activeIndex + delta + visible.length) % visible.length);
  }

  function activate(item: CommandItem | undefined) {
    if (!item || item.disabled) return;
    item.onSelect();
  }

  function handleKeyDown(e: KeyboardEvent<HTMLInputElement>) {
    switch (e.key) {
      case 'ArrowDown':
        e.preventDefault();
        step(1);
        break;
      case 'ArrowUp':
        e.preventDefault();
        step(-1);
        break;
      case 'Home':
        e.preventDefault();
        setActiveIndex(0);
        break;
      case 'End':
        e.preventDefault();
        setActiveIndex(Math.max(0, visible.length - 1));
        break;
      case 'Enter':
        e.preventDefault();
        activate(visible[activeIndex]);
        break;
      default:
        break;
    }
  }

  const activeId = visible[activeIndex] ? `${listId}-${visible[activeIndex].id}` : undefined;

  return (
    <div className={cn('flex flex-col overflow-hidden', className)}>
      <input
        id={inputId}
        type="text"
        role="combobox"
        value={query}
        onChange={(e) => {
          setQuery(e.target.value);
          setActiveIndex(0);
        }}
        onKeyDown={handleKeyDown}
        autoFocus={autoFocus}
        placeholder={placeholder}
        aria-label={label}
        aria-expanded
        aria-controls={listId}
        aria-activedescendant={activeId}
        aria-autocomplete="list"
        autoComplete="off"
        spellCheck={false}
        className={cn(
          'w-full shrink-0 border-b border-default bg-transparent',
          'h-(--control-h-lg) px-(--control-px-md)',
          'text-sm text-fg-primary placeholder:text-fg-muted',
          'outline-none',
        )}
      />

      <div
        ref={listRef}
        id={listId}
        role="listbox"
        aria-label={label}
        className="max-h-80 overflow-y-auto overscroll-contain py-1"
      >
        {visible.length === 0 && (
          <p className="px-3 py-6 text-center text-sm text-fg-muted">{emptyMessage}</p>
        )}

        {groups.map(([groupLabel, groupItems]) => (
          <div key={groupLabel || '__ungrouped'} role="group" aria-label={groupLabel || undefined}>
            {groupLabel && (
              <p className="px-3 pb-1 pt-2 text-2xs font-semibold uppercase tracking-caps text-fg-muted">
                {groupLabel}
              </p>
            )}
            {groupItems.map((item) => {
              const index = visible.indexOf(item);
              const isActive = index === activeIndex;
              const Icon = item.icon;
              return (
                <div
                  key={item.id}
                  id={`${listId}-${item.id}`}
                  role="option"
                  aria-selected={isActive}
                  aria-disabled={item.disabled || undefined}
                  data-active={isActive}
                  data-testid="command-item"
                  /* Pointer only. The row is never a tab stop and never takes
                     focus — focus belongs to the input for the whole
                     interaction, which is what `aria-activedescendant` is for.
                     `onMouseDown` with preventDefault so clicking a row does not
                     blur the input first and close a palette out from under the
                     click. */
                  onMouseDown={(e) => e.preventDefault()}
                  onClick={() => activate(item)}
                  onMouseEnter={() => !item.disabled && setActiveIndex(index)}
                  className={cn(
                    'mx-1 flex cursor-pointer items-center gap-2 rounded-sm px-2',
                    'min-h-(--control-h-sm) text-sm text-fg-primary',
                    isActive && 'bg-surface-3',
                    item.disabled && 'cursor-not-allowed opacity-55',
                  )}
                >
                  {Icon && <Icon size={14} aria-hidden="true" className="shrink-0" />}
                  <span className="flex-1 truncate">{item.label}</span>
                  {item.shortcut && (
                    <kbd className="shrink-0 font-mono text-2xs text-fg-muted">{item.shortcut}</kbd>
                  )}
                </div>
              );
            })}
          </div>
        ))}
      </div>
    </div>
  );
}
