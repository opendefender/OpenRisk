// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

export interface Risk {
  id: string;
  title: string;
  description?: string;
  score: number;
  impact: number;
  probability: number;
  status: string;
  tags?: string[];
  frameworks?: string[];
  source?: string;
  custom_fields?: Record<string, any>;
  created_at?: string;
  updated_at?: string;
}

export type PartialRisk = Partial<Risk> & { id?: string };
