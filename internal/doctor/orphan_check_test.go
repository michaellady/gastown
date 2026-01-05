package doctor

import (
	"os"
	"testing"
)

func TestIsCrewSession(t *testing.T) {
	tests := []struct {
		name     string
		session  string
		expected bool
	}{
		{
			name:     "crew session format",
			session:  "gt-gastown-crew-joe",
			expected: true,
		},
		{
			name:     "crew session with hyphen in name",
			session:  "gt-gastown-crew-mary-jane",
			expected: true,
		},
		{
			name:     "witness session",
			session:  "gt-gastown-witness",
			expected: false,
		},
		{
			name:     "refinery session",
			session:  "gt-gastown-refinery",
			expected: false,
		},
		{
			name:     "polecat session",
			session:  "gt-gastown-nux",
			expected: false,
		},
		{
			name:     "mayor session",
			session:  "gt-gt1-mayor",
			expected: false,
		},
		{
			name:     "deacon session",
			session:  "gt-gt1-deacon",
			expected: false,
		},
		{
			name:     "non-gastown session",
			session:  "my-custom-session",
			expected: false,
		},
		{
			name:     "short session name",
			session:  "gt-rig",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isCrewSession(tt.session)
			if result != tt.expected {
				t.Errorf("isCrewSession(%q) = %v, want %v", tt.session, result, tt.expected)
			}
		})
	}
}

func TestGetCurrentProcessAncestors(t *testing.T) {
	check := &OrphanProcessCheck{}

	ancestors := check.getCurrentProcessAncestors()

	// Should include current process PID
	currentPID := os.Getpid()
	if !ancestors[currentPID] {
		t.Errorf("getCurrentProcessAncestors() should include current PID %d", currentPID)
	}

	// Should have at least one entry (the current process)
	if len(ancestors) < 1 {
		t.Error("getCurrentProcessAncestors() should return at least one entry")
	}

	// Should not include PID 0 or 1 (init/launchd)
	if ancestors[0] {
		t.Error("getCurrentProcessAncestors() should not include PID 0")
	}
	if ancestors[1] {
		t.Error("getCurrentProcessAncestors() should not include PID 1")
	}
}

func TestGetCurrentProcessAncestors_ContainsParent(t *testing.T) {
	check := &OrphanProcessCheck{}

	ancestors := check.getCurrentProcessAncestors()

	// Get parent PID using os.Getppid()
	parentPID := os.Getppid()

	// Parent should be in ancestors (unless we're at the top of the tree)
	if parentPID > 1 && !ancestors[parentPID] {
		t.Errorf("getCurrentProcessAncestors() should include parent PID %d", parentPID)
	}
}

func TestOrphanSessionCheck_New(t *testing.T) {
	check := NewOrphanSessionCheck()

	if check.Name() != "orphan-sessions" {
		t.Errorf("Name() = %q, want %q", check.Name(), "orphan-sessions")
	}

	if check.Description() != "Detect orphaned tmux sessions" {
		t.Errorf("Description() = %q, want %q", check.Description(), "Detect orphaned tmux sessions")
	}
}

func TestOrphanProcessCheck_New(t *testing.T) {
	check := NewOrphanProcessCheck()

	if check.Name() != "orphan-processes" {
		t.Errorf("Name() = %q, want %q", check.Name(), "orphan-processes")
	}

	if check.Description() != "Detect orphaned Claude processes" {
		t.Errorf("Description() = %q, want %q", check.Description(), "Detect orphaned Claude processes")
	}
}

func TestOrphanProcessCheck_FixSkipsAncestors(t *testing.T) {
	check := &OrphanProcessCheck{}

	// Simulate orphan PIDs that include current process ancestors
	currentPID := os.Getpid()
	parentPID := os.Getppid()

	check.orphanPIDs = []int{currentPID, parentPID}

	// Fix should not kill our own ancestors
	// Since we can't easily verify kills didn't happen,
	// we verify the check completes without error (no self-kill)
	ctx := &CheckContext{TownRoot: "/tmp"}
	err := check.Fix(ctx)

	// Should complete without error (not killing itself)
	if err != nil {
		t.Errorf("Fix() returned error: %v", err)
	}
}

func TestOrphanSessionCheck_IsValidSession(t *testing.T) {
	check := NewOrphanSessionCheck()
	validRigs := []string{"gastown", "beads"}
	mayorSession := "hq-gt1-mayor"
	deaconSession := "hq-gt1-deacon"

	tests := []struct {
		name     string
		session  string
		expected bool
	}{
		{
			name:     "mayor session",
			session:  "hq-gt1-mayor",
			expected: true,
		},
		{
			name:     "deacon session",
			session:  "hq-gt1-deacon",
			expected: true,
		},
		{
			name:     "witness session",
			session:  "gt-gastown-witness",
			expected: true,
		},
		{
			name:     "refinery session",
			session:  "gt-gastown-refinery",
			expected: true,
		},
		{
			name:     "polecat session",
			session:  "gt-gastown-nux",
			expected: true,
		},
		{
			name:     "unknown rig",
			session:  "gt-unknownrig-witness",
			expected: false,
		},
		{
			name:     "malformed session - too short",
			session:  "gt-rig",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := check.isValidSession(tt.session, validRigs, mayorSession, deaconSession)
			if result != tt.expected {
				t.Errorf("isValidSession(%q) = %v, want %v", tt.session, result, tt.expected)
			}
		})
	}
}
