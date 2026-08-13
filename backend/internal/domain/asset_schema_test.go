// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

package domain

import (
	"testing"

	"github.com/google/uuid"
)

// Every shipped schema must survive the same validation a tenant edit goes
// through. If a default were invalid, the category it governs would be
// unwritable the moment it was seeded — and the failure would surface as a
// mysterious 400 on asset creation, far from its cause.
func TestDefaultSchemas_AreValid(t *testing.T) {
	for _, cat := range AssetCategories {
		defs := DefaultAttributes(cat)
		if len(defs) == 0 {
			t.Fatalf("category %s ships no attributes", cat)
		}
		if err := ValidateSchema(defs); err != nil {
			t.Errorf("category %s ships an invalid schema: %v", cat, err)
		}
		if DefaultCategoryLabel(cat) == string(cat) {
			t.Errorf("category %s has no human label", cat)
		}
	}
}

// The 8 categories the spec names must all exist and be parseable.
func TestAssetCategories_Complete(t *testing.T) {
	want := []string{"server", "workstation", "application", "database",
		"network", "cloud", "vendor", "data_processing"}
	if len(AssetCategories) != len(want) {
		t.Fatalf("expected %d categories, got %d", len(want), len(AssetCategories))
	}
	for _, w := range want {
		if _, err := ParseAssetCategory(w); err != nil {
			t.Errorf("category %q should parse: %v", w, err)
		}
	}
	if _, err := ParseAssetCategory("toaster"); err == nil {
		t.Error("an unknown category must be rejected, not accepted as free text")
	}
	if _, err := ParseAssetCategory(""); err == nil {
		t.Error("an empty category must be rejected")
	}
}

func TestValidateSchema_RejectsBrokenDefinitions(t *testing.T) {
	cases := []struct {
		name string
		defs []AttributeDef
	}{
		{"empty", nil},
		{"bad key", []AttributeDef{{Key: "Bad Key", Label: "x", Type: AttrString}}},
		{"no label", []AttributeDef{{Key: "ok_key", Type: AttrString}}},
		{"unknown type", []AttributeDef{{Key: "ok_key", Label: "x", Type: "gemstone"}}},
		{"enum without values", []AttributeDef{{Key: "ok_key", Label: "x", Type: AttrEnum}}},
		{"duplicate key", []AttributeDef{
			{Key: "ok_key", Label: "x", Type: AttrString},
			{Key: "ok_key", Label: "y", Type: AttrString},
		}},
		{"min above max", []AttributeDef{{Key: "ok_key", Label: "x", Type: AttrNumber, Min: f(10), Max: f(1)}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateSchema(tc.defs); err == nil {
				t.Fatal("expected the schema to be rejected")
			}
		})
	}
}

func TestValidateAttributes_CoercesAndValidates(t *testing.T) {
	defs := []AttributeDef{
		{Key: "hostname", Label: "Nom d'hôte", Type: AttrHostname, Required: true},
		{Key: "port", Label: "Port", Type: AttrInteger, Min: f(1), Max: f(65535)},
		{Key: "internet_exposed", Label: "Exposé", Type: AttrBoolean},
		{Key: "environment", Label: "Environnement", Type: AttrEnum, Enum: []string{"production", "test"}},
		{Key: "ips", Label: "IPs", Type: AttrIPList},
		{Key: "last_patched", Label: "Dernier correctif", Type: AttrDate},
		{Key: "contact", Label: "Contact", Type: AttrEmail},
	}

	got, err := ValidateAttributes(defs, map[string]any{
		"hostname":         " web-01.example.com ",
		"port":             "8443", // string from a form field
		"internet_exposed": "true", // string from a checkbox
		"environment":      "production",
		"ips":              "10.0.0.1, 10.0.0.2",   // comma-separated from a plain input
		"last_patched":     "2026-08-01T00:00:00Z", // full timestamp from a date picker
		"contact":          "ops@example.com",
	})
	if err != nil {
		t.Fatalf("expected valid attributes, got %v", err)
	}
	if got["hostname"] != "web-01.example.com" {
		t.Errorf("hostname not trimmed: %q", got["hostname"])
	}
	if got["port"] != float64(8443) {
		t.Errorf("port not coerced to a number: %#v", got["port"])
	}
	if got["internet_exposed"] != true {
		t.Errorf("boolean not coerced: %#v", got["internet_exposed"])
	}
	if got["last_patched"] != "2026-08-01" {
		t.Errorf("date not normalised: %#v", got["last_patched"])
	}
	ips, _ := got["ips"].([]string)
	if len(ips) != 2 || ips[1] != "10.0.0.2" {
		t.Errorf("ip list not parsed: %#v", got["ips"])
	}
}

func TestValidateAttributes_Rejections(t *testing.T) {
	defs := []AttributeDef{
		{Key: "hostname", Label: "Nom d'hôte", Type: AttrHostname, Required: true},
		{Key: "port", Label: "Port", Type: AttrInteger, Min: f(1), Max: f(65535)},
		{Key: "environment", Label: "Environnement", Type: AttrEnum, Enum: []string{"production", "test"}},
		{Key: "ip", Label: "IP", Type: AttrIP},
	}
	cases := []struct {
		name string
		in   map[string]any
	}{
		{"missing required", map[string]any{"port": 443}},
		{"unknown attribute", map[string]any{"hostname": "a", "colour": "blue"}},
		{"out of range", map[string]any{"hostname": "a", "port": 70000}},
		{"not a whole number", map[string]any{"hostname": "a", "port": 44.5}},
		{"value outside enum", map[string]any{"hostname": "a", "environment": "prod"}},
		{"malformed ip", map[string]any{"hostname": "a", "ip": "999.1.1.1"}},
		{"malformed hostname", map[string]any{"hostname": "not a hostname!"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ValidateAttributes(defs, tc.in); err == nil {
				t.Fatal("expected a validation error")
			}
		})
	}
}

