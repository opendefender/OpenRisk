// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"fmt"
	"sort"
	"strings"
)

const (
	celexNIS2 = "32022L2555"
	celexDORA = "32022R2554"
	celexIR   = "32024R2690"

	eliNIS2 = "https://eur-lex.europa.eu/eli/dir/2022/2555/oj"
	eliDORA = "https://eur-lex.europa.eu/eli/reg/2022/2554/oj"
	eliIR   = "https://eur-lex.europa.eu/eli/reg_impl/2024/2690/oj"
)

var euReuse = []string{
	"",
	"Reproduced from the Official Journal of the European Union under Commission",
	"Decision 2011/833/EU on the reuse of Commission documents; only the French",
	"Official Journal text is authentic.",
}

func fetchAct(f fetcher, celex string) (act, source, error) {
	src, err := f.get(celex+"-fra.xhtml", cellarURL(celex), cellarHeader)
	if err != nil {
		return act{}, src, err
	}
	a, err := parseAct(textLines(src.Body))
	return a, src, err
}

func euRows(a act, spec []euRow, prefix, eli string) ([]row, error) {
	rows := make([]row, 0, len(spec))
	for _, s := range spec {
		text, err := euText(a, s)
		if err != nil {
			return nil, err
		}
		rows = append(rows, row{
			Code:        s.Code,
			Name:        s.Name,
			Description: text,
			Source:      prefix + s.cite() + " — " + eli,
		})
	}
	return rows, nil
}

