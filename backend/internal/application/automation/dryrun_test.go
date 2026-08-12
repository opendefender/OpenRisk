// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial

package automation

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/opendefender/openrisk/internal/domain"
)

// countingPorts records whether ANY action port was touched. A dry run that
// increments any of these has a side effect, which is the one thing it must not.
type countingPorts struct {
	notifies, tickets, risks, assigns, scans, resolves int
}

func (c *countingPorts) Notify(context.Context, NotifyRequest) ([]string, error) {
	c.notifies++
	return []string{"email"}, nil
}
func (c *countingPorts) OpenTicket(context.Context, TicketRequest) (TicketResult, error) {
	c.tickets++
	return TicketResult{Provider: "jira", Key: "SEC-1"}, nil
}
func (c *countingPorts) EnsureRisk(context.Context, RiskRequest) (RiskResult, error) {
	c.risks++
	return RiskResult{RiskID: uuid.New(), Created: true}, nil
}
func (c *countingPorts) Assign(context.Context, uuid.UUID, uuid.UUID, string) (uuid.UUID, error) {
	c.assigns++
	return uuid.New(), nil
}
func (c *countingPorts) ScanAsset(context.Context, uuid.UUID, uuid.UUID) (string, error) {
	c.scans++
	return "job-1", nil
}
func (c *countingPorts) Resolve(context.Context, uuid.UUID, uuid.UUID) error {
	c.resolves++
	return nil
}
func (c *countingPorts) total() int {
	return c.notifies + c.tickets + c.risks + c.assigns + c.scans + c.resolves
}

type stubSubjects struct {
	tc     *TriggerContext
	source string
	err    error
}

func (s stubSubjects) Resolve(context.Context, uuid.UUID, domain.AutomationTrigger, string, string) (*TriggerContext, string, error) {
	return s.tc, s.source, s.err
}

type stubChannels []string

func (s stubChannels) ConfiguredChannels(context.Context, uuid.UUID) ([]string, error) {
	return []string(s), nil
}

func dryRunFixture(t *testing.T, rule *domain.AutomationRule) (*Engine, *countingPorts, *mockSLARepo) {
	t.Helper()
	rules := newMockRuleRepo()
	rules.add(rule)
	execs := newMockExecRepo()
	slas := newMockSLARepo()
	ports := &countingPorts{}
	e := NewEngine(rules, execs, slas, zerolog.Nop()).
		WithNotifier(ports).WithTicketer(ports).WithRiskCreator(ports).
		WithRiskAssigner(ports).WithAssetScanner(ports).WithRiskResolver(ports)
	return e, ports, slas
}

func fullChainRule(tenant uuid.UUID) *domain.AutomationRule {
	return &domain.AutomationRule{
		ID: uuid.New(), TenantID: tenant, Name: "Full chain", Enabled: true,
		Trigger:    domain.TriggerVulnerabilityDetected,
		Conditions: domain.AutomationConditions{MinSeverity: "high"},
		Actions: domain.AutomationActionList{
			{Type: domain.ActionCreateRisk},
			{Type: domain.ActionAssignOwner, Target: "admin"},
			{Type: domain.ActionCreateTicket},
			{Type: domain.ActionNotify, Channels: []string{"email"}},
			{Type: domain.ActionStartSLA},
		},
		SLA: domain.AutomationSLAConfig{CriticalMinutes: 60},
	}
}

