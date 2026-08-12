// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial
// This file is part of the OpenRisk Enterprise Edition and is NOT covered by the
// AGPL; it is licensed under the OpenRisk Commercial License (see LICENSE.commercial).

package automation

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// ---------------------------------------------------------------------------
// Dry run — "what would this rule do, on real data, without doing it".
//
// The old /test endpoint RAN the rule: it created risks, opened tickets and
// sent alerts against production. That is not a test, it is a live fire drill
// nobody consented to. A dry run must be able to answer "would this work?"
// without changing anything, and must say exactly WHERE it would stop and with
// WHAT payload in hand at that moment.
//
// So the dry run never touches an action port. It inspects: which capabilities
// are wired, whether the conditions match this subject, and what each step
// would receive and produce — carrying the simulated context forward exactly as
// a real run would (create_risk sets a risk id that assign/ticket then use).
// ---------------------------------------------------------------------------

// Dry-run step verdicts. Deliberately distinct from execution statuses: nothing
// happened, so "success" would be a lie.
const (
	DryRunWouldRun   = "would_run"   // the capability is wired and the inputs are there
	DryRunWouldSkip  = "would_skip"  // the step would be recorded as skipped
	DryRunWouldFail  = "would_fail"  // the step would fail — this is the failure point
	DryRunNotReached = "not_reached" // an earlier step stops the chain
	DryRunNotMatched = "not_matched" // the rule's conditions rejected the subject
)

// DryRunStep is one simulated action, with the payload as it stood at that
// instant — the thing you actually need when a rule "does nothing" in prod.
type DryRunStep struct {
	Index      int            `json:"index"`
	Action     string         `json:"action"`
	Verdict    string         `json:"verdict"`
	Detail     string         `json:"detail"`
	Capability string         `json:"capability,omitempty"` // which port this step needs
	Wired      bool           `json:"wired"`
	Params     map[string]any `json:"params,omitempty"`   // the action's own configuration
	Payload    map[string]any `json:"payload"`            // the trigger context entering this step
	Produces   map[string]any `json:"produces,omitempty"` // what it would add to the context
}

// DryRunReport is the whole trace. SideEffects is always false and is part of
// the response on purpose: the caller should be able to see the guarantee, not
// just be told about it in a doc.
type DryRunReport struct {
	ID       uuid.UUID `json:"id"`
	RuleID   uuid.UUID `json:"rule_id"`
	RuleName string    `json:"rule_name"`
	TenantID uuid.UUID `json:"tenant_id"`

	Trigger domain.AutomationTrigger `json:"trigger"`
	Enabled bool                     `json:"rule_enabled"`

	// Where the sample came from. RealSubject=false means no live record matched
	// the request and the trace ran on a synthetic subject — said out loud so a
	// green dry run on fake data is never mistaken for a green run on real data.
	RealSubject   bool   `json:"real_subject"`
	SubjectSource string `json:"subject_source"`
	Subject       string `json:"subject"`

	Matched      bool           `json:"conditions_matched"`
	MatchDetail  string         `json:"conditions_detail"`
	InitialInput map[string]any `json:"initial_input"`

	Steps []DryRunStep `json:"steps"`

	// The failure point, hoisted so the UI does not have to hunt for it.
	FailedAtIndex    *int           `json:"failed_at_index,omitempty"`
	FailedAction     string         `json:"failed_action,omitempty"`
	FailureReason    string         `json:"failure_reason,omitempty"`
	PayloadAtFailure map[string]any `json:"payload_at_failure,omitempty"`

	// Verdict summary.
	WouldRun    int  `json:"would_run"`
	WouldSkip   int  `json:"would_skip"`
	WouldFail   int  `json:"would_fail"`
	SideEffects bool `json:"side_effects"` // always false

	Status     string    `json:"status"` // completed | cancelled
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
	DurationMS int64     `json:"duration_ms"`
}

