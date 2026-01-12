package ratelimit

import (
	"context"
	"errors"
	"testing"
	"time"
)

// MockSessionOps mocks session operations for testing.
type MockSessionOps struct {
	RunningPolecats map[string]bool           // polecat -> running
	HookedWork      map[string]string         // polecat -> bead ID
	StopCalls       []string                  // polecats that were stopped
	StartCalls      []SessionStartCall        // start calls made
	HookCalls       []HookCall                // hook calls made
	NudgeCalls      []NudgeCall               // nudge calls made
	StopErr         error                     // error to return on stop
	StartErr        error                     // error to return on start
	HookErr         error                     // error to return on hook
}

type SessionStartCall struct {
	RigName     string
	PolecatName string
	Profile     string
}

type HookCall struct {
	BeadID      string
	PolecatName string
}

type NudgeCall struct {
	PolecatName string
	Message     string
}

func NewMockSessionOps() *MockSessionOps {
	return &MockSessionOps{
		RunningPolecats: make(map[string]bool),
		HookedWork:      make(map[string]string),
	}
}

func (m *MockSessionOps) IsRunning(rigName, polecatName string) (bool, error) {
	key := rigName + "/" + polecatName
	return m.RunningPolecats[key], nil
}

func (m *MockSessionOps) Stop(rigName, polecatName string, force bool) error {
	key := rigName + "/" + polecatName
	m.StopCalls = append(m.StopCalls, key)
	if m.StopErr != nil {
		return m.StopErr
	}
	m.RunningPolecats[key] = false
	return nil
}

func (m *MockSessionOps) Start(rigName, polecatName, profile string) (string, error) {
	m.StartCalls = append(m.StartCalls, SessionStartCall{
		RigName:     rigName,
		PolecatName: polecatName,
		Profile:     profile,
	})
	if m.StartErr != nil {
		return "", m.StartErr
	}
	key := rigName + "/" + polecatName
	m.RunningPolecats[key] = true
	return "gt-" + rigName + "-" + polecatName, nil
}

func (m *MockSessionOps) GetHookedWork(rigName, polecatName string) (string, error) {
	key := rigName + "/" + polecatName
	return m.HookedWork[key], nil
}

func (m *MockSessionOps) HookWork(rigName, polecatName, beadID string) error {
	m.HookCalls = append(m.HookCalls, HookCall{
		BeadID:      beadID,
		PolecatName: polecatName,
	})
	if m.HookErr != nil {
		return m.HookErr
	}
	key := rigName + "/" + polecatName
	m.HookedWork[key] = beadID
	return nil
}

func (m *MockSessionOps) Nudge(rigName, polecatName, message string) error {
	m.NudgeCalls = append(m.NudgeCalls, NudgeCall{
		PolecatName: polecatName,
		Message:     message,
	})
	return nil
}

func TestSwapper_TerminatesOldSession(t *testing.T) {
	mock := NewMockSessionOps()
	mock.RunningPolecats["gastown/Toast"] = true

	swapper := NewSwapper(mock)
	req := SwapRequest{
		RigName:     "gastown",
		PolecatName: "Toast",
		OldProfile:  "anthropic_acctA",
		NewProfile:  "anthropic_acctB",
		Reason:      "rate_limit",
	}

	_, err := swapper.Swap(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mock.StopCalls) != 1 {
		t.Errorf("expected 1 stop call, got %d", len(mock.StopCalls))
	}
	if mock.StopCalls[0] != "gastown/Toast" {
		t.Errorf("expected stop call for gastown/Toast, got %s", mock.StopCalls[0])
	}
}

func TestSwapper_SpawnsWithNewProfile(t *testing.T) {
	mock := NewMockSessionOps()
	mock.RunningPolecats["gastown/Toast"] = true

	swapper := NewSwapper(mock)
	req := SwapRequest{
		RigName:     "gastown",
		PolecatName: "Toast",
		OldProfile:  "anthropic_acctA",
		NewProfile:  "openai_acctA",
		Reason:      "rate_limit",
	}

	result, err := swapper.Swap(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mock.StartCalls) != 1 {
		t.Fatalf("expected 1 start call, got %d", len(mock.StartCalls))
	}

	startCall := mock.StartCalls[0]
	if startCall.Profile != "openai_acctA" {
		t.Errorf("expected new profile openai_acctA, got %s", startCall.Profile)
	}
	if startCall.RigName != "gastown" {
		t.Errorf("expected rig gastown, got %s", startCall.RigName)
	}
	if startCall.PolecatName != "Toast" {
		t.Errorf("expected polecat Toast, got %s", startCall.PolecatName)
	}

	if !result.Success {
		t.Error("expected success")
	}
	if result.NewSessionID != "gt-gastown-Toast" {
		t.Errorf("unexpected session ID: %s", result.NewSessionID)
	}
}

