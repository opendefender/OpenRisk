// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).
//
// The risk import (#755). The contract shared with the asset import lives in
// shared/csvImport; this file keeps the risk template and re-exports the rest
// for the risk page and its tests.

export {
  MAX_IMPORT_BYTES,
  importErrorMessage,
  importFileSchema,
  importLimitSchema,
  importRejectedSchema,
  importRowErrorSchema,
  importSuccessSchema,
  type ImportRowError,
} from '../../shared/csvImport/csvImportSchema';

/** The current template: the product's scales, P in [0,1] and I in [0,10]. */
export const IMPORT_TEMPLATE = [
  'title,description,probability,impact,tags,frameworks,assets',
  '"Phishing campaign against finance staff","Credential theft leading to fraudulent transfers",0.6,8,"email;people","ISO27001"',
  '"Ransomware on file servers","Encryption of shared drives, no tested restore",0.3,10,"backup","ISO27001;NIST CSF"',
  '"Cloud provider outage","Loss of the hosted CRM for more than 24 hours",0.2,5,"supplier",',
].join('\n');