// DryRunRequest asks for a trace. When SubjectID is set the engine loads that
// real record; otherwise it takes the tenant's most recent matching record, and
// falls back to a synthetic subject only if the tenant has none.
type DryRunRequest struct {
	SubjectType string // vulnerability | risk | incident (empty = pick from the trigger)
	SubjectID   string
	// Overrides let an operator ask "what if this were critical?" without
	// editing production data.
	Overrides SubjectOverrides
	ActorID   uuid.UUID
}

// SubjectOverrides are optional what-if values layered on the loaded subject.
type SubjectOverrides struct {
	Severity     string   `json:"severity,omitempty"`
	CVSS         float64  `json:"cvss,omitempty"`
	KEV          *bool    `json:"kev,omitempty"`
	PriorityTier string   `json:"priority_tier,omitempty"`
	AssetTags    []string `json:"asset_tags,omitempty"`
}

// SubjectResolver loads a REAL record from the tenant and turns it into the
// trigger context a live event would have produced. Optional: without it a dry
// run still traces the rule, on a synthetic subject it labels as such.
type SubjectResolver interface {
	// Resolve returns the context and a human description of where it came from.
	// A nil context (with no error) means "no such record" — the caller then
	// falls back to a sample rather than failing.
	Resolve(ctx context.Context, tenantID uuid.UUID, trigger domain.AutomationTrigger, subjectType, subjectID string) (*TriggerContext, string, error)
}

// ChannelProbe reports which notification channels a tenant has configured, so
// a dry run can say "would notify via slack, email" instead of guessing.
type ChannelProbe interface {
	ConfiguredChannels(ctx context.Context, tenantID uuid.UUID) ([]string, error)
}

// AssetFactsLookup resolves an asset's display name and its tag vocabulary. It
// is what makes a rule's asset-tag condition able to match a live event: the
// event payload carries an asset id, not the asset's tags.
type AssetFactsLookup interface {
	AssetFacts(ctx context.Context, tenantID, assetID uuid.UUID) (name string, tags []string)
}

// WithAssetFacts attaches the asset tag lookup used by live events and dry runs.
func (e *Engine) WithAssetFacts(l AssetFactsLookup) *Engine { e.assetFacts = l; return e }

// WithSubjectResolver attaches the real-data loader used by dry runs.
func (e *Engine) WithSubjectResolver(r SubjectResolver) *Engine { e.subjects = r; return e }

// WithChannelProbe attaches the channel configuration probe used by dry runs.
func (e *Engine) WithChannelProbe(p ChannelProbe) *Engine { e.channels = p; return e }

// DryRun traces a rule without invoking a single action port.
func (e *Engine) DryRun(ctx context.Context, ruleID, tenantID uuid.UUID, req DryRunRequest) (*DryRunReport, error) {
	rule, err := e.rules.GetByID(ctx, ruleID, tenantID)
	if err != nil {
		return nil, err
	}
	if rule == nil {
		return nil, domain.NewNotFoundError("automation rule", ruleID)
	}

	start := time.Now()
	rep := &DryRunReport{
		ID:          uuid.New(),
		RuleID:      rule.ID,
		RuleName:    rule.Name,
		TenantID:    tenantID,
		Trigger:     rule.Trigger,
		Enabled:     rule.Enabled,
		Steps:       []DryRunStep{},
		SideEffects: false,
		Status:      "completed",
		StartedAt:   start,
	}

	tc := e.resolveSubject(ctx, rule, tenantID, req, rep)
	tc.TriggeredBy = req.ActorID
	rep.Subject = firstNonEmpty(tc.Subject, tc.Title, tc.Ref)
	rep.InitialInput = contextPayload(&tc)

	ok, reason := matchConditions(rule.Conditions, tc)
	rep.Matched = ok
	if ok {
		rep.MatchDetail = "every condition is satisfied by this subject"
	} else {
		rep.MatchDetail = reason
	}

	configured := e.probeChannels(ctx, tenantID)

	stopped := false
	for i, action := range rule.Actions {
		if err := ctx.Err(); err != nil {
			// The operator cancelled the test.
			rep.Status = "cancelled"
			break
		}
		step := DryRunStep{Index: i, Action: string(action.Type), Params: actionParams(action)}
		step.Payload = contextPayload(&tc)

		switch {
		case !rep.Matched:
			step.Verdict = DryRunNotMatched
			step.Detail = "the rule's conditions reject this subject, so no action runs"
		case stopped:
			step.Verdict = DryRunNotReached
			step.Detail = "an earlier step stops the chain"
		default:
			e.simulate(rule, action, &tc, &step, configured)
		}

		switch step.Verdict {
		case DryRunWouldRun:
			rep.WouldRun++
		case DryRunWouldSkip:
			rep.WouldSkip++
		case DryRunWouldFail:
			rep.WouldFail++
			if rep.FailedAtIndex == nil {
				idx := i
				rep.FailedAtIndex = &idx
				rep.FailedAction = string(action.Type)
				rep.FailureReason = step.Detail
				rep.PayloadAtFailure = step.Payload
			}
		}
		rep.Steps = append(rep.Steps, step)
	}

	rep.FinishedAt = time.Now()
	rep.DurationMS = rep.FinishedAt.Sub(start).Milliseconds()
	return rep, nil
}

