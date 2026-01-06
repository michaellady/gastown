package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuiltinPresets(t *testing.T) {
	// Ensure all built-in presets are accessible
	presets := []AgentPreset{AgentClaude, AgentGemini, AgentCodex, AgentCursor}

	for _, preset := range presets {
		info := GetAgentPreset(preset)
		if info == nil {
			t.Errorf("GetAgentPreset(%s) returned nil", preset)
			continue
		}

		if info.Command == "" {
			t.Errorf("preset %s has empty Command", preset)
		}

		// All presets should have ProcessNames for agent detection
		if len(info.ProcessNames) == 0 {
			t.Errorf("preset %s has empty ProcessNames", preset)
		}
	}
}

func TestGetAgentPresetByName(t *testing.T) {
	tests := []struct {
		name    string
		want    AgentPreset
		wantNil bool
	}{
		{"claude", AgentClaude, false},
		{"gemini", AgentGemini, false},
		{"codex", AgentCodex, false},
		{"cursor", AgentCursor, false},
		{"aider", "", true},    // Not built-in, can be added via config
		{"opencode", "", true}, // Not built-in, can be added via config
		{"unknown", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetAgentPresetByName(tt.name)
			if tt.wantNil && got != nil {
				t.Errorf("GetAgentPresetByName(%s) = %v, want nil", tt.name, got)
			}
			if !tt.wantNil && got == nil {
				t.Errorf("GetAgentPresetByName(%s) = nil, want preset", tt.name)
			}
			if !tt.wantNil && got != nil && got.Name != tt.want {
				t.Errorf("GetAgentPresetByName(%s).Name = %v, want %v", tt.name, got.Name, tt.want)
			}
		})
	}
}

func TestRuntimeConfigFromPreset(t *testing.T) {
	tests := []struct {
		preset      AgentPreset
		wantCommand string
	}{
		{AgentClaude, "claude"},
		{AgentGemini, "gemini"},
		{AgentCodex, "codex"},
		{AgentCursor, "cursor-agent"},
	}

	for _, tt := range tests {
		t.Run(string(tt.preset), func(t *testing.T) {
			rc := RuntimeConfigFromPreset(tt.preset)
			if rc.Command != tt.wantCommand {
				t.Errorf("RuntimeConfigFromPreset(%s).Command = %v, want %v",
					tt.preset, rc.Command, tt.wantCommand)
			}
		})
	}
}

