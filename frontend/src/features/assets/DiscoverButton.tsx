// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// "Lancer une découverte" (#906) runs a real scan: with one discovery source it
// starts it, with several it asks which, with none it goes to Infrastructure to
// set one up. The toast follows the job on its own page. The vulnerability
// queue reuses it as "Lancer un scan" (#909) with its own strings.

import { useNavigate } from 'react-router';
import { ScanSearch } from 'lucide-react';
import { toast } from 'sonner';
import { useMutation, useQuery } from '@tanstack/react-query';

import { useI18n } from '../../hooks/useI18n';
import { useAuthStore } from '../../hooks/useAuthStore';
import { Button, Menu } from '../../shared/ds';
import { scannerService, type ScanConfig } from '../infrastructure/scannerService';

export function DiscoverButton({
  strings = 'inventory.discover',
  testId = 'inv-discover',
}: {
  /** Locale prefix holding label, started, follow and failed. */
  strings?: string;
  testId?: string;
} = {}) {
  const { t } = useI18n();
  const navigate = useNavigate();
  const canScan = useAuthStore((s) => s.hasPermission('scanner:scan'));
  const canRead = useAuthStore((s) => s.hasPermission('scanner:read'));
  const configs = useQuery({
    queryKey: ['scanner', 'configs'],
    queryFn: scannerService.listConfigs,
    enabled: canScan && canRead,
    staleTime: 60_000,
  });
  const run = useMutation({
    mutationFn: (c: ScanConfig) => scannerService.triggerScan(c.id),
    retry: false,
    onSuccess: (job, c) =>
      toast.success(t(`${strings}.started`, { name: c.name }), {
        action: {
          label: t(`${strings}.follow`),
          onClick: () => navigate(`/infrastructure/scans/${job.id}`),
        },
      }),
    onError: () => toast.error(t(`${strings}.failed`)),
  });

  if (!canScan || !canRead) return null;
  const enabled = (configs.data ?? []).filter((c) => c.enabled);

  const button = (onClick?: () => void) => (
    <Button
      variant="primary"
      icon={ScanSearch}
      loading={run.isPending || configs.isLoading}
      onClick={onClick}
      data-testid={testId}
    >
      {t(`${strings}.label`)}
    </Button>
  );

  if (enabled.length > 1) {
    return (
      <Menu
        trigger={button()}
        items={enabled.map((c) => ({ label: c.name, onSelect: () => run.mutate(c) }))}
      />
    );
  }
  return button(() =>
    enabled.length === 1 ? run.mutate(enabled[0]) : navigate('/infrastructure'),
  );
}
