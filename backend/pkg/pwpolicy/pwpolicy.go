// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package pwpolicy is the single, server-authoritative password policy.
//
// Every path that accepts a password — registration, password reset, password
// change — runs Assess. The browser runs an equivalent check for live feedback,
// but the browser's verdict is advisory: a client can be modified, so nothing
// here trusts it.
//
// The policy has four independent gates:
//
//  1. length     — at least MinLength runes;
//  2. classes    — at least 3 of {lower, UPPER, digit, symbol};
//  3. strength   — zxcvbn score >= MinScore, with the user's own email and name
//     fed in as dictionary inputs, so "alexdembele2026!" scores as
//     the giveaway it is;
//  4. breach     — not present in the HaveIBeenPwned corpus (k-anonymity).
//
// Feedback is the reason this package exists rather than a pile of ifs at each
// call site. "Password invalid" produces retry loops; "add one more word" tells
// someone what to actually do. Every Reason carries a stable machine Code plus a
// French and English rendering, so the API can stay locale-agnostic and the SPA
// can render in the user's language without a second policy implementation.
package pwpolicy

import (
	"context"
	"fmt"
	"strings"
	"unicode"

	"github.com/trustelem/zxcvbn"
	"github.com/trustelem/zxcvbn/match"
)

const (
	// MinLength is the enforced minimum, in runes.
	//
	// Twelve, not eight: the README promised 12 while the code enforced 8 (audit
	// finding F-05), and the spec settles it at 12. A security tool that accepts
	// eight-character passwords loses the argument at the first customer audit.
	MinLength = 12

	// MinScore is the required zxcvbn strength score, on zxcvbn's 0..4 scale.
	//
	// 3 means "safely unguessable: moderate protection from an offline slow-hash
	// scenario" — roughly 10^8 guesses. Combined with Argon2id at our parameters
	// that is a defensible floor.
	MinScore = 3

	// requiredClasses is how many of {lower, UPPER, digit, symbol} must appear.
	//
	// Three of four rather than all four: mandating every class is a documented
	// way to push people to "Password1!" — the rule gets satisfied by a capital
	// in front and a bang at the end. Three classes at twelve characters raises
	// the floor without inviting that shape.
	requiredClasses = 3
)

// Reason is one policy finding, with a stable code and both renderings.
//
// Code is what tests and the SPA switch on; FR/EN are what a human reads. They
// are phrased as instructions ("add another word"), never as verdicts
// ("invalid"), because the point is to get the next attempt right.
type Reason struct {
	Code string `json:"code"`
	FR   string `json:"fr"`
	EN   string `json:"en"`
}

// Localised returns the rendering for a locale, defaulting to French.
//
// Both renderings still travel in the JSON payload; this is only for the times
// the server has to pick one itself, such as a plain `error` string.
func (r Reason) Localised(locale string) string {
	if locale == "en" {
		return r.EN
	}
	return r.FR
}

// Assessment is the full verdict on a candidate password.
type Assessment struct {
	// OK is the only field that decides acceptance.
	OK bool `json:"ok"`
	// Score is the zxcvbn strength score, 0..4. Drives the strength meter.
	Score int `json:"score"`
	// MinScore is echoed so the meter can draw the pass threshold.
	MinScore int `json:"min_score"`
	// Length in runes, not bytes.
	Length int `json:"length"`
	// Breached reports presence in the HaveIBeenPwned corpus.
	Breached bool `json:"breached"`
	// BreachCount is how many times it appears there.
	BreachCount int `json:"breach_count"`
	// BreachCheckSkipped is true when the corpus could not be consulted. It
	// distinguishes "checked and clean" from "could not check" — see Assess for
	// why that difference is preserved rather than collapsed.
	BreachCheckSkipped bool `json:"breach_check_skipped"`
	// Blocking lists the reasons the password was refused. Empty when OK.
	Blocking []Reason `json:"blocking"`
	// Advisory lists non-blocking improvements. Present even when OK.
	Advisory []Reason `json:"advisory"`
}

// BreachChecker consults a breach corpus. Satisfied by *hibp.Client.
type BreachChecker interface {
	Check(ctx context.Context, password string) (int, error)
}

// Policy evaluates candidate passwords.
type Policy struct {
	breaches BreachChecker
}

// New returns a policy with breach checking disabled.
//
// Breach checking is opt-in via WithBreachChecker because it is the only gate
// that reaches the network, and unit tests must not.
func New() *Policy { return &Policy{} }

// WithBreachChecker enables the HaveIBeenPwned gate.
func (p *Policy) WithBreachChecker(b BreachChecker) *Policy {
	p.breaches = b
	return p
}

