// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import { useMemo } from 'react';
import { useI18n } from '../../hooks/useI18n';
import type { TagInputLabels } from '../../shared/ds';

/**
 * The copy `TagInput` needs, for the risk tag field.
 *
 * Both risk forms render the same field, so the strings live in one place: the
 * create and edit modals drifting apart is exactly how the edit form ended up
 * telling users the delimiter was a comma while the create form rendered no
 * control at all.
 */
export function useRiskTagLabels(): TagInputLabels {
  const { t } = useI18n();

  return useMemo(
    () => ({
      placeholder: t('risks.tagInput.placeholder'),
      hint: t('risks.tagInput.hint'),
      removeLabel: (tag: string) => t('risks.tagInput.remove', { tag }),
      addedAnnouncement: (tag: string) => t('risks.tagInput.added', { tag }),
      removedAnnouncement: (tag: string) => t('risks.tagInput.removed', { tag }),
      rejectedAnnouncement: (reason, tag) =>
        // `invalid` cannot arise here: the risk field passes no `validate`.
        reason === 'duplicate'
          ? t('risks.tagInput.duplicate', { tag })
          : t('risks.tagInput.max', { tag }),
    }),
    [t],
  );
}
