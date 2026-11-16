// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// The NIST source is pinned to one commit of usnistgov/oscal-content, so a
// regeneration reproduces the same rows until someone moves the pin on purpose.
const (
	nistCommit  = "f0de6d8d8c059d374f15e0ae07508d82e370fc1d"
	nistPath    = "nist.gov/SP800-53/rev5/json/NIST_SP-800-53_rev5_MODERATE-baseline-resolved-profile_catalog.json"
	nistFile    = "nist-800-53-r5-moderate.json"
	nistBaseURL = "https://raw.githubusercontent.com/usnistgov/oscal-content/"
)

// Only the parts of OSCAL this generator reads.
type oscalCatalog struct {
	Catalog struct {
		Metadata struct {
			Title   string `json:"title"`
			Version string `json:"version"`
		} `json:"metadata"`
		Groups []oscalGroup `json:"groups"`
	} `json:"catalog"`
}

type oscalGroup struct {
	ID       string         `json:"id"`
	Title    string         `json:"title"`
	Controls []oscalControl `json:"controls"`
}

type oscalProp struct {
	Name  string `json:"name"`
	Value string `json:"value"`
	Class string `json:"class"`
}

type oscalParam struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Select *struct {
		HowMany string   `json:"how-many"`
		Choice  []string `json:"choice"`
	} `json:"select"`
}

type oscalPart struct {
	Name  string      `json:"name"`
	Props []oscalProp `json:"props"`
	Prose string      `json:"prose"`
	Parts []oscalPart `json:"parts"`
}

type oscalControl struct {
	ID       string         `json:"id"`
	Title    string         `json:"title"`
	Props    []oscalProp    `json:"props"`
	Params   []oscalParam   `json:"params"`
	Parts    []oscalPart    `json:"parts"`
	Controls []oscalControl `json:"controls"`
}

func label(props []oscalProp) string {
	for _, p := range props {
		if p.Name == "label" && p.Class == "" {
			return p.Value
		}
	}
	return ""
}

func withdrawn(props []oscalProp) bool {
	for _, p := range props {
		if p.Name == "status" && p.Value == "withdrawn" {
			return true
		}
	}
	return false
}

var insertRE = regexp.MustCompile(`\{\{\s*insert:\s*param,\s*([^\s}]+)\s*\}\}`)

// renderParam writes a parameter the way NIST's own PDF does:
// "[Assignment: organization-defined …]" or "[Selection (one or more): a; b]".
func renderParam(id string, params map[string]oscalParam, depth int) string {
	p, ok := params[id]
	if !ok || depth > 4 {
		return "[Assignment: organization-defined parameter]"
	}
	if p.Select != nil {
		choices := make([]string, len(p.Select.Choice))
		for i, c := range p.Select.Choice {
			choices[i] = substitute(c, params, depth+1)
		}
		head := "Selection"
		if p.Select.HowMany == "one-or-more" {
			head = "Selection (one or more)"
		}
		return "[" + head + ": " + strings.Join(choices, "; ") + "]"
	}
	// Most labels are bare ("frequency"); a few already say "organization-defined".
	return "[Assignment: organization-defined " + strings.TrimPrefix(p.Label, "organization-defined ") + "]"
}

func substitute(prose string, params map[string]oscalParam, depth int) string {
	return insertRE.ReplaceAllStringFunc(prose, func(m string) string {
		return renderParam(insertRE.FindStringSubmatch(m)[1], params, depth)
	})
}

// statementText flattens a control's statement part into one line, keeping
// NIST's own item labels ("a.", "1.") so a reader can cite them.
func statementText(parts []oscalPart, params map[string]oscalParam) string {
	var b strings.Builder
	var walk func(p oscalPart)
	walk = func(p oscalPart) {
		if l := label(p.Props); l != "" {
			b.WriteString(l + " ")
		}
		if p.Prose != "" {
			b.WriteString(substitute(p.Prose, params, 0) + " ")
		}
		for _, c := range p.Parts {
			walk(c)
		}
	}
	for _, p := range parts {
		if p.Name == "statement" {
			walk(p)
		}
	}
	return clean(b.String())
}

