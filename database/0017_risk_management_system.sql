-- Migration 0013: Risk Management System (ISO 31000 and NIST RMF Compliant)
-- Implements complete risk lifecycle management with full audit trail and compliance tracking
-- Follows ISO 31000 phases: Identify -> Analyze -> Evaluate -> Treat -> Monitor -> Review

-- Risk Management Policy Table
CREATE TABLE IF NOT EXISTS risk_management_policies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    policy_name VARCHAR(255) NOT NULL,
    version VARCHAR(50) NOT NULL,
    effective_date DATE NOT NULL,
    review_date DATE NOT NULL,
    governance_framework VARCHAR(100) NOT NULL,
    risk_appetite TEXT,
    risk_tolerance_levels JSONB,
    methodology VARCHAR(255),
    roles_responsibilities JSONB,
    approval_chain JSONB,
    status VARCHAR(50) DEFAULT 'DRAFT',
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP,
    CONSTRAINT chk_version_format CHECK (version ~ '^\d+\.\d+\.\d+$'),
    INDEX idx_tenant_policy (tenant_id),
    INDEX idx_policy_status (status)
);

-- Risk Register (Extended from existing risks table)
CREATE TABLE IF NOT EXISTS risk_register (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    risk_id UUID NOT NULL UNIQUE REFERENCES risks(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    
    identification_date TIMESTAMP NOT NULL,
    identified_by UUID NOT NULL REFERENCES users(id),
    identification_method VARCHAR(100),
    risk_category VARCHAR(100),
    risk_context TEXT,
    
    analysis_date TIMESTAMP,
    analysis_methodology VARCHAR(100),
    probability_score INT CHECK (probability_score >= 1 AND probability_score <= 5),
    impact_score INT CHECK (impact_score >= 1 AND impact_score <= 5),
    risk_score DECIMAL(8,2),
    affected_areas TEXT[],
    root_causes TEXT,
    potential_consequences TEXT,
    analysis_notes TEXT,
    analyzed_by UUID REFERENCES users(id),
    
    evaluation_date TIMESTAMP,
    inherent_risk_level VARCHAR(50),
    residual_risk_level VARCHAR(50) DEFAULT 'HIGH',
    risk_priority INT,
    evaluation_criteria JSONB,
    evaluated_by UUID REFERENCES users(id),
    
    risk_owner UUID NOT NULL REFERENCES users(id),
    risk_owner_email VARCHAR(255),
    secondary_owner UUID REFERENCES users(id),
    responsible_department VARCHAR(255),
    
    external_reference VARCHAR(255),
    compliance_frameworks TEXT[],
    
    status VARCHAR(50) DEFAULT 'IDENTIFIED',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP,
    
    INDEX idx_tenant_register (tenant_id),
    INDEX idx_risk_owner (risk_owner),
    INDEX idx_register_status (status),
    INDEX idx_inherent_risk (inherent_risk_level),
    INDEX idx_identification_date (identification_date)
);

-- Risk Treatment Plan (Mitigation Strategies - ISO 31000 Phase 4)
CREATE TABLE IF NOT EXISTS risk_treatment_plans (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    risk_register_id UUID NOT NULL REFERENCES risk_register(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    
    treatment_type VARCHAR(50) NOT NULL,
    treatment_name VARCHAR(255) NOT NULL,
    description TEXT NOT NULL,
    
    treatment_strategy TEXT,
    implementation_timeline_start DATE,
    implementation_timeline_end DATE,
    
    estimated_cost DECIMAL(12,2),
    budget_allocated DECIMAL(12,2),
    responsible_person UUID REFERENCES users(id),
    required_resources TEXT,
    
    status VARCHAR(50) DEFAULT 'PLANNED',
    expected_residual_risk VARCHAR(50),
    approval_status VARCHAR(50) DEFAULT 'PENDING',
    approved_by UUID REFERENCES users(id),
    approved_date TIMESTAMP,
    
    review_frequency VARCHAR(50),
    last_review_date TIMESTAMP,
    next_review_date TIMESTAMP,
    
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP,
    
    INDEX idx_tenant_treatment (tenant_id),
    INDEX idx_treatment_type (treatment_type),
    INDEX idx_treatment_status (status)
);

-- Risk Treatment Actions
CREATE TABLE IF NOT EXISTS risk_treatment_actions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    treatment_plan_id UUID NOT NULL REFERENCES risk_treatment_plans(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    
    action_name VARCHAR(255) NOT NULL,
    action_description TEXT,
    action_owner UUID NOT NULL REFERENCES users(id),
    
    start_date DATE,
    due_date DATE NOT NULL,
    completion_date TIMESTAMP,
    
    status VARCHAR(50) DEFAULT 'NOT_STARTED',
    priority VARCHAR(50) DEFAULT 'MEDIUM',
    
    completion_evidence TEXT,
    completion_verified_by UUID REFERENCES users(id),
    
    dependencies TEXT[],
    comments TEXT,
    
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP,
    
    INDEX idx_tenant_action (tenant_id),
    INDEX idx_action_owner (action_owner),
    INDEX idx_action_due_date (due_date),
    INDEX idx_action_status (status)
);

-- Risk Monitoring and Review
CREATE TABLE IF NOT EXISTS risk_monitoring_reviews (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    risk_register_id UUID NOT NULL REFERENCES risk_register(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    
    review_type VARCHAR(50) NOT NULL,
    review_date TIMESTAMP NOT NULL,
    reviewed_by UUID NOT NULL REFERENCES users(id),
    
    current_probability_score INT CHECK (current_probability_score >= 1 AND current_probability_score <= 5),
    current_impact_score INT CHECK (current_impact_score >= 1 AND current_impact_score <= 5),
    current_risk_score DECIMAL(8,2),
    current_risk_level VARCHAR(50),
    
    status_changed_from VARCHAR(50),
    status_changed_to VARCHAR(50),
    
    key_findings TEXT,
    trends_identified TEXT,
    treatment_effectiveness TEXT,
    emerging_issues TEXT,
    
    recommended_actions TEXT,
    effectiveness_rating VARCHAR(50),
    
    next_review_date TIMESTAMP,
    escalation_required BOOLEAN DEFAULT FALSE,
    escalation_reason TEXT,
    
    review_evidence JSONB,
    
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP,
    
    INDEX idx_tenant_review (tenant_id),
    INDEX idx_review_date (review_date),
    INDEX idx_reviewed_by (reviewed_by)
);

-- Risk Decisions and Traceability
CREATE TABLE IF NOT EXISTS risk_decisions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    risk_register_id UUID NOT NULL REFERENCES risk_register(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    
    decision_type VARCHAR(100) NOT NULL,
    decision_title VARCHAR(255) NOT NULL,
    decision_description TEXT NOT NULL,
    
    decision_maker UUID NOT NULL REFERENCES users(id),
    decision_maker_role VARCHAR(100),
    decision_date TIMESTAMP NOT NULL,
    
    rationale TEXT NOT NULL,
    risk_factors_considered TEXT,
    alternatives_considered TEXT,
    
    decision_authority VARCHAR(100),
    approval_required BOOLEAN DEFAULT FALSE,
    approved_by UUID REFERENCES users(id),
    approved_date TIMESTAMP,
    
    risk_acceptance_terms TEXT,
    risk_acceptance_valid_until DATE,
    
    status VARCHAR(50) DEFAULT 'PROPOSED',
    
    supporting_evidence JSONB,
    related_decisions UUID[],
    
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP,
    
    INDEX idx_tenant_decision (tenant_id),
    INDEX idx_decision_date (decision_date),
    INDEX idx_decision_maker (decision_maker),
    INDEX idx_decision_type (decision_type)
);

-- Risk Management Meeting Minutes
CREATE TABLE IF NOT EXISTS risk_meeting_minutes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    
    meeting_title VARCHAR(255) NOT NULL,
    meeting_type VARCHAR(100),
    meeting_date TIMESTAMP NOT NULL,
    
    facilitator UUID NOT NULL REFERENCES users(id),
    attendees UUID[] NOT NULL,
    attendee_list JSONB,
    
    agenda TEXT,
    summary TEXT,
    key_decisions JSONB,
    action_items JSONB,
    risks_discussed UUID[],
    
    risks_identified JSONB,
    escalations JSONB,
    
    approval_status VARCHAR(50) DEFAULT 'DRAFT',
    approved_by UUID REFERENCES users(id),
    
    distribution_list TEXT[],
    is_confidential BOOLEAN DEFAULT FALSE,
    
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP,
    
    INDEX idx_tenant_meeting (tenant_id),
    INDEX idx_meeting_date (meeting_date),
    INDEX idx_meeting_type (meeting_type)
);

-- Audit-Ready Reports
CREATE TABLE IF NOT EXISTS risk_audit_reports (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    
    report_title VARCHAR(255) NOT NULL,
    report_type VARCHAR(100) NOT NULL,
    
    reporting_period_start DATE NOT NULL,
    reporting_period_end DATE NOT NULL,
    
    generated_by UUID NOT NULL REFERENCES users(id),
    generated_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    frameworks_audited TEXT[],
    compliance_status JSONB,
    
    executive_summary TEXT,
    key_findings TEXT,
    metrics_and_analytics JSONB,
    
    total_risks INT,
    risks_by_status JSONB,
    risks_by_severity JSONB,
    
    treatments_active INT,
    treatments_completed INT,
    treatments_overdue INT,
    
    risk_snapshots JSONB,
    decision_history JSONB,
    policy_changes JSONB,
    
    reviewed_by UUID REFERENCES users(id),
    review_date TIMESTAMP,
    review_comments TEXT,
    
    status VARCHAR(50) DEFAULT 'DRAFT',
    is_signed_off BOOLEAN DEFAULT FALSE,
    signed_off_by UUID REFERENCES users(id),
    signed_off_date TIMESTAMP,
    
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP,
    
    INDEX idx_tenant_report (tenant_id),
    INDEX idx_report_type (report_type),
    INDEX idx_generated_date (generated_date)
);

-- Risk Change Log
CREATE TABLE IF NOT EXISTS risk_change_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    risk_register_id UUID REFERENCES risk_register(id) ON DELETE CASCADE,
    
    entity_type VARCHAR(100) NOT NULL,
    entity_id UUID NOT NULL,
    
    change_type VARCHAR(50) NOT NULL,
    changed_by UUID NOT NULL REFERENCES users(id),
    changed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    field_name VARCHAR(255),
    old_value TEXT,
    new_value TEXT,
    
    reason_for_change TEXT,
    approval_required BOOLEAN DEFAULT FALSE,
    approved_by UUID REFERENCES users(id),
    
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP,
    
    INDEX idx_tenant_changelog (tenant_id),
    INDEX idx_entity (entity_type, entity_id),
    INDEX idx_changed_at (changed_at),
    INDEX idx_changed_by (changed_by)
);

-- Risk Compliance Evidence
CREATE TABLE IF NOT EXISTS risk_compliance_evidence (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    risk_register_id UUID REFERENCES risk_register(id) ON DELETE CASCADE,
    
    evidence_type VARCHAR(100) NOT NULL,
    evidence_title VARCHAR(255) NOT NULL,
    evidence_description TEXT,
    
    evidence_date DATE NOT NULL,
    collected_by UUID NOT NULL REFERENCES users(id),
    
    file_path VARCHAR(500),
    file_type VARCHAR(50),
    file_size INT,
    
    compliance_framework VARCHAR(100),
    requirement_reference VARCHAR(255),
    
    verified_by UUID REFERENCES users(id),
    verification_date TIMESTAMP,
    is_verified BOOLEAN DEFAULT FALSE,
    
    valid_from DATE,
    valid_until DATE,
    
    status VARCHAR(50) DEFAULT 'PENDING',
    
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP,
    
    INDEX idx_tenant_evidence (tenant_id),
    INDEX idx_evidence_type (evidence_type),
    INDEX idx_compliance_framework (compliance_framework)
);

-- Performance Indexes
CREATE INDEX idx_risk_register_analysis_date ON risk_register(analysis_date);
CREATE INDEX idx_risk_register_evaluation_date ON risk_register(evaluation_date);
CREATE INDEX idx_treatment_plan_end_date ON risk_treatment_plans(implementation_timeline_end);
CREATE INDEX idx_treatment_action_owner ON risk_treatment_actions(action_owner);
CREATE INDEX idx_monitoring_review_date ON risk_monitoring_reviews(review_date);
CREATE INDEX idx_decision_decision_date ON risk_decisions(decision_date);

COMMIT;
