// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"strings"
	"time"
)

// RiskView is a named slice of the register (#913): the tabs of the register
// screen. GET /risks?view= filters the list with it and GET /risks/summary
// counts it, so both read the SAME definition, below. A definition copied into
// a second place is how a tab says 12 and its table shows 11.
type RiskView string

const (
	RiskViewAll       RiskView = "all"
	RiskViewMine      RiskView = "mine"
	RiskViewHot       RiskView = "hot"
	RiskViewReviewDue RiskView = "review_due"
	RiskViewAccepted  RiskView = "accepted"
)

// AllRiskViews lists every view, in the order the tabs are drawn.
var AllRiskViews = []RiskView{RiskViewAll, RiskViewMine, RiskViewHot, RiskViewReviewDue, RiskViewAccepted}

// ReviewDueWindowDays is how far ahead the "review due" view looks. Overdue
// reviews are included: the window is an upper bound, with no lower one.
const ReviewDueWindowDays = 30

// ParseRiskView reads a view name. The empty string means "all" (the parameter
// is optional); anything else that is not a known view is a validation error,
// never a silent fallback to "all", which would show an unfiltered register
// under a tab that claims to be filtered.
func ParseRiskView(raw string) (RiskView, error) {
	v := RiskView(strings.ToLower(strings.TrimSpace(raw)))
	if v == "" {
		return RiskViewAll, nil
	}
	for _, known := range AllRiskViews {
		if v == known {
			return v, nil
		}
	}
	return "", NewValidationError("view must be one of: all, mine, hot, review_due, accepted")
}

// RiskViewPredicate is a view's definition as data. A repository turns it into
// its query dialect; it does not reinterpret it.
type RiskViewPredicate struct {
	// InvolvedOnly keeps what the caller owns, works on or must validate. The
	// caller is always the AUTHENTICATED user, never a parameter.
	InvolvedOnly bool
	// CriticalityIn keeps these criticality levels (lower-case, compared
	// case-insensitively: the column carries two historical vocabularies).
	CriticalityIn []CriticalityLevel
	// StatusIs keeps one status; StatusNot excludes one.
	StatusIs  RiskStatus
	StatusNot RiskStatus
	// NextReviewBy keeps risks whose next_review_at is set and on or before it.
	NextReviewBy *time.Time
}

// IsZero reports a predicate that constrains nothing (the "all" view).
func (p RiskViewPredicate) IsZero() bool {
	return !p.InvolvedOnly && len(p.CriticalityIn) == 0 && p.StatusIs == "" &&
		p.StatusNot == "" && p.NextReviewBy == nil
}

// Predicate returns the view's definition as of now.
//
//	all        no constraint
//	mine       owner, assignee or reviewer is the caller (same as ?mine=true)
//	hot        criticality high or critical, and not closed
//	review_due next_review_at set and within 30 days (overdue included), not closed
//	accepted   status accepted
func (v RiskView) Predicate(now time.Time) RiskViewPredicate {
	switch v {
	case RiskViewMine:
		return RiskViewPredicate{InvolvedOnly: true}
	case RiskViewHot:
		return RiskViewPredicate{
			CriticalityIn: []CriticalityLevel{CriticalityHighNew, CriticalityCriticalNew},
			StatusNot:     RiskClosed,
		}
	case RiskViewReviewDue:
		by := now.UTC().AddDate(0, 0, ReviewDueWindowDays)
		return RiskViewPredicate{NextReviewBy: &by, StatusNot: RiskClosed}
	case RiskViewAccepted:
		return RiskViewPredicate{StatusIs: RiskAccepted}
	default:
		return RiskViewPredicate{}
	}
}
