package sandbox

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// BuildCommandTokens assembles the sandbox policy and returns the tokens
// to insert into the startup command's ExecWrapper position.
//
// The returned tokens look like:
//
//	["sandbox-exec", "-D", "_HOME=/Users/x", "-D", "TOWN_ROOT=/Users/x/gt",
//	 "-D", "RIG_NAME=gastown", "-D", "WORKTREE=/path/to/worktree",
//	 "-f", "/tmp/gt-sandbox-abc123.sb"]
//
// These are inserted between `exec env VAR=val ...` and the agent command.
func BuildCommandTokens(cfg PolicyConfig) ([]string, error) {
	builder := NewBuilder()
	policy, err := builder.Assemble(cfg)
	if err != nil {
		return nil, fmt.Errorf("assembling sandbox policy: %w", err)
	}

	policyPath, err := WritePolicyFile(policy.SBPL)
	if err != nil {
		return nil, fmt.Errorf("writing sandbox policy: %w", err)
	}

	// Build sandbox-exec tokens with deterministic param ordering.
	tokens := []string{"sandbox-exec"}

	// Sort params for deterministic output.
	paramKeys := make([]string, 0, len(policy.Params))
	for k := range policy.Params {
		paramKeys = append(paramKeys, k)
	}
	sort.Strings(paramKeys)

	for _, k := range paramKeys {
		tokens = append(tokens, "-D", k+"="+policy.Params[k])
	}
	tokens = append(tokens, "-f", policyPath)

	return tokens, nil
}

// WritePolicyFile writes assembled SBPL to a content-hashed temp file.
// If a file with the same hash already exists, it is reused.
// Returns the path to the policy file.
func WritePolicyFile(sbpl string) (string, error) {
	h := sha256.Sum256([]byte(sbpl))
	name := fmt.Sprintf("gt-sandbox-%x.sb", h[:8])
	path := filepath.Join(os.TempDir(), name)

	// Reuse if same content hash already exists.
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}

	if err := os.WriteFile(path, []byte(sbpl), 0600); err != nil {
		return "", fmt.Errorf("writing policy file: %w", err)
	}
	return path, nil
}