// An unknown key is rejected rather than dropped. Dropping it would lose data
// the user typed without telling them; keeping it would make the schema a
// suggestion instead of a contract.
func TestValidateAttributes_UnknownKeyIsNamed(t *testing.T) {
	defs := []AttributeDef{{Key: "hostname", Label: "Nom d'hôte", Type: AttrString}}
	_, err := ValidateAttributes(defs, map[string]any{"hostname": "a", "zzz": 1, "aaa": 2})
	if err == nil {
		t.Fatal("expected rejection")
	}
	msg := err.Error()
	if !contains(msg, "aaa") || !contains(msg, "zzz") {
		t.Errorf("the error should name every unknown attribute, got %q", msg)
	}
}

func TestFingerprintSignals_ExtractedFromSchema(t *testing.T) {
	defs := DefaultAttributes(CategoryServer)
	attrs, err := ValidateAttributes(defs, map[string]any{
		"hostname":         "web-01",
		"ip_addresses":     []string{"10.0.0.5"},
		"operating_system": "Ubuntu",
		"environment":      "production",
		"cpes":             []string{"cpe:2.3:a:apache:log4j:2.14.1"},
	})
	if err != nil {
		t.Fatalf("valid server attributes rejected: %v", err)
	}
	sig := FingerprintSignalsFrom(defs, attrs)
	if len(sig.Hostnames) != 1 || sig.Hostnames[0] != "web-01" {
		t.Errorf("hostname fingerprint not extracted: %#v", sig.Hostnames)
	}
	if len(sig.IPs) != 1 || sig.IPs[0] != "10.0.0.5" {
		t.Errorf("ip fingerprint not extracted: %#v", sig.IPs)
	}
	if len(sig.CPEs) != 1 {
		t.Errorf("cpe fingerprint not extracted: %#v", sig.CPEs)
	}
}

// An application declares its hostname fingerprint through a URL attribute; the
// extractor must reduce it to the host, so correlation works without the tenant
// typing the hostname twice.
func TestFingerprintSignals_URLReducedToHost(t *testing.T) {
	defs := DefaultAttributes(CategoryApplication)
	attrs, err := ValidateAttributes(defs, map[string]any{
		"application_name": "Portail client",
		"url":              "https://portail.example.com/login",
		"environment":      "production",
	})
	if err != nil {
		t.Fatalf("valid application attributes rejected: %v", err)
	}
	sig := FingerprintSignalsFrom(defs, attrs)
	if len(sig.Hostnames) != 1 || sig.Hostnames[0] != "portail.example.com" {
		t.Errorf("expected the URL host as fingerprint, got %#v", sig.Hostnames)
	}
}

// The scanner learns CPEs the operator never typed. A later manual edit must not
// erase them — that is how a working correlation silently starts failing.
func TestRefreshFingerprints_PreservesDiscoveredValues(t *testing.T) {
	defs := DefaultAttributes(CategoryServer)
	a := &Asset{
		ID:   uuid.New(),
		CPEs: []string{"cpe:2.3:o:canonical:ubuntu_linux:22.04"}, // pushed by the scanner
		Attributes: AssetAttributes{
			"hostname": "web-01",
			"cpes":     []string{"cpe:2.3:a:apache:log4j:2.14.1"}, // typed by the operator
		},
	}
	a.RefreshFingerprints(defs)

	if len(a.CPEs) != 2 {
		t.Fatalf("expected the scanner CPE to be kept alongside the typed one, got %#v", a.CPEs)
	}
	if a.CPEs[0] != "cpe:2.3:o:canonical:ubuntu_linux:22.04" {
		t.Errorf("pre-existing values must keep their order, got %#v", a.CPEs)
	}
	// Idempotent: a second refresh must not duplicate anything.
	a.RefreshFingerprints(defs)
	if len(a.CPEs) != 2 {
		t.Errorf("refresh is not idempotent: %#v", a.CPEs)
	}
}

func TestMatchesAttributes(t *testing.T) {
	attrs := AssetAttributes{
		"environment":      "production",
		"internet_exposed": true,
		"port":             float64(8443),
		"certifications":   []string{"iso-27001", "soc-2"},
	}
	ok := []AttributeSearchTerm{
		{Key: "environment", Value: "PRODUCTION"}, // case-insensitive
		{Key: "internet_exposed", Value: "true"},
		{Key: "port", Value: "8443"},
		{Key: "certifications", Value: "soc-2"}, // list contains
	}
	for _, term := range ok {
		if !MatchesAttributes(attrs, []AttributeSearchTerm{term}) {
			t.Errorf("expected %v to match", term)
		}
	}
	no := []AttributeSearchTerm{
		{Key: "environment", Value: "test"},
		{Key: "certifications", Value: "pci-dss"},
		{Key: "absent_key", Value: "anything"},
	}
	for _, term := range no {
		if MatchesAttributes(attrs, []AttributeSearchTerm{term}) {
			t.Errorf("expected %v not to match", term)
		}
	}
	// Terms are AND-ed.
	if MatchesAttributes(attrs, []AttributeSearchTerm{
		{Key: "environment", Value: "production"},
		{Key: "port", Value: "22"},
	}) {
		t.Error("terms must be AND-ed, not OR-ed")
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (func() bool {
		for i := 0; i+len(needle) <= len(haystack); i++ {
			if haystack[i:i+len(needle)] == needle {
				return true
			}
		}
		return false
	})()
}
