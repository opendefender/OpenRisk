// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package service

import (
	"testing"

	"github.com/opendefender/openrisk/internal/domain"
)

func TestAuditLogAction_String(t *testing.T) {
	tests := []struct {
		action domain.AuditLogAction
		want   string
	}{
		{domain.ActionLogin, "login"},
		{domain.ActionLoginFailed, "login_failed"},
		{domain.ActionRegister, "register"},
		{domain.ActionLogout, "logout"},
		{domain.ActionTokenRefresh, "token_refresh"},
		{domain.ActionRoleChange, "role_change"},
		{domain.ActionUserDelete, "user_delete"},
		{domain.ActionUserDeactivate, "user_deactivate"},
		{domain.ActionUserActivate, "user_activate"},
		{domain.ActionPasswordChange, "password_change"},
	}

	for _, tt := range tests {
		if got := tt.action.String(); got != tt.want {
			t.Errorf("AuditLogAction.String() = %v, want %v", got, tt.want)
		}
	}
}

func TestAuditLogResource_String(t *testing.T) {
	tests := []struct {
		resource domain.AuditLogResource
		want     string
	}{
		{domain.ResourceAuth, "auth"},
		{domain.ResourceUser, "user"},
		{domain.ResourceRole, "role"},
	}

	for _, tt := range tests {
		if got := tt.resource.String(); got != tt.want {
			t.Errorf("AuditLogResource.String() = %v, want %v", got, tt.want)
		}
	}
}

func TestAuditLogResult_String(t *testing.T) {
	tests := []struct {
		result domain.AuditLogResult
		want   string
	}{
		{domain.ResultSuccess, "success"},
		{domain.ResultFailure, "failure"},
	}

	for _, tt := range tests {
		if got := tt.result.String(); got != tt.want {
			t.Errorf("AuditLogResult.String() = %v, want %v", got, tt.want)
		}
	}
}

func TestAuditLog_TableName(t *testing.T) {
	log := &domain.AuditLog{}
	expected := "audit_logs"
	if got := log.TableName(); got != expected {
		t.Errorf("AuditLog.TableName() = %v, want %v", got, expected)
	}
}
