package sandbox

import (
	"strings"
	"testing"
)

func TestAssemble_DefaultConfig(t *testing.T) {
	b := NewBuilder()
	policy, err := b.Assemble(PolicyConfig{
		Home:     "/Users/test",
		TownRoot: "/Users/test/gt",
		RigName:  "gastown",
		Worktree: "/Users/test/gt/gastown/polecats/nova/gastown",
	})
	if err != nil {
		t.Fatalf("Assemble failed: %v", err)
	}

	// Verify base layer present.
	if !strings.Contains(policy.SBPL, "(version 1)") {
		t.Error("missing (version 1)")
	}
	if !strings.Contains(policy.SBPL, "(deny default)") {
		t.Error("missing (deny default)")
	}

	// Verify system runtime present.
	if !strings.Contains(policy.SBPL, "(allow process-fork)") {
		t.Error("missing process-fork from system-runtime")
	}

	// Verify network loopback is included by default.
	if !strings.Contains(policy.SBPL, `(allow network-bind (local ip "localhost:*"))`) {
		t.Error("missing loopback network rule")
	}

	// Verify polecat worktree grant.
	if !strings.Contains(policy.SBPL, `(subpath (param "WORKTREE"))`) {
		t.Error("missing WORKTREE grant")
	}

	// Verify Claude Code agent profile.
	if !strings.Contains(policy.SBPL, `"/.claude"`) {
		t.Error("missing Claude Code agent profile")
	}

	// Verify IPC layer.
	if !strings.Contains(policy.SBPL, "com.apple.system.notification_center") {
		t.Error("missing IPC mach-lookup rules")
	}

	// Verify default features (beads-write, runtime-write, keychain) are included.
	if !strings.Contains(policy.SBPL, "Beads write access") {
		t.Error("default feature beads-write not included")
	}
	if !strings.Contains(policy.SBPL, "Runtime writes") {
		t.Error("default feature runtime-write not included")
	}
	if !strings.Contains(policy.SBPL, "Keychain access") {
		t.Error("default feature keychain not included")
	}

	// Verify params.
	if policy.Params["_HOME"] != "/Users/test" {
		t.Errorf("_HOME param = %q, want /Users/test", policy.Params["_HOME"])
	}
	if policy.Params["TOWN_ROOT"] != "/Users/test/gt" {
		t.Errorf("TOWN_ROOT param = %q, want /Users/test/gt", policy.Params["TOWN_ROOT"])
	}

	t.Logf("Assembled %d bytes, %d layers", len(policy.SBPL), len(policy.Layers))
}

func TestAssemble_LayerOrdering(t *testing.T) {
	b := NewBuilder()
	policy, err := b.Assemble(PolicyConfig{
		Home:     "/Users/test",
		TownRoot: "/Users/test/gt",
		RigName:  "gastown",
		Worktree: "/tmp/wt",
	})
	if err != nil {
		t.Fatalf("Assemble failed: %v", err)
	}

	// Verify ordering: base before runtime before network before shared...
	baseIdx := strings.Index(policy.SBPL, "00-base.sb")
	runtimeIdx := strings.Index(policy.SBPL, "10-system-runtime.sb")
	networkIdx := strings.Index(policy.SBPL, "20-network-loopback.sb")
	ipcIdx := strings.Index(policy.SBPL, "70-ipc.sb")

	if baseIdx == -1 || runtimeIdx == -1 || networkIdx == -1 || ipcIdx == -1 {
		t.Fatal("missing expected layer markers")
	}

	if baseIdx >= runtimeIdx {
		t.Error("00-base should come before 10-system-runtime")
	}
	if runtimeIdx >= networkIdx {
		t.Error("10-system-runtime should come before 20-network")
	}
	if networkIdx >= ipcIdx {
		t.Error("20-network should come before 70-ipc")
	}
}