// checkCoverage fails when a paragraph of articles [from, to] is neither a row
// nor a recorded exclusion, so "covers Art. 5-30" is checked, not asserted.
func checkCoverage(a act, spec []euRow, excl []euExclusion, from, to int) error {
	covered := map[[2]int]bool{}
	whole := map[int]bool{}
	for _, s := range spec {
		if s.Para == 0 {
			whole[s.Art] = true
		}
		covered[[2]int{s.Art, s.Para}] = true
	}
	for _, e := range excl {
		if e.Paras == nil {
			whole[e.Art] = true
		}
		for _, p := range e.Paras {
			covered[[2]int{e.Art, p}] = true
		}
	}
	var missing []string
	for n := from; n <= to; n++ {
		ar, ok := a.articles[n]
		if !ok {
			return fmt.Errorf("article %d not in the source", n)
		}
		if whole[n] {
			continue
		}
		paras := 0
		for _, l := range ar.body {
			if m := paraRE.FindStringSubmatch(l); m != nil {
				paras++
			}
		}
		if paras == 0 {
			missing = append(missing, fmt.Sprintf("art. %d", n))
		}
		for p := 1; p <= paras; p++ {
			if !covered[[2]int{n, p}] {
				missing = append(missing, fmt.Sprintf("art. %d(%d)", n, p))
			}
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("neither a row nor an exclusion: %s", strings.Join(missing, ", "))
	}
	return nil
}

func genDORA(f fetcher) (string, []byte, error) {
	a, src, err := fetchAct(f, celexDORA)
	if err != nil {
		return "", nil, err
	}
	if err := checkCoverage(a, doraRows, doraExcluded, 5, 30); err != nil {
		return "", nil, err
	}
	rows, err := euRows(a, doraRows, "Règlement (UE) 2022/2554 (DORA), ", eliDORA)
	if err != nil {
		return "", nil, err
	}
	prov := []string{
		"DORA — Règlement (UE) 2022/2554 du 14 décembre 2022 sur la résilience",
		"opérationnelle numérique du secteur financier, JO L 333 du 27.12.2022.",
		"",
		"Source: " + src.URL + " (French XHTML, " + eliDORA + ")",
		"SHA-256 of the source: " + src.SHA256,
		"",
		"One row per paragraph of Art. 5-30 that places an obligation on a financial",
		"entity, plus Art. 45. The description is the paragraph, verbatim. Left out,",
		"because they address the ESAs, the authorities or the Member States:",
	}
	for _, e := range doraExcluded {
		what := fmt.Sprintf("art. %d", e.Art)
		if e.Paras != nil {
			ps := make([]string, len(e.Paras))
			for i, p := range e.Paras {
				ps[i] = fmt.Sprint(p)
			}
			what += "(" + strings.Join(ps, ", ") + ")"
		}
		prov = append(prov, "  - "+what+" — "+e.Reason)
	}
	prov = append(prov, euReuse...)
	file := catalogFile{
		Key:         "dora-2022-2554",
		Name:        "DORA (UE 2022/2554)",
		Version:     "2022",
		Description: fmt.Sprintf("Digital Operational Resilience Act — %d exigences, paragraphe par paragraphe, citant le texte officiel : gestion du risque TIC (art. 5-16), incidents (art. 17-23), tests (art. 24-27), prestataires tiers TIC (art. 28-30) et partage d'informations (art. 45).", len(rows)),
		VarName:     "dora20222554Controls",
		Provenance:  prov,
		Rows:        rows,
	}
	code, err := file.render("dora")
	return "catalog_dora_2022_2554.go", code, err
}

func genNIS2(f fetcher) (string, []byte, error) {
	a, srcA, err := fetchAct(f, celexNIS2)
	if err != nil {
		return "", nil, err
	}
	rows, err := euRows(a, nis2Rows, "Directive (UE) 2022/2555 (NIS2), ", eliNIS2)
	if err != nil {
		return "", nil, err
	}

	ir, srcIR, err := fetchAct(f, celexIR)
	if err != nil {
		return "", nil, err
	}
	sections := parseAnnex(ir.annex)
	if len(sections) == 0 {
		return "", nil, fmt.Errorf("annex of %s: no section found", celexIR)
	}
	for _, s := range sections {
		parent, ref := splitHeading(s.Parent)
		if ref == "" {
			return "", nil, fmt.Errorf("annex %s: heading %q cites no NIS2 measure", s.Num, s.Parent)
		}
		name := s.Heading
		if name == "" {
			name = parent + " (" + s.Num + ")"
		}
		rows = append(rows, row{
			Code:        "RE2690-" + s.Num,
			Name:        name,
			Description: joinText(s.Body),
			Source:      "Règlement d'exécution (UE) 2024/2690, annexe, point " + s.Num + " — mesure de l'" + ref + " NIS2 — " + eliIR,
		})
	}

	file := catalogFile{
		Key:     "nis2-2022-2555",
		Name:    "NIS2 (UE 2022/2555)",
		Version: "2022", // unchanged: the picker matches a tenant's framework by name and version
		Description: fmt.Sprintf("Directive NIS2 — %d exigences citant le texte officiel : gouvernance (art. 20), les dix mesures de l'art. 21(2), les obligations d'information de l'art. 23, et les %d exigences techniques de l'annexe du règlement d'exécution (UE) 2024/2690 (entités visées à son article 1er : DNS, TLD, informatique en nuage, centres de données, CDN, services gérés, services de sécurité gérés, places de marché, moteurs de recherche, réseaux sociaux, services de confiance).",
			len(rows), len(sections)),
		VarName: "nis220222555Controls",
		Provenance: append([]string{
			"NIS2 — Directive (UE) 2022/2555 du 14 décembre 2022, JO L 333 du 27.12.2022,",
			"and the annex of Implementing Regulation (EU) 2024/2690 of 17 October 2024,",
			"JO L, 2024/2690, 18.10.2024.",
			"",
			"Source: " + srcA.URL + " (French XHTML, " + eliNIS2 + ")",
			"SHA-256 of the source: " + srcA.SHA256,
			"Source: " + srcIR.URL + " (French XHTML, " + eliIR + ")",
			"SHA-256 of the source: " + srcIR.SHA256,
			"",
			"Directive rows: one per provision chosen in cmd/catalogsync/spec_nis2.go,",
			"the description being the provision verbatim (a point is preceded by its",
			"paragraph's opening words). Annex rows: one per section \"x.y\" of the",
			"annex, named by its own heading, the description being the section verbatim.",
			"The annex binds only the entities listed in Art. 1 of 2024/2690.",
		}, euReuse...),
		Rows: rows,
	}
	code, err := file.render("nis2")
	return "catalog_nis2_2022_2555.go", code, err
}
