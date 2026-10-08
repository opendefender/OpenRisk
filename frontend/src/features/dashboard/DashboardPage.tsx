// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The landing page.
//
// Every member gets the redesigned dashboard (#901); their business role picks
// its counters and priorities. ?view=executive switches to the consolidated
// executive view, which answers the same "how are we doing" question for a
// committee. /analytics still redirects there, so old links resolve.

import { useSearchParams } from 'react-router';
import { ExecutiveDashboard } from '../analytics/ExecutiveDashboard';
import { ConsoleDashboard } from './ConsoleDashboard';

export const DashboardPage = () => {
  const [params] = useSearchParams();
  if (params.get('view') === 'executive') return <ExecutiveDashboard />;
  return <ConsoleDashboard />;
};
