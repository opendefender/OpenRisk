// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The role line under the signed-in person's name in the sidebar (#695).
//
// It used to read `user.role`, which the auth store fills from /auth/me's legacy
// `user.role.name` — empty for every account created by sign-up — and fell back
// to the literal "Membre". So an organisation's founder, its owner, was told they
// were an ordinary member of it. The organisation role was on the client all
// along: the verified access token carries `org_roles`, keyed by organisation,
// and the store copies it with `tenant_id` naming the active one.
//
// Order, and why:
//   1. a business role (RSSI, auditor, …) is the more specific answer, and its
//      label comes from the server catalogue in both languages rather than a
//      table kept here;
//   2. otherwise the organisation role for the ACTIVE organisation;
//   3. otherwise nothing. Defaulting to "member" is the defect being fixed.

import { useAuthStore } from '../../hooks/useAuthStore';
import { useI18n } from '../../hooks/useI18n';
import { useRbacCatalog } from '../../features/rbac/useRbac';

const ORG_ROLE_KEYS: Record<string, string> = {
  root: 'common.orgRole.root',
  admin: 'common.orgRole.admin',
  user: 'common.orgRole.user',
};

export function SidebarRoleLabel() {
  const { t, locale } = useI18n();
  const user = useAuthStore((s) => s.user);
  const businessRole = user?.business_role ?? '';
  const { data: catalog, isLoading } = useRbacCatalog();

  const orgRole = user?.tenant_id ? user.org_roles?.[user.tenant_id] : undefined;
  const orgRoleKey = orgRole ? ORG_ROLE_KEYS[orgRole] : undefined;

  let label: string | undefined;
  if (businessRole) {
    const preset = catalog?.business_roles.find((r) => r.key === businessRole);
    // Picks between the two translations the server sent; no copy lives here.
    if (preset) label = locale === 'fr' ? preset.label_fr : preset.label_en;
    else if (isLoading) {
      return (
        <div
          className="h-[9px] w-16 mt-[3px] rounded or-skeleton"
          aria-hidden="true"
          data-testid="sidebar-role-loading"
        />
      );
    }
  }
  if (!label && orgRoleKey) label = t(orgRoleKey);
  if (!label) return null;

  return (
    <div className="text-[10.5px] text-ink-soft truncate" data-testid="sidebar-role">
      {label}
    </div>
  );
}
