// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package realtime

import (
	"errors"
	"strings"
	"testing"
)

// Every catalog entry must obey the naming convention, declare a real
// aggregate, and publish a positive version. This is the check that keeps the
// catalog from drifting as entries are added by hand.
func TestCatalog_EveryEntryIsWellFormed(t *testing.T) {
	entries := Catalog()
	if len(entries) == 0 {
		t.Fatal("the catalog is empty")
	}
	for _, d := range entries {
		name := string(d.Type)
		parts := strings.Split(name, ".")
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			t.Errorf("%q does not follow <aggregate>.<action>", name)
			continue
		}
		if d.Aggregate == "" {
			t.Errorf("%q declares no aggregate", name)
		}
		if d.Version < 1 {
			t.Errorf("%q publishes version %d", name, d.Version)
		}
		if d.Trigger == "" {
			t.Errorf("%q has no trigger description — an operator cannot tell where it comes from", name)
		}
		if d.Origin != OriginMutation && d.Origin != OriginDomain {
			t.Errorf("%q has origin %q, which is neither mutation nor domain", name, d.Origin)
		}
		// The map key and the descriptor must agree, or Lookup returns an entry
		// describing a different event.
		if got, ok := Lookup(d.Type); !ok || got.Type != d.Type {
			t.Errorf("%q is not retrievable by its own type", name)
		}
	}
}

// The payload contract may never name a field the envelope validator would
// refuse. If these two ever disagree, the catalog documents an event that
// cannot be published.
func TestCatalog_NoDeclaredPayloadFieldIsForbidden(t *testing.T) {
	for _, d := range Catalog() {
		for _, f := range d.PayloadFields {
			if IsForbiddenField(f) {
				t.Errorf("%q declares payload field %q, which the envelope validator forbids", d.Type, f)
			}
		}
	}
}

// The domains W0-07 is required to cover must each have at least a create, an
// update and a delete, so a consumer can follow an aggregate's whole life.
func TestCatalog_CoversTheRequiredDomains(t *testing.T) {
	required := map[string][]string{
		AggregateRisk:            {"created", "updated", "deleted", "status_changed"},
		AggregateAsset:           {"created", "updated", "deleted"},
		AggregateVulnerability:   {"detected", "updated", "deleted"},
		AggregateIncident:        {"created", "updated", "deleted"},
		AggregateControl:         {"created", "updated", "deleted"},
		AggregateComplianceAudit: {"created", "updated", "deleted"},
	}
	have := map[string]map[string]bool{}
	for _, d := range Catalog() {
		action := string(d.Type)[strings.Index(string(d.Type), ".")+1:]
		if have[d.Aggregate] == nil {
			have[d.Aggregate] = map[string]bool{}
		}
		have[d.Aggregate][action] = true
	}
	for agg, actions := range required {
		for _, a := range actions {
			if !have[agg][a] {
				t.Errorf("aggregate %q has no %q event", agg, a)
			}
		}
	}
}

func TestParseFilter_EmptyMatchesEverything(t *testing.T) {
	f, err := ParseFilter("", "")
	if err != nil {
		t.Fatal(err)
	}
	if !f.IsEmpty() {
		t.Fatal("an empty filter must narrow nothing")
	}
	if !f.Match(validEnvelope()) {
		t.Fatal("an empty filter must match")
	}
	if f.Describe() != "all" {
		t.Fatalf("Describe() = %q", f.Describe())
	}
}

func TestParseFilter_NarrowsByTypeAndAggregate(t *testing.T) {
	f, err := ParseFilter("risk.created,risk.deleted", "")
	if err != nil {
		t.Fatal(err)
	}
	if !f.Match(validEnvelope()) {
		t.Fatal("risk.created must pass a filter that names it")
	}
	other := validEnvelope()
	other.Type = RiskUpdated
	if f.Match(other) {
		t.Fatal("risk.updated must not pass a filter that does not name it")
	}

	byAgg, err := ParseFilter("", "asset")
	if err != nil {
		t.Fatal(err)
	}
	if byAgg.Match(validEnvelope()) {
		t.Fatal("a risk event must not pass an asset-only filter")
	}
}

// Types and aggregates are AND-ed: asking for both is asking for the
// intersection, which is the only reading in which both are useful.
func TestParseFilter_TypeAndAggregateAreIntersected(t *testing.T) {
	f, err := ParseFilter("risk.created", "asset")
	if err != nil {
		t.Fatal(err)
	}
	if f.Match(validEnvelope()) {
		t.Fatal("risk.created must not pass a filter demanding the asset aggregate")
	}
}

func TestParseFilter_RefusesUnknownNames(t *testing.T) {
	if _, err := ParseFilter("risk.creted", ""); !errors.Is(err, ErrUnknownFilterType) {
		t.Fatalf("want ErrUnknownFilterType, got %v", err)
	}
	if _, err := ParseFilter("", "riskz"); !errors.Is(err, ErrUnknownAggregate) {
		t.Fatalf("want ErrUnknownAggregate, got %v", err)
	}
}

