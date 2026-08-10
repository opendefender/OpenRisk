// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package service

import (
	"errors"
	"sync"
	"time"
)

// OAuth state validation errors
var (
	ErrOAuthStateNotFound         = errors.New("oauth state not found")
	ErrOAuthStateExpired          = errors.New("oauth state expired")
	ErrOAuthStateProviderMismatch = errors.New("oauth provider mismatch")
)

// OAuthState represents a stored OAuth state with expiration.
//
// It also carries the PKCE code verifier for the flow. The verifier is the half
// of the PKCE pair that must NOT travel through the browser: only its S256 hash
// goes out in the authorization URL, and the verifier is produced here again at
// token-exchange time. Parking it beside the state is what makes that possible
// without a round trip through the user agent.
type OAuthState struct {
	State     string
	Provider  string
	ExpiresAt time.Time

	// CodeVerifier is the PKCE secret (RFC 7636). Never rendered anywhere.
	CodeVerifier string
	// ReturnTo is where to send the browser once the flow completes.
	ReturnTo string
	// Locale is the language the user started the flow in, so an error lands in
	// the language they were reading.
	Locale string
}

// OAuthStateService manages OAuth state storage for CSRF protection
type OAuthStateService struct {
	mu     sync.RWMutex
	states map[string]*OAuthState // state -> OAuthState
}

// NewOAuthStateService creates a new OAuth state service
func NewOAuthStateService() *OAuthStateService {
	service := &OAuthStateService{
		states: make(map[string]*OAuthState),
	}

	// Start cleanup goroutine for expired states
	go service.cleanupExpiredStates()

	return service
}

// StoreState stores an OAuth state value with expiration.
func (s *OAuthStateService) StoreState(state, provider string, duration time.Duration) {
	s.StoreFlow(&OAuthState{State: state, Provider: provider}, duration)
}

// StoreFlow stores a full flow record (state + PKCE verifier + return context).
//
// NOTE — this store is in-process. It is correct for a single replica and for
// development, and it is what the deployment currently runs. Behind more than
// one replica, a callback landing on a different instance than the one that
// started the flow would find no state and be refused: the failure is a clean
// "state validation failed", never a silent bypass. Moving it to Redis (the
// project already runs one) is the fix when horizontal scaling arrives.
func (s *OAuthStateService) StoreFlow(flow *OAuthState, duration time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	flow.ExpiresAt = time.Now().Add(duration)
	s.states[flow.State] = flow
}

// ValidateState validates an OAuth state and removes it from storage.
// Returns the provider if valid, or an error if invalid/expired.
func (s *OAuthStateService) ValidateState(state, expectedProvider string) (string, error) {
	flow, err := s.ConsumeFlow(state, expectedProvider)
	if err != nil {
		return "", err
	}
	return flow.Provider, nil
}

// ConsumeFlow validates a state and returns the whole flow record, removing it.
//
// Single-use: the record is deleted whether validation succeeds or fails on
// expiry/provider mismatch. A state that could be replayed would defeat the CSRF
// binding it exists to provide.
func (s *OAuthStateService) ConsumeFlow(state, expectedProvider string) (*OAuthState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	oauthState, exists := s.states[state]
	if !exists {
		return nil, ErrOAuthStateNotFound
	}

	// Remove up front: this record is single-use regardless of the verdict.
	delete(s.states, state)

	if time.Now().After(oauthState.ExpiresAt) {
		return nil, ErrOAuthStateExpired
	}

	if oauthState.Provider != expectedProvider {
		return nil, ErrOAuthStateProviderMismatch
	}

	return oauthState, nil
}

// cleanupExpiredStates periodically removes expired states
func (s *OAuthStateService) cleanupExpiredStates() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.mu.Lock()
		now := time.Now()
		for state, oauthState := range s.states {
			if now.After(oauthState.ExpiresAt) {
				delete(s.states, state)
			}
		}
		s.mu.Unlock()
	}
}
