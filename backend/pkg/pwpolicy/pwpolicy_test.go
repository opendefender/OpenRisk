// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

package pwpolicy

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// stubBreaches is a BreachChecker that answers from a fixed map, so the policy
// tests never touch the network.
type stubBreaches struct {
	counts map[string]int
	err    error
}

func (s stubBreaches) Check(_ context.Context, password string) (int, error) {
	if s.err != nil {
		return 0, s.err
	}
	return s.counts[password], nil
}

func codes(a Assessment) []string {
	out := make([]string, 0, len(a.Blocking))
	for _, r := range a.Blocking {
		out = append(out, r.Code)
	}
	return out
}

func hasCode(a Assessment, code string) bool {
	for _, c := range codes(a) {
		if c == code {
			return true
		}
	}
	return false
}

func TestAssess_AcceptsAStrongPassphrase(t *testing.T) {
	a := New().Assess(context.Background(), "Ancre-Vitrail7-Cobalt", nil)

	if !a.OK {
		t.Fatalf("expected a strong passphrase to pass, blocked by %v", codes(a))
	}
	if a.Score < MinScore {
		t.Errorf("expected score >= %d, got %d", MinScore, a.Score)
	}
}

func TestAssess_RejectsShortPasswordWithCountdown(t *testing.T) {
	// 11 runes: one short of the 12 minimum.
	a := New().Assess(context.Background(), "Ab3!xyzQw9", nil)

	if a.OK {
		t.Fatal("expected a sub-minimum password to be refused")
	}
	if !hasCode(a, "too_short") {
		t.Fatalf("expected too_short, got %v", codes(a))
	}
	// The message must say how many more are needed — that is the actionable part.
	for _, r := range a.Blocking {
		if r.Code != "too_short" {
			continue
		}
		if !strings.Contains(r.EN, "2 more characters") {
			t.Errorf("expected the shortfall spelled out, got %q", r.EN)
		}
		if !strings.Contains(r.FR, "2 caractères") {
			t.Errorf("expected the French shortfall spelled out, got %q", r.FR)
		}
	}
}

func TestAssess_RejectsMissingCharacterClasses(t *testing.T) {
	// Long, but lowercase only — two classes short.
	a := New().Assess(context.Background(), "correcthorsebatterystaple", nil)

	if !hasCode(a, "needs_more_classes") {
		t.Fatalf("expected needs_more_classes, got %v", codes(a))
	}
	for _, r := range a.Blocking {
		if r.Code != "needs_more_classes" {
			continue
		}
		// Naming what is missing is the whole point.
		if !strings.Contains(r.EN, "an uppercase letter") || !strings.Contains(r.EN, "a digit") {
			t.Errorf("expected the missing classes named, got %q", r.EN)
		}
	}
}

func TestAssess_RejectsWeakScoreWithActionableAdvice(t *testing.T) {
	// Long and multi-class, but a dictionary word with predictable decoration —
	// exactly the shape a naive length+classes rule lets through.
	a := New().Assess(context.Background(), "Password1234!", nil)

	if a.OK {
		t.Fatalf("expected a decorated dictionary word to be refused, score %d", a.Score)
	}
	// The advice must instruct, never merely judge.
	joined := strings.ToLower(strings.Join(func() []string {
		var s []string
		for _, r := range a.Blocking {
			s = append(s, r.EN, r.FR)
		}
		return s
	}(), " | "))
	if !strings.Contains(joined, "add another word") && !strings.Contains(joined, "mot supplémentaire") {
		t.Errorf("expected actionable 'add another word' advice, got %q", joined)
	}
	if strings.Contains(joined, "invalid") {
		t.Errorf("feedback must not be a bare verdict, got %q", joined)
	}
}

func TestAssess_UsesIdentityAsDictionaryInput(t *testing.T) {
	// The same 12-character password assessed two ways. Judged in a vacuum it
	// clears the bar; judged against the account it belongs to, it is a giveaway.
	// Feeding identity to zxcvbn is what closes that gap — an attacker targeting
	// this user starts from exactly these words.
	const candidate = "Dembele-Alex"

	anonymous := New().Assess(context.Background(), candidate, nil)
	if !anonymous.OK {
		t.Fatalf("precondition: expected %q to pass without identity hints, blocked by %v",
			candidate, codes(anonymous))
	}

	withIdentity := New().Assess(context.Background(), candidate,
		[]string{"alex.dembele@opendefender.io", "Alex Dembele", "OpenDefender"})

	if withIdentity.OK {
		t.Errorf("expected a password built from the account's own identity to be refused, score %d",
			withIdentity.Score)
	}
	if withIdentity.Score >= anonymous.Score {
		t.Errorf("identity inputs must lower the score: %d with identity vs %d without",
			withIdentity.Score, anonymous.Score)
	}
}

func TestAssess_RejectsProductAndFrenchWeakWords(t *testing.T) {
	// zxcvbn's corpora are English-centric and know nothing about this product,
	// so these all cleared twelve characters and three classes on their own.
	// Feeding localWeakWords in as a dictionary is what catches them — including
	// the l33t and padded variants, which a literal blocklist would have to
	// enumerate by hand.
	for _, candidate := range []string{
		"OpenRisk1234!",
		"MotDePasse12!",
		"ChangeMe1234!",
		"0p3nR15k1234!",
		"Bienvenue123!",
	} {
		a := New().Assess(context.Background(), candidate, nil)
		if a.OK {
			t.Errorf("expected %q to be refused (score %d)", candidate, a.Score)
		}
	}
}

