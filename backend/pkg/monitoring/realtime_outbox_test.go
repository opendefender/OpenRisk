// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package monitoring

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestRealtimeOutboxCollector_ReportsBacklogAndLag(t *testing.T) {
	oldest := time.Now().Add(-10 * time.Second)
	c := NewRealtimeOutboxCollector(func(context.Context) (int64, *time.Time, error) { return 3, &oldest, nil })
	reg := prometheus.NewRegistry()
	reg.MustRegister(c)

	mfs, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]float64{}
	for _, mf := range mfs {
		got[mf.GetName()] = mf.GetMetric()[0].GetGauge().GetValue()
	}
	if got["openrisk_realtime_outbox_pending"] != 3 {
		t.Fatalf("pending = %v, want 3", got["openrisk_realtime_outbox_pending"])
	}
	if lag := got["openrisk_realtime_outbox_oldest_pending_seconds"]; lag < 10 || lag > 12 {
		t.Fatalf("lag = %v, want within [10,12]", lag)
	}
}

func TestRealtimeOutboxCollector_EmptyBacklogIsZero(t *testing.T) {
	c := NewRealtimeOutboxCollector(func(context.Context) (int64, *time.Time, error) { return 0, nil, nil })
	if n := testutil.CollectAndCount(c); n != 2 {
		t.Fatalf("want 2 samples, got %d", n)
	}
	reg := prometheus.NewRegistry()
	reg.MustRegister(c)
	mfs, _ := reg.Gather()
	for _, mf := range mfs {
		if v := mf.GetMetric()[0].GetGauge().GetValue(); v != 0 {
			t.Fatalf("%s = %v, want 0", mf.GetName(), v)
		}
	}
}

// A read error must not report a stale or zero value: no sample at all, so an
// alert on absent() fires.
func TestRealtimeOutboxCollector_ErrorEmitsNoSample(t *testing.T) {
	c := NewRealtimeOutboxCollector(func(context.Context) (int64, *time.Time, error) {
		return 0, nil, errors.New("db down")
	})
	if n := testutil.CollectAndCount(c); n != 0 {
		t.Fatalf("an unreadable backlog must emit no sample, got %d", n)
	}
}