// Assess evaluates a password and explains itself.
//
// userInputs are personal strings — email, username, full name, company — fed to
// zxcvbn as dictionary entries so a password built from the user's own identity
// scores as weak. Callers should pass everything they know about the user.
//
// When the breach corpus is unreachable, Assess fails OPEN: it records
// BreachCheckSkipped and lets the password through on the strength of the other
// three gates. Failing closed would mean a HIBP outage blocks every password
// reset in the product — a self-inflicted denial of service on the one flow
// someone locked out of their account needs. The other gates are local and still
// apply.
func (p *Policy) Assess(ctx context.Context, password string, userInputs []string) Assessment {
	a := Assessment{MinScore: MinScore, Length: len([]rune(password))}

	if password == "" {
		a.Blocking = append(a.Blocking, Reason{
			Code: "required",
			FR:   "Saisissez un mot de passe.",
			EN:   "Enter a password.",
		})
		return a
	}

	// --- Gate 1: length -------------------------------------------------------
	if a.Length < MinLength {
		missing := MinLength - a.Length
		a.Blocking = append(a.Blocking, Reason{
			Code: "too_short",
			FR:   fmt.Sprintf("Ajoutez %s : il en faut %d au minimum.", pluralCharsFR(missing), MinLength),
			EN:   fmt.Sprintf("Add %s — %d is the minimum.", pluralCharsEN(missing), MinLength),
		})
	}

	// --- Gate 2: character classes -------------------------------------------
	if missing := missingClasses(password); len(missing) > 0 && countClasses(password) < requiredClasses {
		a.Blocking = append(a.Blocking, Reason{
			Code: "needs_more_classes",
			FR:   "Mélangez au moins trois types de caractères — il vous manque : " + joinFR(missing, "fr") + ".",
			EN:   "Mix at least three character types — still missing: " + joinFR(missing, "en") + ".",
		})
	}

	// --- Gate 3: zxcvbn strength ---------------------------------------------
	// Feed identity strings in as dictionary inputs so a password derived from
	// the account itself cannot pass on raw length.
	res := zxcvbn.PasswordStrength(password, cleanInputs(userInputs))
	a.Score = res.Score

	if res.Score < MinScore {
		a.Blocking = append(a.Blocking, strengthReason(res.Sequence, password))
	} else if res.Score == MinScore {
		a.Advisory = append(a.Advisory, Reason{
			Code: "could_be_stronger",
			FR:   "Correct. Un mot supplémentaire le rendrait nettement plus résistant.",
			EN:   "Good. One more word would make it markedly tougher.",
		})
	}

	// --- Gate 4: breach corpus -----------------------------------------------
	if p.breaches != nil {
		count, err := p.breaches.Check(ctx, password)
		switch {
		case err != nil:
			// Unreachable corpus: record it, do not punish the user. See doc above.
			a.BreachCheckSkipped = true
		case count > 0:
			a.Breached = true
			a.BreachCount = count
			a.Blocking = append(a.Blocking, Reason{
				Code: "breached",
				FR: fmt.Sprintf("Ce mot de passe est apparu dans %s de fuites de données. Choisissez-en un que vous n'avez jamais utilisé ailleurs.",
					occurrencesFR(count)),
				EN: fmt.Sprintf("This password has appeared in %s in known data breaches. Pick one you have never used anywhere else.",
					occurrencesEN(count)),
			})
		}
	}

	a.OK = len(a.Blocking) == 0
	return a
}

// Validate is the boolean form of Assess for call sites that only need to accept
// or refuse. The returned error carries the first blocking reason in English.
func (p *Policy) Validate(ctx context.Context, password string, userInputs []string) error {
	a := p.Assess(ctx, password, userInputs)
	if a.OK {
		return nil
	}
	return fmt.Errorf("%s", a.Blocking[0].EN)
}

// strengthReason turns the zxcvbn match sequence into an instruction.
//
// zxcvbn explains weakness structurally — which spans it recognised and why they
// were cheap. Naming the actual pattern ("dragon is a common word") is what makes
// the advice usable; a generic "too weak" leaves someone guessing.
func strengthReason(sequence []*match.Match, password string) Reason {
	switch dominantPattern(sequence, password) {
	case "dictionary":
		return Reason{
			Code: "weak_dictionary",
			FR:   "Ce mot de passe repose sur un mot courant. Ajoutez un mot supplémentaire, sans rapport avec le premier.",
			EN:   "This leans on a common word. Add another word, unrelated to the first.",
		}
	case "spatial":
		return Reason{
			Code: "weak_spatial",
			FR:   "C'est un tracé sur le clavier (type « azerty »). Composez plutôt une phrase de plusieurs mots.",
			EN:   "That is a keyboard pattern (like \"qwerty\"). Use a phrase of several words instead.",
		}
	case "repeat":
		return Reason{
			Code: "weak_repeat",
			FR:   "Les caractères répétés n'ajoutent presque rien. Remplacez la répétition par un mot supplémentaire.",
			EN:   "Repeated characters add almost nothing. Replace the repetition with another word.",
		}
	case "sequence":
		return Reason{
			Code: "weak_sequence",
			FR:   "Les suites du type « abcd » ou « 1234 » sont testées en premier. Ajoutez un mot supplémentaire.",
			EN:   "Runs like \"abcd\" or \"1234\" are tried first. Add another word.",
		}
	case "date":
		return Reason{
			Code: "weak_date",
			FR:   "Les dates et les années sont vite épuisées. Ajoutez un mot supplémentaire, sans lien avec vous.",
			EN:   "Dates and years are exhausted quickly. Add another word, unrelated to you.",
		}
	default:
		return Reason{
			Code: "weak_score",
			FR:   "Trop facile à deviner. Ajoutez un mot supplémentaire — quatre mots sans rapport valent mieux que des symboles à la place des lettres.",
			EN:   "Too easy to guess. Add another word — four unrelated words beat letter-for-symbol substitutions.",
		}
	}
}

