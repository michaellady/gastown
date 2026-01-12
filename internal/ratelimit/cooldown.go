package ratelimit

import (
	"sync"
	"time"
)

// CooldownStoreInterface defines the interface for managing profile cooldowns.
type CooldownStoreInterface interface {
	// MarkCooldown marks a profile as cooling down until the specified time.
	MarkCooldown(profile string, until time.Time)

	// ClearCooldown removes a profile from the cooldown list.
	ClearCooldown(profile string)

	// IsAvailable checks if a profile is available (not cooling down).
	IsAvailable(profile string) bool

	// GetCooldownUntil returns when the cooldown ends for a profile.
	// Returns zero time if not cooling down.
	GetCooldownUntil(profile string) time.Time
}

// CooldownStore is an in-memory implementation of CooldownStoreInterface.
type CooldownStore struct {
	mu        sync.RWMutex
	cooldowns map[string]time.Time
}

// NewCooldownStore creates a new CooldownStore.
func NewCooldownStore() *CooldownStore {
	return &CooldownStore{
		cooldowns: make(map[string]time.Time),
	}
}

// MarkCooldown marks a profile as cooling down until the specified time.
func (s *CooldownStore) MarkCooldown(profile string, until time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cooldowns[profile] = until
}

// ClearCooldown removes a profile from the cooldown list.
func (s *CooldownStore) ClearCooldown(profile string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.cooldowns, profile)
}

// IsAvailable checks if a profile is available (not cooling down).
func (s *CooldownStore) IsAvailable(profile string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	until, exists := s.cooldowns[profile]
	if !exists {
		return true
	}

	// Profile is available if cooldown has expired
	return time.Now().After(until)
}

// GetCooldownUntil returns when the cooldown ends for a profile.
// Returns zero time if not cooling down.
func (s *CooldownStore) GetCooldownUntil(profile string) time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cooldowns[profile]
}
