package ratelimit

import (
	"errors"
	"time"
)

// Errors for profile selection.
var (
	ErrAllProfilesCoolingDown = errors.New("all profiles in fallback chain are cooling down")
	ErrNoProfilesConfigured   = errors.New("no profiles configured in fallback chain")
)

// RolePolicy defines how profiles are selected for a given role.
type RolePolicy struct {
	FallbackChain   []string // Profile names in priority order
	CooldownMinutes int      // How long to wait after rate limit
	Stickiness      string   // Preferred provider (optional)
}

// RateLimitEvent represents a rate limit occurrence.
type RateLimitEvent struct {
	Profile   string
	Timestamp time.Time
	RetryAt   time.Time // When we can retry (from API header)
}

// Selector selects profiles based on availability and policy.
type Selector struct {
	store *CooldownStore
}

// NewSelector creates a new profile selector with the given cooldown store.
func NewSelector(store *CooldownStore) *Selector {
	return &Selector{store: store}
}

// SelectNext returns the next available profile for the role based on policy.
// It iterates through the fallback chain in order, returning the first
// available profile (one that is not cooling down).
func (s *Selector) SelectNext(role string, policy RolePolicy) (string, error) {
	if len(policy.FallbackChain) == 0 {
		return "", ErrNoProfilesConfigured
	}

	for _, profile := range policy.FallbackChain {
		if s.store.IsAvailable(profile) {
			return profile, nil
		}
	}

	return "", ErrAllProfilesCoolingDown
}

// MarkCooldown marks a profile as cooling down for the configured duration.
func (s *Selector) MarkCooldown(profile string, policy RolePolicy) {
	until := time.Now().Add(time.Duration(policy.CooldownMinutes) * time.Minute)
	s.store.MarkCooldown(profile, until)
}

// MarkCooldownUntil marks a profile as cooling down until a specific time.
// This is useful when the API provides a retry-at timestamp.
func (s *Selector) MarkCooldownUntil(profile string, until time.Time) {
	s.store.MarkCooldown(profile, until)
}

// IsAvailable checks if a profile is currently available.
func (s *Selector) IsAvailable(profile string) bool {
	return s.store.IsAvailable(profile)
}
