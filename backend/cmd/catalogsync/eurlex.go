// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// cellarURL is the Publications Office's content-negotiated address for an act.
// EUR-Lex serves the same text but challenges automated clients.
func cellarURL(celex string) string {
	return "https://publications.europa.eu/resource/celex/" + celex
}

var cellarHeader = map[string]string{
	"Accept":          "application/xhtml+xml",
	"Accept-Language": "fra",
}

var (
	blockTagRE = regexp.MustCompile(`(?i)<(p|div|td|tr|table|br|li|h\d)\b[^>]*>`)
	anyTagRE   = regexp.MustCompile(`<[^>]+>`)
	spaceRE    = regexp.MustCompile(`[ \t\x{00a0}\x{202f}]+`)
	articleRE  = regexp.MustCompile(`^Article (\d+|premier)$`)
	paraRE     = regexp.MustCompile(`^(\d+)\. `)
	pointRE    = regexp.MustCompile(`^([a-z]{1,2})\)$`)
	// A division heading ends the article before it: its text and the title
	// line after it belong to no article.
	divisionRE = regexp.MustCompile(`^(CHAPITRE|TITRE|Section|SECTION) [IVXLC0-9]+$`)
)

// textLines turns the Official Journal XHTML into the lines a reader sees:
// one per block element, whitespace collapsed, entities decoded.
func textLines(xhtml []byte) []string {
	s := blockTagRE.ReplaceAllString(string(xhtml), "\n")
	s = anyTagRE.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	var out []string
	for _, l := range strings.Split(s, "\n") {
		l = strings.TrimSpace(spaceRE.ReplaceAllString(l, " "))
		if l != "" {
			out = append(out, l)
		}
	}
	return out
}

// act is the enacting terms of one legal act, cut into articles.
type act struct {
	articles map[int]article
	annex    []string // lines after the "ANNEXE" heading, empty if none
}

type article struct {
	title string
	body  []string // lines after the title, up to the next article
}

func parseAct(lines []string) (act, error) {
	a := act{articles: map[int]article{}}
	cur := -1
	for i := 0; i < len(lines); i++ {
		l := lines[i]
		if l == "ANNEXE" {
			a.annex = lines[i+1:]
			break
		}
		m := articleRE.FindStringSubmatch(l)
		if m != nil && i+1 < len(lines) {
			n := 1
			if m[1] != "premier" {
				n, _ = strconv.Atoi(m[1])
			}
			// The first heading wins: an amending provision quoting another
			// act's "Article 6" comes after this act's own Article 6.
			if _, seen := a.articles[n]; !seen {
				cur = n
				a.articles[n] = article{title: lines[i+1]}
				i++
				continue
			}
			cur = -1
			continue
		}
		if divisionRE.MatchString(l) {
			cur = -1
			continue
		}
		if cur > 0 {
			art := a.articles[cur]
			art.body = append(art.body, l)
			a.articles[cur] = art
		}
	}
	if len(a.articles) == 0 {
		return a, fmt.Errorf("no article found")
	}
	return a, nil
}

// paragraph returns the lines of paragraph n ("n. …") of an article.
func (ar article) paragraph(n int) ([]string, error) {
	start, end := -1, len(ar.body)
	for i, l := range ar.body {
		m := paraRE.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		k, _ := strconv.Atoi(m[1])
		if k == n && start < 0 {
			start = i
			continue
		}
		if start >= 0 && k == n+1 {
			end = i
			break
		}
	}
	if start < 0 {
		return nil, fmt.Errorf("paragraph %d not found", n)
	}
	return ar.body[start:end], nil
}

// point returns the chapeau of a block (the lines before its first point) and
// the lines of point p ("p)" on its own line, then its text), up to the next
// point of the same series.
func point(block []string, p string) (chapeau, lines []string, err error) {
	first, start, end := -1, -1, len(block)
	next := nextLetter(p)
	for i, l := range block {
		m := pointRE.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		if first < 0 && m[1] == "a" {
			first = i
		}
		if m[1] == p && start < 0 {
			start = i
			continue
		}
		if start >= 0 && m[1] == next {
			end = i
			break
		}
	}
	// A point's text starts in lower case ("sans retard …", "un rapport …").
	// A line in upper case after the last point is the paragraph's next
	// subparagraph ("Par dérogation …"), not the point's continuation.
	if start >= 0 && end == len(block) {
		for i := start + 2; i < end; i++ {
			if pointRE.MatchString(block[i]) {
				continue
			}
			if r := []rune(block[i]); len(r) > 0 && unicode.IsUpper(r[0]) {
				end = i
				break
			}
		}
	}
	if start < 0 || first < 0 {
		return nil, nil, fmt.Errorf("point %s) not found", p)
	}
	return block[:first], block[start:end], nil
}

