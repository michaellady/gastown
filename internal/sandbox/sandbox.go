// Package sandbox provides macOS sandbox-exec policy composition for Gas Town.
//
// It embeds decomposed SBPL profile fragments, assembles them based on feature
// flags and runtime context, and produces sandbox-exec command tokens for
// insertion into polecat startup commands.
//
// Architecture follows the Agent Safe House pattern (composable numbered layers)
// but implemented natively in Go with embedded profiles.
package sandbox

import (
	"embed"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed profiles/*
var profilesFS embed.FS

// PolicyConfig holds all inputs needed to assemble a sandbox policy.
type PolicyConfig struct {
	// Home is the user home directory (becomes _HOME param).
	Home string

	// TownRoot is the town root directory, e.g. ~/gt.
	TownRoot string

	// RigName is the rig name, e.g. "gastown".
	RigName string

	// Worktree is the polecat worktree path (RW).
	Worktree string

	// Agent is the agent name for profile selection (e.g., "claude-code").
	// If empty, defaults to "claude-code".
	Agent string

	// Features are enabled optional capabilities.
	Features []Feature

	// ExtraDirsRO are additional read-only path grants.
	ExtraDirsRO []string

	// ExtraDirsRW are additional read-write path grants.
	ExtraDirsRW []string

	// Debug enables (debug deny) for sandbox violation logging.
	Debug bool
}

// Policy is an assembled sandbox policy ready to be written to disk.
type Policy struct {
	// SBPL is the assembled policy text.
	SBPL string

	// Params are the sandbox-exec -D parameters.
	Params map[string]string

	// Layers lists the profile fragments that were included.
	Layers []string
}

// Builder assembles sandbox policies from embedded profile fragments.
type Builder struct {
	fs embed.FS
}

// NewBuilder creates a new sandbox policy builder using embedded profiles.
func NewBuilder() *Builder {
	return &Builder{fs: profilesFS}
}

// Assemble reads embedded profile fragments, selects them based on features,
// concatenates them in numbered order, and returns the assembled policy.
func (b *Builder) Assemble(cfg PolicyConfig) (*Policy, error) {
	if cfg.Agent == "" {
		cfg.Agent = "claude-code"
	}

	features := resolveFeatures(cfg.Features)

	// Collect all profile paths from the embedded FS.
	var allPaths []string
	err := fs.WalkDir(b.fs, "profiles", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".sb" {
			return nil
		}
		allPaths = append(allPaths, path)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking embedded profiles: %w", err)
	}

	// Select which profiles to include.
	selected := b.selectProfiles(allPaths, cfg.Agent, features)

	// Sort by filename for deterministic numbered ordering.
	sort.Slice(selected, func(i, j int) bool {
		return profileSortKey(selected[i]) < profileSortKey(selected[j])
	})

	// Concatenate selected profiles.
	var sb strings.Builder
	var layers []string

	for _, path := range selected {
		content, err := fs.ReadFile(b.fs, path)
		if err != nil {
			return nil, fmt.Errorf("reading profile %s: %w", path, err)
		}

		// Strip "profiles/" prefix for layer name.
		layer := strings.TrimPrefix(path, "profiles/")
		layers = append(layers, layer)

		sb.WriteString(";; --- ")
		sb.WriteString(layer)
		sb.WriteString(" ---\n")
		sb.Write(content)
		sb.WriteString("\n")
	}

	// Append debug directive right after version line if requested.
	assembled := sb.String()
	if cfg.Debug {
		assembled = strings.Replace(assembled, "(deny default)", "(deny default)\n(debug deny)", 1)
	}

	// Append dynamic path grants.
	if len(cfg.ExtraDirsRO) > 0 || len(cfg.ExtraDirsRW) > 0 {
		assembled += "\n;; --- dynamic path grants ---\n"
		for _, dir := range cfg.ExtraDirsRO {
			assembled += EmitPathGrant(dir, false)
		}
		for _, dir := range cfg.ExtraDirsRW {
			assembled += EmitPathGrant(dir, true)
		}
	}

	params := map[string]string{
		"_HOME":     cfg.Home,
		"TOWN_ROOT": cfg.TownRoot,
		"RIG_NAME":  cfg.RigName,
		"WORKTREE":  cfg.Worktree,
	}

	return &Policy{
		SBPL:   assembled,
		Params: params,
		Layers: layers,
	}, nil
}

// selectProfiles filters the full list of embedded profiles based on features
// and agent selection.
func (b *Builder) selectProfiles(allPaths []string, agent string, features featureSet) []string {
	var selected []string

	networkOverride := features.has(FeatureNetworkWide)

	for _, path := range allPaths {
		rel := strings.TrimPrefix(path, "profiles/")

		// Skip overrides directory — they are explicitly added below.
		if strings.HasPrefix(rel, "overrides/") {
			continue
		}

		// 20-network-loopback.sb: skip if network-wide override is active.
		if rel == "20-network-loopback.sb" && networkOverride {
			continue
		}

		// 55-gastown-optional/*: only include if matching feature is enabled.
		if strings.HasPrefix(rel, "55-gastown-optional/") {
			feature := optionalProfileToFeature(rel)
			if feature == "" || !features.has(feature) {
				continue
			}
		}

		// 60-agents/*: only include the matching agent profile.
		if strings.HasPrefix(rel, "60-agents/") {
			name := strings.TrimSuffix(filepath.Base(rel), ".sb")
			if name != agent {
				continue
			}
		}

		selected = append(selected, path)
	}

	// Add override profiles for enabled features.
	if networkOverride {
		selected = append(selected, "profiles/overrides/network-wide.sb")
	}
	if features.has(FeatureDocker) {
		selected = append(selected, "profiles/overrides/docker.sb")
	}
	if features.has(FeatureSSH) {
		selected = append(selected, "profiles/overrides/ssh.sb")
	}

	return selected
}

// profileSortKey returns a sort key that orders profiles by their numbered prefix,
// with overrides sorted after the layer they replace.
func profileSortKey(path string) string {
	rel := strings.TrimPrefix(path, "profiles/")

	// Override profiles sort as if they were in the layer they replace:
	// network-wide.sb replaces 20-network-loopback.sb, so sort as "20-..."
	// docker.sb and ssh.sb are additive, sort after 55-*
	switch {
	case strings.Contains(rel, "overrides/network-wide"):
		return "20-network-wide.sb"
	case strings.Contains(rel, "overrides/docker"):
		return "56-docker.sb"
	case strings.Contains(rel, "overrides/ssh"):
		return "56-ssh.sb"
	}

	return rel
}

// optionalProfileToFeature maps a 55-gastown-optional/* profile path to its feature.
func optionalProfileToFeature(rel string) Feature {
	base := strings.TrimSuffix(filepath.Base(rel), ".sb")
	switch base {
	case "beads-write":
		return FeatureBeadsWrite
	case "runtime-write":
		return FeatureRuntimeWrite
	case "keychain":
		return FeatureKeychain
	default:
		return ""
	}
}

// Explain returns a human-readable summary of what the assembled policy contains.
func (b *Builder) Explain(cfg PolicyConfig) (string, error) {
	policy, err := b.Assemble(cfg)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	sb.WriteString("Sandbox Policy Summary\n")
	sb.WriteString("======================\n\n")
	fmt.Fprintf(&sb, "  Home:      %s\n", cfg.Home)
	fmt.Fprintf(&sb, "  TownRoot:  %s\n", cfg.TownRoot)
	fmt.Fprintf(&sb, "  RigName:   %s\n", cfg.RigName)
	fmt.Fprintf(&sb, "  Worktree:  %s\n", cfg.Worktree)
	fmt.Fprintf(&sb, "  Agent:     %s\n", cfg.Agent)
	fmt.Fprintf(&sb, "  Debug:     %v\n", cfg.Debug)

	if len(cfg.Features) > 0 {
		names := make([]string, len(cfg.Features))
		for i, f := range cfg.Features {
			names[i] = string(f)
		}
		fmt.Fprintf(&sb, "  Features:  %s\n", strings.Join(names, ", "))
	}

	if len(cfg.ExtraDirsRO) > 0 {
		fmt.Fprintf(&sb, "  Extra RO:  %s\n", strings.Join(cfg.ExtraDirsRO, ", "))
	}
	if len(cfg.ExtraDirsRW) > 0 {
		fmt.Fprintf(&sb, "  Extra RW:  %s\n", strings.Join(cfg.ExtraDirsRW, ", "))
	}

	sb.WriteString("\nLayers:\n")
	for _, layer := range policy.Layers {
		fmt.Fprintf(&sb, "  - %s\n", layer)
	}

	sb.WriteString(fmt.Sprintf("\nAssembled SBPL: %d bytes\n", len(policy.SBPL)))

	return sb.String(), nil
}
