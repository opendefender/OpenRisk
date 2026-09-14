// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: Apache-2.0

/**
 * TagInput — a set of short free-text labels, entered one at a time.
 *
 * WHAT IT REPLACES. Two risk forms handled the same field, both badly.
 * `EditRiskModal` bound tags to a comma-separated `<input>` and split on submit,
 * so the user had to know that the comma was the delimiter, could not tell
 * whether "a, b" meant one tag or two until after saving, and could never enter
 * a tag containing a comma. `CreateRiskModal` declared `tags` in its schema,
 * watched the value, and rendered no control at all — every risk created from
 * that form was submitted with an empty array.
 *
 * A COMMITTED TAG IS A THING, NOT TEXT. The point of this control over a text
 * box is that the user sees what the system parsed *before* saving: each tag
 * becomes a chip the moment it is committed, so "two tags or one?" is never a
 * question. Committing is Enter or comma — the comma stays because it is what
 * people type out of habit, but it is now one of two ways in rather than a
 * hidden protocol.
 *
 * A11Y     The hard part of this pattern is that removing a chip destroys the
 *          element that had focus, and most implementations drop focus to
 *          `<body>` — which strands a keyboard user at the top of the page.
 *          Focus moves deliberately here: removing a chip focuses the next one,
 *          or the text input when the removed chip was last.
 *
 *          - the chip list is a `role="list"`, so a screen reader announces how
 *            many tags there are instead of reading a run of loose buttons
 *          - every remove button carries the tag in its accessible name
 *            ("Remove Conformité"), never a bare "×", so the announcement says
 *            what it would remove (WCAG 2.4.6)
 *          - additions, removals and rejections are announced through one
 *            polite live region; a sighted user sees the chip appear, and this
 *            is the equivalent for someone who cannot
 *          - the input keeps a persistent `aria-describedby` hint stating how to
 *            commit, because the interaction is not discoverable from the box
 *          - Backspace on an empty input removes the last tag, which is the
 *            convention every comparable control uses; it is announced like any
 *            other removal rather than happening silently
 *
 * WHY NOT A COMBOBOX. A combobox implies a known set to choose from. These are
 * free text, with no catalogue behind them: `suggestions` only offers a datalist
 * of what other records already use, and typing something absent from it is a
 * normal, supported outcome rather than an error. Announcing this as a combobox
 * would promise a list the user cannot exhaust.
 */

import {
  forwardRef,
  useCallback,
  useId,
  useLayoutEffect,
  useRef,
  useState,
  type KeyboardEvent,
  type ReactNode,
} from 'react';
import { X } from 'lucide-react';
import { cn } from './cn';
import { useControlWiring } from './fieldContext';

/** How a rejected entry is reported back to the caller. */
export type TagRejection = 'duplicate' | 'max' | 'invalid';

export interface TagInputLabels {
  /** Placeholder in the text box. */
  placeholder?: string;
  /** Builds the remove button's accessible name. Receives the tag. */
  removeLabel: (tag: string) => string;
  /** Persistent hint under the control: how to commit a tag. */
  hint?: string;
  /** Announced when a tag is added. Receives the tag. */
  addedAnnouncement?: (tag: string) => string;
  /** Announced when a tag is removed. Receives the tag. */
  removedAnnouncement?: (tag: string) => string;
  /** Announced when an entry is refused. Receives the reason and the tag. */
  rejectedAnnouncement?: (reason: TagRejection, tag: string) => string;
}

