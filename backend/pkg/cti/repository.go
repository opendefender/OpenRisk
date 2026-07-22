// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package cti

// Repository interface is defined in cti.go alongside the Service interface.
// This file previously contained the Repository interface definition.
// The interface has been moved to cti.go for cohesion.
//
// The GORM implementation lives in:
//   internal/infrastructure/repository/gorm_cti_repository.go
