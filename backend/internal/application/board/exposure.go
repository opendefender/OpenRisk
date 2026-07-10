// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package board

// ExposureModel maps each risk criticality level to a reference annual-loss value
// in FCFA. The board report estimates financial exposure as the sum over the
// register of (count at level × reference value). This is a deliberately simple,
// transparent order-of-magnitude model — NOT an actuarial figure — so the board
// gets a sense of scale it can reason about. Values are configurable (env) in the
// composition root; the defaults below are sensible orders of magnitude for the
// target market (banks/insurers in the UEMOA/CEMAC zones).
type ExposureModel struct {
	Critical int64
	High     int64
	Medium   int64
	Low      int64
}

// DefaultExposureModel returns the built-in reference values (FCFA).
func DefaultExposureModel() ExposureModel {
	return ExposureModel{
		Critical: 50_000_000,
		High:     15_000_000,
		Medium:   3_000_000,
		Low:      500_000,
	}
}

// Compute returns the estimated annual exposure in FCFA for the given counts.
func (m ExposureModel) Compute(critical, high, medium, low int) int64 {
	return int64(critical)*m.Critical +
		int64(high)*m.High +
		int64(medium)*m.Medium +
		int64(low)*m.Low
}
