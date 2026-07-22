// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial
// This file is part of the OpenRisk Enterprise Edition and is NOT covered by the
// AGPL; it is licensed under the OpenRisk Commercial License (see LICENSE.commercial).

package automation

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/rs/zerolog"
)

// Engine is the SOAR orchestrator. On a trigger it loads the tenant's enabled
// rules for that trigger, evaluates each rule's conditions against the trigger
// context, and — for matching rules — runs the ordered action chain, recording
// an AutomationExecution for audit. Action ports are optional; a missing port
// degrades its action to a "skipped" step, never a hard failure.
type Engine struct {
	rules      domain.AutomationRuleRepository
	executions domain.AutomationExecutionRepository
	slas       domain.SLATrackerRepository
	logger     zerolog.Logger

	notifier   Notifier
	ticketer   Ticketer
	riskCreate RiskCreator
	assigner   RiskAssigner
	scanner    AssetScanner
	resolver   RiskResolver
}

// NewEngine builds the engine with the persistence ports. Action ports are
// attached via the With* setters.
func NewEngine(
	rules domain.AutomationRuleRepository,
	executions domain.AutomationExecutionRepository,
	slas domain.SLATrackerRepository,
	logger zerolog.Logger,
) *Engine {
	return &Engine{rules: rules, executions: executions, slas: slas, logger: logger}
}

func (e *Engine) WithNotifier(n Notifier) *Engine         { e.notifier = n; return e }
func (e *Engine) WithTicketer(t Ticketer) *Engine         { e.ticketer = t; return e }
func (e *Engine) WithRiskCreator(r RiskCreator) *Engine   { e.riskCreate = r; return e }
func (e *Engine) WithRiskAssigner(a RiskAssigner) *Engine { e.assigner = a; return e }
func (e *Engine) WithAssetScanner(s AssetScanner) *Engine { e.scanner = s; return e }
func (e *Engine) WithRiskResolver(r RiskResolver) *Engine { e.resolver = r; return e }

// HandleTrigger evaluates every enabled rule for the trigger and runs the
// matching ones. It never returns an error to its caller (a background worker);
// failures are recorded on the execution and logged.
func (e *Engine) HandleTrigger(ctx context.Context, trigger domain.AutomationTrigger, tc TriggerContext) {
	if tc.TenantID == uuid.Nil {
		return
	}
	rules, err := e.rules.ListEnabledByTrigger(ctx, tc.TenantID, trigger)
	if err != nil {
		e.logger.Warn().Err(err).Str("trigger", string(trigger)).Msg("automation: could not list rules")
		return
	}
	for i := range rules {
		rule := rules[i]
		if ok, reason := matchConditions(rule.Conditions, tc); !ok {
			e.logger.Debug().Str("rule", rule.Name).Str("reason", reason).Msg("automation: rule skipped")
			continue
		}
		e.runRule(ctx, &rule, tc)
	}
}

// RunRuleByID runs one rule immediately against a supplied context, bypassing
// the enabled/trigger filters. Used by the "test / dry-run" endpoint.
func (e *Engine) RunRuleByID(ctx context.Context, ruleID, tenantID uuid.UUID, tc TriggerContext) (*domain.AutomationExecution, error) {
	rule, err := e.rules.GetByID(ctx, ruleID, tenantID)
	if err != nil {
		return nil, err
	}
	tc.TenantID = tenantID
	return e.runRule(ctx, rule, tc), nil
}

