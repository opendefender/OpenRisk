// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Reading a failure. Separate from <WidgetState> because it is a function, not a
// component — exporting both from one module breaks fast refresh, and this
// classification is also wanted by callers that render their own shell.

/** HTTP status from an axios-shaped error, if there is one. */
function statusOf(error: unknown): number | undefined {
  if (typeof error !== 'object' || error === null) return undefined;
  const response = (error as { response?: { status?: number } }).response;
  return typeof response?.status === 'number' ? response.status : undefined;
}

/**
 * True when the failure means "you may not read this", not "this broke".
 *
 * The distinction is not cosmetic: one is resolved by asking an administrator
 * for a permission, the other by pressing retry. Collapsing them sends the user
 * to the wrong person.
 *
 * A timeout or a dropped connection has no response at all, and is therefore a
 * failure to retry rather than a refusal — which is why this reads the status
 * rather than assuming any error without one is a denial.
 */
export function isPermissionError(error: unknown): boolean {
  const status = statusOf(error);
  return status === 401 || status === 403;
}
