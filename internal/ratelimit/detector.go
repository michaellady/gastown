// Package ratelimit provides rate limit detection, profile selection, and session swapping
// for handling API rate limits in Gas Town agents.
package ratelimit

import (
	"regexp"
	"strings"
	"time"
)

// Exit codes that indicate rate limiting.
const (
	ExitCodeRateLimit = 2 // Claude Code rate limit exit
)

// rateLimitPatterns are regex patterns that indicate rate limiting in stderr.
var rateLimitPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)429`),
	regexp.MustCompile(`(?i)rate.?limit`),
	regexp.MustCompile(`(?i)too many requests`),
	regexp.MustCompile(`(?i)overloaded`),
	regexp.MustCompile(`(?i)at capacity`),
}

// RateLimitEvent represents a detected rate limit occurrence.
type RateLimitEvent struct {
	AgentID      string    `json:"agent_id"`
	Profile      string    `json:"profile"`
	Timestamp    time.Time `json:"timestamp"`
	ExitCode     int       `json:"exit_code"`
	ErrorSnippet string    `json:"error_snippet"`
	Provider     string    `json:"provider"`
}

// Detector detects rate limit events from exit codes and stderr output.
type Detector struct {
	agentID  string
	profile  string
	provider string
}

// NewDetector creates a new rate limit detector.
func NewDetector() *Detector {
	return &Detector{}
}

// SetAgentInfo sets the agent context for detected events.
func (d *Detector) SetAgentInfo(agentID, profile, provider string) {
	d.agentID = agentID
	d.profile = profile
	d.provider = provider
}

// Detect checks if the given exit code and stderr indicate a rate limit.
// Returns the event details and whether a rate limit was detected.
func (d *Detector) Detect(exitCode int, stderr string) (*RateLimitEvent, bool) {
	// Check exit code first (most reliable signal)
	if exitCode == ExitCodeRateLimit {
		return d.createEvent(exitCode, extractSnippet(stderr)), true
	}

	// Check stderr patterns
	if matchesRateLimitPattern(stderr) {
		return d.createEvent(exitCode, extractSnippet(stderr)), true
	}

	return nil, false
}

// createEvent builds a RateLimitEvent with current timestamp.
func (d *Detector) createEvent(exitCode int, snippet string) *RateLimitEvent {
	return &RateLimitEvent{
		AgentID:      d.agentID,
		Profile:      d.profile,
		Provider:     d.provider,
		Timestamp:    time.Now(),
		ExitCode:     exitCode,
		ErrorSnippet: snippet,
	}
}

// matchesRateLimitPattern checks if stderr matches any rate limit pattern.
func matchesRateLimitPattern(stderr string) bool {
	if stderr == "" {
		return false
	}
	for _, pattern := range rateLimitPatterns {
		if pattern.MatchString(stderr) {
			return true
		}
	}
	return false
}

// extractSnippet extracts a relevant portion of stderr for the event.
// Limits output to first 200 characters to avoid bloating events.
func extractSnippet(stderr string) string {
	stderr = strings.TrimSpace(stderr)
	if len(stderr) > 200 {
		return stderr[:200] + "..."
	}
	return stderr
}
