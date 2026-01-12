package ratelimit

import (
	"errors"
	"sync"
	"time"
)

// Common errors for profile selection.
var (
	ErrAllProfilesCooling = errors.New("all profiles are cooling down")
	ErrNoPolicyForRole    = errors.New("no policy configured for role")
	ErrEmptyFallbackChain = errors.New("fallback chain is empty")
)

// RolePolicy defines the profile fallback chain and cooldown settings for a role.
type RolePolicy struct {
	// FallbackChain is the ordered list of profile names to try.
	FallbackChain []string

	// CooldownMinutes is how long to wait after a rate limit before retrying a profile.
	CooldownMinutes int

	// Stickiness is the preferred provider (optional, for future use).
	Stickiness string
}

// Selector manages profile selection with fallback chains and cooldown tracking.
type Selector struct {
	mu        sync.RWMutex
	policies  map[string]RolePolicy
	cooldowns map[string]time.Time // profile -> cooldown until
}

// NewSelector creates a new profile selector.
func NewSelector() *Selector {
	return &Selector{
		policies:  make(map[string]RolePolicy),
		cooldowns: make(map[string]time.Time),
	}
}

// SetPolicy configures the fallback policy for a role.
func (s *Selector) SetPolicy(role string, policy RolePolicy) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.policies[role] = policy
}

// GetPolicy returns the policy for a role, or nil if not configured.
func (s *Selector) GetPolicy(role string) *RolePolicy {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if policy, ok := s.policies[role]; ok {
		return &policy
	}
	return nil
}

// SelectNext returns the next available profile for the role.
// If event is provided, the current profile will be marked as cooling down.
func (s *Selector) SelectNext(role, currentProfile string, event *RateLimitEvent) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	policy, ok := s.policies[role]
	if !ok {
		return "", ErrNoPolicyForRole
	}

	if len(policy.FallbackChain) == 0 {
		return "", ErrEmptyFallbackChain
	}

	// If we have an event, mark the current profile as cooling down
	if event != nil && currentProfile != "" {
		cooldownDuration := time.Duration(policy.CooldownMinutes) * time.Minute
		s.cooldowns[currentProfile] = time.Now().Add(cooldownDuration)
	}

	// Find current profile's position in chain
	currentIdx := -1
	for i, p := range policy.FallbackChain {
		if p == currentProfile {
			currentIdx = i
			break
		}
	}

	// Try each profile in order, starting after current
	chainLen := len(policy.FallbackChain)
	for i := 0; i < chainLen; i++ {
		// Start from next profile after current (or from beginning if current not found)
		idx := (currentIdx + 1 + i) % chainLen
		profile := policy.FallbackChain[idx]

		// Skip if cooling down
		if s.isCooling(profile) {
			continue
		}

		return profile, nil
	}

	return "", ErrAllProfilesCooling
}

// MarkCooldown marks a profile as cooling down until the specified time.
func (s *Selector) MarkCooldown(profile string, until time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cooldowns[profile] = until
}

// IsAvailable checks if a profile is available (not cooling down).
func (s *Selector) IsAvailable(profile string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return !s.isCooling(profile)
}

// isCooling checks if a profile is currently cooling down.
// Must be called with lock held.
func (s *Selector) isCooling(profile string) bool {
	until, ok := s.cooldowns[profile]
	if !ok {
		return false
	}
	return time.Now().Before(until)
}

// ClearCooldown removes the cooldown for a profile.
func (s *Selector) ClearCooldown(profile string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.cooldowns, profile)
}

// CooldownRemaining returns the time remaining in a profile's cooldown.
// Returns zero if the profile is not cooling down.
func (s *Selector) CooldownRemaining(profile string) time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()

	until, ok := s.cooldowns[profile]
	if !ok {
		return 0
	}

	remaining := time.Until(until)
	if remaining < 0 {
		return 0
	}
	return remaining
}