// resolveSubject loads a real record when it can, and is explicit when it can't.
func (e *Engine) resolveSubject(ctx context.Context, rule *domain.AutomationRule, tenantID uuid.UUID, req DryRunRequest, rep *DryRunReport) TriggerContext {
	if e.subjects != nil {
		tc, source, err := e.subjects.Resolve(ctx, tenantID, rule.Trigger, req.SubjectType, req.SubjectID)
		if err == nil && tc != nil {
			rep.RealSubject = true
			rep.SubjectSource = source
			out := *tc
			out.TenantID = tenantID
			applyOverrides(&out, req.Overrides)
			return out
		}
		if err != nil {
			rep.SubjectSource = "no live record could be loaded (" + err.Error() + ") — traced on a synthetic subject"
		} else {
			rep.SubjectSource = "this tenant has no matching record yet — traced on a synthetic subject"
		}
	} else {
		rep.SubjectSource = "real-data lookup is not wired on this deployment — traced on a synthetic subject"
	}
	tc := sampleContext(rule.Trigger, tenantID)
	applyOverrides(&tc, req.Overrides)
	return tc
}

func (e *Engine) probeChannels(ctx context.Context, tenantID uuid.UUID) []string {
	if e.channels == nil {
		return nil
	}
	chans, err := e.channels.ConfiguredChannels(ctx, tenantID)
	if err != nil {
		return nil
	}
	return chans
}

