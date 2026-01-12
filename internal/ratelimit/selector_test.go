package ratelimit

import (
	"testing"
	"time"
)

func TestSelectProfile_FirstAvailable(t *testing.T) {
	store := NewCooldownStore()
	selector := NewSelector(store)

	policy := RolePolicy{
		FallbackChain:   []string{"primary", "secondary", "tertiary"},
		CooldownMinutes: 5,
	}

	// All profiles available - should return first
	profile, err := selector.SelectNext("worker", policy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if profile != "primary" {
		t.Errorf("got %q, want %q", profile, "primary")
	}
}

func TestSelectProfile_SkipCoolingDown(t *testing.T) {
	store := NewCooldownStore()
	selector := NewSelector(store)

	policy := RolePolicy{
		FallbackChain:   []string{"primary", "secondary", "tertiary"},
		CooldownMinutes: 5,
	}

	// Mark primary as cooling down
	store.MarkCooldown("primary", time.Now().Add(5*time.Minute))

	// Should skip primary, return secondary
	profile, err := selector.SelectNext("worker", policy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if profile != "secondary" {
		t.Errorf("got %q, want %q", profile, "secondary")
	}
}

func TestSelectProfile_AllCoolingDown_ReturnsError(t *testing.T) {
	store := NewCooldownStore()
	selector := NewSelector(store)

	policy := RolePolicy{
		FallbackChain:   []string{"primary", "secondary"},
		CooldownMinutes: 5,
	}

	// Mark all profiles as cooling down
	cooldownUntil := time.Now().Add(5 * time.Minute)
	store.MarkCooldown("primary", cooldownUntil)
	store.MarkCooldown("secondary", cooldownUntil)

	// Should return error
	_, err := selector.SelectNext("worker", policy)
	if err == nil {
		t.Fatal("expected error when all profiles cooling down")
	}
	if err != ErrAllProfilesCoolingDown {
		t.Errorf("got error %v, want ErrAllProfilesCoolingDown", err)
	}
}

func TestSelectProfile_RespectsOrder(t *testing.T) {
	store := NewCooldownStore()
	selector := NewSelector(store)

	// Different order
	policy := RolePolicy{
		FallbackChain:   []string{"tertiary", "primary", "secondary"},
		CooldownMinutes: 5,
	}

	// All available - should return first in chain (tertiary)
	profile, err := selector.SelectNext("worker", policy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if profile != "tertiary" {
		t.Errorf("got %q, want %q", profile, "tertiary")
	}
}

func TestCooldownStore_MarkAndCheck(t *testing.T) {
	store := NewCooldownStore()

	// Initially available
	if !store.IsAvailable("profile1") {
		t.Error("profile1 should be available initially")
	}

	// Mark as cooling down for 5 minutes
	store.MarkCooldown("profile1", time.Now().Add(5*time.Minute))

	// Should not be available
	if store.IsAvailable("profile1") {
		t.Error("profile1 should not be available while cooling down")
	}

	// Other profiles still available
	if !store.IsAvailable("profile2") {
		t.Error("profile2 should still be available")
	}
}

func TestCooldownStore_ExpiredCooldown(t *testing.T) {
	store := NewCooldownStore()

	// Mark with past time (expired cooldown)
	store.MarkCooldown("profile1", time.Now().Add(-1*time.Minute))

	// Should be available again
	if !store.IsAvailable("profile1") {
		t.Error("profile1 should be available after cooldown expires")
	}
}

func TestCooldownStore_ClearCooldown(t *testing.T) {
	store := NewCooldownStore()

	// Mark as cooling down
	store.MarkCooldown("profile1", time.Now().Add(5*time.Minute))
	if store.IsAvailable("profile1") {
		t.Error("profile1 should not be available while cooling down")
	}

	// Clear the cooldown
	store.ClearCooldown("profile1")

	// Should be available again
	if !store.IsAvailable("profile1") {
		t.Error("profile1 should be available after clearing cooldown")
	}
}

func TestSelector_EmptyFallbackChain(t *testing.T) {
	store := NewCooldownStore()
	selector := NewSelector(store)

	policy := RolePolicy{
		FallbackChain:   []string{},
		CooldownMinutes: 5,
	}

	_, err := selector.SelectNext("worker", policy)
	if err == nil {
		t.Fatal("expected error for empty fallback chain")
	}
	if err != ErrNoProfilesConfigured {
		t.Errorf("got error %v, want ErrNoProfilesConfigured", err)
	}
}

func TestCooldownStore_GetCooldownUntil(t *testing.T) {
	store := NewCooldownStore()

	// No cooldown set
	until := store.GetCooldownUntil("profile1")
	if !until.IsZero() {
		t.Error("expected zero time for profile without cooldown")
	}

	// Set cooldown
	expected := time.Now().Add(5 * time.Minute)
	store.MarkCooldown("profile1", expected)

	until = store.GetCooldownUntil("profile1")
	if until.Sub(expected) > time.Second {
		t.Errorf("got cooldown until %v, want %v", until, expected)
	}
}
