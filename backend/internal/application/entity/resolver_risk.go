// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package entity

import (
	"context"
	"fmt"
	"strconv"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// RiskReader is the narrow port over the risk repository.
type RiskReader interface {
	GetByID(ctx context.Context, id uuid.UUID, tenantID uuid.UUID) (*domain.Risk, error)
}

// RiskResolver is the reference resolver — the risk register is the product's
// core, so this is the one the other seven are shaped after.
type RiskResolver struct {
	risks     RiskReader
	relations RelationReader
}

func NewRiskResolver(risks RiskReader, relations RelationReader) *RiskResolver {
	return &RiskResolver{risks: risks, relations: relations}
}

func (r *RiskResolver) load(ctx context.Context, c Caller, id string) (*domain.Risk, uuid.UUID, error) {
	rid, err := uuid.Parse(id)
	if err != nil {
		return nil, uuid.Nil, domain.NewNotFoundError("risk", id)
	}
	if r.risks == nil {
		return nil, uuid.Nil, domain.NewNotFoundError("risk", id)
	}
	risk, err := r.risks.GetByID(ctx, rid, c.TenantID)
	if err != nil {
		return nil, uuid.Nil, err
	}
	if risk == nil {
		return nil, uuid.Nil, domain.NewNotFoundError("risk", id)
	}
	return risk, rid, nil
}

func (r *RiskResolver) Summary(ctx context.Context, c Caller, id string) (*Summary, error) {
	risk, _, err := r.load(ctx, c, id)
	if err != nil {
		return nil, err
	}

	name := risk.Name
	if name == "" {
		name = risk.Title
	}

	s := &Summary{
		ID:        risk.ID.String(),
		Title:     name,
		Subtitle:  risk.Description,
		Status:    riskStatusChip(string(risk.Status)),
		Severity:  severityChip(string(risk.Criticality)),
		CreatedAt: timePtr(risk.CreatedAt),
		UpdatedAt: timePtr(risk.UpdatedAt),
	}

	// The headline score is the Score Engine's P × I × AC product, read from the
	// column the engine wrote. A risk that has never been through the engine has
	// score 0 with criticality unset — and 0 is a real score for a risk that
	// genuinely cannot happen, so "never scored" is told apart by the criticality
	// band rather than by the number.
	if risk.Criticality != "" {
		s.Score = Score{
			Available: true,
			Key:       "risk_score",
			Label:     "Risk score",
			Value:     risk.Score,
			Max:       30.0, // 1.0 probability × 10.0 impact × 3.0 asset criticality
			Tone:      severityToneOf(string(risk.Criticality)),
			Basis:     "Score Engine — probability × impact × asset criticality",
		}
	} else {
		s.Score = unavailableScore("risk_score", "Risk score", "this risk has not been scored yet")
	}

	s.Owner = actorFor(risk.OwnerID)
	if s.Owner == nil && risk.Owner != "" {
		s.Owner = &Actor{Label: risk.Owner}
	}

	s.Fields = appendField(s.Fields, field("lifecycle_state", "Lifecycle", title(string(risk.LifecycleState)), FieldBadge))
	s.Fields = append(s.Fields, Field{
		Key: "probability", Label: "Probability", Kind: FieldNumber,
		Value: strconv.FormatFloat(risk.Probability, 'f', -1, 64),
	})
	s.Fields = append(s.Fields, Field{
		Key: "impact", Label: "Impact", Kind: FieldNumber,
		Value: strconv.FormatFloat(risk.Impact, 'f', -1, 64),
	})
	s.Fields = appendField(s.Fields, field("source", "Source", title(string(risk.Source)), FieldBadge))
	if risk.Category != nil {
		s.Fields = appendField(s.Fields, field("category", "Category", risk.Category.Name, FieldBadge))
	}
	if len(risk.Tags) > 0 {
		s.Fields = append(s.Fields, Field{Key: "tags", Label: "Tags", Kind: FieldTagList, Values: risk.Tags})
	}
	if risk.AssigneeID != nil {
		s.Fields = appendField(s.Fields, field("assignee", "Assignee", risk.AssigneeID.String(), FieldUser))
	}
	if risk.ReviewerID != nil {
		s.Fields = appendField(s.Fields, field("reviewer", "Reviewer", risk.ReviewerID.String(), FieldUser))
	}

	// The smart score is a SECOND, additional engine (spec §8). It is shown only
	// when it has actually been computed and cached — never recomputed here,
	// because recomputing on a drawer open would turn a read into eight module
	// queries and would disagree with the cached breakdown the radar renders.
	if risk.SmartComputedAt != nil && risk.SmartLevel != "" {
		s.Fields = append(s.Fields, Field{
			Key: "smart_score", Label: "Smart score", Kind: FieldNumber,
			Value: strconv.FormatFloat(risk.SmartScore, 'f', 1, 64),
			Tone:  severityToneOf(string(risk.SmartLevel)),
		})
	}

	// Financial exposure comes from the CRQ engine's persisted inputs. ALE is
	// shown only when both of its factors are present: SLE alone is a single
	// loss, not an annualised one, and labelling it ALE would overstate by
	// however many times a year the event was expected.
	sle, aro := deref(risk.SLEXAF), deref(risk.ARO)
	switch {
	case sle > 0 && aro > 0:
		s.Fields = append(s.Fields, Field{
			Key: "ale", Label: "Annualised loss expectancy", Kind: FieldMoney,
			Value: fmt.Sprintf("%.0f XAF", sle*aro),
		})
	case sle > 0:
		s.Fields = append(s.Fields, Field{
			Key: "sle", Label: "Single loss expectancy", Kind: FieldMoney,
			Value: fmt.Sprintf("%.0f XAF", sle),
		})
	}

	return s, nil
}

func (r *RiskResolver) Relations(ctx context.Context, c Caller, id string) ([]RelationGroup, error) {
	_, rid, err := r.load(ctx, c, id)
	if err != nil {
		return nil, err
	}
	if r.relations == nil {
		return []RelationGroup{}, nil
	}

	assets, assetTotal, assetErr := r.relations.AssetsForRisk(ctx, c.TenantID, rid, relationCap)
	ctrls, ctrlTotal, ctrlErr := r.relations.ControlsForRisk(ctx, c.TenantID, rid, relationCap)
	vulns, vulnTotal, vulnErr := r.relations.VulnerabilitiesForRisk(ctx, c.TenantID, rid, relationCap)
	incs, incTotal, incErr := r.relations.IncidentsForRisk(ctx, c.TenantID, rid, relationCap)
	mits, mitTotal, mitErr := r.relations.MitigationsForRisk(ctx, c.TenantID, rid, relationCap)

	return []RelationGroup{
		group("assets", "Assets", TypeAsset, assets, assetTotal, assetErr, assetChips),
		group("controls", "Controls", TypeControl, ctrls, ctrlTotal, ctrlErr, controlChips),
		group("findings", "Findings", TypeFinding, vulns, vulnTotal, vulnErr, vulnChips),
		group("incidents", "Incidents", TypeIncident, incs, incTotal, incErr, incidentChips),
		// Mitigations have no drawer of their own — they are a workflow board,
		// not an inspectable object — so the group points at the risk itself and
		// the client links each row to the mitigation board. It is listed because
		// "what is being done about this" is the first question a risk drawer
		// must answer; it is not given a fake target type to look uniform.
		mitigationGroup(mits, mitTotal, mitErr),
	}, nil
}

// mitigationGroup renders the treatment plans without pretending they are a
// drawer-addressable type.
func mitigationGroup(rows []RelationRow, total int, err error) RelationGroup {
	g := RelationGroup{GroupKey: "mitigations", Label: "Mitigations", TargetType: TypeRisk, Items: []Relation{}}
	if err != nil {
		g.Error = "could not be loaded"
		return g
	}
	for _, row := range rows {
		status, sev := mitigationChips(row)
		g.Items = append(g.Items, Relation{
			Ref:      Ref{Type: TypeRisk, ID: row.ID},
			Title:    row.Title,
			Subtitle: row.Subtitle,
			Status:   status,
			Severity: sev,
			URL:      "/mitigations/" + row.ID,
		})
	}
	g.Total = total
	g.Truncated = total > len(rows)
	return g
}

func (r *RiskResolver) Actions(ctx context.Context, c Caller, id string) []Action {
	base := "/api/v1/risks/" + id
	out := []Action{}
	if c.Can("risks:update") {
		out = append(out,
			Action{Key: "edit", Label: "Edit", Kind: ActionPrimary,
				Method: "PATCH", Path: base, Permission: "risks:update"},
			Action{Key: "transition", Label: "Change lifecycle state", Kind: ActionSecondary,
				Method: "POST", Path: base + "/transition", Permission: "risks:update"},
			Action{Key: "review", Label: "Mark reviewed", Kind: ActionSecondary,
				Method: "POST", Path: base + "/review", Permission: "risks:update"},
		)
	}
	if c.Can("mitigations:create") {
		out = append(out, Action{
			Key: "create_mitigation", Label: "Create mitigation", Kind: ActionSecondary,
			Method: "POST", Path: base + "/mitigations", Permission: "mitigations:create",
		})
	}
	if c.Can("risks:delete") {
		out = append(out, Action{
			Key: "delete", Label: "Delete", Kind: ActionDanger,
			Method: "DELETE", Path: base, Permission: "risks:delete",
		})
	}
	return out
}

// deref reads an optional numeric column. A nil pointer means the user never
// supplied the figure, which is not the same as supplying zero — and the caller
// above only renders a money field when the figure exists.
func deref(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}