func nextLetter(p string) string {
	if len(p) != 1 {
		return ""
	}
	return string(rune(p[0] + 1))
}

// joinText joins lines into one paragraph, keeping point labels with their text.
func joinText(lines []string) string {
	return clean(strings.Join(lines, " "))
}

// euRow is one row a person chose: which provision, under which name.
// Para 0 takes the whole article; Point "" takes the whole paragraph.
type euRow struct {
	Code  string
	Name  string
	Art   int
	Para  int
	Point string
}

// euExclusion records a provision deliberately left out, and why. The test
// on the generated catalogue reads nothing from here; it exists so the choice
// is written down next to the rows, not lost.
type euExclusion struct {
	Art    int
	Paras  []int // nil: the whole article
	Reason string
}

func (s euRow) cite() string {
	c := "art. " + strconv.Itoa(s.Art)
	if s.Para > 0 {
		c += ", paragraphe " + strconv.Itoa(s.Para)
	}
	if s.Point != "" {
		c += ", point " + s.Point + ")"
	}
	return c
}

// euText returns the verbatim text of the provision a row points at.
func euText(a act, s euRow) (string, error) {
	ar, ok := a.articles[s.Art]
	if !ok {
		return "", fmt.Errorf("%s: article %d not found", s.Code, s.Art)
	}
	block := ar.body
	if s.Para > 0 {
		p, err := ar.paragraph(s.Para)
		if err != nil {
			return "", fmt.Errorf("%s: %w", s.Code, err)
		}
		block = p
	}
	if s.Point == "" {
		return joinText(block), nil
	}
	chapeau, pt, err := point(block, s.Point)
	if err != nil {
		return "", fmt.Errorf("%s: %w", s.Code, err)
	}
	return joinText(chapeau) + " […] " + joinText(pt), nil
}

// annexSection is one "x.y." section of an implementing regulation's annex.
type annexSection struct {
	Num     string // "1.1"
	Heading string // its own heading, or "" when the annex gives it none
	Parent  string // the "x." heading it sits under
	Body    []string
}

var (
	annexTopRE = regexp.MustCompile(`^(\d+)\.$`)
	// Some top headings share their number's line: "7. Politiques … [article 21 …]".
	annexTopInlineRE = regexp.MustCompile(`^\d+\. (.+\[article .+\])$`)
	annexSecRE       = regexp.MustCompile(`^(\d+\.\d+)\.$`)
	annexPtRE        = regexp.MustCompile(`^\d+\.\d+\.\d+\.$`)
	// Most sections share their number's line: "1.2. Rôles, responsabilités et pouvoirs".
	annexSecInlineRE = regexp.MustCompile(`^(\d+\.\d+)\. (.+)$`)
)

func parseAnnex(lines []string) []annexSection {
	var out []annexSection
	parent := ""
	for i := 0; i < len(lines); i++ {
		l := lines[i]
		if m := annexTopRE.FindStringSubmatch(l); m != nil && i+1 < len(lines) {
			parent = lines[i+1]
			i++
			continue
		}
		if m := annexTopInlineRE.FindStringSubmatch(l); m != nil {
			parent = m[1]
			continue
		}
		if m := annexSecRE.FindStringSubmatch(l); m != nil {
			s := annexSection{Num: m[1], Parent: parent}
			// A section has a heading when the line after its number is not
			// already its first numbered point or its text.
			if i+2 < len(lines) && annexPtRE.MatchString(lines[i+2]) {
				s.Heading = lines[i+1]
				i++
			}
			out = append(out, s)
			continue
		}
		if m := annexSecInlineRE.FindStringSubmatch(l); m != nil {
			out = append(out, annexSection{Num: m[1], Heading: m[2], Parent: parent})
			continue
		}
		if len(out) > 0 {
			out[len(out)-1].Body = append(out[len(out)-1].Body, l)
		}
	}
	return out
}

// directiveRef pulls "[article 21, paragraphe 2, point a), de la directive …]"
// out of an annex heading, so a row can say which NIS2 measure it implements.
var directiveRefRE = regexp.MustCompile(`\[(article [^\]]*?), de la directive \(UE\) 2022/2555\]`)

func splitHeading(h string) (title, ref string) {
	if m := directiveRefRE.FindStringSubmatchIndex(h); m != nil {
		return strings.TrimSpace(h[:m[0]]), h[m[2]:m[3]]
	}
	return h, ""
}
