// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: Apache-2.0

/**
 * AlertDialog — a decision the user must answer before anything continues.
 *
 * NOT the same as Modal, and the difference is behavioural rather than visual.
 * A Modal is a surface: it can be dismissed, it may carry a form, and closing it
 * is a legitimate outcome. An AlertDialog is a QUESTION: it has exactly two
 * answers, dismissing it means "no", and it is built on `role="alertdialog"`, an
 * ARIA role that tells a screen reader to announce the whole thing at once
 * rather than waiting to be explored. That is the right behaviour when the
 * message is "this deletes 14 controls" and the wrong behaviour for a form.
 *
 * WHAT THIS IS NOT   `shared/DangerConfirm.tsx` is the product's opinionated
 * destructive confirmation: it shows an impact radiography and offers safer
 * alternatives inline. This is the primitive underneath that idea — the generic
 * two-answer dialog, with no opinion about what the action affects. Migrating
 * DangerConfirm onto it is deliberately out of scope for #443, which forbids
 * touching the existing components' public APIs.
 *
 * FOCUS GOES TO THE SAFE ANSWER. The cancel button takes initial focus, always,
 * including when the confirm button is destructive — especially then. A dialog
 * that focuses "Delete" turns a reflexive Enter, or a screen reader user's
 * default action, into data loss. Escape and the scrim both resolve to cancel,
 * so every way out that is not a deliberate click on the confirm button means
 * "no".
 *
 * IN-FLIGHT   While `busy` is true the dialog cannot be dismissed and neither
 * button can be pressed again. A confirmation that can be double-submitted is
 * the same defect as a button that can, and it is worse here because the action
 * is by definition the one that matters.
 */

import { useEffect, useRef, type ReactNode } from 'react';
import { Button, type ButtonVariant } from './Button';
import { Modal } from './Modal';

export interface AlertDialogProps {
  open: boolean;
  /** Called for every "no": the cancel button, Escape, and the scrim. */
  onCancel: () => void;
  onConfirm: () => void;
  title: ReactNode;
  /** The consequence, in the user's terms. This is the part they actually read. */
  description?: ReactNode;
  confirmLabel: string;
  cancelLabel?: string;
  /** `destructive` for anything that removes or revokes. */
  tone?: 'default' | 'destructive';
  /** Action in flight: both buttons lock and the dialog refuses to dismiss. */
  busy?: boolean;
  /** Extra detail between the description and the buttons — a list of what is
   *  affected, a warning, a checkbox. Keep it short; this is not a form. */
  children?: ReactNode;
  className?: string;
}

export function AlertDialog({
  open,
  onCancel,
  onConfirm,
  title,
  description,
  confirmLabel,
  cancelLabel = 'Cancel',
  tone = 'default',
  busy = false,
  children,
  className,
}: AlertDialogProps) {
  const cancelRef = useRef<HTMLButtonElement>(null);

  /* Focus the SAFE answer once the dialog is up. Modal moves focus to the panel;
     this moves it one step further, onto the button that does nothing. */
  useEffect(() => {
    if (!open) return;
    const id = requestAnimationFrame(() => cancelRef.current?.focus());
    return () => cancelAnimationFrame(id);
  }, [open]);

  const confirmVariant: ButtonVariant = tone === 'destructive' ? 'destructive' : 'primary';

  return (
    <Modal
      open={open}
      onClose={onCancel}
      title={title}
      /* Not dismissable while the action is running: a half-applied delete must
         not be escapable into an unknown state. */
      dismissable={!busy}
      role="alertdialog"
      /* The description goes through Modal's `subtitle`, not into the body.
         Modal wires aria-describedby to the subtitle's id, so this is what makes
         the consequence part of the dialog's accessible description rather than
         loose text a screen reader only meets if it goes looking. Caught by the
         test: rendered as a child it read out as an unnamed paragraph. */
      subtitle={description}
      size="sm"
      className={className}
      footer={
        <>
          <Button ref={cancelRef} variant="secondary" onClick={onCancel} disabled={busy}>
            {cancelLabel}
          </Button>
          <Button variant={confirmVariant} onClick={onConfirm} loading={busy}>
            {confirmLabel}
          </Button>
        </>
      }
    >
      {children}
    </Modal>
  );
}
