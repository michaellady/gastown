package convoy

import (
	"testing"
	"time"
)

func TestWorkStateSymbol(t *testing.T) {
	tests := []struct {
		state    WorkState
		expected string
	}{
		{WorkStateActive, "▶"},
		{WorkStateIdle, "◑"},
		{WorkStateStuck, "!"},
		{WorkStatePRPending, "⏳"},
		{WorkStateComplete, "✓"},
		{WorkStateWaiting, "○"},
	}

	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			if got := tt.state.Symbol(); got != tt.expected {
				t.Errorf("Symbol() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestWorkStateColor(t *testing.T) {
	tests := []struct {
		state    WorkState
		expected string
	}{
		{WorkStateActive, "green"},
		{WorkStateIdle, "yellow"},
		{WorkStateStuck, "red"},
		{WorkStatePRPending, "blue"},
		{WorkStateComplete, "green"},
		{WorkStateWaiting, "dim"},
	}

	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			if got := tt.state.Color(); got != tt.expected {
				t.Errorf("Color() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestCalculateState(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name         string
		hasWorker    bool
		lastActivity time.Time
		completed    int
		total        int
		hasPR        bool
		prMerged     bool
		expected     WorkState
	}{
		{
			name:     "complete - all done",
			total:    3,
			completed: 3,
			expected: WorkStateComplete,
		},
		{
			name:     "complete - PR merged",
			prMerged: true,
			expected: WorkStateComplete,
		},
		{
			name:     "pr-pending",
			hasPR:    true,
			expected: WorkStatePRPending,
		},
		{
			name:     "waiting - no worker",
			total:    3,
			expected: WorkStateWaiting,
		},
		{
			name:         "active - recent activity",
			hasWorker:    true,
			lastActivity: now.Add(-2 * time.Minute),
			total:        3,
			expected:     WorkStateActive,
		},
		{
			name:         "idle - 5+ min inactive",
			hasWorker:    true,
			lastActivity: now.Add(-10 * time.Minute),
			total:        3,
			expected:     WorkStateIdle,
		},
		{
			name:         "stuck - 30+ min inactive",
			hasWorker:    true,
			lastActivity: now.Add(-45 * time.Minute),
			total:        3,
			expected:     WorkStateStuck,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateState(tt.hasWorker, tt.lastActivity, tt.completed, tt.total, tt.hasPR, tt.prMerged)
			if got != tt.expected {
				t.Errorf("CalculateState() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestValidTransition(t *testing.T) {
	tests := []struct {
		from     WorkState
		to       WorkState
		expected bool
	}{
		// Complete is terminal
		{WorkStateComplete, WorkStateActive, false},
		{WorkStateComplete, WorkStateWaiting, false},

		// Any state can go to complete
		{WorkStateActive, WorkStateComplete, true},
		{WorkStateStuck, WorkStateComplete, true},
		{WorkStateWaiting, WorkStateComplete, true},

		// Valid transitions
		{WorkStateWaiting, WorkStateActive, true},
		{WorkStateActive, WorkStateIdle, true},
		{WorkStateActive, WorkStateStuck, true},
		{WorkStateActive, WorkStatePRPending, true},
		{WorkStateIdle, WorkStateActive, true},
		{WorkStateStuck, WorkStateActive, true},

		// Invalid transitions
		{WorkStateWaiting, WorkStateStuck, false},
		{WorkStatePRPending, WorkStateStuck, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.from)+"->"+string(tt.to), func(t *testing.T) {
			if got := ValidTransition(tt.from, tt.to); got != tt.expected {
				t.Errorf("ValidTransition(%v, %v) = %v, want %v", tt.from, tt.to, got, tt.expected)
			}
		})
	}
}

func TestIsTerminal(t *testing.T) {
	if !WorkStateComplete.IsTerminal() {
		t.Error("Complete should be terminal")
	}
	if WorkStateActive.IsTerminal() {
		t.Error("Active should not be terminal")
	}
}

func TestIsWorking(t *testing.T) {
	if !WorkStateActive.IsWorking() {
		t.Error("Active should be working")
	}
	if !WorkStateIdle.IsWorking() {
		t.Error("Idle should be working")
	}
	if WorkStateStuck.IsWorking() {
		t.Error("Stuck should not be working")
	}
}

func TestNeedsAttention(t *testing.T) {
	if !WorkStateStuck.NeedsAttention() {
		t.Error("Stuck should need attention")
	}
	if !WorkStateWaiting.NeedsAttention() {
		t.Error("Waiting should need attention")
	}
	if WorkStateActive.NeedsAttention() {
		t.Error("Active should not need attention")
	}
}

func TestParseWorkState(t *testing.T) {
	tests := []struct {
		input    string
		expected WorkState
	}{
		{"active", WorkStateActive},
		{"idle", WorkStateIdle},
		{"stuck", WorkStateStuck},
		{"pr-pending", WorkStatePRPending},
		{"complete", WorkStateComplete},
		{"waiting", WorkStateWaiting},
		{"unknown", WorkStateWaiting}, // Unknown defaults to waiting
		{"", WorkStateWaiting},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := ParseWorkState(tt.input); got != tt.expected {
				t.Errorf("ParseWorkState(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}