// simulate decides what one action WOULD do. It mutates tc the way the real
// action would, so later steps see the same context a live run would give them.
func (e *Engine) simulate(rule *domain.AutomationRule, action domain.AutomationAction, tc *TriggerContext, step *DryRunStep, configured []string) {
	switch action.Type {
	case domain.ActionScanAsset:
		step.Capability = "asset scanner"
		step.Wired = e.scanner != nil
		switch {
		case !step.Wired:
			step.Verdict, step.Detail = DryRunWouldSkip, "no scanner is configured, so this step would be recorded as skipped"
		case tc.AssetID == nil:
			step.Verdict, step.Detail = DryRunWouldSkip, "this subject is not linked to an asset, so there is nothing to re-scan"
		default:
			step.Verdict = DryRunWouldRun
			step.Detail = "would queue a targeted re-scan of asset " + tc.AssetName
			step.Produces = map[string]any{"scan_job": "(a job reference)"}
		}

	case domain.ActionCreateRisk:
		step.Capability = "risk register"
		step.Wired = e.riskCreate != nil
		switch {
		case tc.RiskID != nil:
			step.Verdict, step.Detail = DryRunWouldSkip, "a risk is already linked to this subject; the rule reuses it instead of opening a duplicate"
		case !step.Wired:
			step.Verdict, step.Detail = DryRunWouldSkip, "the risk-creation capability is not wired on this deployment"
		default:
			step.Verdict = DryRunWouldRun
			step.Detail = "would open (or reuse) a risk titled " + strconv.Quote(firstNonEmpty(tc.Title, tc.Subject))
			simulated := dryRunPlaceholderRisk
			tc.RiskID = &simulated
			step.Produces = map[string]any{"risk_id": "(a new risk id, simulated here)"}
		}

	case domain.ActionAssignOwner:
		step.Capability = "owner assignment"
		step.Wired = e.assigner != nil
		switch {
		case !step.Wired:
			step.Verdict, step.Detail = DryRunWouldSkip, "the assignment capability is not wired on this deployment"
		case tc.RiskID == nil:
			step.Verdict, step.Detail = DryRunWouldFail, "there is no risk to assign — put a create_risk step before this one, or this action will never do anything"
		case strings.TrimSpace(action.Target) == "":
			step.Verdict, step.Detail = DryRunWouldFail, "this action has no target: it does not say which role or user should own the risk"
		default:
			step.Verdict = DryRunWouldRun
			step.Detail = "would assign the risk to " + action.Target
			owner := dryRunPlaceholderUser
			tc.OwnerID = &owner
			step.Produces = map[string]any{"owner_id": "(the resolved " + action.Target + ")"}
		}

	case domain.ActionCreateTicket:
		step.Capability = "ITSM ticketing"
		step.Wired = e.ticketer != nil
		if !step.Wired {
			step.Verdict, step.Detail = DryRunWouldSkip, "no ITSM integration is configured (Jira / ServiceNow), so this step would be recorded as skipped"
		} else {
			step.Verdict = DryRunWouldRun
			provider := action.TicketProvider
			if provider == "" {
				provider = "the tenant default provider"
			}
			step.Detail = "would open a ticket with " + provider
			tc.TicketRef = "DRY-RUN"
			step.Produces = map[string]any{"ticket_ref": "(the created ticket key)"}
		}

	case domain.ActionNotify:
		step.Capability = "notification channels"
		step.Wired = e.notifier != nil
		wanted := action.Channels
		if len(wanted) == 0 {
			wanted = configured
		}
		usable := intersect(wanted, configured)
		switch {
		case !step.Wired:
			step.Verdict, step.Detail = DryRunWouldSkip, "the notifier is not wired on this deployment"
		case len(configured) == 0:
			step.Verdict = DryRunWouldFail
			step.Detail = "no channel is configured for this tenant: the alert would be composed and then delivered nowhere. Configure a channel under Automation → Channels."
		case len(usable) == 0:
			step.Verdict = DryRunWouldFail
			step.Detail = "this action asks for " + strings.Join(wanted, ", ") +
				" but only " + strings.Join(configured, ", ") + " is configured, so nothing would be delivered"
		default:
			step.Verdict = DryRunWouldRun
			step.Detail = "would alert via " + strings.Join(usable, ", ")
			if len(usable) < len(wanted) {
				step.Detail += " (requested " + strings.Join(wanted, ", ") + "; the rest are not configured)"
			}
			step.Produces = map[string]any{"delivered_via": usable}
		}

	case domain.ActionStartSLA:
		step.Capability = "SLA tracker"
		step.Wired = true // the SLA store is a required dependency of the engine
		minutes := rule.SLA.MinutesFor(tc.Severity)
		if minutes <= 0 {
			step.Verdict = DryRunWouldFail
			step.Detail = "this rule defines no SLA budget for severity " + strconv.Quote(tc.Severity) +
				", so the countdown would never start — set a budget for that tier in the rule's SLA policy"
		} else {
			due := time.Now().Add(time.Duration(minutes) * time.Minute)
			step.Verdict = DryRunWouldRun
			step.Detail = fmt.Sprintf("would start a %d-minute countdown (due %s)", minutes, due.Format(time.RFC3339))
			step.Produces = map[string]any{
				"due_at":            due.Format(time.RFC3339),
				"escalate_to_role":  firstNonEmpty(rule.SLA.EscalateToRole, "admin"),
				"escalate_channels": rule.SLA.EscalateChannels,
			}
		}

	case domain.ActionResolveRisk:
		step.Capability = "risk resolution"
		step.Wired = e.resolver != nil
		switch {
		case !step.Wired:
			step.Verdict, step.Detail = DryRunWouldSkip, "the resolution capability is not wired on this deployment"
		case tc.RiskID == nil:
			step.Verdict, step.Detail = DryRunWouldFail, "there is no risk to resolve at this point in the chain"
		default:
			step.Verdict = DryRunWouldRun
			step.Detail = "would mark the linked risk resolved"
		}

	case domain.ActionCloseTicket:
		step.Capability = "ITSM ticketing"
		step.Wired = e.ticketer != nil
		if tc.TicketRef == "" {
			step.Verdict, step.Detail = DryRunWouldSkip, "no ticket was opened earlier in this chain, so there is none to close"
		} else if !step.Wired {
			step.Verdict, step.Detail = DryRunWouldSkip, "no ITSM integration is configured"
		} else {
			step.Verdict, step.Detail = DryRunWouldRun, "would close ticket "+tc.TicketRef
		}

	default:
		step.Verdict = DryRunWouldFail
		step.Detail = "unknown action type " + strconv.Quote(string(action.Type)) + " — this rule cannot run as written"
	}
}

