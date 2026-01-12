package ratelimit

import (
	"testing"
	"time"
)

func TestDetector_ExitCode2_IsRateLimit(t *testing.T) {
	d := NewDetector()

	event, isRateLimit := d.Detect(2, "")
	if !isRateLimit {
		t.Error("exit code 2 should be detected as rate limit")
	}
	if event == nil {
		t.Fatal("expected non-nil event")
	}
	if event.ExitCode != 2 {
		t.Errorf("expected exit code 2, got %d", event.ExitCode)
	}
}

func TestDetector_ExitCode0_NotRateLimit(t *testing.T) {
	d := NewDetector()

	_, isRateLimit := d.Detect(0, "")
	if isRateLimit {
		t.Error("exit code 0 should not be detected as rate limit")
	}
}

func TestDetector_ExitCode1_NotRateLimit(t *testing.T) {
	d := NewDetector()

	_, isRateLimit := d.Detect(1, "")
	if isRateLimit {
		t.Error("exit code 1 should not be detected as rate limit")
	}
}

func TestDetector_Stderr429_IsRateLimit(t *testing.T) {
	d := NewDetector()

	event, isRateLimit := d.Detect(1, "Error: 429 Too Many Requests")
	if !isRateLimit {
		t.Error("stderr containing 429 should be detected as rate limit")
	}
	if event == nil {
		t.Fatal("expected non-nil event")
	}
	if event.ErrorSnippet == "" {
		t.Error("expected error snippet to be populated")
	}
}

func TestDetector_StderrRateLimit_IsRateLimit(t *testing.T) {
	tests := []struct {
		name   string
		stderr string
	}{
		{"rate limit words", "You have hit a rate limit. Please wait."},
		{"rate_limit underscore", "rate_limit exceeded"},
		{"too many requests", "Error: too many requests"},
		{"overloaded", "The API is overloaded, please retry later"},
		{"capacity", "We're at capacity right now"},
	}

	d := NewDetector()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, isRateLimit := d.Detect(1, tt.stderr)
			if !isRateLimit {
				t.Errorf("stderr %q should be detected as rate limit", tt.stderr)
			}
		})
	}
}

func TestDetector_StderrNormal_NotRateLimit(t *testing.T) {
	tests := []struct {
		name   string
		stderr string
	}{
		{"empty", ""},
		{"normal error", "Error: file not found"},
		{"syntax error", "SyntaxError: unexpected token"},
		{"connection error", "Connection refused"},
	}

	d := NewDetector()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, isRateLimit := d.Detect(0, tt.stderr)
			if isRateLimit {
				t.Errorf("stderr %q should not be detected as rate limit", tt.stderr)
			}
		})
	}
}

func TestDetector_EventTimestamp(t *testing.T) {
	d := NewDetector()
	before := time.Now()

	event, isRateLimit := d.Detect(2, "")
	if !isRateLimit {
		t.Fatal("expected rate limit detection")
	}

	after := time.Now()

	if event.Timestamp.Before(before) || event.Timestamp.After(after) {
		t.Error("event timestamp should be set to current time")
	}
}

func TestDetector_WithAgentInfo(t *testing.T) {
	d := NewDetector()
	d.SetAgentInfo("polecat-1", "default", "anthropic")

	event, isRateLimit := d.Detect(2, "")
	if !isRateLimit {
		t.Fatal("expected rate limit detection")
	}

	if event.AgentID != "polecat-1" {
		t.Errorf("expected AgentID 'polecat-1', got %q", event.AgentID)
	}
	if event.Profile != "default" {
		t.Errorf("expected Profile 'default', got %q", event.Profile)
	}
	if event.Provider != "anthropic" {
		t.Errorf("expected Provider 'anthropic', got %q", event.Provider)
	}
}
