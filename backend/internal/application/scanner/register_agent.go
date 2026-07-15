// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package scanner

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
	scanpkg "github.com/opendefender/openrisk/internal/scanner"
	authpkg "github.com/opendefender/openrisk/pkg/auth"
)

// RegisterAgentInput is the Agent's registration payload. TenantID and ConfigID
// come from the validated 24h registration token, never from the body.
type RegisterAgentInput struct {
	TenantID uuid.UUID
	ConfigID *uuid.UUID
	Name     string
	Version  string
	IP       string
	Hostname string
	OS       string
}

// RegisterAgentResult is returned ONCE to the Agent. Token and PushSecret are
// shown only here; the SaaS stores only their hash / ciphertext.
type RegisterAgentResult struct {
	Agent       *domain.ScannerAgent `json:"agent"`
	Token       string               `json:"token"`        // scoped scanner JWT (7d)
	PushSecret  string               `json:"push_secret"`  // HMAC key for signing pushes
	RotateAfter time.Time            `json:"rotate_after"` // when to re-register for a fresh token
}

// RegisterAgentUseCase enrols (or re-enrols) an on-prem Agent: it mints the
// scoped token + HMAC push secret, stores their hash/ciphertext, and marks the
// Agent online. Re-running the installer on the same host re-registers (rotates
// the token) rather than creating a duplicate.
type RegisterAgentUseCase struct {
	repo    domain.ScannerAgentRepository
	rsaKeys *authpkg.RSAKeys
	cipher  *CredentialCipher
}

func NewRegisterAgentUseCase(repo domain.ScannerAgentRepository, rsaKeys *authpkg.RSAKeys, cipher *CredentialCipher) *RegisterAgentUseCase {
	return &RegisterAgentUseCase{repo: repo, rsaKeys: rsaKeys, cipher: cipher}
}

func (uc *RegisterAgentUseCase) Execute(ctx context.Context, in RegisterAgentInput) (*RegisterAgentResult, error) {
	if in.TenantID == uuid.Nil {
		return nil, domain.NewUnauthorizedError("missing tenant")
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = strings.TrimSpace(in.Hostname)
	}
	if name == "" {
		name = "OpenRisk Agent"
	}

	agent, err := uc.findReenrollable(ctx, in)
	if err != nil {
		return nil, domain.NewInternalError(err.Error())
	}

	now := time.Now()
	isNew := agent == nil
	if isNew {
		agent = &domain.ScannerAgent{
			ID:                   uuid.New(),
			TenantID:             in.TenantID,
			RegistrationConfigID: in.ConfigID,
			RegisteredAt:         now,
		}
	}
	agent.Name = name
	agent.Version = in.Version
	agent.IP = in.IP
	agent.Hostname = in.Hostname
	agent.OS = in.OS
	agent.Status = domain.AgentOnline
	agent.LastHeartbeat = now
	agent.TokenRotatedAt = now

	// Mint the scoped token and per-agent HMAC push secret.
	token, err := scanpkg.MintAgentToken(uc.rsaKeys, in.TenantID, agent.ID)
	if err != nil {
		return nil, domain.NewInternalError("mint agent token: " + err.Error())
	}
	pushSecret, err := scanpkg.GenerateHMACSecret()
	if err != nil {
		return nil, domain.NewInternalError("generate push secret: " + err.Error())
	}
	pushSecretEnc, err := uc.cipher.EncryptString(pushSecret)
	if err != nil {
		return nil, err
	}
	agent.TokenHash = scanpkg.HashToken(token)
	agent.PushSecretEnc = pushSecretEnc

	if isNew {
		if err := uc.repo.Create(ctx, agent); err != nil {
			return nil, domain.NewInternalError(err.Error())
		}
	} else {
		if err := uc.repo.Update(ctx, agent); err != nil {
			return nil, domain.NewInternalError(err.Error())
		}
	}

	result := &RegisterAgentResult{
		Agent:       agent,
		Token:       token,
		PushSecret:  pushSecret,
		RotateAfter: now.Add(scanpkg.AgentTokenTTL),
	}
	// Strip secrets from the embedded agent copy (they live only in Token/PushSecret).
	result.Agent.TokenHash = ""
	result.Agent.PushSecretEnc = ""
	return result, nil
}

// findReenrollable returns a non-revoked agent on the same host+config to update
// in place (Agent restart / installer re-run), or nil to create a fresh one.
func (uc *RegisterAgentUseCase) findReenrollable(ctx context.Context, in RegisterAgentInput) (*domain.ScannerAgent, error) {
	if strings.TrimSpace(in.Hostname) == "" {
		return nil, nil
	}
	agents, err := uc.repo.List(ctx, in.TenantID)
	if err != nil {
		return nil, err
	}
	for i := range agents {
		a := agents[i]
		if a.Status == domain.AgentRevoked {
			continue
		}
		if !strings.EqualFold(a.Hostname, in.Hostname) {
			continue
		}
		if in.ConfigID != nil && a.RegistrationConfigID != nil && *a.RegistrationConfigID != *in.ConfigID {
			continue
		}
		return &agents[i], nil
	}
	return nil, nil
}