func TestCleanInputs_SplitsIdentityIntoWordParts(t *testing.T) {
	// An email is one string but several dictionary words. Splitting it is what
	// lets "dembele" match when the caller only passed the full address.
	got := cleanInputs([]string{"alex.dembele@opendefender.io", "", "ab"})

	want := map[string]bool{"alex": false, "dembele": false, "opendefender": false}
	for _, g := range got {
		if _, tracked := want[g]; tracked {
			want[g] = true
		}
	}
	for word, found := range want {
		if !found {
			t.Errorf("expected %q among the split inputs, got %v", word, got)
		}
	}
	// Fragments shorter than 3 runes are noise, not dictionary entries.
	for _, g := range got {
		if len(g) < 3 {
			t.Errorf("expected sub-3-rune fragments dropped, got %q", g)
		}
	}
}

func TestAssess_RejectsBreachedPassword(t *testing.T) {
	// Strong by every local measure, but known to the breach corpus.
	const candidate = "Ancre-Vitrail7-Cobalt"
	p := New().WithBreachChecker(stubBreaches{counts: map[string]int{candidate: 4213}})

	a := p.Assess(context.Background(), candidate, nil)

	if a.OK {
		t.Fatal("expected a breached password to be refused")
	}
	if !a.Breached || a.BreachCount != 4213 {
		t.Errorf("expected breached=true count=4213, got %v/%d", a.Breached, a.BreachCount)
	}
	if !hasCode(a, "breached") {
		t.Fatalf("expected breached code, got %v", codes(a))
	}
	for _, r := range a.Blocking {
		if r.Code == "breached" && !strings.Contains(r.EN, "4213") {
			t.Errorf("expected the occurrence count surfaced, got %q", r.EN)
		}
	}
}

func TestAssess_FailsOpenWhenBreachCorpusUnreachable(t *testing.T) {
	// A HIBP outage must not block password resets — the local gates still apply.
	p := New().WithBreachChecker(stubBreaches{err: errors.New("dial tcp: no route to host")})

	a := p.Assess(context.Background(), "Ancre-Vitrail7-Cobalt", nil)

	if !a.OK {
		t.Fatalf("expected fail-open on corpus outage, blocked by %v", codes(a))
	}
	if !a.BreachCheckSkipped {
		t.Error("expected BreachCheckSkipped to record that the corpus was not consulted")
	}
	if a.Breached {
		t.Error("an unreachable corpus must never be reported as a breach hit")
	}
}

func TestAssess_CleanCorpusIsDistinctFromSkipped(t *testing.T) {
	p := New().WithBreachChecker(stubBreaches{counts: map[string]int{}})

	a := p.Assess(context.Background(), "Ancre-Vitrail7-Cobalt", nil)

	if !a.OK {
		t.Fatalf("expected pass, blocked by %v", codes(a))
	}
	if a.BreachCheckSkipped {
		t.Error("a successful lookup must not be reported as skipped")
	}
}

func TestAssess_EmptyPasswordIsRequired(t *testing.T) {
	a := New().Assess(context.Background(), "", nil)

	if a.OK {
		t.Fatal("expected empty password to be refused")
	}
	if !hasCode(a, "required") {
		t.Fatalf("expected required, got %v", codes(a))
	}
}

func TestAssess_EveryReasonIsBilingual(t *testing.T) {
	// The API is locale-agnostic: it ships both renderings and lets the SPA pick.
	// A reason missing either one would render blank for half the users.
	samples := []string{"", "short", "correcthorsebatterystaple", "Password1234!", "aaaaaaaaaaaaaa"}
	p := New().WithBreachChecker(stubBreaches{counts: map[string]int{"Ancre-Vitrail7-Cobalt": 9}})
	samples = append(samples, "Ancre-Vitrail7-Cobalt")

	for _, s := range samples {
		a := p.Assess(context.Background(), s, nil)
		for _, r := range append(append([]Reason{}, a.Blocking...), a.Advisory...) {
			if strings.TrimSpace(r.FR) == "" || strings.TrimSpace(r.EN) == "" {
				t.Errorf("password %q: reason %q missing a rendering (fr=%q en=%q)", s, r.Code, r.FR, r.EN)
			}
			if strings.TrimSpace(r.Code) == "" {
				t.Errorf("password %q: reason without a machine code", s)
			}
		}
	}
}

func TestValidate_MirrorsAssess(t *testing.T) {
	p := New()

	if err := p.Validate(context.Background(), "Ancre-Vitrail7-Cobalt", nil); err != nil {
		t.Errorf("expected strong passphrase to validate, got %v", err)
	}
	if err := p.Validate(context.Background(), "short", nil); err == nil {
		t.Error("expected a short password to fail validation")
	}
}

func TestMinimums_MatchTheDecidedPolicy(t *testing.T) {
	// These are contractual: the spec settles the minimum at 12 and zxcvbn >= 3.
	// Pinning them here means a future edit has to be deliberate.
	if MinLength != 12 {
		t.Errorf("MinLength must be 12, got %d", MinLength)
	}
	if MinScore != 3 {
		t.Errorf("MinScore must be 3, got %d", MinScore)
	}
}
