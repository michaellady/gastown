package sandbox

// AllowedEnvVars defines environment variables that are safe to pass through
// to sandboxed agent sessions. This serves as documentation and validation —
// the actual env filtering happens in config.SanitizeAgentEnv and the
// exec env VAR=val construction in BuildStartupCommand.
var AllowedEnvVars = []string{
	// System essentials
	"HOME", "USER", "LOGNAME", "SHELL", "TERM", "LANG", "LC_ALL",
	"PATH", "TMPDIR", "SDKROOT",
	"EDITOR", "VISUAL",
	"XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_DATA_HOME",

	// Gas Town
	"GT_ROOT", "GT_RIG", "GT_ROLE", "GT_POLECAT", "GT_POLECAT_PATH",
	"GT_BRANCH", "GT_TOWN_ROOT", "GT_RUN", "GT_CREW",
	"GT_AGENT", "GT_PROCESS_NAMES", "GT_SESSION_ID_ENV",
	"BD_ACTOR", "BD_DOLT_AUTO_COMMIT", "BD_DOLT_HOST", "BD_DOLT_PORT",

	// Agent runtime
	"ANTHROPIC_API_KEY", "CLAUDE_CONFIG_DIR", "CLAUDE_SESSION_ID",
	"GIT_AUTHOR_NAME", "GIT_AUTHOR_EMAIL",
	"GIT_COMMITTER_NAME", "GIT_COMMITTER_EMAIL",
	"NODE_OPTIONS",
	"POLECAT_SLOT",

	// Observability
	"OTEL_RESOURCE_ATTRIBUTES", "OTEL_EXPORTER_OTLP_ENDPOINT",
}

// IsAllowedEnvVar returns true if the variable name is in the allowlist.
func IsAllowedEnvVar(name string) bool {
	for _, allowed := range AllowedEnvVars {
		if allowed == name {
			return true
		}
	}
	return false
}
