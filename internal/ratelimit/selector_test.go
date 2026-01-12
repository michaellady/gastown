package ratelimit

import (
	"testing"
	"time"
)

func TestSelectProfile_FirstAvailable(t *testing.T) {
	store := NewCooldownStore()
	selector := NewSelector(store)

	policy := RolePolicy{
		FallbackChain:   []string{"profile-a", "profile-b", "profile-c"},
		CooldownMinutes: 5,
	}

	profile, err := selector.SelectNext(policy, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if profile != "profile-a" {
		t.Errorf("got %q, want %q", profile, "profile-a")
	}
}

func TestSelectProfile_SkipCoolingDown(t *testing.T) {
	store := NewCooldownStore()
	selector := NewSelector(store)

	// Mark first profile as cooling down
	store.MarkCooldown("profile-a", time.Now().Add(5*time.Minute))

	policy := RolePolicy{
		FallbackChain:   []string{"profile-a", "profile-b", "profile-c"},
		CooldownMinutes: 5,
	}

	profile, err := selector.SelectNext(policy, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if profile != "profile-b" {
		t.Errorf("got %q, want %q (profile-a is cooling down)", profile, "profile-b")
	}
}

func TestSelectProfile_AllCoolingDown_ReturnsError(t *testing.T) {
	store := NewCooldownStore()
	selector := NewSelector(store)

	// Mark all profiles as cooling down
	store.MarkCooldown("profile-a", time.Now().Add(5*time.Minute))
	store.MarkCooldown("profile-b", time.Now().Add(5*time.Minute))
	store.MarkCooldown("profile-c", time.Now().Add(5*time.Minute))

	policy := RolePolicy{
		FallbackChain:   []string{"profile-a", "profile-b", "profile-c"},
		CooldownMinutes: 5,
	}

	_, err := selector.SelectNext(policy, "")
	if err == nil {
		t.Fatal("expected error when all profiles are cooling down")
	}
	if err != ErrAllProfilesCoolingDown {
		t.Errorf("got error %v, want ErrAllProfilesCoolingDown", err)
	}
}

func TestSelectProfile_RespectsOrder(t *testing.T) {
	store := NewCooldownStore()
	selector := NewSelector(store)

	// Mark first two profiles as cooling down
	store.MarkCooldown("profile-a", time.Now().Add(5*time.Minute))
	store.MarkCooldown("profile-b", time.Now().Add(5*time.Minute))

	policy := RolePolicy{
		FallbackChain:   []string{"profile-a", "profile-b", "profile-c", "profile-d"},
		CooldownMinutes: 5,
	}

	// Should get profile-c (first available in order)
	profile, err := selector.SelectNext(policy, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if profile != "profile-c" {
		t.Errorf("got %q, want %q (respecting chain order)", profile, "profile-c")
	}
}

func TestSelectProfile_ExpiredCooldown(t *testing.T) {
	store := NewCooldownStore()
	selector := NewSelector(store)

	// Mark profile as cooling down with expired time
	store.MarkCooldown("profile-a", time.Now().Add(-1*time.Minute))

	policy := RolePolicy{
		FallbackChain:   []string{"profile-a", "profile-b"},
		CooldownMinutes: 5,
	}

	// profile-a should be available again since cooldown expired
	profile, err := selector.SelectNext(policy, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if profile != "profile-a" {
		t.Errorf("got %q, want %q (expired cooldown)", profile, "profile-a")
	}
}

func TestSelectProfile_EmptyChain(t *testing.T) {
	store := NewCooldownStore()
	selector := NewSelector(store)

	policy := RolePolicy{
		FallbackChain:   []string{},
		CooldownMinutes: 5,
	}

	_, err := selector.SelectNext(policy, "")
	if err == nil {
		t.Fatal("expected error for empty fallback chain")
	}
	if err != ErrEmptyFallbackChain {
		t.Errorf("got error %v, want ErrEmptyFallbackChain", err)
	}
}

func TestSelectProfile_WithStickiness(t *testing.T) {
	store := NewCooldownStore()
	selector := NewSelector(store)

	policy := RolePolicy{
		FallbackChain:   []string{"profile-a", "profile-b", "profile-c"},
		CooldownMinutes: 5,
		Stickiness:      "profile-b", // Prefer profile-b
	}

	// Should get profile-b due to stickiness, even though profile-a is first in chain
	profile, err := selector.SelectNext(policy, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if profile != "profile-b" {
		t.Errorf("got %q, want %q (stickiness preference)", profile, "profile-b")
	}
}

func TestSelectProfile_StickinessIgnoredWhenCoolingDown(t *testing.T) {
	store := NewCooldownStore()
	selector := NewSelector(store)

	// Mark sticky profile as cooling down
	store.MarkCooldown("profile-b", time.Now().Add(5*time.Minute))

	policy := RolePolicy{
		FallbackChain:   []string{"profile-a", "profile-b", "profile-c"},
		CooldownMinutes: 5,
		Stickiness:      "profile-b",
	}

	// Should fall back to profile-a since sticky profile is cooling down
	profile, err := selector.SelectNext(policy, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if profile != "profile-a" {
		t.Errorf("got %q, want %q (sticky profile cooling down)", profile, "profile-a")
	}
}

func TestCooldownStore_IsAvailable(t *testing.T) {
	store := NewCooldownStore()

	// Initially available
	if !store.IsAvailable("profile-a") {
		t.Error("profile-a should be available initially")
	}

	// Mark as cooling down
	store.MarkCooldown("profile-a", time.Now().Add(5*time.Minute))

	// Should not be available
	if store.IsAvailable("profile-a") {
		t.Error("profile-a should not be available while cooling down")
	}
}

func TestCooldownStore_ClearCooldown(t *testing.T) {
	store := NewCooldownStore()

	store.MarkCooldown("profile-a", time.Now().Add(5*time.Minute))
	if store.IsAvailable("profile-a") {
		t.Error("profile-a should not be available while cooling down")
	}

	store.ClearCooldown("profile-a")
	if !store.IsAvailable("profile-a") {
		t.Error("profile-a should be available after clearing cooldown")
	}
}

func TestCooldownStore_GetCooldownUntil(t *testing.T) {
	store := NewCooldownStore()

	until := time.Now().Add(5 * time.Minute)
	store.MarkCooldown("profile-a", until)

	got := store.GetCooldownUntil("profile-a")
	if got.IsZero() {
		t.Fatal("expected non-zero time")
	}
	if !got.Equal(until) {
		t.Errorf("got %v, want %v", got, until)
	}
}
