// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

package dashboard

import "testing"

func TestComputeCyberScore_AllAxes(t *testing.T) {
	cs := computeCyberScore([]scoreAxis{
		{key: "compliance", label: "Conformité", weight: weightCompliance, value: 80, present: true},
		{key: "risk", label: "Risques", weight: weightRisk, value: 60, present: true},
		{key: "vulnerabilities", label: "Vulnérabilités", weight: weightVuln, value: 40, present: true},
		{key: "incidents", label: "Incidents", weight: weightIncident, value: 100, present: true},
	})
	// 0.35*80 + 0.30*60 + 0.20*40 + 0.15*100 = 28 + 18 + 8 + 15 = 69 → D (60..69)
	if cs.Score != 69 {
		t.Fatalf("score = %d, want 69", cs.Score)
	}
	if cs.Grade != "D" {
		t.Fatalf("grade = %q, want D", cs.Grade)
	}
	if len(cs.Components) != 4 {
		t.Fatalf("components = %d, want 4", len(cs.Components))
	}
}

func TestComputeCyberScore_RenormalisesMissingAxis(t *testing.T) {
	// Only compliance present: its effective weight must be 1.0 and score == its value.
	cs := computeCyberScore([]scoreAxis{
		{key: "compliance", label: "Conformité", weight: weightCompliance, value: 82, present: true},
		{key: "risk", label: "Risques", weight: weightRisk, value: 10, present: false},
	})
	if cs.Score != 82 {
		t.Fatalf("score = %d, want 82 (single present axis)", cs.Score)
	}
	if len(cs.Components) != 1 || cs.Components[0].Weight != 1 {
		t.Fatalf("expected 1 component at weight 1.0, got %+v", cs.Components)
	}
	if cs.Grade != "B" {
		t.Fatalf("grade = %q, want B", cs.Grade)
	}
}

func TestComputeCyberScore_NoAxesIsNeutral(t *testing.T) {
	cs := computeCyberScore(nil)
	if cs.Score != 50 || cs.Grade != "E" {
		t.Fatalf("empty axes = %d/%s, want 50/E", cs.Score, cs.Grade)
	}
}

func TestGradeBoundaries(t *testing.T) {
	cases := map[int]string{95: "A", 90: "A", 89: "B", 80: "B", 70: "C", 65: "D", 55: "E", 40: "F", 0: "F"}
	for score, want := range cases {
		if g, _ := gradeFor(score); g != want {
			t.Errorf("gradeFor(%d) = %q, want %q", score, g, want)
		}
	}
}

func TestAxisBuilders(t *testing.T) {
	if v, ok := complianceAxisValue(0, 0); ok {
		t.Errorf("no controls should be absent, got %v", v)
	}
	if v, ok := complianceAxisValue(50, 100); !ok || v != 50 {
		t.Errorf("coverage 50/100 = %v/%v, want 50/true", v, ok)
	}
	// All-critical register scores 0.
	if v, ok := riskAxisValue(10, 0, 10); !ok || v != 0 {
		t.Errorf("all-critical = %v, want 0", v)
	}
	// No risks → absent.
	if _, ok := riskAxisValue(0, 0, 0); ok {
		t.Errorf("no risks should be absent")
	}
	// KEV docks 12 each.
	if v, ok := vulnAxisValue(2, 0, true); !ok || v != 76 {
		t.Errorf("2 KEV = %v, want 76", v)
	}
	if _, ok := vulnAxisValue(0, 0, false); ok {
		t.Errorf("no vuln data should be absent")
	}
	// Resolution 90% docked 10 for one open critical → 80.
	if v, ok := incidentAxisValue(90, 1, 5); !ok || v != 80 {
		t.Errorf("incident axis = %v, want 80", v)
	}
}
