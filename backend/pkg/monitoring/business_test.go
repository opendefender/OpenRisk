// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

package monitoring

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestRecordReportGenerated(t *testing.T) {
	before := testutil.ToFloat64(ReportsGeneratedTotal.WithLabelValues("board"))
	RecordReportGenerated("board")
	RecordReportGenerated("board")
	after := testutil.ToFloat64(ReportsGeneratedTotal.WithLabelValues("board"))
	if after-before != 2 {
		t.Fatalf("expected +2 for board, got +%v", after-before)
	}

	// Empty kind is bucketed as "unknown", never dropped.
	u0 := testutil.ToFloat64(ReportsGeneratedTotal.WithLabelValues("unknown"))
	RecordReportGenerated("")
	if got := testutil.ToFloat64(ReportsGeneratedTotal.WithLabelValues("unknown")); got-u0 != 1 {
		t.Fatalf("empty kind should count as unknown, got +%v", got-u0)
	}
}
