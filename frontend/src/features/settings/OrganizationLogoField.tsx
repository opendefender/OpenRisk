// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Settings › General — upload, replace or remove the organization logo (#718).
// Rendered only for callers the server allows to edit (`can_edit`). The browser
// pre-checks type and size to save a round trip; the server decides by bytes.

import { useRef } from 'react';
import { toast } from 'sonner';

import { Button } from '../../shared/ds';
import { apiErrorMessage } from '../../lib/apiError';
import type { OrganizationView } from '../organization/organizationService';
import {
  useDeleteOrganizationLogo,
  useUploadOrganizationLogo,
} from '../organization/useOrganization';

type Tr = (fr: string, en: string) => string;

const MAX_LOGO_BYTES = 1024 * 1024;
const LOGO_TYPES = ['image/png', 'image/jpeg', 'image/webp'];

export function OrganizationLogoField({ org, tr }: { org: OrganizationView; tr: Tr }) {
  const input = useRef<HTMLInputElement>(null);
  const upload = useUploadOrganizationLogo();
  const remove = useDeleteOrganizationLogo();
  const busy = upload.isPending || remove.isPending;

  const onFile = (file: File | undefined) => {
    if (input.current) input.current.value = '';
    if (!file) return;
    if (!LOGO_TYPES.includes(file.type)) {
      toast.error(
        tr('Choisissez une image PNG, JPEG ou WebP.', 'Choose a PNG, JPEG or WebP image.'),
      );
      return;
    }
    if (file.size > MAX_LOGO_BYTES) {
      toast.error(tr('Le logo doit faire 1 Mo au plus.', 'The logo must be 1 MB or smaller.'));
      return;
    }
    upload.mutate(file, {
      onSuccess: () => toast.success(tr('Logo mis à jour', 'Logo updated')),
      onError: (err) =>
        toast.error(apiErrorMessage(err) || tr('Téléversement impossible', 'Upload failed')),
    });
  };

  return (
    <div className="flex items-center gap-2 flex-wrap mb-5" data-testid="org-logo-field">
      <input
        ref={input}
        type="file"
        accept={LOGO_TYPES.join(',')}
        className="sr-only"
        data-testid="org-logo-input"
        aria-label={tr('Fichier du logo', 'Logo file')}
        onChange={(e) => onFile(e.target.files?.[0])}
      />
      <Button
        type="button"
        variant="secondary"
        disabled={busy}
        onClick={() => input.current?.click()}
      >
        {org.has_logo
          ? tr('Remplacer le logo', 'Replace logo')
          : tr('Ajouter un logo', 'Add a logo')}
      </Button>
      {org.has_logo && (
        <Button
          type="button"
          variant="ghost"
          disabled={busy}
          onClick={() =>
            remove.mutate(undefined, {
              onSuccess: () => toast.success(tr('Logo retiré', 'Logo removed')),
              onError: (err) =>
                toast.error(
                  apiErrorMessage(err) || tr('Suppression impossible', 'Could not remove'),
                ),
            })
          }
        >
          {tr('Retirer', 'Remove')}
        </Button>
      )}
      <span className="text-[11.5px] text-ink-muted">
        {tr('PNG, JPEG ou WebP, 1 Mo maximum.', 'PNG, JPEG or WebP, 1 MB maximum.')}
      </span>
    </div>
  );
}
