//go:build linux

package sandbox

import (
	"fmt"
	"sort"
	"strings"
)

// BuildCommandTokensLinux assembles a bubblewrap (bwrap) command from PolicyConfig.
// This is the Linux equivalent of BuildCommandTokens (macOS sandbox-exec).
//
// The returned tokens look like:
//
//	["bwrap", "--ro-bind", "/usr", "/usr", "--bind", "/path/to/worktree", "/path/to/worktree",
//	 "--unshare-all", "--share-net", "--dev", "/dev", "--proc", "/proc", ...]
//
// These are inserted between `exec env VAR=val ...` and the agent command.
func BuildCommandTokensLinux(cfg PolicyConfig) ([]string, error) {
	features := resolveFeatures(cfg.Features)

	tokens := []string{"bwrap"}

	// Namespace isolation.
	tokens = append(tokens, "--unshare-all", "--share-net")

	// /proc and /dev are needed for basic operation.
	tokens = append(tokens, "--proc", "/proc")
	tokens = append(tokens, "--dev", "/dev")

	// System directories (read-only).
	for _, dir := range []string{
		"/usr", "/bin", "/sbin", "/lib", "/lib64",
		"/etc/ssl", "/etc/resolv.conf", "/etc/hosts",
		"/etc/passwd", "/etc/group", "/etc/nsswitch.conf",
		"/etc/localtime",
	} {
		tokens = append(tokens, "--ro-bind-try", dir, dir)
	}

	// Homebrew / Linuxbrew (if present).
	tokens = append(tokens, "--ro-bind-try", "/home/linuxbrew", "/home/linuxbrew")

	// Tmpdir.
	tokens = append(tokens, "--tmpfs", "/tmp")

	// User home — selective read-only mounts.
	home := cfg.Home
	tokens = append(tokens, "--tmpfs", home) // Base home as tmpfs to prevent leaks.

	// User tool reads.
	for _, rel := range []string{".local", ".claude", ".npm", ".gitconfig", ".config/git", ".ssh"} {
		tokens = append(tokens, "--ro-bind-try", home+"/"+rel, home+"/"+rel)
	}

	// Claude Code state (RW).
	for _, rel := range []string{".claude", ".cache/claude", ".config/claude", ".local/state/claude", ".local/share/claude", ".npm", ".cache/npm", ".cache/node"} {
		tokens = append(tokens, "--bind-try", home+"/"+rel, home+"/"+rel)
	}

	// Town shared directories (read-only).
	townRoot := cfg.TownRoot
	for _, rel := range []string{".beads", ".runtime", "CLAUDE.md", "mayor", "docs"} {
		tokens = append(tokens, "--ro-bind-try", townRoot+"/"+rel, townRoot+"/"+rel)
	}

	// Rig shared (read-only).
	rigBase := townRoot + "/" + cfg.RigName
	for _, rel := range []string{".beads", ".repo.git", ".git"} {
		tokens = append(tokens, "--ro-bind-try", rigBase+"/"+rel, rigBase+"/"+rel)
	}

	// Worktree — full read-write (primary workspace).
	tokens = append(tokens, "--bind", cfg.Worktree, cfg.Worktree)

	// Feature-gated mounts.
	if features.has(FeatureBeadsWrite) {
		beadsDir := rigBase + "/.beads"
		tokens = append(tokens, "--bind-try", beadsDir, beadsDir)
	}
	if features.has(FeatureRuntimeWrite) {
		runtimeDir := townRoot + "/.runtime"
		tokens = append(tokens, "--bind-try", runtimeDir, runtimeDir)
	}
	if features.has(FeatureDocker) {
		tokens = append(tokens, "--bind-try", "/var/run/docker.sock", "/var/run/docker.sock")
		tokens = append(tokens, "--ro-bind-try", home+"/.docker", home+"/.docker")
	}
	if features.has(FeatureSSH) {
		tokens = append(tokens, "--bind-try", home+"/.ssh", home+"/.ssh")
	}

	// Extra path grants.
	for _, dir := range cfg.ExtraDirsRO {
		tokens = append(tokens, "--ro-bind", dir, dir)
	}
	for _, dir := range cfg.ExtraDirsRW {
		tokens = append(tokens, "--bind", dir, dir)
	}

	// Set CWD to worktree.
	tokens = append(tokens, "--chdir", cfg.Worktree)

	// Set hostname for identification.
	tokens = append(tokens, "--hostname", "gt-sandbox")

	return tokens, nil
}

// ExplainLinux returns a human-readable summary of the bwrap policy.
func ExplainLinux(cfg PolicyConfig) string {
	features := resolveFeatures(cfg.Features)

	var sb strings.Builder
	sb.WriteString("Linux Sandbox Policy Summary (bubblewrap)\n")
	sb.WriteString("==========================================\n\n")
	fmt.Fprintf(&sb, "  Home:      %s\n", cfg.Home)
	fmt.Fprintf(&sb, "  TownRoot:  %s\n", cfg.TownRoot)
	fmt.Fprintf(&sb, "  RigName:   %s\n", cfg.RigName)
	fmt.Fprintf(&sb, "  Worktree:  %s (RW)\n", cfg.Worktree)
	fmt.Fprintf(&sb, "  Agent:     %s\n", cfg.Agent)

	sb.WriteString("\nFeatures:\n")
	featureNames := make([]string, 0)
	for f := range features {
		featureNames = append(featureNames, string(f))
	}
	sort.Strings(featureNames)
	for _, name := range featureNames {
		fmt.Fprintf(&sb, "  - %s\n", name)
	}

	if len(cfg.ExtraDirsRO) > 0 {
		sb.WriteString("\nExtra RO:\n")
		for _, dir := range cfg.ExtraDirsRO {
			fmt.Fprintf(&sb, "  - %s\n", dir)
		}
	}
	if len(cfg.ExtraDirsRW) > 0 {
		sb.WriteString("\nExtra RW:\n")
		for _, dir := range cfg.ExtraDirsRW {
			fmt.Fprintf(&sb, "  - %s\n", dir)
		}
	}

	sb.WriteString("\nIsolation: namespaces (all), shared network\n")
	return sb.String()
}