func (e *Engine) runRule(ctx context.Context, rule *domain.AutomationRule, tc TriggerContext) *domain.AutomationExecution {
	now := time.Now()
	exec := &domain.AutomationExecution{
		ID:         uuid.New(),
		TenantID:   rule.TenantID,
		RuleID:     rule.ID,
		RuleName:   rule.Name,
		Trigger:    rule.Trigger,
		TriggerRef: tc.Ref,
		Subject:    tc.Subject,
		Severity:   tc.Severity,
		Status:     domain.ExecutionRunning,
		Steps:      domain.ExecutionStepList{},
		StartedAt:  now,
		CreatedAt:  now,
	}
	if err := e.executions.Create(ctx, exec); err != nil {
		e.logger.Warn().Err(err).Str("rule", rule.Name).Msg("automation: could not create execution record")
		// Continue running actions even if we can't persist the record.
	}

	failures := 0
	for _, action := range rule.Actions {
		step := e.runAction(ctx, rule, action, &tc, exec)
		exec.Steps = append(exec.Steps, step)
		if step.Status == "failed" {
			failures++
		}
	}

	switch {
	case failures == 0:
		exec.Status = domain.ExecutionSuccess
	case failures < len(rule.Actions):
		exec.Status = domain.ExecutionPartial
	default:
		exec.Status = domain.ExecutionFailed
	}
	fin := time.Now()
	exec.FinishedAt = &fin
	if err := e.executions.Update(ctx, exec); err != nil {
		e.logger.Warn().Err(err).Msg("automation: could not finalise execution record")
	}
	if err := e.rules.RecordTriggered(ctx, rule.ID, rule.TenantID, now); err != nil {
		e.logger.Debug().Err(err).Msg("automation: could not bump rule trigger count")
	}
	e.logger.Info().
		Str("rule", rule.Name).
		Str("status", string(exec.Status)).
		Int("actions", len(rule.Actions)).
		Str("ref", tc.Ref).
		Msg("automation: rule executed")
	return exec
}

func step(action domain.AutomationActionType, status, detail string) domain.ExecutionStep {
	return domain.ExecutionStep{Action: string(action), Status: status, Detail: detail, At: time.Now()}
}

func (e *Engine) runAction(ctx context.Context, rule *domain.AutomationRule, action domain.AutomationAction, tc *TriggerContext, exec *domain.AutomationExecution) domain.ExecutionStep {
	switch action.Type {
	case domain.ActionScanAsset:
		return e.doScan(ctx, tc)
	case domain.ActionCreateRisk:
		return e.doCreateRisk(ctx, tc)
	case domain.ActionAssignOwner:
		return e.doAssign(ctx, action, tc)
	case domain.ActionCreateTicket:
		return e.doTicket(ctx, action, tc)
	case domain.ActionNotify:
		return e.doNotify(ctx, action, tc)
	case domain.ActionStartSLA:
		return e.doStartSLA(ctx, rule, tc, exec.ID)
	case domain.ActionResolveRisk:
		return e.doResolveRisk(ctx, tc)
	case domain.ActionCloseTicket:
		// Closing tickets is handled by the ticketer if wired; otherwise a no-op.
		return step(action.Type, "skipped", "close_ticket not wired")
	default:
		return step(action.Type, "failed", "unknown action")
	}
}

func (e *Engine) doScan(ctx context.Context, tc *TriggerContext) domain.ExecutionStep {
	if e.scanner == nil {
		return step(domain.ActionScanAsset, "skipped", "no scanner configured")
	}
	if tc.AssetID == nil {
		return step(domain.ActionScanAsset, "skipped", "no asset to scan")
	}
	ref, err := e.scanner.ScanAsset(ctx, tc.TenantID, *tc.AssetID)
	if err != nil {
		return step(domain.ActionScanAsset, "failed", err.Error())
	}
	return step(domain.ActionScanAsset, "success", "scan triggered: "+ref)
}

func (e *Engine) doCreateRisk(ctx context.Context, tc *TriggerContext) domain.ExecutionStep {
	if tc.RiskID != nil {
		return step(domain.ActionCreateRisk, "skipped", "risk already linked")
	}
	if e.riskCreate == nil {
		return step(domain.ActionCreateRisk, "skipped", "no risk creator configured")
	}
	res, err := e.riskCreate.EnsureRisk(ctx, RiskRequest{
		TenantID:  tc.TenantID,
		Title:     firstNonEmpty(tc.Title, tc.Subject),
		CVEID:     tc.CVEID,
		Severity:  tc.Severity,
		AssetID:   tc.AssetID,
		CreatedBy: tc.TriggeredBy,
	})
	if err != nil {
		return step(domain.ActionCreateRisk, "failed", err.Error())
	}
	tc.RiskID = &res.RiskID
	if res.Created {
		return step(domain.ActionCreateRisk, "success", "risk created: "+res.RiskID.String())
	}
	return step(domain.ActionCreateRisk, "success", "risk reused: "+res.RiskID.String())
}

func (e *Engine) doAssign(ctx context.Context, action domain.AutomationAction, tc *TriggerContext) domain.ExecutionStep {
	if e.assigner == nil {
		return step(domain.ActionAssignOwner, "skipped", "no assigner configured")
	}
	if tc.RiskID == nil {
		return step(domain.ActionAssignOwner, "skipped", "no risk to assign")
	}
	assignedTo, err := e.assigner.Assign(ctx, tc.TenantID, *tc.RiskID, action.Target)
	if err != nil {
		return step(domain.ActionAssignOwner, "failed", err.Error())
	}
	tc.OwnerID = &assignedTo
	return step(domain.ActionAssignOwner, "success", "assigned to "+assignedTo.String())
}