func TestIsKnownPreset(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"claude", true},
		{"gemini", true},
		{"codex", true},
		{"cursor", true},
		{"aider", false},    // Not built-in, can be added via config
		{"opencode", false}, // Not built-in, can be added via config
		{"unknown", false},
		{"chatgpt", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsKnownPreset(tt.name); got != tt.want {
				t.Errorf("IsKnownPreset(%s) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestLoadAgentRegistry(t *testing.T) {
	// Create temp directory for test config
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "agents.json")

	// Write custom agent config
	customRegistry := AgentRegistry{
		Version: CurrentAgentRegistryVersion,
		Agents: map[string]*AgentPresetInfo{
			"my-agent": {
				Name:    "my-agent",
				Command: "my-agent-bin",
				Args:    []string{"--auto"},
			},
		},
	}

	data, err := json.Marshal(customRegistry)
	if err != nil {
		t.Fatalf("failed to marshal test config: %v", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	// Reset global registry for test isolation
	ResetRegistryForTesting()

	// Load the custom registry
	if err := LoadAgentRegistry(configPath); err != nil {
		t.Fatalf("LoadAgentRegistry failed: %v", err)
	}

	// Check custom agent is available
	myAgent := GetAgentPresetByName("my-agent")
	if myAgent == nil {
		t.Fatal("custom agent 'my-agent' not found after loading registry")
	}
	if myAgent.Command != "my-agent-bin" {
		t.Errorf("my-agent.Command = %v, want my-agent-bin", myAgent.Command)
	}

	// Check built-ins still accessible
	claude := GetAgentPresetByName("claude")
	if claude == nil {
		t.Fatal("built-in 'claude' not found after loading registry")
	}

	// Reset for other tests
	ResetRegistryForTesting()
}

func TestAgentPresetYOLOFlags(t *testing.T) {
	// Verify YOLO flags are set correctly for each E2E tested agent
	tests := []struct {
		preset  AgentPreset
		wantArg string // At least this arg should be present
	}{
		{AgentClaude, "--dangerously-skip-permissions"},
		{AgentGemini, "yolo"}, // Part of "--approval-mode yolo"
		{AgentCodex, "--yolo"},
	}

	for _, tt := range tests {
		t.Run(string(tt.preset), func(t *testing.T) {
			info := GetAgentPreset(tt.preset)
			if info == nil {
				t.Fatalf("preset %s not found", tt.preset)
			}

			found := false
			for _, arg := range info.Args {
				if arg == tt.wantArg || (tt.preset == AgentGemini && arg == "yolo") {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("preset %s args %v missing expected %s", tt.preset, info.Args, tt.wantArg)
			}
		})
	}
}

func TestMergeWithPreset(t *testing.T) {
	// Test that user config overrides preset defaults
	userConfig := &RuntimeConfig{
		Command: "/custom/claude",
		Args:    []string{"--custom-arg"},
	}

	merged := userConfig.MergeWithPreset(AgentClaude)

	if merged.Command != "/custom/claude" {
		t.Errorf("merged command should be user value, got %s", merged.Command)
	}
	if len(merged.Args) != 1 || merged.Args[0] != "--custom-arg" {
		t.Errorf("merged args should be user value, got %v", merged.Args)
	}

	// Test nil config gets preset defaults
	var nilConfig *RuntimeConfig
	merged = nilConfig.MergeWithPreset(AgentClaude)

	if merged.Command != "claude" {
		t.Errorf("nil config merge should get preset command, got %s", merged.Command)
	}

	// Test empty config gets preset defaults
	emptyConfig := &RuntimeConfig{}
	merged = emptyConfig.MergeWithPreset(AgentGemini)

	if merged.Command != "gemini" {
		t.Errorf("empty config merge should get preset command, got %s", merged.Command)
	}
}

func TestBuildResumeCommand(t *testing.T) {
	tests := []struct {
		name      string
		agentName string
		sessionID string
		wantEmpty bool
		contains  []string // strings that should appear in result
	}{
		{
			name:      "claude with session",
			agentName: "claude",
			sessionID: "session-123",
			wantEmpty: false,
			contains:  []string{"claude", "--dangerously-skip-permissions", "--resume", "session-123"},
		},
		{
			name:      "gemini with session",
			agentName: "gemini",
			sessionID: "gemini-sess-456",
			wantEmpty: false,
			contains:  []string{"gemini", "--approval-mode", "yolo", "--resume", "gemini-sess-456"},
		},
		{
			name:      "codex subcommand style",
			agentName: "codex",
			sessionID: "codex-sess-789",
			wantEmpty: false,
			contains:  []string{"codex", "resume", "codex-sess-789", "--yolo"},
		},
		{
			name:      "empty session ID",
			agentName: "claude",
			sessionID: "",
			wantEmpty: true,
		},
		{
			name:      "unknown agent",
			agentName: "unknown-agent",
			sessionID: "session-123",
			wantEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildResumeCommand(tt.agentName, tt.sessionID)
			if tt.wantEmpty {
				if result != "" {
					t.Errorf("BuildResumeCommand(%s, %s) = %q, want empty", tt.agentName, tt.sessionID, result)
				}
				return
			}
			for _, s := range tt.contains {
				if !strings.Contains(result, s) {
					t.Errorf("BuildResumeCommand(%s, %s) = %q, missing %q", tt.agentName, tt.sessionID, result, s)
				}
			}
		})
	}
}

func TestSupportsSessionResume(t *testing.T) {
	tests := []struct {
		agentName string
		want      bool
	}{
		{"claude", true},
		{"gemini", true},
		{"codex", true},
		{"cursor", true},
		{"unknown", false},
	}

	for _, tt := range tests {
		t.Run(tt.agentName, func(t *testing.T) {
			if got := SupportsSessionResume(tt.agentName); got != tt.want {
				t.Errorf("SupportsSessionResume(%s) = %v, want %v", tt.agentName, got, tt.want)
			}
		})
	}
}

func TestGetSessionIDEnvVar(t *testing.T) {
	tests := []struct {
		agentName string
		want      string
	}{
		{"claude", "CLAUDE_SESSION_ID"},
		{"gemini", "GEMINI_SESSION_ID"},
		{"codex", ""},              // Codex uses JSONL output instead
		{"cursor", "CURSOR_CHAT_ID"}, // Cursor uses chat ID
		{"unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.agentName, func(t *testing.T) {
			if got := GetSessionIDEnvVar(tt.agentName); got != tt.want {
				t.Errorf("GetSessionIDEnvVar(%s) = %q, want %q", tt.agentName, got, tt.want)
			}
		})
	}
}

func TestGetProcessNames(t *testing.T) {
	tests := []struct {
		agentName string
		want      []string
	}{
		{"claude", []string{"node"}},
		{"gemini", []string{"gemini"}},
		{"codex", []string{"codex"}},
		{"cursor", []string{"cursor-agent"}},
		{"unknown", []string{"node"}}, // Falls back to Claude's process
	}

	for _, tt := range tests {
		t.Run(tt.agentName, func(t *testing.T) {
			got := GetProcessNames(tt.agentName)
			if len(got) != len(tt.want) {
				t.Errorf("GetProcessNames(%s) = %v, want %v", tt.agentName, got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("GetProcessNames(%s)[%d] = %q, want %q", tt.agentName, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestCursorAgentPreset(t *testing.T) {
	// Verify cursor agent preset is correctly configured
	info := GetAgentPreset(AgentCursor)
	if info == nil {
		t.Fatal("cursor preset not found")
	}

	// Check command
	if info.Command != "cursor-agent" {
		t.Errorf("cursor command = %q, want cursor-agent", info.Command)
	}

	// Check YOLO-equivalent flags (-p for headless, -f for force)
	hasP := false
	hasF := false
	for _, arg := range info.Args {
		if arg == "-p" {
			hasP = true
		}
		if arg == "-f" {
			hasF = true
		}
	}
	if !hasP {
		t.Error("cursor args missing -p (headless mode)")
	}
	if !hasF {
		t.Error("cursor args missing -f (force/YOLO mode)")
	}

	// Check ProcessNames for detection
	if len(info.ProcessNames) == 0 {
		t.Error("cursor ProcessNames is empty")
	}
	if info.ProcessNames[0] != "cursor-agent" {
		t.Errorf("cursor ProcessNames[0] = %q, want cursor-agent", info.ProcessNames[0])
	}

	// Check resume support
	if info.ResumeFlag != "--resume" {
		t.Errorf("cursor ResumeFlag = %q, want --resume", info.ResumeFlag)
	}
	if info.ResumeStyle != "flag" {
		t.Errorf("cursor ResumeStyle = %q, want flag", info.ResumeStyle)
	}
}

func TestListAgentPresetsMatchesConstants(t *testing.T) {
	// Verify that ListAgentPresets() returns all AgentPreset constants.
	// This test prevents future agents from being accidentally omitted
	// from the registry.

	// All known AgentPreset constants
	knownPresets := []AgentPreset{
		AgentClaude,
		AgentGemini,
		AgentCodex,
		AgentCursor,
	}

	// Get presets from registry
	listedPresets := ListAgentPresets()

	// Create a map for quick lookup
	listedMap := make(map[string]bool)
	for _, name := range listedPresets {
		listedMap[name] = true
	}

	// Verify all known constants are in the list
	for _, preset := range knownPresets {
		if !listedMap[string(preset)] {
			t.Errorf("AgentPreset constant %q not found in ListAgentPresets()", preset)
		}
	}

	// Verify the list has at least as many items as constants
	// (could have more if user-defined agents are loaded)
	if len(listedPresets) < len(knownPresets) {
		t.Errorf("ListAgentPresets() returned %d items, expected at least %d",
			len(listedPresets), len(knownPresets))
	}
}

func TestAgentCommandGeneration(t *testing.T) {
	// Test full command line generation for each agent type.
	// Verifies YOLO flags, resume flags, and env var setup.

	tests := []struct {
		name      string
		preset    AgentPreset
		wantCmd   string
		wantYOLO  string // A key YOLO flag that should be present
		wantEnv   string // Session ID env var
		sessionID string
	}{
		{
			name:      "claude fresh start",
			preset:    AgentClaude,
			wantCmd:   "claude",
			wantYOLO:  "--dangerously-skip-permissions",
			wantEnv:   "CLAUDE_SESSION_ID",
			sessionID: "",
		},
		{
			name:      "claude resume",
			preset:    AgentClaude,
			wantCmd:   "claude",
			wantYOLO:  "--dangerously-skip-permissions",
			wantEnv:   "CLAUDE_SESSION_ID",
			sessionID: "claude-sess-123",
		},
		{
			name:      "gemini fresh start",
			preset:    AgentGemini,
			wantCmd:   "gemini",
			wantYOLO:  "yolo",
			wantEnv:   "GEMINI_SESSION_ID",
			sessionID: "",
		},
		{
			name:      "gemini resume",
			preset:    AgentGemini,
			wantCmd:   "gemini",
			wantYOLO:  "yolo",
			wantEnv:   "GEMINI_SESSION_ID",
			sessionID: "gemini-sess-456",
		},
		{
			name:      "codex fresh start",
			preset:    AgentCodex,
			wantCmd:   "codex",
			wantYOLO:  "--yolo",
			wantEnv:   "", // Codex uses JSONL output
			sessionID: "",
		},
		{
			name:      "codex resume",
			preset:    AgentCodex,
			wantCmd:   "codex",
			wantYOLO:  "--yolo",
			wantEnv:   "",
			sessionID: "codex-sess-789",
		},
		{
			name:      "cursor fresh start",
			preset:    AgentCursor,
			wantCmd:   "cursor-agent",
			wantYOLO:  "-f",
			wantEnv:   "CURSOR_CHAT_ID",
			sessionID: "",
		},
		{
			name:      "cursor resume",
			preset:    AgentCursor,
			wantCmd:   "cursor-agent",
			wantYOLO:  "-f",
			wantEnv:   "CURSOR_CHAT_ID",
			sessionID: "cursor-sess-abc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test RuntimeConfig generation
			rc := RuntimeConfigFromPreset(tt.preset)
			if rc.Command != tt.wantCmd {
				t.Errorf("RuntimeConfigFromPreset(%s).Command = %q, want %q",
					tt.preset, rc.Command, tt.wantCmd)
			}

			// Verify YOLO flag is present
			foundYOLO := false
			for _, arg := range rc.Args {
				if arg == tt.wantYOLO {
					foundYOLO = true
					break
				}
			}
			if !foundYOLO {
				t.Errorf("RuntimeConfigFromPreset(%s).Args = %v, missing YOLO flag %q",
					tt.preset, rc.Args, tt.wantYOLO)
			}

			// Verify session ID env var
			gotEnv := GetSessionIDEnvVar(string(tt.preset))
			if gotEnv != tt.wantEnv {
				t.Errorf("GetSessionIDEnvVar(%s) = %q, want %q",
					tt.preset, gotEnv, tt.wantEnv)
			}

			// Test resume command generation
			if tt.sessionID != "" {
				resumeCmd := BuildResumeCommand(string(tt.preset), tt.sessionID)
				if resumeCmd == "" {
					t.Errorf("BuildResumeCommand(%s, %s) returned empty string",
						tt.preset, tt.sessionID)
				}
				// Should contain the command
				if !strings.Contains(resumeCmd, tt.wantCmd) {
					t.Errorf("BuildResumeCommand result %q missing command %q",
						resumeCmd, tt.wantCmd)
				}
				// Should contain the session ID
				if !strings.Contains(resumeCmd, tt.sessionID) {
					t.Errorf("BuildResumeCommand result %q missing session ID %q",
						resumeCmd, tt.sessionID)
				}
				// Should contain YOLO flag
				if !strings.Contains(resumeCmd, tt.wantYOLO) {
					t.Errorf("BuildResumeCommand result %q missing YOLO flag %q",
						resumeCmd, tt.wantYOLO)
				}
			}
		})
	}
}

func TestAgentCommandGeneration_ResumeStyles(t *testing.T) {
	// Test the two resume styles: "flag" and "subcommand"

	// Claude uses flag style: claude --resume <id>
	claudeResume := BuildResumeCommand("claude", "sess-123")
	if !strings.Contains(claudeResume, "--resume sess-123") {
		t.Errorf("Claude resume should use flag style, got: %s", claudeResume)
	}

	// Codex uses subcommand style: codex resume <id>
	codexResume := BuildResumeCommand("codex", "sess-456")
	if !strings.Contains(codexResume, "codex resume sess-456") {
		t.Errorf("Codex resume should use subcommand style, got: %s", codexResume)
	}

	// Verify codex resume has YOLO flag after session ID
	if !strings.Contains(codexResume, "--yolo") {
		t.Errorf("Codex resume should include --yolo, got: %s", codexResume)
	}
}
