// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

/** The current template. Names must be new to the inventory. */
export const ASSET_IMPORT_TEMPLATE = [
  'name,type,criticality,owner',
  '"Core banking database",Database,CRITICAL,"IT department"',
  '"Customer web portal",Server,HIGH,',
  '"HR laptop fleet",Laptop,MEDIUM,"HR"',
].join('\n');