func (e *Engine) doTicket(ctx context.Context, action domain.AutomationAction, tc *TriggerContext) domain.ExecutionStep {
	if e.ticketer == nil {
		return step(domain.ActionCreateTicket, "skipped", "no ticketing configured")
	}
	res, err := e.ticketer.OpenTicket(ctx, TicketRequest{
		TenantID:    tc.TenantID,
		Provider:    action.TicketProvider,
		Summary:     firstNonEmpty(tc.Title, tc.Subject),
		Description: buildTicketBody(tc),
		Severity:    tc.Severity,
		Labels:      []string{"openrisk-automation"},
	})
	if err != nil {
		return step(domain.ActionCreateTicket, "failed", err.Error())
	}
	tc.TicketRef = res.Key
	return step(domain.ActionCreateTicket, "success", fmt.Sprintf("%s ticket %s (%s)", res.Provider, res.Key, res.URL))
}

func (e *Engine) doNotify(ctx context.Context, action domain.AutomationAction, tc *TriggerContext) domain.ExecutionStep {
	if e.notifier == nil {
		return step(domain.ActionNotify, "skipped", "no notifier configured")
	}
	subject := firstNonEmpty(tc.Subject, tc.Title)
	message := action.Message
	if message == "" {
		message = defaultAlertMessage(tc)
	}
	delivered, err := e.notifier.Notify(ctx, NotifyRequest{
		TenantID:     tc.TenantID,
		Channels:     action.Channels,
		Severity:     tc.Severity,
		Subject:      subject,
		Message:      message,
		TargetRole:   action.Target,
		OwnerID:      tc.OwnerID,
		Facts:        alertFacts(tc),
		ResourceID:   tc.RiskID,
		ResourceType: "risk",
	})
	if err != nil && len(delivered) == 0 {
		return step(domain.ActionNotify, "failed", err.Error())
	}
	if len(delivered) == 0 {
		return step(domain.ActionNotify, "skipped", "no channel delivered")
	}
	return step(domain.ActionNotify, "success", "notified via "+strings.Join(delivered, ", "))
}