// TestDryRun_HasNoSideEffects is the promise the whole feature rests on.
func TestDryRun_HasNoSideEffects(t *testing.T) {
	tenant := uuid.New()
	rule := fullChainRule(tenant)
	e, ports, slas := dryRunFixture(t, rule)
	e.WithChannelProbe(stubChannels{"in_app", "email"})

	rep, err := e.DryRun(context.Background(), rule.ID, tenant, DryRunRequest{})
	if err != nil {
		t.Fatalf("DryRun: %v", err)
	}

	if ports.total() != 0 {
		t.Fatalf("a dry run must not touch any action port, got %+v", ports)
	}
	if len(slas.trackers) != 0 {
		t.Fatalf("a dry run must not create an SLA tracker, got %d", len(slas.trackers))
	}
	if rep.SideEffects {
		t.Fatal("the report must state that nothing happened")
	}
	if len(rep.Steps) != len(rule.Actions) {
		t.Fatalf("expected %d traced steps, got %d", len(rule.Actions), len(rep.Steps))
	}
	for _, s := range rep.Steps {
		if s.Verdict != DryRunWouldRun {
			t.Errorf("step %s: expected would_run with every port wired, got %s (%s)", s.Action, s.Verdict, s.Detail)
		}
	}
}

// TestDryRun_CarriesThePayloadForward proves the trace shows the state each step
// would actually receive — the thing you need when a chain silently does nothing.
func TestDryRun_CarriesThePayloadForward(t *testing.T) {
	tenant := uuid.New()
	rule := fullChainRule(tenant)
	e, _, _ := dryRunFixture(t, rule)
	e.WithChannelProbe(stubChannels{"email"})

	rep, _ := e.DryRun(context.Background(), rule.ID, tenant, DryRunRequest{})

	// create_risk runs first, so the assign step must see a risk id that the
	// create step did not have.
	if _, ok := rep.Steps[0].Payload["risk_id"]; ok {
		t.Fatal("create_risk should not already see a risk id")
	}
	if _, ok := rep.Steps[1].Payload["risk_id"]; !ok {
		t.Fatalf("assign_owner should see the risk id create_risk would mint: %+v", rep.Steps[1].Payload)
	}
	if rep.Steps[0].Produces["risk_id"] == nil {
		t.Fatal("create_risk should declare what it produces")
	}
}

// TestDryRun_ReportsTheExactFailurePointAndPayload is the diagnosis the old
// "test" endpoint could never give.
func TestDryRun_ReportsTheExactFailurePointAndPayload(t *testing.T) {
	tenant := uuid.New()
	rule := &domain.AutomationRule{
		ID: uuid.New(), TenantID: tenant, Name: "Assign without a risk", Enabled: true,
		Trigger: domain.TriggerVulnerabilityDetected,
		Actions: domain.AutomationActionList{
			{Type: domain.ActionNotify, Channels: []string{"email"}},
			{Type: domain.ActionAssignOwner, Target: "admin"}, // no create_risk before it
		},
	}
	e, _, _ := dryRunFixture(t, rule)
	e.WithChannelProbe(stubChannels{"email"})

	rep, _ := e.DryRun(context.Background(), rule.ID, tenant, DryRunRequest{})

	if rep.FailedAtIndex == nil {
		t.Fatal("the trace must locate the failing step")
	}
	if *rep.FailedAtIndex != 1 || rep.FailedAction != string(domain.ActionAssignOwner) {
		t.Fatalf("expected the failure at step 1 (assign_owner), got %d (%s)", *rep.FailedAtIndex, rep.FailedAction)
	}
	if !strings.Contains(rep.FailureReason, "create_risk") {
		t.Fatalf("the reason should tell the user how to fix it, got %q", rep.FailureReason)
	}
	if rep.PayloadAtFailure == nil {
		t.Fatal("the payload as it stood at the failure must be reported")
	}
	if _, ok := rep.PayloadAtFailure["risk_id"]; ok {
		t.Fatal("the failure payload should show the risk id was absent — that is the diagnosis")
	}
}

