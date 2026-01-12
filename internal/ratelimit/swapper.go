// Package ratelimit provides rate limit handling for Gas Town agents.
package ratelimit

import (
	"context"
	"fmt"
	"time"
)

// SwapRequest contains the parameters for swapping a polecat session.
type SwapRequest struct {
	RigName     string // Rig name (e.g., "gastown")
	PolecatName string // Polecat name (e.g., "Toast")
	OldProfile  string // Current profile being replaced
	NewProfile  string // New profile to switch to
	HookedWork  string // Bead ID of hooked work (if any)
	Reason      string // Reason for swap: "rate_limit", "stuck", "manual"
}

// SwapResult contains the outcome of a swap operation.
type SwapResult struct {
	Success      bool       // Whether the swap completed successfully
	NewSessionID string     // Session ID of the new session
	Error        error      // Error if swap failed
	Event        *SwapEvent // Event record for audit
}

// SwapEvent records a swap for audit purposes.
type SwapEvent struct {
	RigName      string    // Rig name
	PolecatName  string    // Polecat name
	OldProfile   string    // Previous profile
	NewProfile   string    // New profile
	Reason       string    // Reason for swap
	Timestamp    time.Time // When the swap occurred
	NewSessionID string    // New session ID
	HookedWork   string    // Work that was re-hooked (if any)
}

// SessionOps defines the interface for session operations.
// This allows for mocking in tests.
type SessionOps interface {
	IsRunning(rigName, polecatName string) (bool, error)
	Stop(rigName, polecatName string, force bool) error
	Start(rigName, polecatName, profile string) (string, error)
	GetHookedWork(rigName, polecatName string) (string, error)
	HookWork(rigName, polecatName, beadID string) error
	Nudge(rigName, polecatName, message string) error
}

// Swapper handles graceful replacement of polecat sessions.
type Swapper struct {
	ops SessionOps
}

// NewSwapper creates a new session swapper.
func NewSwapper(ops SessionOps) *Swapper {
	return &Swapper{ops: ops}
}

// Swap terminates the old session and spawns a replacement with a new profile.
// The swap preserves hooked work and nudges the new session to resume.
func (s *Swapper) Swap(ctx context.Context, req SwapRequest) (*SwapResult, error) {
	result := &SwapResult{
		Success: false,
	}

	// Check context early
	if err := ctx.Err(); err != nil {
		result.Error = err
		return result, err
	}

	// Step 1: Check if old session is running
	running, err := s.ops.IsRunning(req.RigName, req.PolecatName)
	if err != nil {
		result.Error = fmt.Errorf("checking session status: %w", err)
		return result, result.Error
	}

	// Step 2: Stop old session if running
	if running {
		if err := s.ops.Stop(req.RigName, req.PolecatName, false); err != nil {
			result.Error = fmt.Errorf("stopping old session: %w", err)
			return result, result.Error
		}
	}

	// Check context after stop
	if err := ctx.Err(); err != nil {
		result.Error = err
		return result, err
	}

	// Step 3: Start new session with new profile
	sessionID, err := s.ops.Start(req.RigName, req.PolecatName, req.NewProfile)
	if err != nil {
		result.Error = fmt.Errorf("starting new session: %w", err)
		return result, result.Error
	}
	result.NewSessionID = sessionID

	// Step 4: Re-hook work if provided
	if req.HookedWork != "" {
		if err := s.ops.HookWork(req.RigName, req.PolecatName, req.HookedWork); err != nil {
			// Log warning but don't fail the swap
			fmt.Printf("Warning: failed to re-hook work %s: %v\n", req.HookedWork, err)
		}
	}

	// Step 5: Nudge new session to resume
	nudgeMsg := fmt.Sprintf("Resuming from %s swap. Profile changed from %s to %s. Check your hook for work.",
		req.Reason, req.OldProfile, req.NewProfile)
	if err := s.ops.Nudge(req.RigName, req.PolecatName, nudgeMsg); err != nil {
		// Log warning but don't fail the swap
		fmt.Printf("Warning: failed to nudge new session: %v\n", err)
	}

	// Step 6: Create swap event for audit
	result.Event = &SwapEvent{
		RigName:      req.RigName,
		PolecatName:  req.PolecatName,
		OldProfile:   req.OldProfile,
		NewProfile:   req.NewProfile,
		Reason:       req.Reason,
		Timestamp:    time.Now(),
		NewSessionID: sessionID,
		HookedWork:   req.HookedWork,
	}

	result.Success = true
	return result, nil
}
