package services

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

// OAuthState represents a stored OAuth state with expiration
type OAuthState struct {
	State     string
	Provider  string
	ExpiresAt time.Time
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

// StoreState stores an OAuth state value with expiration
func (s *OAuthStateService) StoreState(state, provider string, duration time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.states[state] = &OAuthState{
		State:     state,
		Provider:  provider,
		ExpiresAt: time.Now().Add(duration),
	}
}

// ValidateState validates an OAuth state and removes it from storage
// Returns the provider if valid, or error message if invalid/expired
func (s *OAuthStateService) ValidateState(state, expectedProvider string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	oauthState, exists := s.states[state]
	if !exists {
		return "", ErrOAuthStateNotFound
	}

	// Check expiration
	if time.Now().After(oauthState.ExpiresAt) {
		delete(s.states, state)
		return "", ErrOAuthStateExpired
	}

	// Validate provider matches
	if oauthState.Provider != expectedProvider {
		delete(s.states, state)
		return "", ErrOAuthStateProviderMismatch
	}

	// Remove state after validation (one-time use)
	delete(s.states, state)

	return oauthState.Provider, nil
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
