package sandbox

import (
	"os"
	"strings"
	"testing"
)

func TestBuildCommandTokens(t *testing.T) {
	tokens, err := BuildCommandTokens(PolicyConfig{
		Home:     "/Users/test",
		TownRoot: "/Users/test/gt",
		RigName:  "gastown",
		Worktree: "/Users/test/gt/gastown/polecats/nova/gastown",
	})
	if err != nil {
		t.Fatalf("BuildCommandTokens failed: %v", err)
	}

	if len(tokens) == 0 {
		t.Fatal("expected non-empty tokens")
	}

	if tokens[0] != "sandbox-exec" {
		t.Errorf("first token should be 'sandbox-exec', got %q", tokens[0])
	}

	// Check that -D params are present.
	joined := strings.Join(tokens, " ")
	if !strings.Contains(joined, "-D _HOME=/Users/test") {
		t.Error("missing _HOME param")
	}
	if !strings.Contains(joined, "-D TOWN_ROOT=/Users/test/gt") {
		t.Error("missing TOWN_ROOT param")
	}
	if !strings.Contains(joined, "-D RIG_NAME=gastown") {
		t.Error("missing RIG_NAME param")
	}
	if !strings.Contains(joined, "-D WORKTREE=") {
		t.Error("missing WORKTREE param")
	}

	// Check that -f points to a real file.
	fIdx := -1
	for i, tok := range tokens {
		if tok == "-f" {
			fIdx = i
			break
		}
	}
	if fIdx == -1 || fIdx+1 >= len(tokens) {
		t.Fatal("missing -f flag in tokens")
	}
	policyPath := tokens[fIdx+1]
	if _, err := os.Stat(policyPath); err != nil {
		t.Errorf("policy file should exist at %q: %v", policyPath, err)
	}

	// Clean up.
	os.Remove(policyPath)
}

func TestWritePolicyFile_Deduplication(t *testing.T) {
	content := "(version 1)\n(deny default)\n"

	path1, err := WritePolicyFile(content)
	if err != nil {
		t.Fatalf("first write failed: %v", err)
	}
	defer os.Remove(path1)

	path2, err := WritePolicyFile(content)
	if err != nil {
		t.Fatalf("second write failed: %v", err)
	}

	if path1 != path2 {
		t.Errorf("same content should produce same path: %q != %q", path1, path2)
	}
}

func TestWritePolicyFile_DifferentContent(t *testing.T) {
	path1, err := WritePolicyFile("(version 1)\n(deny default)\n")
	if err != nil {
		t.Fatalf("first write failed: %v", err)
	}
	defer os.Remove(path1)

	path2, err := WritePolicyFile("(version 1)\n(allow process-fork)\n")
	if err != nil {
		t.Fatalf("second write failed: %v", err)
	}
	defer os.Remove(path2)

	if path1 == path2 {
		t.Error("different content should produce different paths")
	}
}