// TestDryRun_NotifyWithoutAConfiguredChannelFails: a notify step that resolves to
// nobody is the most common silent failure in this module.
func TestDryRun_NotifyWithoutAConfiguredChannelFails(t *testing.T) {
	tenant := uuid.New()
	rule := &domain.AutomationRule{
		ID: uuid.New(), TenantID: tenant, Name: "Alert", Enabled: true,
		Trigger: domain.TriggerVulnerabilityDetected,
		Actions: domain.AutomationActionList{{Type: domain.ActionNotify, Channels: []string{"slack"}}},
	}
	e, _, _ := dryRunFixture(t, rule)
	e.WithChannelProbe(stubChannels{"in_app"}) // slack is NOT configured

	rep, _ := e.DryRun(context.Background(), rule.ID, tenant, DryRunRequest{})

	if rep.WouldFail != 1 {
		t.Fatalf("asking for an unconfigured channel must be reported as a failure: %+v", rep.Steps)
	}
	if !strings.Contains(rep.Steps[0].Detail, "slack") || !strings.Contains(rep.Steps[0].Detail, "in_app") {
		t.Fatalf("the detail should name what was asked for and what is available, got %q", rep.Steps[0].Detail)
	}
}

// TestDryRun_UnmatchedConditionsStopEverything: if the guard rejects the subject,
// no step is reported as runnable.
func TestDryRun_UnmatchedConditionsStopEverything(t *testing.T) {
	tenant := uuid.New()
	rule := &domain.AutomationRule{
		ID: uuid.New(), TenantID: tenant, Name: "KEV only", Enabled: true,
		Trigger:    domain.TriggerVulnerabilityDetected,
		Conditions: domain.AutomationConditions{KEVOnly: true},
		Actions:    domain.AutomationActionList{{Type: domain.ActionCreateRisk}},
	}
	e, ports, _ := dryRunFixture(t, rule)
	e.WithSubjectResolver(stubSubjects{
		tc:     &TriggerContext{Ref: "vuln:1", Subject: "Not exploited", Severity: "critical", KEV: false},
		source: "live vulnerability: CVE-2024-0001",
	})

	rep, _ := e.DryRun(context.Background(), rule.ID, tenant, DryRunRequest{})

	if rep.Matched {
		t.Fatal("a non-KEV subject must not match a KEV-only rule")
	}
	if rep.Steps[0].Verdict != DryRunNotMatched {
		t.Fatalf("expected not_matched, got %s", rep.Steps[0].Verdict)
	}
	if !strings.Contains(rep.MatchDetail, "KEV") {
		t.Fatalf("the report should say which condition rejected it, got %q", rep.MatchDetail)
	}
	if ports.total() != 0 {
		t.Fatal("nothing may run when conditions do not match")
	}
}

// TestDryRun_SaysWhenTheSubjectIsSynthetic — a green trace on invented data must
// never read as a green trace on the tenant's data.
func TestDryRun_SaysWhenTheSubjectIsSynthetic(t *testing.T) {
	tenant := uuid.New()
	rule := fullChainRule(tenant)

	e, _, _ := dryRunFixture(t, rule) // no subject resolver wired
	rep, _ := e.DryRun(context.Background(), rule.ID, tenant, DryRunRequest{})
	if rep.RealSubject {
		t.Fatal("without a resolver the subject cannot be real")
	}
	if !strings.Contains(rep.SubjectSource, "synthetic") {
		t.Fatalf("the report must say the subject is synthetic, got %q", rep.SubjectSource)
	}

	e2, _, _ := dryRunFixture(t, rule)
	e2.WithSubjectResolver(stubSubjects{
		tc:     &TriggerContext{Ref: "vuln:42", Subject: "Log4Shell", Severity: "critical"},
		source: "live vulnerability: CVE-2021-44228 — Log4Shell",
	})
	rep2, _ := e2.DryRun(context.Background(), rule.ID, tenant, DryRunRequest{})
	if !rep2.RealSubject || !strings.Contains(rep2.SubjectSource, "CVE-2021-44228") {
		t.Fatalf("a resolved subject must be reported as real and named, got %+v", rep2.SubjectSource)
	}
}