func nistRows(cat oscalCatalog, sourceURL string) ([]row, error) {
	version := cat.Catalog.Metadata.Version
	var rows []row
	var add func(c oscalControl, parentTitle string) error
	add = func(c oscalControl, parentTitle string) error {
		if withdrawn(c.Props) {
			return nil
		}
		code := label(c.Props)
		if code == "" {
			return fmt.Errorf("control %s has no label", c.ID)
		}
		params := make(map[string]oscalParam, len(c.Params))
		for _, p := range c.Params {
			params[p.ID] = p
		}
		name := c.Title
		if parentTitle != "" {
			// NIST names an enhancement "Base Title | Enhancement Title".
			name = parentTitle + " | " + c.Title
		}
		desc := statementText(c.Parts, params)
		if desc == "" {
			return fmt.Errorf("control %s has no statement", code)
		}
		rows = append(rows, row{
			Code:        code,
			Name:        name,
			Description: desc,
			Source:      "NIST SP 800-53 Rev. " + version + ", " + code + " (MODERATE baseline) — " + sourceURL,
		})
		for _, e := range c.Controls {
			if err := add(e, c.Title); err != nil {
				return err
			}
		}
		return nil
	}
	for _, g := range cat.Catalog.Groups {
		for _, c := range g.Controls {
			if err := add(c, ""); err != nil {
				return nil, err
			}
		}
	}
	return rows, nil
}

func genNIST(f fetcher) (string, []byte, error) {
	url := nistBaseURL + nistCommit + "/" + nistPath
	src, err := f.get(nistFile, url, nil)
	if err != nil {
		return "", nil, err
	}
	var cat oscalCatalog
	if err := json.Unmarshal(src.Body, &cat); err != nil {
		return "", nil, fmt.Errorf("parse OSCAL: %w", err)
	}
	rows, err := nistRows(cat, url)
	if err != nil {
		return "", nil, err
	}
	families := make([]string, 0, len(cat.Catalog.Groups))
	for _, g := range cat.Catalog.Groups {
		families = append(families, strings.ToUpper(g.ID))
	}
	file := catalogFile{
		Key:  "nist-800-53-r5",
		Name: "NIST SP 800-53",
		// Name and Version stay what the family-level catalogue used: the import
		// picker finds a tenant's existing framework by name and version, so a
		// re-import completes it instead of creating a second one.
		Version: "Rev. 5",
		Description: fmt.Sprintf("Security and Privacy Controls for Information Systems and Organizations — the %d controls and control enhancements of the MODERATE baseline (SP 800-53B, release %s), across %d families (%s). Generated from NIST's official OSCAL catalogue.",
			len(rows), cat.Catalog.Metadata.Version, len(families), strings.Join(families, ", ")),
		VarName: "nist80053r5Controls",
		Provenance: []string{
			"NIST SP 800-53 Rev. 5 — " + cat.Catalog.Metadata.Title + ".",
			"",
			"Source: " + url,
			"SHA-256 of the source: " + src.SHA256,
			"",
			"Each row is one control or control enhancement selected by the MODERATE",
			"baseline (NIST SP 800-53B), with NIST's identifier (AC-2, AC-2(1)), NIST's",
			"title, and NIST's control statement with its parameters written as NIST",
			"writes them ([Assignment: …], [Selection: …]). The PM and PT families carry",
			"no MODERATE-baseline controls, so they have no row here.",
			"",
			"Reproduced under 15 U.S.C. § 105: NIST SP 800-53 is a work of the United",
			"States Government and is not subject to copyright in the United States.",
			"",
			"Tenants who imported the earlier 20-row, family-level catalogue keep those",
			"rows: an import is additive by reference code, and family codes (\"AC\")",
			"are not control codes (\"AC-1\").",
		},
		Rows: rows,
	}
	code, err := file.render("nist")
	return "catalog_nist_800_53_r5.go", code, err
}
