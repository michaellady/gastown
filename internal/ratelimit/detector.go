package ratelimit

import "time"

// RateLimitEvent represents a detected rate limit occurrence
type RateLimitEvent struct {
	AgentID      string
	Profile      string
	Timestamp    time.Time
	ExitCode     int
	ErrorSnippet string
	Provider     string
}

// Detector detects rate limiting from process exit codes and stderr
type Detector interface {
	Detect(exitCode int, stderr string) (*RateLimitEvent, bool)
}

// detector implements the Detector interface
type detector struct{}

// NewDetector creates a new rate limit detector
func NewDetector() Detector {
	return &detector{}
}

// maxSnippetLen is the maximum length of error snippets stored in events
const maxSnippetLen = 500

// Detect checks if an exit code and stderr indicate rate limiting
// Returns the rate limit event and true if detected, nil and false otherwise
func (d *detector) Detect(exitCode int, stderr string) (*RateLimitEvent, bool) {
	isRateLimit := isRateLimitExitCode(exitCode) || matchesRateLimitPattern(stderr)

	if !isRateLimit {
		return nil, false
	}

	snippet := stderr
	if len(snippet) > maxSnippetLen {
		snippet = snippet[:maxSnippetLen]
	}

	return &RateLimitEvent{
		Timestamp:    time.Now(),
		ExitCode:     exitCode,
		ErrorSnippet: snippet,
	}, true
}
