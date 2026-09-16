// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package tprm

import (
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/pkg/vendorscore"
)

// scoreAssessment runs pkg/vendorscore over an assessment's snapshotted items
// (#671, ADR 0004 D5). It is the one place a vendor score is computed.
//
// It writes nothing. Its result is stored by the submission on the assessment
// itself, and it reaches no risk, no SmartScore input and no asset criticality
// (D-044): the vendor score is shown beside the risks it relates to, never
// multiplied into them.
func scoreAssessment(a *domain.VendorAssessment) domain.VendorAssessmentScoring {
	items := make([]vendorscore.Item, 0, len(a.Items))
	for _, it := range a.Items {
		in := vendorscore.Item{
			ID:       it.ID,
			Scorable: it.AnswerType == domain.VendorAnswerChoice,
			Weight:   it.Weight,
			NA:       it.AnswerNA,
		}
		// The points come from the SNAPSHOTTED options, never from the template:
		// an edit made after sending cannot move a vendor's score.
		if !it.AnswerNA && it.AnswerValue != nil {
			for _, o := range it.Options {
				if o.Value == *it.AnswerValue {
					p := o.Points
					in.Points = &p
					break
				}
			}
		}
		items = append(items, in)
	}

	r := vendorscore.Compute(items)
	out := domain.VendorAssessmentScoring{
		Score:     r.Score,
		Version:   r.Version,
		Breakdown: make(domain.VendorScoreBreakdown, 0, len(r.Breakdown)),
	}
	if r.Tier != nil {
		tier := string(*r.Tier)
		out.Tier = &tier
	}
	for _, c := range r.Breakdown {
		out.Breakdown = append(out.Breakdown, domain.VendorScoreContribution{
			ItemID:       c.ItemID,
			Weight:       c.Weight,
			Points:       c.Points,
			NA:           c.NA,
			Contribution: c.Contribution,
		})
	}
	return out
}