export interface TagInputProps {
  value: string[];
  onValueChange: (next: string[]) => void;
  /**
   * Accessible name. Required when used OUTSIDE a `Field` — a bare tag box with
   * no label announces as an unnamed text field.
   */
  'aria-label'?: string;
  /** Copy. Every string the control can render comes from here: it holds no
   *  English or French of its own, so a caller cannot half-translate it. */
  labels: TagInputLabels;
  /** Refuses further entries once reached. The input stays readable, not hidden. */
  maxTags?: number;
  /**
   * Rejects an entry before it is committed. Return false to refuse; the
   * rejection is announced as `invalid`. Trimming and the duplicate check run
   * first, so this only sees entries that would otherwise be accepted.
   */
  validate?: (tag: string) => boolean;
  /** Offered as a `<datalist>`. Typing something absent is still accepted. */
  suggestions?: string[];
  disabled?: boolean;
  id?: string;
  className?: string;
  /** Rendered after the chips, before the input — a leading icon, a count. */
  leadingSlot?: ReactNode;
}

/** Trim and collapse internal whitespace, so "  a  b " and "a b" are one tag. */
function normalise(raw: string): string {
  return raw.trim().replace(/\s+/g, ' ');
}

/* Mirrors Field's own STATUS_BORDER. The wrapper here plays the part the
   control's border plays elsewhere, so it has to carry the same four states —
   a field that shows its warning colour as an Input but not as a TagInput is
   the kind of drift the design system exists to prevent. */
const STATUS_BORDER = {
  default: 'border-control',
  invalid: 'border-danger',
  warning: 'border-warning',
  success: 'border-success',
} as const;

