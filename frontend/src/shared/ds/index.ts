// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: Apache-2.0

/**
 * The OpenRisk design system.
 *
 * A new screen should be buildable from this module alone — no new colour, no
 * new spacing step, no new button, no new animation. If something here does not
 * cover a genuine product need, the fix is to extend the primitive (and its
 * documentation, and its tests), not to write a local variant.
 *
 * See docs/W1-01_OPENRISK_DESIGN_SYSTEM.md.
 */

export { cn } from './cn';

export { Button, type ButtonVariant, type ButtonSize } from './Button';
export {
  Field,
  Input,
  Textarea,
  Select,
  type FieldProps,
  type FieldStatus,
  type InputProps,
  type TextareaProps,
  type SelectProps,
} from './Field';
export { Label, type LabelProps } from './Label';
export { Fieldset, type FieldsetProps } from './Fieldset';
export { InputGroup, type InputGroupProps, type InputGroupSize } from './InputGroup';
export { Checkbox, CheckboxGroup, type CheckboxProps, type CheckboxGroupProps } from './Checkbox';
export { RadioGroup, type RadioGroupProps, type RadioOption } from './RadioGroup';
export { Switch, type SwitchProps, type SwitchSize } from './Switch';
export { Badge, type BadgeIntent, type BadgeSize, type BadgeProps } from './Badge';
export {
  riskStatusIntent,
  severityIntent,
  type RiskStatusValue,
  type SeverityValue,
} from './badgeIntents';
export { Modal, type ModalProps, type ModalSize } from './Modal';
export { AlertDialog, type AlertDialogProps } from './AlertDialog';
export { Spinner, type SpinnerProps, type SpinnerSize } from './Spinner';
export { Drawer, type DrawerProps, type DrawerSide, type DrawerSize } from './Drawer';
export { Tabs, TabPanel, type TabItem, type TabsProps } from './Tabs';
export { Tooltip, type TooltipProps, type TooltipPlacement } from './Tooltip';
export { Popover, type PopoverProps, type PopoverPlacement } from './Popover';
export { Menu, type MenuProps, type MenuItem } from './Menu';
export { Command, type CommandProps, type CommandItem } from './Command';
export { OtpField, type OtpFieldProps, type OtpAlphabet } from './OtpField';
export {
  ErrorState,
  PermissionDenied,
  LoadingState,
  Skeleton,
  SkeletonRows,
  AuditEntry,
  type ErrorStateProps,
  type PermissionDeniedProps,
  type LoadingStateProps,
  type AuditEntryProps,
} from './States';
export { useDismissableLayer } from './useDismissableLayer';

/** Visualisation contract — palette, axes, tooltip, graph. */
export * as chart from './chart';