// dryRunPlaceholder* stand in for ids a real run would mint. Fixed values (not
// random) so a trace is reproducible and obviously synthetic.
var (
	dryRunPlaceholderRisk = uuid.MustParse("00000000-0000-4000-8000-00000000d0e1")
	dryRunPlaceholderUser = uuid.MustParse("00000000-0000-4000-8000-00000000d0e2")
)

// contextPayload renders the trigger context as the payload the UI shows at
// each step. Deliberately a copy: mutating a later step must not rewrite an
// earlier step's recorded payload.
func contextPayload(tc *TriggerContext) map[string]any {
	p := map[string]any{
		"ref":      tc.Ref,
		"subject":  tc.Subject,
		"title":    tc.Title,
		"severity": tc.Severity,
	}
	if tc.CVEID != "" {
		p["cve_id"] = tc.CVEID
	}
	if tc.CVSS > 0 {
		p["cvss"] = tc.CVSS
	}
	if tc.KEV {
		p["kev"] = true
	}
	if tc.PriorityTier != "" {
		p["priority_tier"] = tc.PriorityTier
	}
	if tc.AssetID != nil {
		p["asset_id"] = tc.AssetID.String()
	}
	if tc.AssetName != "" {
		p["asset_name"] = tc.AssetName
	}
	if len(tc.AssetTags) > 0 {
		p["asset_tags"] = append([]string{}, tc.AssetTags...)
	}
	if tc.RiskID != nil {
		p["risk_id"] = tc.RiskID.String()
	}
	if tc.OwnerID != nil {
		p["owner_id"] = tc.OwnerID.String()
	}
	if tc.TicketRef != "" {
		p["ticket_ref"] = tc.TicketRef
	}
	return p
}

func actionParams(a domain.AutomationAction) map[string]any {
	p := map[string]any{}
	if len(a.Channels) > 0 {
		p["channels"] = a.Channels
	}
	if a.Target != "" {
		p["target"] = a.Target
	}
	if a.Message != "" {
		p["message"] = a.Message
	}
	if a.TicketProvider != "" {
		p["ticket_provider"] = a.TicketProvider
	}
	if len(p) == 0 {
		return nil
	}
	return p
}

func intersect(want, have []string) []string {
	if len(have) == 0 {
		return nil
	}
	set := map[string]bool{}
	for _, h := range have {
		set[strings.ToLower(strings.TrimSpace(h))] = true
	}
	var out []string
	for _, w := range want {
		k := strings.ToLower(strings.TrimSpace(w))
		if set[k] {
			out = append(out, k)
		}
	}
	return out
}

func applyOverrides(tc *TriggerContext, o SubjectOverrides) {
	if o.Severity != "" {
		tc.Severity = o.Severity
	}
	if o.CVSS > 0 {
		tc.CVSS = o.CVSS
	}
	if o.KEV != nil {
		tc.KEV = *o.KEV
	}
	if o.PriorityTier != "" {
		tc.PriorityTier = o.PriorityTier
	}
	if len(o.AssetTags) > 0 {
		tc.AssetTags = o.AssetTags
	}
}

