// Package convoy provides convoy lifecycle state management.
package convoy

import (
	"time"
)

// WorkState represents the lifecycle work state of a convoy.
// This tracks the actual work progress, separate from the beads open/closed status.
type WorkState string

const (
	// WorkStateActive means polecat is actively working (recent tool calls).
	WorkStateActive WorkState = "active"

	// WorkStateIdle means polecat session exists but Claude is waiting at prompt.
	WorkStateIdle WorkState = "idle"

	// WorkStateStuck means polecat has stalled (no progress for extended period).
	WorkStateStuck WorkState = "stuck"

	// WorkStatePRPending means work complete, PR created, awaiting merge.
	WorkStatePRPending WorkState = "pr-pending"

	// WorkStateComplete means PR merged, convoy can be closed.
	WorkStateComplete WorkState = "complete"

	// WorkStateWaiting means no worker assigned yet.
	WorkStateWaiting WorkState = "waiting"
)

// Thresholds for state transitions.
const (
	// ThresholdIdle is the time of no activity before transitioning active → idle.
	ThresholdIdle = 5 * time.Minute

	// ThresholdStuck is the time of no progress before transitioning to stuck.
	ThresholdStuck = 30 * time.Minute
)

// IsTerminal returns true if the convoy work is complete.
func (s WorkState) IsTerminal() bool {
	return s == WorkStateComplete
}

// IsWorking returns true if work is actively happening.
func (s WorkState) IsWorking() bool {
	return s == WorkStateActive || s == WorkStateIdle
}

// NeedsAttention returns true if the convoy may need intervention.
func (s WorkState) NeedsAttention() bool {
	return s == WorkStateStuck || s == WorkStateWaiting
}

// Symbol returns a single-character symbol for the state.
func (s WorkState) Symbol() string {
	switch s {
	case WorkStateActive:
		return "▶"
	case WorkStateIdle:
		return "◑"
	case WorkStateStuck:
		return "!"
	case WorkStatePRPending:
		return "⏳"
	case WorkStateComplete:
		return "✓"
	case WorkStateWaiting:
		return "○"
	default:
		return "?"
	}
}

// Color returns the color name for dashboard display.
func (s WorkState) Color() string {
	switch s {
	case WorkStateActive:
		return "green"
	case WorkStateIdle:
		return "yellow"
	case WorkStateStuck:
		return "red"
	case WorkStatePRPending:
		return "blue"
	case WorkStateComplete:
		return "green"
	case WorkStateWaiting:
		return "dim"
	default:
		return "dim"
	}
}

// String returns the state as a string.
func (s WorkState) String() string {
	return string(s)
}

// StateInfo holds detailed convoy state information.
type StateInfo struct {
	// State is the current work state.
	State WorkState `json:"state"`

	// LastActivity is the timestamp of last polecat activity.
	LastActivity time.Time `json:"last_activity,omitempty"`

	// StateChangedAt is when the current state was entered.
	StateChangedAt time.Time `json:"state_changed_at,omitempty"`

	// PRURL is the PR URL if in pr-pending state.
	PRURL string `json:"pr_url,omitempty"`

	// PRNumber is the PR number if in pr-pending state.
	PRNumber int `json:"pr_number,omitempty"`

	// Worker is the currently assigned worker identity.
	Worker string `json:"worker,omitempty"`

	// DurationInState is how long the convoy has been in current state.
	DurationInState time.Duration `json:"duration_in_state,omitempty"`
}

// CalculateState determines the convoy work state from activity data.
// Parameters:
//   - hasWorker: whether a worker is assigned
//   - lastActivity: timestamp of last polecat activity
//   - completed: number of completed tracked issues
//   - total: total number of tracked issues
//   - hasPR: whether a PR exists for this convoy's work
//   - prMerged: whether the PR has been merged
func CalculateState(hasWorker bool, lastActivity time.Time, completed, total int, hasPR, prMerged bool) WorkState {
	// Complete: PR merged or all work done
	if prMerged || (total > 0 && completed == total) {
		return WorkStateComplete
	}

	// PR pending: PR exists but not merged
	if hasPR {
		return WorkStatePRPending
	}

	// No worker assigned
	if !hasWorker {
		return WorkStateWaiting
	}

	// Worker assigned - check activity
	if lastActivity.IsZero() {
		return WorkStateWaiting
	}

	elapsed := time.Since(lastActivity)

	// Stuck: no activity for 30+ minutes
	if elapsed >= ThresholdStuck {
		return WorkStateStuck
	}

	// Idle: no activity for 5+ minutes
	if elapsed >= ThresholdIdle {
		return WorkStateIdle
	}

	// Active: recent activity
	return WorkStateActive
}

// ParseWorkState parses a string into a WorkState.
// Returns WorkStateWaiting for unknown values.
func ParseWorkState(s string) WorkState {
	switch WorkState(s) {
	case WorkStateActive, WorkStateIdle, WorkStateStuck,
		WorkStatePRPending, WorkStateComplete, WorkStateWaiting:
		return WorkState(s)
	default:
		return WorkStateWaiting
	}
}

// ValidTransition checks if a state transition is valid.
// Returns true if transitioning from 'from' to 'to' is allowed.
func ValidTransition(from, to WorkState) bool {
	// Complete is terminal - no transitions out
	if from == WorkStateComplete {
		return false
	}

	// Any state can go to complete (via pr merge or manual)
	if to == WorkStateComplete {
		return true
	}

	// Define valid transitions
	validTransitions := map[WorkState][]WorkState{
		WorkStateWaiting:   {WorkStateActive, WorkStateIdle},
		WorkStateActive:    {WorkStateIdle, WorkStateStuck, WorkStatePRPending},
		WorkStateIdle:      {WorkStateActive, WorkStateStuck, WorkStatePRPending},
		WorkStateStuck:     {WorkStateActive, WorkStateIdle, WorkStatePRPending},
		WorkStatePRPending: {WorkStateActive, WorkStateIdle}, // PR closed without merge
	}

	allowed, ok := validTransitions[from]
	if !ok {
		return false
	}

	for _, s := range allowed {
		if s == to {
			return true
		}
	}

	return false
}