func TestAssemble_NetworkWideOverride(t *testing.T) {
	b := NewBuilder()
	policy, err := b.Assemble(PolicyConfig{
		Home:     "/Users/test",
		TownRoot: "/Users/test/gt",
		RigName:  "gastown",
		Worktree: "/tmp/wt",
		Features: []Feature{FeatureNetworkWide},
	})
	if err != nil {
		t.Fatalf("Assemble failed: %v", err)
	}

	// Should NOT have loopback-specific rules.
	if strings.Contains(policy.SBPL, `(allow network-bind (local ip "localhost:*"))`) {
		t.Error("loopback profile should be replaced by network-wide")
	}

	// Should have wide network.
	if !strings.Contains(policy.SBPL, "(allow network*)") {
		t.Error("missing network-wide override")
	}
}

func TestAssemble_NoOptionalWithoutFeature(t *testing.T) {
	b := NewBuilder()
	policy, err := b.Assemble(PolicyConfig{
		Home:     "/Users/test",
		TownRoot: "/Users/test/gt",
		RigName:  "gastown",
		Worktree: "/tmp/wt",
		// Default features include beads-write and runtime-write.
		// Docker and SSH should NOT be included.
	})
	if err != nil {
		t.Fatalf("Assemble failed: %v", err)
	}

	if strings.Contains(policy.SBPL, "docker.sock") {
		t.Error("docker profile should not be included without feature flag")
	}
}

func TestAssemble_DockerFeature(t *testing.T) {
	b := NewBuilder()
	policy, err := b.Assemble(PolicyConfig{
		Home:     "/Users/test",
		TownRoot: "/Users/test/gt",
		RigName:  "gastown",
		Worktree: "/tmp/wt",
		Features: []Feature{FeatureDocker},
	})
	if err != nil {
		t.Fatalf("Assemble failed: %v", err)
	}

	if !strings.Contains(policy.SBPL, "docker.sock") {
		t.Error("docker profile should be included when feature is enabled")
	}
}

func TestAssemble_DebugMode(t *testing.T) {
	b := NewBuilder()
	policy, err := b.Assemble(PolicyConfig{
		Home:     "/Users/test",
		TownRoot: "/Users/test/gt",
		RigName:  "gastown",
		Worktree: "/tmp/wt",
		Debug:    true,
	})
	if err != nil {
		t.Fatalf("Assemble failed: %v", err)
	}

	if !strings.Contains(policy.SBPL, "(debug deny)") {
		t.Error("debug mode should insert (debug deny)")
	}
}

func TestAssemble_DynamicPathGrants(t *testing.T) {
	b := NewBuilder()
	policy, err := b.Assemble(PolicyConfig{
		Home:        "/Users/test",
		TownRoot:    "/Users/test/gt",
		RigName:     "gastown",
		Worktree:    "/tmp/wt",
		ExtraDirsRO: []string{"/Users/test/shared/lib"},
		ExtraDirsRW: []string{"/Users/test/scratch"},
	})
	if err != nil {
		t.Fatalf("Assemble failed: %v", err)
	}

	// RO grant.
	if !strings.Contains(policy.SBPL, `(allow file-read* (subpath "/Users/test/shared/lib"))`) {
		t.Error("missing RO path grant")
	}

	// RW grant.
	if !strings.Contains(policy.SBPL, `(allow file-read* file-write* (subpath "/Users/test/scratch"))`) {
		t.Error("missing RW path grant")
	}

	// Ancestor literals.
	if !strings.Contains(policy.SBPL, `(allow file-read-metadata (literal "/Users/test/shared"))`) {
		t.Error("missing ancestor literal for RO path")
	}
}

func TestAssemble_AgentSelection(t *testing.T) {
	b := NewBuilder()

	// Default agent is claude-code.
	policy, err := b.Assemble(PolicyConfig{
		Home:     "/Users/test",
		TownRoot: "/Users/test/gt",
		RigName:  "gastown",
		Worktree: "/tmp/wt",
	})
	if err != nil {
		t.Fatalf("Assemble failed: %v", err)
	}

	if !strings.Contains(policy.SBPL, "Claude Code state") {
		t.Error("default agent should include claude-code profile")
	}

	// Non-existent agent: should not include claude-code profile.
	policy2, err := b.Assemble(PolicyConfig{
		Home:     "/Users/test",
		TownRoot: "/Users/test/gt",
		RigName:  "gastown",
		Worktree: "/tmp/wt",
		Agent:    "nonexistent-agent",
	})
	if err != nil {
		t.Fatalf("Assemble failed: %v", err)
	}

	if strings.Contains(policy2.SBPL, "Claude Code state") {
		t.Error("non-matching agent should not include claude-code profile")
	}
}

