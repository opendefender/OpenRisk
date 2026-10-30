// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package main

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"
)

func TestNmapArgs_Vulnerability(t *testing.T) {
	got := nmapArgs("vulnerability", false, []string{"10.0.0.0/24"})
	want := []string{"-sV", "--script", "vuln", "-oX", "-", "--min-rate", "1000", "--max-retries", "3", "--host-timeout", "15m", "10.0.0.0/24"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("vulnerability args = %v, want %v", got, want)
	}
}

func TestNmapArgs_Discovery(t *testing.T) {
	got := nmapArgs("discovery", false, []string{"10.0.0.5"})
	want := []string{"-sV", "--version-light", "-oX", "-", "--min-rate", "1000", "--max-retries", "2", "--host-timeout", "5m", "10.0.0.5"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("discovery args = %v, want %v", got, want)
	}
	if slices.Contains(got, "vuln") || slices.Contains(got, "--script") {
		t.Fatalf("discovery must not run vulnerability scripts: %v", got)
	}
}

// A server older than this agent sends no scan_type, and a newer one might
// send a type this agent does not know. Both run today's vulnerability scan.
func TestNmapArgs_MissingOrUnknownTypeIsVulnerability(t *testing.T) {
	want := nmapArgs("vulnerability", false, []string{"h"})
	for _, st := range []string{"", "compliance"} {
		if got := nmapArgs(st, false, []string{"h"}); !reflect.DeepEqual(got, want) {
			t.Fatalf("scan_type %q args = %v, want %v", st, got, want)
		}
	}
}

func TestNmapArgs_RootAddsOSDetection(t *testing.T) {
	for _, st := range []string{"discovery", "vulnerability"} {
		got := nmapArgs(st, true, []string{"h"})
		if !slices.Contains(got, "-O") || got[len(got)-1] != "h" {
			t.Fatalf("%s as root: args = %v", st, got)
		}
	}
}

func TestJobDispatch_DecodesScanType(t *testing.T) {
	var d jobDispatch
	if err := json.Unmarshal([]byte(`{"type":"scan.job","job_id":"j","scan_type":"discovery","targets":["h"]}`), &d); err != nil {
		t.Fatal(err)
	}
	if d.ScanType != "discovery" {
		t.Fatalf("scan_type = %q", d.ScanType)
	}
	var old jobDispatch
	_ = json.Unmarshal([]byte(`{"type":"scan.job","job_id":"j","targets":["h"]}`), &old)
	if scanTypeOf(old.ScanType) != "vulnerability" {
		t.Fatalf("dispatch without scan_type = %q", scanTypeOf(old.ScanType))
	}
}
