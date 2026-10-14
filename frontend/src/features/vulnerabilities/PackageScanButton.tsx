// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// "Lancer un scan" on the vulnerability queue (#910): the tenant's online
// agents report the packages installed on their host, and each inventory is
// checked against known vulnerabilities as it arrives. The answer is honest
// about the three ways it can not start: no agent online, a scan less than
// 10 minutes ago, or a failure.

import { useNavigate } from 'react-router';
import { useMutation } from '@tanstack/react-query';
import { isAxiosError } from 'axios';
import { ScanSearch } from 'lucide-react';
import { toast } from 'sonner';

import { useI18n } from '../../hooks/useI18n';
import { useAuthStore } from '../../hooks/useAuthStore';
import { Button } from '../../shared/ds';
import { vulnerabilityService } from './vulnerabilityService';

export function PackageScanButton() {
  const { t } = useI18n();
  const navigate = useNavigate();
  const canScan = useAuthStore((s) => s.hasPermission('scanner:scan'));
  const run = useMutation({
    mutationFn: vulnerabilityService.scan,
    // A 404 or a 429 will not change on a second try.
    retry: false,
    onSuccess: (r) => toast.success(t('vulns.scan.started', { count: r.agents })),
    onError: (err) => {
      const status = isAxiosError(err) ? err.response?.status : undefined;
      if (status === 404) {
        toast.error(t('vulns.scan.noAgent'), {
          action: { label: t('vulns.scan.configure'), onClick: () => navigate('/infrastructure') },
        });
      } else if (status === 429) {
        toast.info(t('vulns.scan.cooldown'));
      } else {
        toast.error(t('vulns.scan.failed'));
      }
    },
  });

  if (!canScan) return null;
  return (
    <Button
      variant="primary"
      icon={ScanSearch}
      loading={run.isPending}
      onClick={() => run.mutate()}
      title={t('vulns.scan.hint')}
      data-testid="vuln-scan"
    >
      {t('vulns.scan.label')}
    </Button>
  );
}