func TestAssemble_KeychainFeature(t *testing.T) {
	b := NewBuilder()

	// Keychain is in defaults — verify it's present.
	policy, err := b.Assemble(PolicyConfig{
		Home:     "/Users/test",
		TownRoot: "/Users/test/gt",
		RigName:  "gastown",
		Worktree: "/tmp/wt",
	})
	if err != nil {
		t.Fatalf("Assemble failed: %v", err)
	}

	if !strings.Contains(policy.SBPL, "com.apple.SecurityServer") {
		t.Error("keychain should include SecurityServer mach-lookup")
	}
	if !strings.Contains(policy.SBPL, "Library/Keychains") {
		t.Error("keychain should include Keychains file access")
	}
	if !strings.Contains(policy.SBPL, "com.apple.AppleDatabaseChanged") {
		t.Error("keychain should include AppleDatabaseChanged shm")
	}
}

func TestAssemble_AllFeaturesCombined(t *testing.T) {
	b := NewBuilder()
	policy, err := b.Assemble(PolicyConfig{
		Home:     "/Users/test",
		TownRoot: "/Users/test/gt",
		RigName:  "gastown",
		Worktree: "/tmp/wt",
		Features: []Feature{FeatureDocker, FeatureSSH, FeatureNetworkWide},
	})
	if err != nil {
		t.Fatalf("Assemble failed: %v", err)
	}

	// All defaults should be present.
	if !strings.Contains(policy.SBPL, "Beads write access") {
		t.Error("missing beads-write default")
	}
	if !strings.Contains(policy.SBPL, "com.apple.SecurityServer") {
		t.Error("missing keychain default")
	}

	// All explicit features should be present.
	if !strings.Contains(policy.SBPL, "docker.sock") {
		t.Error("missing docker")
	}
	if !strings.Contains(policy.SBPL, "/.ssh") {
		t.Error("missing ssh")
	}

	// Network-wide should replace loopback.
	if strings.Contains(policy.SBPL, `(allow network-bind (local ip "localhost:*"))`) {
		t.Error("loopback should be replaced by network-wide")
	}
	if !strings.Contains(policy.SBPL, "(allow network*)") {
		t.Error("missing network-wide")
	}
}

func TestAssemble_EmptyFeaturesGetsDefaults(t *testing.T) {
	b := NewBuilder()
	policy, err := b.Assemble(PolicyConfig{
		Home:     "/Users/test",
		TownRoot: "/Users/test/gt",
		RigName:  "gastown",
		Worktree: "/tmp/wt",
		Features: []Feature{}, // Explicit empty — defaults should still apply.
	})
	if err != nil {
		t.Fatalf("Assemble failed: %v", err)
	}

	if !strings.Contains(policy.SBPL, "Beads write access") {
		t.Error("defaults should still apply with empty features")
	}
	if !strings.Contains(policy.SBPL, "Keychain access") {
		t.Error("keychain default should still apply with empty features")
	}
}

func TestExplain(t *testing.T) {
	b := NewBuilder()
	summary, err := b.Explain(PolicyConfig{
		Home:     "/Users/test",
		TownRoot: "/Users/test/gt",
		RigName:  "gastown",
		Worktree: "/tmp/wt",
		Features: []Feature{FeatureDocker},
	})
	if err != nil {
		t.Fatalf("Explain failed: %v", err)
	}

	if !strings.Contains(summary, "TownRoot:") {
		t.Error("explain should include TownRoot")
	}
	if !strings.Contains(summary, "Layers:") {
		t.Error("explain should list layers")
	}
	if !strings.Contains(summary, "docker") {
		t.Error("explain should mention docker feature")
	}
}
