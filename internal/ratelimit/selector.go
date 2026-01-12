package ratelimit

import (
	"errors"
)

var (
	ErrAllProfilesCoolingDown = errors.New("all profiles are cooling down")
	ErrEmptyFallbackChain     = errors.New("fallback chain is empty")
)

// RolePolicy defines the profile selection policy for a role.
type RolePolicy struct {
	FallbackChain   []string // Profile names in priority order
	CooldownMinutes int      // How long to wait after rate limit
	Stickiness      string   // Preferred provider (optional)
}

// Selector selects the next available profile based on the policy.
type Selector interface {
	// SelectNext returns the next available profile for the role
	SelectNext(policy RolePolicy, currentProfile string) (string, error)
}

// ProfileSelector implements the Selector interface.
type ProfileSelector struct {
	store CooldownStoreInterface
}

// NewSelector creates a new ProfileSelector with the given cooldown store.
func NewSelector(store CooldownStoreInterface) *ProfileSelector {
	return &ProfileSelector{store: store}
}

// SelectNext returns the next available profile based on the policy.
// It respects stickiness if set, otherwise selects the first available
// profile from the fallback chain.
func (s *ProfileSelector) SelectNext(policy RolePolicy, currentProfile string) (string, error) {
	if len(policy.FallbackChain) == 0 {
		return "", ErrEmptyFallbackChain
	}

	// If stickiness is set and that profile is available, use it
	if policy.Stickiness != "" && s.store.IsAvailable(policy.Stickiness) {
		// Verify sticky profile is in the chain
		for _, p := range policy.FallbackChain {
			if p == policy.Stickiness {
				return policy.Stickiness, nil
			}
		}
	}

	// Select first available profile from chain
	for _, profile := range policy.FallbackChain {
		if s.store.IsAvailable(profile) {
			return profile, nil
		}
	}

	return "", ErrAllProfilesCoolingDown
}
