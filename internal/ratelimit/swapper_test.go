package ratelimit

import (
	"context"
	"testing"
)

// MockSessionController is a test double for session operations.
type MockSessionController struct {
	StopCalled    bool
	StartCalled   bool
	StopError     error
	StartError    error
	StoppedAgent  string
	StartedAgent  string
	StartedOpts   MockStartOptions
	HookedWork    string
	NudgedMessage string
}

type MockStartOptions struct {
	Account string
}

func (m *MockSessionController) StopSession(rig, polecat string, force bool) error {
	m.StopCalled = true
	m.StoppedAgent = polecat
	return m.StopError
}

func (m *MockSessionController) StartSession(rig, polecat string, opts MockStartOptions) error {
	m.StartCalled = true
	m.StartedAgent = polecat
	m.StartedOpts = opts
	return m.StartError
}

func (m *MockSessionController) HookWork(rig, polecat, beadID string) error {
	m.HookedWork = beadID
	return nil
}

func (m *MockSessionController) Nudge(rig, polecat, message string) error {
	m.NudgedMessage = message
	return nil
}

func TestSwapper_TerminatesOldSession(t *testing.T) {
	mock := &MockSessionController{}
	s := NewSwapper(mock)

	req := SwapRequest{
		RigName:     "testrig",
		PolecatName: "polecat-1",
		OldProfile:  "profile-a",
		NewProfile:  "profile-b",
		Reason:      "rate_limit",
	}

	_, _ = s.Swap(context.Background(), req)

	if !mock.StopCalled {
		t.Error("expected old session to be stopped")
	}
	if mock.StoppedAgent != "polecat-1" {
		t.Errorf("expected polecat-1 to be stopped, got %s", mock.StoppedAgent)
	}
}

func TestSwapper_SpawnsWithNewProfile(t *testing.T) {
	mock := &MockSessionController{}
	s := NewSwapper(mock)

	req := SwapRequest{
		RigName:     "testrig",
		PolecatName: "polecat-1",
		OldProfile:  "profile-a",
		NewProfile:  "profile-b",
		Reason:      "rate_limit",
	}

	_, _ = s.Swap(context.Background(), req)

	if !mock.StartCalled {
		t.Error("expected new session to be started")
	}
	if mock.StartedOpts.Account != "profile-b" {
		t.Errorf("expected account profile-b, got %s", mock.StartedOpts.Account)
	}
}

func TestSwapper_PreservesHookedWork(t *testing.T) {
	mock := &MockSessionController{}
	s := NewSwapper(mock)

	req := SwapRequest{
		RigName:     "testrig",
		PolecatName: "polecat-1",
		OldProfile:  "profile-a",
		NewProfile:  "profile-b",
		HookedWork:  "issue-123",
		Reason:      "rate_limit",
	}

	_, _ = s.Swap(context.Background(), req)

	if mock.HookedWork != "issue-123" {
		t.Errorf("expected hooked work issue-123, got %s", mock.HookedWork)
	}
}

func TestSwapper_NudgesNewSession(t *testing.T) {
	mock := &MockSessionController{}
	s := NewSwapper(mock)

	req := SwapRequest{
		RigName:     "testrig",
		PolecatName: "polecat-1",
		OldProfile:  "profile-a",
		NewProfile:  "profile-b",
		Reason:      "rate_limit",
	}

	_, _ = s.Swap(context.Background(), req)

	if mock.NudgedMessage == "" {
		t.Error("expected nudge to be sent")
	}
}

func TestSwapper_ReturnsSuccessResult(t *testing.T) {
	mock := &MockSessionController{}
	s := NewSwapper(mock)

	req := SwapRequest{
		RigName:     "testrig",
		PolecatName: "polecat-1",
		OldProfile:  "profile-a",
		NewProfile:  "profile-b",
		Reason:      "rate_limit",
	}

	result, err := s.Swap(context.Background(), req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected success result")
	}
}

func TestSwapper_StopError_ReturnsError(t *testing.T) {
	mock := &MockSessionController{
		StopError: ErrAllProfilesCooling, // using any error
	}
	s := NewSwapper(mock)

	req := SwapRequest{
		RigName:     "testrig",
		PolecatName: "polecat-1",
		OldProfile:  "profile-a",
		NewProfile:  "profile-b",
		Reason:      "rate_limit",
	}

	result, err := s.Swap(context.Background(), req)

	if err == nil {
		t.Error("expected error when stop fails")
	}
	if result != nil && result.Success {
		t.Error("expected failure result")
	}
}

func TestSwapper_StartError_ReturnsError(t *testing.T) {
	mock := &MockSessionController{
		StartError: ErrAllProfilesCooling,
	}
	s := NewSwapper(mock)

	req := SwapRequest{
		RigName:     "testrig",
		PolecatName: "polecat-1",
		OldProfile:  "profile-a",
		NewProfile:  "profile-b",
		Reason:      "rate_limit",
	}

	result, err := s.Swap(context.Background(), req)

	if err == nil {
		t.Error("expected error when start fails")
	}
	if result != nil && result.Success {
		t.Error("expected failure result")
	}
}
