// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package entity

import (
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// =============================================================================
// The organisation's journal (#905)
// =============================================================================
//
// The Activity page reads the tenant feed as a journal: "Fatou a qualifié en P1
// INC-…". That needs three things the raw audit row does not say plainly:
//
//   - which business domain an event belongs to, so the page can filter by
//     Risques / Incidents / Vulnérabilités / Mitigations / Preuves / Conformité /
//     Rapports;
//   - the object's name, so the sentence can end on it;
//   - one entry per thing that happened. Two writers record a mutation (the
//     HTTP middleware and the model hook), so the same change arrives twice.
//
// Visibility keeps the feed's rule: an event is shown to someone who may read
// what it is about. Types with a drawer use the drawer's permission; mitigations,
// reports and frameworks use the permission that guards their own pages;
// governance records stay behind the audit permission. Technical records
// (onboarding celebrations, notification reads, unnamed routes) are not part of
// the journal for anyone; they remain in the audit trail.

// Journal domains, the values of the `domain` filter and of TimelineEvent.Domain.
const (
	DomainRisk          = "risk"
	DomainIncident      = "incident"
	DomainVulnerability = "vulnerability"
	DomainMitigation    = "mitigation"
	DomainEvidence      = "evidence"
	DomainCompliance    = "compliance"
	DomainReport        = "report"
	DomainAsset         = "asset"
	DomainGovernance    = "governance"
)

// journalClass says how one audit entity type appears in the journal.
type journalClass struct {
	domain string
	// drawer is set when the type opens a drawer: its permission and deep link
	// come from the registry.
	drawer Type
	// perm and link serve types without a drawer.
	perm string
	link func(id string) string
}

func listLink(path string) func(string) string { return func(string) string { return path } }

// journalClasses maps every audit entity type the journal shows. A type absent
// from this map is not part of the journal.
var journalClasses = map[string]journalClass{
	"risk":                 {domain: DomainRisk, drawer: TypeRisk},
	"incident":             {domain: DomainIncident, drawer: TypeIncident},
	"incident_post_mortem": {domain: DomainIncident, perm: "incidents:read", link: listLink("/incidents")},
	"vulnerability":        {domain: DomainVulnerability, drawer: TypeVulnerability},
	"mitigation":           {domain: DomainMitigation, perm: "mitigations:read", link: listLink("/mitigations")},
	"evidence":             {domain: DomainEvidence, drawer: TypeEvidence},
	"control_evidence":     {domain: DomainEvidence, drawer: TypeEvidence},
	"compliance_control":   {domain: DomainCompliance, drawer: TypeControl},
	"control":              {domain: DomainCompliance, drawer: TypeControl},
	"compliance_framework": {
		domain: DomainCompliance, perm: "compliance:controls:read",
		link: func(id string) string { return "/compliance/" + id },
	},
	"report":     {domain: DomainReport, perm: "compliance:controls:read", link: listLink("/reports")},
	"report_job": {domain: DomainReport, perm: "compliance:controls:read", link: listLink("/reports")},
	"board":      {domain: DomainReport, perm: "reports:board:read", link: listLink("/reports/board")},
	"asset":      {domain: DomainAsset, drawer: TypeAsset},
	// Governance: who may approve, delegate or automate. Audit permission only,
	// as the feed has always done for entities with no drawer.
	"approval_request":          {domain: DomainGovernance, perm: AuditPermission, link: listLink("/governance")},
	"approval_workflow":         {domain: DomainGovernance, perm: AuditPermission, link: listLink("/governance")},
	"automation_rule":           {domain: DomainGovernance, perm: AuditPermission, link: listLink("/automation")},
	"automation_channel_config": {domain: DomainGovernance, perm: AuditPermission, link: listLink("/automation")},
	"delegation":                {domain: DomainGovernance, perm: AuditPermission, link: listLink("/governance")},
	"organization_member":       {domain: DomainGovernance, perm: AuditPermission, link: listLink("/settings/members")},
	"invitation":                {domain: DomainGovernance, perm: AuditPermission, link: listLink("/settings/members")},
	"mfa_policy":                {domain: DomainGovernance, perm: AuditPermission, link: listLink("/settings")},
}

// journalTypesFor returns the audit entity types of one domain, for the query.
// Unknown domain → nil, ok false.
func journalTypesFor(d string) ([]string, bool) {
	var out []string
	for et, cls := range journalClasses {
		if cls.domain == d {
			out = append(out, et)
		}
	}
	return out, len(out) > 0
}

// visible reports whether the caller may see an event of this class.
func (cls journalClass) visible(c Caller) bool {
	if CanReadAudit(c) {
		return true
	}
	if cls.drawer != "" {
		return c.CanRead(cls.drawer)
	}
	return cls.perm != "" && c.Can(cls.perm)
}

// journalEvent projects an audit row into a journal entry, or false when the
// row is not part of the journal.
func journalEvent(e domain.AuditEvent) (TimelineEvent, journalClass, bool) {
	cls, ok := journalClasses[e.EntityType]
	if !ok {
		return TimelineEvent{}, cls, false
	}
	ev := auditToTimeline(e)
	ev.Domain = cls.domain
	ev.Object = objectLabel(e)
	if cls.drawer != "" {
		ev.Target = Ref{Type: cls.drawer, ID: e.EntityID}
		ev.TargetURL = DeepLink(cls.drawer, e.EntityID)
	} else if cls.link != nil {
		ev.TargetURL = cls.link(e.EntityID)
	}
	return ev, cls, true
}

var (
	affectedSuffix = regexp.MustCompile(`\s*\(\d+ records? affected\)\s*$`)
	quoted         = regexp.MustCompile(`"([^"]+)"`)
	forApproval    = regexp.MustCompile(`for approval: (.+)$`)
	bareNumber     = regexp.MustCompile(`^\d+$`)
)

// objectLabel names the object of an entry: the record's own title or name when
// the row carries it, else the name the summary quotes. Only that one field is
// read from the snapshot; the reader is already cleared for the type, and the
// summary itself states the same name.
func objectLabel(e domain.AuditEvent) string {
	for _, snap := range []domain.JSONMap{e.After, e.Before} {
		for _, k := range []string{"title", "name", "file_name", "period_label"} {
			if v, ok := snap[k].(string); ok && strings.TrimSpace(v) != "" {
				return strings.TrimSpace(v)
			}
		}
	}
	s := affectedSuffix.ReplaceAllString(strings.TrimSpace(e.Summary), "")
	// 'approve step "Comité" of "Prolongation…"': the object is the last name
	// quoted, not the step.
	if all := quoted.FindAllStringSubmatch(s, -1); len(all) > 0 {
		return all[len(all)-1][1]
	}
	if m := forApproval.FindStringSubmatch(s); m != nil {
		return strings.TrimSpace(m[1])
	}
	// "<action> <entity_type> <label>" is how both writers phrase it.
	prefix := string(e.Action) + " " + e.EntityType + " "
	if strings.HasPrefix(s, prefix) {
		label := strings.TrimSpace(strings.TrimPrefix(s, prefix))
		if _, err := uuid.Parse(label); err == nil || bareNumber.MatchString(label) {
			return ""
		}
		return label
	}
	return ""
}

// twinWindow is how far apart the two writers' rows for one mutation may be.
const twinWindow = 5 * time.Second

// dropTwins removes the HTTP middleware's row when the model hook recorded the
// same mutation: same type, entity and action, within a few seconds. The model
// row is kept because it carries the record's fields; the HTTP row knows who
// acted, so its actor moves onto the model row when that one has none. Input
// order is kept.
func dropTwins(rows []domain.AuditEvent) []domain.AuditEvent {
	type key struct{ et, id, action string }
	rows = append([]domain.AuditEvent(nil), rows...)
	model := map[key][]int{}
	for i, r := range rows {
		if r.Source == "gorm" {
			k := key{r.EntityType, r.EntityID, string(r.Action)}
			model[k] = append(model[k], i)
		}
	}
	out := make([]domain.AuditEvent, 0, len(rows))
	drop := make([]bool, len(rows))
	for i, r := range rows {
		if r.Source != "http" {
			continue
		}
		for _, j := range model[key{r.EntityType, r.EntityID, string(r.Action)}] {
			if d := r.CreatedAt.Sub(rows[j].CreatedAt); d < twinWindow && d > -twinWindow {
				drop[i] = true
				if rows[j].ActorID == nil && r.ActorID != nil {
					rows[j].ActorID = r.ActorID
					rows[j].ActorEmail = r.ActorEmail
					rows[j].ActorType = r.ActorType
					rows[j].ActorLabel = r.ActorLabel
				}
				break
			}
		}
	}
	for i, r := range rows {
		if !drop[i] {
			out = append(out, r)
		}
	}
	return out
}
