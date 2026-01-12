package ratelimit

import (
	"context"
	"fmt"
	"time"
)

// SwapRequest contains the parameters for a session swap operation.
type SwapRequest struct {
	// RigName is the rig containing the polecat.
	RigName string

	// PolecatName is the name of the polecat to swap.
	PolecatName string

	// OldProfile is the profile that hit the rate limit.
	OldProfile string

	// NewProfile is the profile to switch to.
	NewProfile string

	// HookedWork is the bead ID of work to re-hook after swap.
	HookedWork string

	// Reason describes why the swap is happening.
	Reason string // "rate_limit", "stuck", "manual"
}

// SwapResult contains the outcome of a swap operation.
type SwapResult struct {
	// Success indicates if the swap completed successfully.
	Success bool

	// NewSessionID is the identifier for the new session.
	NewSessionID string

	// Error contains any error that occurred.
	Error error

	// SwappedAt is when the swap completed.
	SwappedAt time.Time
}

// SessionController abstracts session operations for testing.
type SessionController interface {
	StopSession(rig, polecat string, force bool) error
	StartSession(rig, polecat string, opts MockStartOptions) error
	HookWork(rig, polecat, beadID string) error
	Nudge(rig, polecat, message string) error
}

// Swapper handles graceful session replacement.
type Swapper struct {
	controller SessionController
}

// NewSwapper creates a new session swapper.
func NewSwapper(controller SessionController) *Swapper {
	return &Swapper{
		controller: controller,
	}
}

// Swap terminates the old session and starts a new one with a different profile.
// The swap preserves hooked work and nudges the new session to continue.
func (s *Swapper) Swap(ctx context.Context, req SwapRequest) (*SwapResult, error) {
	result := &SwapResult{}

	// Step 1: Stop the old session
	if err := s.controller.StopSession(req.RigName, req.PolecatName, false); err != nil {
		result.Error = fmt.Errorf("stopping old session: %w", err)
		return result, result.Error
	}

	// Step 2: Start new session with the new profile
	startOpts := MockStartOptions{
		Account: req.NewProfile,
	}
	if err := s.controller.StartSession(req.RigName, req.PolecatName, startOpts); err != nil {
		result.Error = fmt.Errorf("starting new session: %w", err)
		return result, result.Error
	}

	// Step 3: Re-hook the work if there was any
	if req.HookedWork != "" {
		if err := s.controller.HookWork(req.RigName, req.PolecatName, req.HookedWork); err != nil {
			// Non-fatal: log but continue
			fmt.Printf("Warning: could not re-hook work %s: %v\n", req.HookedWork, err)
		}
	}

	// Step 4: Nudge the new session to resume
	nudgeMsg := fmt.Sprintf("Resuming from %s swap. Previous profile: %s, New profile: %s",
		req.Reason, req.OldProfile, req.NewProfile)
	if err := s.controller.Nudge(req.RigName, req.PolecatName, nudgeMsg); err != nil {
		// Non-fatal: log but continue
		fmt.Printf("Warning: could not nudge new session: %v\n", err)
	}

	result.Success = true
	result.SwappedAt = time.Now()
	result.NewSessionID = fmt.Sprintf("gt-%s-%s", req.RigName, req.PolecatName)

	return result, nil
}

// EmitSwapEvent emits an audit event for the swap operation.
// This creates a traceable record of profile switches for monitoring.
func EmitSwapEvent(req SwapRequest, result *SwapResult) {
	// In a full implementation, this would emit to a structured logging system
	// or create an audit bead. For now, we just log.
	if result.Success {
		fmt.Printf("[SWAP] %s/%s: %s -> %s (reason: %s)\n",
			req.RigName, req.PolecatName, req.OldProfile, req.NewProfile, req.Reason)
	} else {
		fmt.Printf("[SWAP FAILED] %s/%s: %s -> %s (reason: %s, error: %v)\n",
			req.RigName, req.PolecatName, req.OldProfile, req.NewProfile, req.Reason, result.Error)
	}
}