// TestDryRun_OverridesLetYouAskWhatIf without editing production data.
func TestDryRun_OverridesLetYouAskWhatIf(t *testing.T) {
	tenant := uuid.New()
	rule := &domain.AutomationRule{
		ID: uuid.New(), TenantID: tenant, Name: "Critical only", Enabled: true,
		Trigger:    domain.TriggerVulnerabilityDetected,
		Conditions: domain.AutomationConditions{MinSeverity: "critical"},
		Actions:    domain.AutomationActionList{{Type: domain.ActionCreateRisk}},
	}
	e, _, _ := dryRunFixture(t, rule)
	e.WithSubjectResolver(stubSubjects{
		tc:     &TriggerContext{Ref: "vuln:1", Subject: "A medium finding", Severity: "medium"},
		source: "live vulnerability: CVE-2024-1",
	})

	plain, _ := e.DryRun(context.Background(), rule.ID, tenant, DryRunRequest{})
	if plain.Matched {
		t.Fatal("a medium subject must not match a critical-only rule")
	}

	whatIf, _ := e.DryRun(context.Background(), rule.ID, tenant, DryRunRequest{
		Overrides: SubjectOverrides{Severity: "critical"},
	})
	if !whatIf.Matched {
		t.Fatal("overriding the severity should make the same rule match")
	}
}

// TestDryRunRegistry_CancelAndTenantScope.
func TestDryRunRegistry_CancelAndTenantScope(t *testing.T) {
	reg := NewDryRunRegistry()
	tenant, other := uuid.New(), uuid.New()

	id, ctx := reg.Start(context.Background(), tenant)
	if ctx.Err() != nil {
		t.Fatal("a fresh run must not start cancelled")
	}
	if !reg.Cancel(id, tenant) {
		t.Fatal("an in-flight run must be cancellable")
	}
	if ctx.Err() == nil {
		t.Fatal("cancelling must cancel the run's context")
	}
	if reg.Cancel(id, tenant) {
		t.Fatal("cancelling twice must report that there was nothing to cancel")
	}

	id2, _ := reg.Start(context.Background(), tenant)
	if reg.Cancel(id2, other) {
		t.Fatal("another tenant must not be able to cancel this run")
	}
	reg.Finish(id2, &DryRunReport{RuleName: "x"})
	if _, ok := reg.Get(id2, other); ok {
		t.Fatal("another tenant must not be able to read this trace")
	}
	if rep, ok := reg.Get(id2, tenant); !ok || rep.ID != id2 {
		t.Fatal("the owning tenant must be able to re-read its trace by id")
	}
}

// TestSuspend_RequiresAReason.
func TestSuspend_RequiresAReason(t *testing.T) {
	tenant, actor := uuid.New(), uuid.New()
	repo := newMockRuleRepo()
	rule := fullChainRule(tenant)
	repo.add(rule)
	svc := NewRuleService(repo)

	if _, err := svc.Suspend(context.Background(), tenant, rule.ID, actor, "   "); err == nil {
		t.Fatal("suspending without a reason must be refused")
	}
	got, err := svc.Suspend(context.Background(), tenant, rule.ID, actor, "noisy while we migrate the scanner")
	if err != nil {
		t.Fatalf("Suspend: %v", err)
	}
	if got.Enabled || got.SuspendedReason == "" || got.SuspendedBy == nil {
		t.Fatalf("a suspension must record who and why: %+v", got)
	}
	back, err := svc.Enable(context.Background(), tenant, rule.ID, actor)
	if err != nil {
		t.Fatalf("Enable: %v", err)
	}
	if !back.Enabled || back.SuspendedReason != "" {
		t.Fatalf("re-enabling must clear the suspension: %+v", back)
	}
}

// TestCreateFromTemplate_StartsSuspended — adopting a template must not start
// firing against production the moment it is clicked.
func TestCreateFromTemplate_StartsSuspended(t *testing.T) {
	tenant, actor := uuid.New(), uuid.New()
	svc := NewRuleService(newMockRuleRepo())

	rule, err := svc.CreateFromTemplate(context.Background(), tenant, actor, "critical-vuln-response", "")
	if err != nil {
		t.Fatalf("CreateFromTemplate: %v", err)
	}
	if rule.Enabled {
		t.Fatal("an adopted template must start suspended so it can be dry-run first")
	}
	if rule.TemplateKey != "critical-vuln-response" || len(rule.Actions) == 0 {
		t.Fatalf("the template's chain must be copied onto the rule: %+v", rule)
	}
	if _, err := svc.CreateFromTemplate(context.Background(), tenant, actor, "no-such-template", ""); err == nil {
		t.Fatal("an unknown template key must be refused")
	}
}

