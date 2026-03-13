//go:build linux

package sandbox

import (
	"slices"
	"testing"
)

func TestBuildCommandTokensLinux_Basic(t *testing.T) {
	tokens, err := BuildCommandTokensLinux(PolicyConfig{
		Home:     "/home/test",
		TownRoot: "/home/test/gt",
		RigName:  "gastown",
		Worktree: "/home/test/gt/gastown/polecats/nova/gastown",
	})
	if err != nil {
		t.Fatalf("BuildCommandTokensLinux failed: %v", err)
	}

	if tokens[0] != "bwrap" {
		t.Errorf("first token should be bwrap, got %q", tokens[0])
	}

	// Should have namespace isolation.
	if !slices.Contains(tokens, "--unshare-all") {
		t.Error("missing --unshare-all")
	}
	if !slices.Contains(tokens, "--share-net") {
		t.Error("missing --share-net (needed for loopback/HTTPS)")
	}

	// Should bind the worktree RW.
	for i, tok := range tokens {
		if tok == "--bind" && i+2 < len(tokens) && tokens[i+1] == "/home/test/gt/gastown/polecats/nova/gastown" {
			return // Found worktree bind.
		}
	}
	t.Error("missing worktree --bind")
}

func TestBuildCommandTokensLinux_DockerFeature(t *testing.T) {
	tokens, err := BuildCommandTokensLinux(PolicyConfig{
		Home:     "/home/test",
		TownRoot: "/home/test/gt",
		RigName:  "gastown",
		Worktree: "/tmp/wt",
		Features: []Feature{FeatureDocker},
	})
	if err != nil {
		t.Fatalf("BuildCommandTokensLinux failed: %v", err)
	}

	found := false
	for i, tok := range tokens {
		if tok == "--bind-try" && i+1 < len(tokens) && tokens[i+1] == "/var/run/docker.sock" {
			found = true
			break
		}
	}
	if !found {
		t.Error("docker feature should bind docker.sock")
	}
}

func TestBuildCommandTokensLinux_ExtraDirs(t *testing.T) {
	tokens, err := BuildCommandTokensLinux(PolicyConfig{
		Home:        "/home/test",
		TownRoot:    "/home/test/gt",
		RigName:     "gastown",
		Worktree:    "/tmp/wt",
		ExtraDirsRO: []string{"/opt/shared"},
		ExtraDirsRW: []string{"/data/scratch"},
	})
	if err != nil {
		t.Fatalf("BuildCommandTokensLinux failed: %v", err)
	}

	foundRO := false
	foundRW := false
	for i, tok := range tokens {
		if tok == "--ro-bind" && i+1 < len(tokens) && tokens[i+1] == "/opt/shared" {
			foundRO = true
		}
		if tok == "--bind" && i+1 < len(tokens) && tokens[i+1] == "/data/scratch" {
			foundRW = true
		}
	}
	if !foundRO {
		t.Error("missing --ro-bind for extra RO dir")
	}
	if !foundRW {
		t.Error("missing --bind for extra RW dir")
	}
}

func TestBuildCommandTokensLinux_CwdSetToWorktree(t *testing.T) {
	tokens, err := BuildCommandTokensLinux(PolicyConfig{
		Home:     "/home/test",
		TownRoot: "/home/test/gt",
		RigName:  "gastown",
		Worktree: "/tmp/wt",
	})
	if err != nil {
		t.Fatalf("BuildCommandTokensLinux failed: %v", err)
	}

	for i, tok := range tokens {
		if tok == "--chdir" && i+1 < len(tokens) && tokens[i+1] == "/tmp/wt" {
			return
		}
	}
	t.Error("should set --chdir to worktree")
}