// sampleContext is the synthetic subject used when a tenant has no real record
// to trace against. Always paired with RealSubject=false in the report.
func sampleContext(trigger domain.AutomationTrigger, tenantID uuid.UUID) TriggerContext {
	tc := TriggerContext{TenantID: tenantID, Severity: "critical"}
	switch trigger {
	case domain.TriggerVulnerabilityDetected:
		tc.Ref = "sample:CVE-0000-0000"
		tc.CVEID = "CVE-0000-0000"
		tc.Subject = "Sample critical vulnerability"
		tc.Title = "Sample critical vulnerability"
		tc.CVSS = 9.8
		tc.KEV = true
		tc.PriorityTier = "P1"
		tc.AssetName = "sample-asset"
	case domain.TriggerRiskCreated, domain.TriggerRiskScoreUpdated:
		tc.Ref = "sample:risk"
		tc.Subject = "Sample critical risk"
		tc.Title = "Sample critical risk"
	case domain.TriggerIncidentCreated:
		tc.Ref = "sample:incident"
		tc.Subject = "Sample critical incident"
		tc.Title = "Sample critical incident"
	default:
		tc.Ref = "sample:manual"
		tc.Subject = "Manual dry run"
		tc.Title = "Manual dry run"
	}
	return tc
}

// =============================================================================
// Dry-run registry — so a test can be cancelled while it is running
// =============================================================================

// DryRunRegistry keeps in-flight and recently finished dry runs so the UI can
// poll one, and cancel one that is taking too long. In memory on purpose: a
// dry run changes nothing, so losing the trace on restart costs nothing, and
// persisting side-effect-free traces would only add a table to prune.
type DryRunRegistry struct {
	mu      sync.Mutex
	entries map[uuid.UUID]*dryRunEntry
	ttl     time.Duration
}

type dryRunEntry struct {
	tenantID uuid.UUID
	report   *DryRunReport
	cancel   context.CancelFunc
	done     bool
	at       time.Time
}

// NewDryRunRegistry builds the registry. Entries are forgotten after ttl.
func NewDryRunRegistry() *DryRunRegistry {
	return &DryRunRegistry{entries: map[uuid.UUID]*dryRunEntry{}, ttl: 30 * time.Minute}
}

// Start registers a run and returns its id plus a cancellable context.
func (r *DryRunRegistry) Start(ctx context.Context, tenantID uuid.UUID) (uuid.UUID, context.Context) {
	id := uuid.New()
	runCtx, cancel := context.WithCancel(ctx)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sweepLocked()
	r.entries[id] = &dryRunEntry{tenantID: tenantID, cancel: cancel, at: time.Now()}
	return id, runCtx
}

// Finish stores the completed trace under its id.
func (r *DryRunRegistry) Finish(id uuid.UUID, rep *DryRunReport) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.entries[id]
	if !ok {
		return
	}
	if rep != nil {
		rep.ID = id
	}
	e.report = rep
	e.done = true
	e.at = time.Now()
}

// Get returns a trace, tenant-scoped: an id from another tenant reads as absent.
func (r *DryRunRegistry) Get(id, tenantID uuid.UUID) (*DryRunReport, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.entries[id]
	if !ok || e.tenantID != tenantID {
		return nil, false
	}
	return e.report, e.report != nil
}

// Cancel stops an in-flight run. Returns false when the id is unknown for this
// tenant or the run already finished.
func (r *DryRunRegistry) Cancel(id, tenantID uuid.UUID) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.entries[id]
	if !ok || e.tenantID != tenantID || e.done {
		return false
	}
	e.cancel()
	e.done = true
	return true
}

func (r *DryRunRegistry) sweepLocked() {
	cutoff := time.Now().Add(-r.ttl)
	for id, e := range r.entries {
		if e.at.Before(cutoff) {
			e.cancel()
			delete(r.entries, id)
		}
	}
}