export const TagInput = forwardRef<HTMLInputElement, TagInputProps>(function TagInput(
  {
    value,
    onValueChange,
    labels,
    maxTags,
    validate,
    suggestions,
    disabled = false,
    id,
    className,
    leadingSlot,
    'aria-label': ariaLabel,
  },
  ref,
) {
  const { id: fieldId, status, aria } = useControlWiring(id);
  const [draft, setDraft] = useState('');
  const [announcement, setAnnouncement] = useState('');
  const [pendingFocus, setPendingFocus] = useState<number | 'input' | null>(null);
  const hintId = `${fieldId}-taginput-hint`;
  const listId = `${fieldId}-taginput-suggestions`;
  const generatedChipId = useId();

  const inputRef = useRef<HTMLInputElement | null>(null);
  const chipRefs = useRef<(HTMLButtonElement | null)[]>([]);
  chipRefs.current.length = value.length;

  const announce = useCallback((message?: string) => {
    // Re-announce an identical message: a live region ignores an unchanged
    // string, and removing two tags of the same name must speak twice.
    if (!message) return;
    setAnnouncement((prev) => (prev === message ? `${message} ` : message));
  }, []);

  const commit = useCallback(
    (raw: string): boolean => {
      const tag = normalise(raw);
      if (!tag) return false;

      if (value.includes(tag)) {
        announce(labels.rejectedAnnouncement?.('duplicate', tag));
        return false;
      }
      if (maxTags !== undefined && value.length >= maxTags) {
        announce(labels.rejectedAnnouncement?.('max', tag));
        return false;
      }
      if (validate && !validate(tag)) {
        announce(labels.rejectedAnnouncement?.('invalid', tag));
        return false;
      }

      onValueChange([...value, tag]);
      announce(labels.addedAnnouncement?.(tag));
      return true;
    },
    [value, onValueChange, labels, maxTags, validate, announce],
  );

  const removeAt = useCallback(
    (index: number, restoreFocus: 'chip' | 'input') => {
      const tag = value[index];
      if (tag === undefined) return;
      onValueChange(value.filter((_, i) => i !== index));
      announce(labels.removedAnnouncement?.(tag));

      // Removing a chip destroys the focused element, so where focus lands has
      // to be decided here rather than left to the browser — which drops it on
      // <body> and strands a keyboard user at the top of the page. The move is
      // applied in a layout effect below, once the removal has been committed
      // and the surviving chips have taken their new indices.
      setPendingFocus(restoreFocus === 'input' || value.length === 1 ? 'input' : index);
    },
    [value, onValueChange, labels, announce],
  );

  // Runs after the DOM reflects the removal: index `i` is now whichever chip
  // took the removed one's place, or the last one when it was the tail.
  useLayoutEffect(() => {
    if (pendingFocus === null) return;
    setPendingFocus(null);
    if (pendingFocus === 'input') {
      inputRef.current?.focus();
      return;
    }
    const next = chipRefs.current[pendingFocus] ?? chipRefs.current[pendingFocus - 1];
    if (next?.isConnected) next.focus();
    else inputRef.current?.focus();
  }, [pendingFocus]);

  const onInputKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key === 'Enter' || event.key === ',') {
      // Enter must not submit the surrounding form while the box holds a draft:
      // the user is committing a tag, not the form.
      event.preventDefault();
      if (commit(draft)) setDraft('');
      return;
    }
    if (event.key === 'Backspace' && draft === '' && value.length > 0) {
      event.preventDefault();
      removeAt(value.length - 1, 'input');
    }
  };

  const atMax = maxTags !== undefined && value.length >= maxTags;

  return (
    <div className={className}>
      <div
        className={cn(
          'w-full bg-surface-1 border rounded-md',
          'flex flex-wrap items-center gap-1.5 px-2 py-1.5 min-h-(--control-h-md)',
          'transition-[border-color,background-color] duration-fast ease-out',
          'focus-within:border-accent hover:border-strong',
          STATUS_BORDER[status],
          disabled && 'opacity-55 pointer-events-none',
        )}
        // Clicking the padding around the chips should reach the text box — the
        // whole rectangle reads as one field, so it must behave as one.
        onMouseDown={(event) => {
          if (event.target === event.currentTarget) {
            event.preventDefault();
            inputRef.current?.focus();
          }
        }}
      >
        {leadingSlot}

        {value.length > 0 && (
          <ul role="list" className="contents">
            {value.map((tag, index) => (
              <li key={`${tag}-${generatedChipId}`} className="contents">
                <span className="inline-flex items-center gap-1 h-6 pl-2 pr-1 rounded-full bg-surface-sunken border border-subtle text-xs text-fg-primary max-w-full">
                  <span className="truncate">{tag}</span>
                  <button
                    type="button"
                    ref={(node) => {
                      chipRefs.current[index] = node;
                    }}
                    onClick={() => removeAt(index, 'chip')}
                    disabled={disabled}
                    aria-label={labels.removeLabel(tag)}
                    className="shrink-0 w-4 h-4 rounded-full inline-flex items-center justify-center text-fg-muted hover:text-fg-primary hover:bg-surface-1"
                  >
                    <X size={11} aria-hidden="true" />
                  </button>
                </span>
              </li>
            ))}
          </ul>
        )}

        <input
          ref={(node) => {
            inputRef.current = node;
            if (typeof ref === 'function') ref(node);
            else if (ref) ref.current = node;
          }}
          id={fieldId}
          type="text"
          value={draft}
          disabled={disabled}
          list={suggestions?.length ? listId : undefined}
          placeholder={atMax ? undefined : labels.placeholder}
          onChange={(event) => setDraft(event.target.value)}
          onKeyDown={onInputKeyDown}
          // Committing on blur loses a half-typed tag the user was still
          // editing; leaving the draft in place keeps it recoverable.
          className="flex-1 min-w-[8ch] bg-transparent text-sm text-fg-primary placeholder:text-fg-muted outline-none"
          aria-label={ariaLabel}
          {...aria}
          aria-describedby={[aria['aria-describedby'], labels.hint ? hintId : undefined]
            .filter(Boolean)
            .join(' ')}
        />

        {suggestions?.length ? (
          <datalist id={listId}>
            {suggestions.map((s) => (
              <option key={s} value={s} />
            ))}
          </datalist>
        ) : null}
      </div>

      {labels.hint && (
        <p id={hintId} className="mt-1 text-xs text-fg-muted">
          {labels.hint}
        </p>
      )}

      {/* One region for every outcome. Polite: adding a tag must not interrupt
          what the user is typing. */}
      <span aria-live="polite" className="sr-only">
        {announcement}
      </span>
    </div>
  );
});