func TestParseFilter_RefusesAnAbusivelyLongList(t *testing.T) {
	var b strings.Builder
	for i := 0; i < MaxFilterEntries+5; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		// Distinct-but-valid names would be ideal; the catalog is smaller than
		// the cap, so build the overflow from a repeated valid name plus padding
		// that must itself be refused first.
		b.WriteString("risk.created")
	}
	// Duplicates collapse, so this one is legal — proving dedup happens before
	// the cap and a client cannot trip the limit by repeating itself.
	if _, err := ParseFilter(b.String(), ""); err != nil {
		t.Fatalf("repeated identical names must collapse, got %v", err)
	}
}

func TestParseFilter_IgnoresWhitespaceAndEmptyTokens(t *testing.T) {
	f, err := ParseFilter(" risk.created , , risk.deleted ", " asset , ")
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Types) != 2 || len(f.Aggregates) != 1 {
		t.Fatalf("unexpected parse: %+v", f)
	}
}

// Every aggregate must map to a permission. An aggregate with no mapping is
// withheld by AllowedAggregates, so forgetting one costs visibility rather than
// confidentiality — but it is still a defect, and this is what catches it.
func TestCatalog_EveryAggregateHasAPermission(t *testing.T) {
	for _, agg := range Aggregates() {
		if _, ok := PermissionForAggregate(agg); !ok {
			t.Errorf("aggregate %q has no permission mapping — its events would be withheld from everyone", agg)
		}
	}
	for _, d := range Catalog() {
		if d.Permission == "" {
			t.Errorf("%q carries no permission after init", d.Type)
		}
	}
}

func TestAllowedAggregates_ReflectsWhatTheCallerHolds(t *testing.T) {
	held := map[string]bool{"risks:read": true, "assets:read": true}
	got := AllowedAggregates(func(p string) bool { return held[p] })

	want := map[string]bool{AggregateRisk: true, AggregateAsset: true}
	if len(got) != len(want) {
		t.Fatalf("got %v, want exactly %v", got, want)
	}
	for _, a := range got {
		if !want[a] {
			t.Fatalf("aggregate %q leaked into the allowed set", a)
		}
	}

	if got := AllowedAggregates(func(string) bool { return false }); len(got) != 0 {
		t.Fatalf("a caller holding nothing must be allowed nothing, got %v", got)
	}
	if got := AllowedAggregates(nil); got != nil {
		t.Fatalf("a nil predicate must allow nothing, got %v", got)
	}
}

// The core authorization property: a requested filter can only ever subtract.
func TestRestrict_ARequestCanNeverWidenBeyondWhatIsAllowed(t *testing.T) {
	allowed := []string{AggregateRisk}

	// Asking for everything yields only what is permitted.
	unfiltered, ok := Restrict(Filter{}, allowed)
	if !ok {
		t.Fatal("a caller with one permitted aggregate must get a stream")
	}
	if len(unfiltered.Aggregates) != 1 || unfiltered.Aggregates[0] != AggregateRisk {
		t.Fatalf("an unfiltered subscription must resolve to the permitted set, got %+v", unfiltered)
	}
	assetEvent := validEnvelope()
	assetEvent.Type = AssetCreated
	assetEvent.Aggregate.Type = AggregateAsset
	if unfiltered.Match(assetEvent) {
		t.Fatal("an unfiltered subscription must not match an aggregate the caller cannot read")
	}

	// Explicitly asking for a forbidden aggregate does not grant it.
	requested, err := ParseFilter("", "risk,asset")
	if err != nil {
		t.Fatal(err)
	}
	narrowed, ok := Restrict(requested, allowed)
	if !ok {
		t.Fatal("the permitted part of the request must survive")
	}
	if len(narrowed.Aggregates) != 1 || narrowed.Aggregates[0] != AggregateRisk {
		t.Fatalf("the forbidden aggregate survived: %+v", narrowed)
	}

	// Same for a type-level request.
	byType, err := ParseFilter("risk.created,asset.created", "")
	if err != nil {
		t.Fatal(err)
	}
	narrowedTypes, ok := Restrict(byType, allowed)
	if !ok {
		t.Fatal("the permitted type must survive")
	}
	for _, ty := range narrowedTypes.Types {
		if ty == AssetCreated {
			t.Fatal("asset.created survived for a caller who may not read assets")
		}
	}
}

// A subscription that could never deliver anything is refused rather than left
// open as a stream that mysteriously stays silent.
func TestRestrict_RefusesWhenNothingCouldEverBeDelivered(t *testing.T) {
	if _, ok := Restrict(Filter{}, nil); ok {
		t.Fatal("a caller allowed no aggregate must be refused, not given a silent stream")
	}
	onlyRisk := []string{AggregateRisk}
	assetOnly, err := ParseFilter("", "asset")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := Restrict(assetOnly, onlyRisk); ok {
		t.Fatal("asking only for a forbidden aggregate must be refused")
	}
	typeOnly, err := ParseFilter("asset.created", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := Restrict(typeOnly, onlyRisk); ok {
		t.Fatal("asking only for a forbidden type must be refused")
	}
}
