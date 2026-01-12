package ratelimit

import (
	"testing"
	"time"
)

func TestDetect_ExitCode(t *testing.T) {
	d := NewDetector()

	tests := []struct {
		name       string
		exitCode   int
		stderr     string
		wantDetect bool
	}{
		{
			name:       "exit code 2 detects rate limit",
			exitCode:   2,
			stderr:     "",
			wantDetect: true,
		},
		{
			name:       "exit code 0 no rate limit",
			exitCode:   0,
			stderr:     "",
			wantDetect: false,
		},
		{
			name:       "exit code 1 no rate limit",
			exitCode:   1,
			stderr:     "",
			wantDetect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, detected := d.Detect(tt.exitCode, tt.stderr)
			if detected != tt.wantDetect {
				t.Errorf("Detect() detected = %v, want %v", detected, tt.wantDetect)
			}
			if tt.wantDetect && event == nil {
				t.Error("Detect() returned nil event when rate limit detected")
			}
			if !tt.wantDetect && event != nil {
				t.Error("Detect() returned event when no rate limit")
			}
		})
	}
}

func TestDetect_StderrPatterns(t *testing.T) {
	d := NewDetector()

	tests := []struct {
		name       string
		exitCode   int
		stderr     string
		wantDetect bool
	}{
		{
			name:       "429 in stderr",
			exitCode:   1,
			stderr:     "Error: received 429 response",
			wantDetect: true,
		},
		{
			name:       "rate limit phrase",
			exitCode:   1,
			stderr:     "Error: rate limit exceeded",
			wantDetect: true,
		},
		{
			name:       "rate-limit with hyphen",
			exitCode:   1,
			stderr:     "rate-limit hit",
			wantDetect: true,
		},
		{
			name:       "too many requests",
			exitCode:   1,
			stderr:     "too many requests, please wait",
			wantDetect: true,
		},
		{
			name:       "overloaded message",
			exitCode:   1,
			stderr:     "API is overloaded",
			wantDetect: true,
		},
		{
			name:       "capacity message",
			exitCode:   1,
			stderr:     "insufficient capacity",
			wantDetect: true,
		},
		{
			name:       "case insensitive rate limit",
			exitCode:   1,
			stderr:     "RATE LIMIT reached",
			wantDetect: true,
		},
		{
			name:       "no rate limit patterns",
			exitCode:   1,
			stderr:     "Error: file not found",
			wantDetect: false,
		},
		{
			name:       "empty stderr",
			exitCode:   1,
			stderr:     "",
			wantDetect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, detected := d.Detect(tt.exitCode, tt.stderr)
			if detected != tt.wantDetect {
				t.Errorf("Detect() detected = %v, want %v", detected, tt.wantDetect)
			}
			if tt.wantDetect && event == nil {
				t.Error("Detect() returned nil event when rate limit detected")
			}
		})
	}
}

func TestDetect_NoRateLimit(t *testing.T) {
	d := NewDetector()

	tests := []struct {
		name     string
		exitCode int
		stderr   string
	}{
		{
			name:     "success exit",
			exitCode: 0,
			stderr:   "",
		},
		{
			name:     "generic error",
			exitCode: 1,
			stderr:   "Error: syntax error in file",
		},
		{
			name:     "timeout error",
			exitCode: 124,
			stderr:   "Error: operation timed out",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, detected := d.Detect(tt.exitCode, tt.stderr)
			if detected {
				t.Errorf("Detect() incorrectly detected rate limit")
			}
			if event != nil {
				t.Errorf("Detect() returned non-nil event for non-rate-limit case")
			}
		})
	}
}

func TestRateLimitEvent_Fields(t *testing.T) {
	d := NewDetector()

	event, detected := d.Detect(2, "rate limit exceeded")
	if !detected {
		t.Fatal("expected rate limit to be detected")
	}

	if event.ExitCode != 2 {
		t.Errorf("ExitCode = %d, want 2", event.ExitCode)
	}
	if event.ErrorSnippet != "rate limit exceeded" {
		t.Errorf("ErrorSnippet = %q, want %q", event.ErrorSnippet, "rate limit exceeded")
	}
	if event.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}
	if time.Since(event.Timestamp) > time.Second {
		t.Error("Timestamp should be recent")
	}
}

func TestDetect_ErrorSnippetTruncation(t *testing.T) {
	d := NewDetector()

	longStderr := "rate limit " + string(make([]byte, 1000))
	event, detected := d.Detect(1, longStderr)
	if !detected {
		t.Fatal("expected rate limit to be detected")
	}

	if len(event.ErrorSnippet) > 500 {
		t.Errorf("ErrorSnippet should be truncated, got len=%d", len(event.ErrorSnippet))
	}
}
