package ratelimit

import "regexp"

// Exit codes that indicate rate limiting
const (
	ExitCodeRateLimit = 2 // Claude Code rate limit exit
)

// rateLimitPatterns are regex patterns that indicate rate limiting in stderr
var rateLimitPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)429`),
	regexp.MustCompile(`(?i)rate.?limit`),
	regexp.MustCompile(`(?i)too many requests`),
	regexp.MustCompile(`(?i)overloaded`),
	regexp.MustCompile(`(?i)capacity`),
}

// isRateLimitExitCode returns true if the exit code indicates rate limiting
func isRateLimitExitCode(exitCode int) bool {
	return exitCode == ExitCodeRateLimit
}

// matchesRateLimitPattern returns true if stderr matches any rate limit pattern
func matchesRateLimitPattern(stderr string) bool {
	for _, pattern := range rateLimitPatterns {
		if pattern.MatchString(stderr) {
			return true
		}
	}
	return false
}
