// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package vulnscan

import (
	"math"
	"strings"
)

// CVSS3BaseScore computes the base score of a CVSS v3.0 or v3.1 vector, per
// the FIRST specification (§7.1, with the v3.1 Roundup). OSV advisories carry
// the vector rather than the score (#910). ok is false for anything that is
// not a complete v3 base vector: no guess is made from a partial one.
func CVSS3BaseScore(vector string) (float64, bool) {
	parts := strings.Split(strings.TrimSpace(vector), "/")
	if len(parts) < 9 || (parts[0] != "CVSS:3.1" && parts[0] != "CVSS:3.0") {
		return 0, false
	}
	m := map[string]string{}
	for _, p := range parts[1:] {
		kv := strings.SplitN(p, ":", 2)
		if len(kv) == 2 {
			m[kv[0]] = kv[1]
		}
	}
	pick := func(key string, table map[string]float64) (float64, bool) {
		v, ok := table[m[key]]
		return v, ok
	}
	changed := m["S"] == "C"
	if m["S"] != "C" && m["S"] != "U" {
		return 0, false
	}
	av, ok1 := pick("AV", map[string]float64{"N": 0.85, "A": 0.62, "L": 0.55, "P": 0.2})
	ac, ok2 := pick("AC", map[string]float64{"L": 0.77, "H": 0.44})
	prTable := map[string]float64{"N": 0.85, "L": 0.62, "H": 0.27}
	if changed {
		prTable = map[string]float64{"N": 0.85, "L": 0.68, "H": 0.5}
	}
	pr, ok3 := pick("PR", prTable)
	ui, ok4 := pick("UI", map[string]float64{"N": 0.85, "R": 0.62})
	cia := map[string]float64{"H": 0.56, "L": 0.22, "N": 0}
	c, ok5 := pick("C", cia)
	i, ok6 := pick("I", cia)
	a, ok7 := pick("A", cia)
	if !(ok1 && ok2 && ok3 && ok4 && ok5 && ok6 && ok7) {
		return 0, false
	}

	iss := 1 - (1-c)*(1-i)*(1-a)
	var impact float64
	if changed {
		impact = 7.52*(iss-0.029) - 3.25*math.Pow(iss-0.02, 15)
	} else {
		impact = 6.42 * iss
	}
	if impact <= 0 {
		return 0, true
	}
	exploitability := 8.22 * av * ac * pr * ui
	if changed {
		return roundup(math.Min(1.08*(impact+exploitability), 10)), true
	}
	return roundup(math.Min(impact+exploitability, 10)), true
}

// roundup is CVSS v3.1's Roundup: the smallest one-decimal number ≥ x,
// computed on integers to avoid floating-point drift (Appendix A).
func roundup(x float64) float64 {
	n := int64(math.Round(x * 100000))
	if n%10000 == 0 {
		return float64(n) / 100000
	}
	return float64(n/10000+1) / 10
}