func (e *Engine) doStartSLA(ctx context.Context, rule *domain.AutomationRule, tc *TriggerContext, execID uuid.UUID) domain.ExecutionStep {
	minutes := rule.SLA.MinutesFor(tc.Severity)
	if minutes <= 0 {
		return step(domain.ActionStartSLA, "skipped", "no SLA budget for severity "+tc.Severity)
	}
	now := time.Now()
	due := now.Add(time.Duration(minutes) * time.Minute)
	escalateAt := due.Add(time.Duration(rule.SLA.EscalateAfterMinutes) * time.Minute)

	subjectType := "vulnerability"
	subjectID := tc.Ref
	if tc.RiskID != nil {
		subjectType = "risk"
		subjectID = tc.RiskID.String()
	}
	tracker := &domain.SLATracker{
		ID:               uuid.New(),
		TenantID:         tc.TenantID,
		RuleID:           rule.ID,
		ExecutionID:      &execID,
		SubjectType:      subjectType,
		SubjectID:        subjectID,
		RiskID:           tc.RiskID,
		Title:            firstNonEmpty(tc.Title, tc.Subject),
		Severity:         strings.ToLower(tc.Severity),
		TicketRef:        tc.TicketRef,
		Status:           domain.SLAOpen,
		DueAt:            due,
		EscalateAt:       &escalateAt,
		EscalateToRole:   firstNonEmpty(rule.SLA.EscalateToRole, "admin"),
		EscalateChannels: domain.StringList(rule.SLA.EscalateChannels),
		OwnerID:          tc.OwnerID,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := e.slas.Create(ctx, tracker); err != nil {
		return step(domain.ActionStartSLA, "failed", err.Error())
	}
	return step(domain.ActionStartSLA, "success", fmt.Sprintf("SLA started, due in %d min (%s)", minutes, due.Format(time.RFC3339)))
}

func (e *Engine) doResolveRisk(ctx context.Context, tc *TriggerContext) domain.ExecutionStep {
	if e.resolver == nil {
		return step(domain.ActionResolveRisk, "skipped", "no resolver configured")
	}
	if tc.RiskID == nil {
		return step(domain.ActionResolveRisk, "skipped", "no risk to resolve")
	}
	if err := e.resolver.Resolve(ctx, tc.TenantID, *tc.RiskID); err != nil {
		return step(domain.ActionResolveRisk, "failed", err.Error())
	}
	return step(domain.ActionResolveRisk, "success", "risk resolved: "+tc.RiskID.String())
}

// ---- helpers ----

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func severityRank(s string) int {
	switch strings.ToLower(s) {
	case "critical":
		return 4
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}

// tierRank turns P1..P4 into 1..4 (P1 strongest). 0 = unknown/none.
func tierRank(t string) int {
	switch strings.ToUpper(strings.TrimSpace(t)) {
	case "P1":
		return 1
	case "P2":
		return 2
	case "P3":
		return 3
	case "P4":
		return 4
	default:
		return 0
	}
}

// matchConditions reports whether a trigger context satisfies a rule's guards.
// A zero-value condition matches everything. Returns a human reason on failure.
func matchConditions(cond domain.AutomationConditions, tc TriggerContext) (bool, string) {
	if cond.MinSeverity != "" && severityRank(tc.Severity) < severityRank(cond.MinSeverity) {
		return false, fmt.Sprintf("severity %s < min %s", tc.Severity, cond.MinSeverity)
	}
	if cond.MinCVSS > 0 && tc.CVSS < cond.MinCVSS {
		return false, fmt.Sprintf("cvss %.1f < min %.1f", tc.CVSS, cond.MinCVSS)
	}
	if cond.KEVOnly && !tc.KEV {
		return false, "not CISA-KEV"
	}
	if cond.MinPriorityTier != "" {
		want := tierRank(cond.MinPriorityTier)
		got := tierRank(tc.PriorityTier)
		// A stronger-or-equal tier qualifies: got must be <= want (and known).
		if want > 0 && (got == 0 || got > want) {
			return false, fmt.Sprintf("tier %s weaker than min %s", tc.PriorityTier, cond.MinPriorityTier)
		}
	}
	if len(cond.AssetTags) > 0 {
		if !hasAnyTag(tc.AssetTags, cond.AssetTags) {
			return false, "asset tags do not match"
		}
	}
	return true, ""
}

func hasAnyTag(have, want []string) bool {
	set := make(map[string]struct{}, len(have))
	for _, h := range have {
		set[strings.ToLower(strings.TrimSpace(h))] = struct{}{}
	}
	for _, w := range want {
		if _, ok := set[strings.ToLower(strings.TrimSpace(w))]; ok {
			return true
		}
	}
	return false
}

func alertFacts(tc *TriggerContext) []Fact {
	facts := []Fact{}
	if tc.CVEID != "" {
		facts = append(facts, Fact{Label: "CVE", Value: tc.CVEID})
	}
	if tc.Severity != "" {
		facts = append(facts, Fact{Label: "Severity", Value: strings.ToUpper(tc.Severity)})
	}
	if tc.CVSS > 0 {
		facts = append(facts, Fact{Label: "CVSS", Value: fmt.Sprintf("%.1f", tc.CVSS)})
	}
	if tc.KEV {
		facts = append(facts, Fact{Label: "CISA KEV", Value: "yes"})
	}
	if tc.AssetName != "" {
		facts = append(facts, Fact{Label: "Asset", Value: tc.AssetName})
	}
	return facts
}

func defaultAlertMessage(tc *TriggerContext) string {
	var b strings.Builder
	b.WriteString(firstNonEmpty(tc.Subject, tc.Title))
	if tc.AssetName != "" {
		b.WriteString(" on " + tc.AssetName)
	}
	b.WriteString(".")
	if tc.KEV {
		b.WriteString(" This vulnerability is on the CISA Known-Exploited list — remediate now.")
	}
	return b.String()
}

func buildTicketBody(tc *TriggerContext) string {
	var b strings.Builder
	b.WriteString(defaultAlertMessage(tc))
	b.WriteString("\n\n")
	for _, f := range alertFacts(tc) {
		b.WriteString(fmt.Sprintf("- %s: %s\n", f.Label, f.Value))
	}
	b.WriteString("\nOpened automatically by OpenRisk Security Automation.")
	return b.String()
}