// TestRuleHealth_DerivedFromTheLastRun.
func TestRuleHealth_DerivedFromTheLastRun(t *testing.T) {
	cases := []struct {
		name   string
		rule   domain.AutomationRule
		expect string
	}{
		{"paused", domain.AutomationRule{Enabled: false, SuspendedReason: "under review"}, RuleHealthSuspended},
		{"never fired", domain.AutomationRule{Enabled: true}, RuleHealthIdle},
		{"last run ok", domain.AutomationRule{Enabled: true, LastStatus: "success"}, RuleHealthOK},
		{"last run partial", domain.AutomationRule{Enabled: true, LastStatus: "partial"}, RuleHealthDegraded},
		{"last run failed", domain.AutomationRule{Enabled: true, LastStatus: "failed", LastError: "notify: no channel"}, RuleHealthFailing},
	}
	for _, tc := range cases {
		got, detail := ruleHealth(&tc.rule)
		if got != tc.expect {
			t.Errorf("%s: expected %s, got %s", tc.name, tc.expect, got)
		}
		if detail == "" {
			t.Errorf("%s: health must come with an explanation", tc.name)
		}
	}
}

// TestDescribe_RendersARuleAsASentence — the near-natural-language builder must
// be able to read a rule back, in both locales.
func TestDescribe_RendersARuleAsASentence(t *testing.T) {
	rule := &domain.AutomationRule{
		Trigger:    domain.TriggerVulnerabilityDetected,
		Conditions: domain.AutomationConditions{MinSeverity: "high", KEVOnly: true},
		Actions: domain.AutomationActionList{
			{Type: domain.ActionCreateRisk},
			{Type: domain.ActionNotify, Channels: []string{"email"}},
		},
	}
	fr := rule.Describe(domain.LocaleFR)
	for _, want := range []string{"Quand", "si", "alors", "ouvrir un risque", "puis"} {
		if !strings.Contains(fr, want) {
			t.Errorf("French sentence missing %q: %s", want, fr)
		}
	}
	en := rule.Describe(domain.LocaleEN)
	for _, want := range []string{"When", "if", "then", "open a risk"} {
		if !strings.Contains(en, want) {
			t.Errorf("English sentence missing %q: %s", want, en)
		}
	}

	empty := &domain.AutomationRule{Trigger: domain.TriggerManual}
	if !strings.Contains(empty.Describe(domain.LocaleFR), "aucune action") {
		t.Error("a rule with no action must say so rather than render a truncated sentence")
	}
}

// TestTemplates_AreWellFormed — every shipped template must be a rule the engine
// would actually accept.
func TestTemplates_AreWellFormed(t *testing.T) {
	templates := domain.AutomationTemplates()
	if len(templates) != 5 {
		t.Fatalf("expected 5 ready-made templates, got %d", len(templates))
	}
	seen := map[string]bool{}
	for _, tpl := range templates {
		if seen[tpl.Key] {
			t.Errorf("duplicate template key %s", tpl.Key)
		}
		seen[tpl.Key] = true
		if tpl.Name == "" || tpl.Description == "" || tpl.UseCase == "" {
			t.Errorf("%s: a template must say what it is and when to use it", tpl.Key)
		}
		rule := domain.AutomationRule{
			Name: tpl.Name, Trigger: tpl.Trigger, Actions: tpl.Actions, SLA: tpl.SLA,
		}
		if err := rule.Validate(); err != nil {
			t.Errorf("%s: shipped template does not validate: %v", tpl.Key, err)
		}
		for _, a := range tpl.Actions {
			for _, ch := range a.Channels {
				if !domain.IsAutomationChannel(ch) {
					t.Errorf("%s: unknown channel %q", tpl.Key, ch)
				}
			}
		}
	}
}