// dominantPattern reports the zxcvbn pattern covering the most of the password.
func dominantPattern(sequence []*match.Match, password string) string {
	best, bestSpan := "", 0
	for _, m := range sequence {
		if m == nil {
			continue
		}
		span := m.J - m.I + 1
		if span > bestSpan {
			best, bestSpan = m.Pattern, span
		}
	}
	// Only claim a pattern explains the password when it covers most of it;
	// otherwise the advice would point at an incidental fragment.
	if bestSpan*2 < len([]rune(password)) {
		return ""
	}
	return best
}

func countClasses(password string) int {
	return 4 - len(missingClasses(password))
}

// missingClasses returns the absent classes, in a stable order so messages and
// tests do not depend on map iteration.
func missingClasses(password string) []string {
	var lower, upper, digit, symbol bool
	for _, r := range password {
		switch {
		case unicode.IsLower(r):
			lower = true
		case unicode.IsUpper(r):
			upper = true
		case unicode.IsDigit(r):
			digit = true
		default:
			symbol = true
		}
	}
	var missing []string
	for _, c := range []struct {
		present bool
		name    string
	}{{lower, "lower"}, {upper, "upper"}, {digit, "digit"}, {symbol, "symbol"}} {
		if !c.present {
			missing = append(missing, c.name)
		}
	}
	return missing
}

var classNames = map[string][2]string{
	"lower":  {"une minuscule", "a lowercase letter"},
	"upper":  {"une majuscule", "an uppercase letter"},
	"digit":  {"un chiffre", "a digit"},
	"symbol": {"un symbole", "a symbol"},
}

func joinFR(classes []string, lang string) string {
	idx := 0
	if lang == "en" {
		idx = 1
	}
	parts := make([]string, 0, len(classes))
	for _, c := range classes {
		parts = append(parts, classNames[c][idx])
	}
	return strings.Join(parts, ", ")
}

// localWeakWords is a dictionary zxcvbn does not ship.
//
// zxcvbn's corpora are English-centric and, naturally, know nothing about this
// product. That leaves two blind spots that a length-and-classes rule waves
// through and that real users reach for first: the product's own name
// ("OpenRisk1234!") and French-language staples ("MotDePasse12!"). Both cleared
// twelve characters and three classes.
//
// These are fed to zxcvbn as dictionary inputs rather than matched literally, so
// its existing l33t, capitalisation and padding analysis applies for free —
// "0p3nR15k!" is caught by the same entry, which a string blocklist would need
// to enumerate by hand.
var localWeakWords = []string{
	// Product and company.
	"openrisk", "opendefender", "grc",
	// French — the primary market, absent from zxcvbn's English corpora.
	"motdepasse", "bonjour", "soleil", "azerty", "chouchou", "coucou",
	"jetaime", "bienvenue", "secret", "changeme", "chatons",
	// English staples that slipped through the old list's decoration handling.
	"welcome", "letmein", "trustno", "changeit", "iloveyou",
}

// cleanInputs drops empties and splits identity strings into their word parts so
// zxcvbn matches "alex.dembele@opendefender.io" on "alex", "dembele" and
// "opendefender" too, not just the whole string. The built-in weak dictionary is
// always appended.
func cleanInputs(inputs []string) []string {
	inputs = append(append([]string{}, inputs...), localWeakWords...)
	return splitInputs(inputs)
}

func splitInputs(inputs []string) []string {
	seen := map[string]struct{}{}
	var out []string
	add := func(s string) {
		s = strings.ToLower(strings.TrimSpace(s))
		if len(s) < 3 {
			return
		}
		if _, dup := seen[s]; dup {
			return
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	for _, in := range inputs {
		add(in)
		for _, part := range strings.FieldsFunc(in, func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsDigit(r)
		}) {
			add(part)
		}
	}
	return out
}

func pluralCharsFR(n int) string {
	if n == 1 {
		return "1 caractère"
	}
	return fmt.Sprintf("%d caractères", n)
}

func pluralCharsEN(n int) string {
	if n == 1 {
		return "1 more character"
	}
	return fmt.Sprintf("%d more characters", n)
}

func occurrencesFR(n int) string {
	if n == 1 {
		return "1 fuite connue"
	}
	return fmt.Sprintf("%d relevés", n)
}

func occurrencesEN(n int) string {
	if n == 1 {
		return "1 record"
	}
	return fmt.Sprintf("%d records", n)
}