func TestSwapper_PreservesHookedWork(t *testing.T) {
	mock := NewMockSessionOps()
	mock.RunningPolecats["gastown/Toast"] = true
	mock.HookedWork["gastown/Toast"] = "gt-123"

	swapper := NewSwapper(mock)
	req := SwapRequest{
		RigName:     "gastown",
		PolecatName: "Toast",
		OldProfile:  "anthropic_acctA",
		NewProfile:  "anthropic_acctB",
		HookedWork:  "gt-123",
		Reason:      "rate_limit",
	}

	_, err := swapper.Swap(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify hook was called to re-attach work
	if len(mock.HookCalls) != 1 {
		t.Fatalf("expected 1 hook call, got %d", len(mock.HookCalls))
	}
	if mock.HookCalls[0].BeadID != "gt-123" {
		t.Errorf("expected bead gt-123, got %s", mock.HookCalls[0].BeadID)
	}
}

func TestSwapper_NudgesNewSession(t *testing.T) {
	mock := NewMockSessionOps()
	mock.RunningPolecats["gastown/Toast"] = true

	swapper := NewSwapper(mock)
	req := SwapRequest{
		RigName:     "gastown",
		PolecatName: "Toast",
		OldProfile:  "anthropic_acctA",
		NewProfile:  "anthropic_acctB",
		Reason:      "rate_limit",
	}

	_, err := swapper.Swap(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mock.NudgeCalls) != 1 {
		t.Fatalf("expected 1 nudge call, got %d", len(mock.NudgeCalls))
	}

	nudge := mock.NudgeCalls[0]
	if nudge.PolecatName != "Toast" {
		t.Errorf("expected nudge to Toast, got %s", nudge.PolecatName)
	}
}

func TestSwapper_HandlesStopError(t *testing.T) {
	mock := NewMockSessionOps()
	mock.RunningPolecats["gastown/Toast"] = true
	mock.StopErr = errors.New("stop failed")

	swapper := NewSwapper(mock)
	req := SwapRequest{
		RigName:     "gastown",
		PolecatName: "Toast",
		OldProfile:  "anthropic_acctA",
		NewProfile:  "anthropic_acctB",
		Reason:      "rate_limit",
	}

	result, err := swapper.Swap(context.Background(), req)
	if err == nil {
		t.Fatal("expected error")
	}

	if result.Success {
		t.Error("expected failure")
	}
}

func TestSwapper_HandlesStartError(t *testing.T) {
	mock := NewMockSessionOps()
	mock.RunningPolecats["gastown/Toast"] = true
	mock.StartErr = errors.New("start failed")

	swapper := NewSwapper(mock)
	req := SwapRequest{
		RigName:     "gastown",
		PolecatName: "Toast",
		OldProfile:  "anthropic_acctA",
		NewProfile:  "anthropic_acctB",
		Reason:      "rate_limit",
	}

	result, err := swapper.Swap(context.Background(), req)
	if err == nil {
		t.Fatal("expected error")
	}

	if result.Success {
		t.Error("expected failure")
	}
}

func TestSwapper_EmitsSwapEvent(t *testing.T) {
	mock := NewMockSessionOps()
	mock.RunningPolecats["gastown/Toast"] = true

	swapper := NewSwapper(mock)
	req := SwapRequest{
		RigName:     "gastown",
		PolecatName: "Toast",
		OldProfile:  "anthropic_acctA",
		NewProfile:  "anthropic_acctB",
		Reason:      "rate_limit",
	}

	result, err := swapper.Swap(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check that swap event was emitted
	if result.Event == nil {
		t.Fatal("expected swap event to be emitted")
	}
	if result.Event.OldProfile != "anthropic_acctA" {
		t.Errorf("expected old profile anthropic_acctA, got %s", result.Event.OldProfile)
	}
	if result.Event.NewProfile != "anthropic_acctB" {
		t.Errorf("expected new profile anthropic_acctB, got %s", result.Event.NewProfile)
	}
	if result.Event.Reason != "rate_limit" {
		t.Errorf("expected reason rate_limit, got %s", result.Event.Reason)
	}
}

func TestSwapper_ContextCancellation(t *testing.T) {
	mock := NewMockSessionOps()
	mock.RunningPolecats["gastown/Toast"] = true

	swapper := NewSwapper(mock)
	req := SwapRequest{
		RigName:     "gastown",
		PolecatName: "Toast",
		OldProfile:  "anthropic_acctA",
		NewProfile:  "anthropic_acctB",
		Reason:      "rate_limit",
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	result, err := swapper.Swap(ctx, req)
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
	if result.Success {
		t.Error("expected failure on cancellation")
	}
}

func TestSwapper_SessionNotRunning(t *testing.T) {
	mock := NewMockSessionOps()
	// Session not running

	swapper := NewSwapper(mock)
	req := SwapRequest{
		RigName:     "gastown",
		PolecatName: "Toast",
		OldProfile:  "anthropic_acctA",
		NewProfile:  "anthropic_acctB",
		Reason:      "rate_limit",
	}

	// Should still try to start new session even if old isn't running
	result, err := swapper.Swap(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should skip stop since not running
	if len(mock.StopCalls) != 0 {
		t.Errorf("expected 0 stop calls for non-running session, got %d", len(mock.StopCalls))
	}

	// Should still start
	if len(mock.StartCalls) != 1 {
		t.Errorf("expected 1 start call, got %d", len(mock.StartCalls))
	}

	if !result.Success {
		t.Error("expected success")
	}
}

func TestSwapEvent_Fields(t *testing.T) {
	event := SwapEvent{
		RigName:      "gastown",
		PolecatName:  "Toast",
		OldProfile:   "anthropic_acctA",
		NewProfile:   "anthropic_acctB",
		Reason:       "rate_limit",
		Timestamp:    time.Now(),
		NewSessionID: "gt-gastown-Toast",
	}

	if event.RigName != "gastown" {
		t.Errorf("expected rig gastown, got %s", event.RigName)
	}
	if event.PolecatName != "Toast" {
		t.Errorf("expected polecat Toast, got %s", event.PolecatName)
	}
}
