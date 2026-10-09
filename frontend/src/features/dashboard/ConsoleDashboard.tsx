// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The dashboard of the October 2026 redesign (#901).
//
// One layout for every member, read top to bottom: the incident that needs
// attention now, the exposure score, the money at stake and four counters,
// the year of the score next to the risk matrix, then what to do next and
// what just happened. The member's business role picks which four counters
// and which priorities they see (dashboardVariant.ts); there is no role switch
// on screen.
//
// Every block fetches its own tenant-scoped source and renders its own
// loading, error and empty state, so one failing source never blanks the page.

import { useAuthStore } from '../../hooks/useAuthStore';
import { useI18n } from '../../hooks/useI18n';
import { formatDate, formatTime } from '../../i18n/format';
import { PageFrame, PageHeader } from '../../shared/ui';
import { MFAEnrollmentBanner } from '../auth/MFAEnrollmentBanner';
import { MFAPostAhaPrompt } from '../auth/MFAPostAhaPrompt';
import { OnboardingChecklist } from '../onboarding/OnboardingChecklist';
import { useDashboardStats } from './useCommandCenter';
import { variantFor } from './dashboardVariant';
import { IncidentBanner } from './console/IncidentBanner';
import { ScoreCard } from './console/ScoreCard';
import { ExposureCard } from './console/ExposureCard';
import { CounterList } from './console/CounterList';
import { ScoreTrendCard } from './console/ScoreTrendCard';
import { RiskMatrixCard } from './console/RiskMatrixCard';
import { PriorityList } from './console/PriorityList';
import { RecentActivity } from './console/RecentActivity';

const ALL_TIME = { kind: 'preset', preset: 'all' } as const;

export function ConsoleDashboard() {
  const { t, locale } = useI18n();
  const variant = variantFor(useAuthStore((s) => s.user?.business_role));
  // The register aggregate is the page's reference read: its timestamp is the
  // "données à" of the eyebrow.
  const stats = useDashboardStats(ALL_TIME);
  const asOf = stats.dataUpdatedAt ? new Date(stats.dataUpdatedAt) : new Date();
  const day = formatDate(locale, new Date(), { dateStyle: 'full' });

  return (
    <PageFrame>
      <PageHeader
        className="!mb-5"
        eyebrow={t('dashboard.eyebrow', {
          date: day.charAt(0).toUpperCase() + day.slice(1),
          time: formatTime(locale, asOf),
        })}
        title={t('dashboard.title')}
      />

      <IncidentBanner />
      {/* Account and onboarding prompts sit under the incident line: an attack
          in progress outranks a setup reminder. */}
      <MFAEnrollmentBanner />
      <OnboardingChecklist />
      <MFAPostAhaPrompt />

      <div className="flex flex-wrap gap-4 mb-4">
        <ScoreCard className="flex-[1.3_1_400px]" />
        <ExposureCard className="flex-[1_1_320px]" />
        <CounterList variant={variant} className="flex-[0.8_1_260px]" />
      </div>

      <div className="flex flex-wrap gap-4 mb-4">
        <ScoreTrendCard className="flex-[1.4_1_460px]" />
        <RiskMatrixCard className="flex-[1_1_340px]" />
      </div>

      <div className="flex flex-wrap gap-4">
        <PriorityList variant={variant} className="flex-[1.4_1_460px]" />
        <RecentActivity className="flex-[1_1_340px]" />
      </div>
    </PageFrame>
  );
}
