package ratelimit

import (
	"testing"
	"time"
)

func TestSelector_SelectNext_FirstAvailable(t *testing.T) {
	s := NewSelector()
	s.SetPolicy("polecat", RolePolicy{
		FallbackChain:   []string{"profile-a", "profile-b", "profile-c"},
		CooldownMinutes: 5,
	})

	profile, err := s.SelectNext("polecat", "profile-a", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should select next in chain after current
	if profile != "profile-b" {
		t.Errorf("expected profile-b, got %s", profile)
	}
}

func TestSelector_SelectNext_SkipsCoolingDown(t *testing.T) {
	s := NewSelector()
	s.SetPolicy("polecat", RolePolicy{
		FallbackChain:   []string{"profile-a", "profile-b", "profile-c"},
		CooldownMinutes: 5,
	})

	// Mark profile-b as cooling down
	s.MarkCooldown("profile-b", time.Now().Add(5*time.Minute))

	profile, err := s.SelectNext("polecat", "profile-a", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should skip profile-b and select profile-c
	if profile != "profile-c" {
		t.Errorf("expected profile-c, got %s", profile)
	}
}

func TestSelector_SelectNext_AllCoolingDown_ReturnsError(t *testing.T) {
	s := NewSelector()
	s.SetPolicy("polecat", RolePolicy{
		FallbackChain:   []string{"profile-a", "profile-b"},
		CooldownMinutes: 5,
	})

	// Mark all profiles as cooling down
	s.MarkCooldown("profile-a", time.Now().Add(5*time.Minute))
	s.MarkCooldown("profile-b", time.Now().Add(5*time.Minute))

	_, err := s.SelectNext("polecat", "profile-a", nil)
	if err == nil {
		t.Error("expected error when all profiles cooling down")
	}
	if err != ErrAllProfilesCooling {
		t.Errorf("expected ErrAllProfilesCooling, got %v", err)
	}
}

func TestSelector_SelectNext_RespectsChainOrder(t *testing.T) {
	s := NewSelector()
	s.SetPolicy("polecat", RolePolicy{
		FallbackChain:   []string{"primary", "secondary", "tertiary"},
		CooldownMinutes: 5,
	})

	// Current is tertiary, should wrap to primary
	profile, err := s.SelectNext("polecat", "tertiary", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if profile != "primary" {
		t.Errorf("expected primary (wrap around), got %s", profile)
	}
}

func TestSelector_SelectNext_UnknownProfile_StartsFromBeginning(t *testing.T) {
	s := NewSelector()
	s.SetPolicy("polecat", RolePolicy{
		FallbackChain:   []string{"profile-a", "profile-b"},
		CooldownMinutes: 5,
	})

	profile, err := s.SelectNext("polecat", "unknown-profile", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Unknown current profile, should start from first available
	if profile != "profile-a" {
		t.Errorf("expected profile-a, got %s", profile)
	}
}

func TestSelector_IsAvailable_NotCooling(t *testing.T) {
	s := NewSelector()

	if !s.IsAvailable("some-profile") {
		t.Error("profile without cooldown should be available")
	}
}

func TestSelector_IsAvailable_CoolingDown(t *testing.T) {
	s := NewSelector()
	s.MarkCooldown("some-profile", time.Now().Add(5*time.Minute))

	if s.IsAvailable("some-profile") {
		t.Error("profile with active cooldown should not be available")
	}
}

func TestSelector_IsAvailable_CooldownExpired(t *testing.T) {
	s := NewSelector()
	s.MarkCooldown("some-profile", time.Now().Add(-1*time.Minute))

	if !s.IsAvailable("some-profile") {
		t.Error("profile with expired cooldown should be available")
	}
}

func TestSelector_MarkCooldown_UpdatesExisting(t *testing.T) {
	s := NewSelector()

	// Set initial cooldown
	s.MarkCooldown("profile-a", time.Now().Add(1*time.Minute))
	if s.IsAvailable("profile-a") {
		t.Error("profile should be cooling")
	}

	// Update to expired cooldown
	s.MarkCooldown("profile-a", time.Now().Add(-1*time.Minute))
	if !s.IsAvailable("profile-a") {
		t.Error("profile should be available after cooldown update")
	}
}

func TestSelector_NoPolicy_ReturnsError(t *testing.T) {
	s := NewSelector()

	_, err := s.SelectNext("unknown-role", "profile-a", nil)
	if err == nil {
		t.Error("expected error for unknown role")
	}
}

func TestSelector_EmptyChain_ReturnsError(t *testing.T) {
	s := NewSelector()
	s.SetPolicy("polecat", RolePolicy{
		FallbackChain:   []string{},
		CooldownMinutes: 5,
	})

	_, err := s.SelectNext("polecat", "profile-a", nil)
	if err == nil {
		t.Error("expected error for empty fallback chain")
	}
}

func TestSelector_CooldownFromEvent(t *testing.T) {
	s := NewSelector()
	s.SetPolicy("polecat", RolePolicy{
		FallbackChain:   []string{"profile-a", "profile-b"},
		CooldownMinutes: 10,
	})

	event := &RateLimitEvent{
		Profile:   "profile-a",
		Timestamp: time.Now(),
	}

	// SelectNext should automatically mark current profile as cooling
	profile, err := s.SelectNext("polecat", "profile-a", event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if profile != "profile-b" {
		t.Errorf("expected profile-b, got %s", profile)
	}

	// profile-a should now be cooling
	if s.IsAvailable("profile-a") {
		t.Error("profile-a should be cooling after rate limit event")
	}
}
