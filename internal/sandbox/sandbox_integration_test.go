package sandbox

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func skipIfNotMacOS(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "darwin" {
		t.Skip("sandbox-exec tests only run on macOS")
	}
}

func skipIfNoSandboxExec(t *testing.T) {
	t.Helper()
	skipIfNotMacOS(t)
	if _, err := exec.LookPath("sandbox-exec"); err != nil {
		t.Skip("sandbox-exec not found in PATH")
	}
}

// runInSandbox executes a shell command inside the assembled sandbox policy.
func runInSandbox(t *testing.T, cfg PolicyConfig, shellCmd string) (stdout, stderr string, err error) {
	t.Helper()
	b := NewBuilder()
	policy, assembleErr := b.Assemble(cfg)
	if assembleErr != nil {
		t.Fatalf("Assemble failed: %v", assembleErr)
	}

	policyPath, writeErr := WritePolicyFile(policy.SBPL)
	if writeErr != nil {
		t.Fatalf("WritePolicyFile failed: %v", writeErr)
	}
	defer os.Remove(policyPath)

	args := []string{}
	for k, v := range policy.Params {
		args = append(args, "-D", k+"="+v)
	}
	args = append(args, "-f", policyPath, "/bin/sh", "-c", shellCmd)

	cmd := exec.Command("sandbox-exec", args...)
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err = cmd.Run()
	return outBuf.String(), errBuf.String(), err
}

// TestIntegration_WriteInsideWorktree verifies that writes to the polecat
// worktree succeed under the assembled policy.
func TestIntegration_WriteInsideWorktree(t *testing.T) {
	skipIfNoSandboxExec(t)
	t.Parallel()

	worktree := t.TempDir()
	townRoot := t.TempDir()
	rigDir := filepath.Join(townRoot, "testrig")
	os.MkdirAll(filepath.Join(rigDir, ".beads"), 0755)

	cfg := PolicyConfig{
		Home:     os.Getenv("HOME"),
		TownRoot: townRoot,
		RigName:  "testrig",
		Worktree: worktree,
	}

	testFile := filepath.Join(worktree, "write-test.txt")
	shellCmd := fmt.Sprintf("echo 'hello sandbox' > %q && cat %q", testFile, testFile)
	stdout, stderr, err := runInSandbox(t, cfg, shellCmd)
	if err != nil {
		t.Fatalf("write inside worktree should succeed: err=%v stderr=%q", err, stderr)
	}
	if !strings.Contains(stdout, "hello sandbox") {
		t.Errorf("expected written content, got: %q", stdout)
	}
}

// TestIntegration_DenyWriteOutside verifies that writes outside the worktree
// are denied by the assembled policy.
func TestIntegration_DenyWriteOutside(t *testing.T) {
	skipIfNoSandboxExec(t)
	t.Parallel()

	worktree := t.TempDir()
	townRoot := t.TempDir()

	cfg := PolicyConfig{
		Home:     os.Getenv("HOME"),
		TownRoot: townRoot,
		RigName:  "testrig",
		Worktree: worktree,
	}

	homeDir := os.Getenv("HOME")
	forbiddenFile := filepath.Join(homeDir, ".sandbox-assembled-test-breach")

	shellCmd := fmt.Sprintf("echo 'breach' > %q 2>&1; echo exit=$?", forbiddenFile)
	stdout, _, _ := runInSandbox(t, cfg, shellCmd)

	if _, err := os.Stat(forbiddenFile); err == nil {
		os.Remove(forbiddenFile)
		t.Fatal("sandbox allowed write outside worktree")
	}

	if strings.Contains(stdout, "exit=0") {
		t.Error("expected non-zero exit from write outside worktree")
	}
}

// TestIntegration_DenyExternalNetwork verifies that external network
// connections are denied (loopback + HTTPS + DNS only).
func TestIntegration_DenyExternalNetwork(t *testing.T) {
	skipIfNoSandboxExec(t)
	t.Parallel()

	worktree := t.TempDir()
	townRoot := t.TempDir()

	cfg := PolicyConfig{
		Home:     os.Getenv("HOME"),
		TownRoot: townRoot,
		RigName:  "testrig",
		Worktree: worktree,
	}

	// Try to connect to an external IP (not DNS, not HTTPS) — should be denied.
	shellCmd := `curl -s --connect-timeout 3 http://1.1.1.1:80/ 2>&1; echo "exit=$?"`
	stdout, _, _ := runInSandbox(t, cfg, shellCmd)

	// Port 80 is not in the allow list (only 443 outbound), so this should fail.
	if strings.Contains(stdout, "exit=0") {
		t.Fatal("sandbox allowed external HTTP connection on port 80")
	}
}

// TestIntegration_AllowLoopback verifies that loopback connections work
// (needed for Dolt SQL access on localhost).
func TestIntegration_AllowLoopback(t *testing.T) {
	skipIfNoSandboxExec(t)
	t.Parallel()

	worktree := t.TempDir()
	townRoot := t.TempDir()

	cfg := PolicyConfig{
		Home:     os.Getenv("HOME"),
		TownRoot: townRoot,
		RigName:  "testrig",
		Worktree: worktree,
	}

	// Start a test HTTP server on loopback.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("start test server: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "assembled-sandbox-ok")
	})
	server := &http.Server{Handler: mux}
	go server.Serve(listener) //nolint:errcheck
	defer server.Close()

	shellCmd := fmt.Sprintf("curl -s http://127.0.0.1:%d/health 2>&1", port)
	stdout, stderr, err := runInSandbox(t, cfg, shellCmd)
	if err != nil {
		t.Fatalf("loopback should succeed: err=%v stderr=%q", err, stderr)
	}
	if !strings.Contains(stdout, "assembled-sandbox-ok") {
		t.Errorf("expected loopback response, got: %q", stdout)
	}
}

// TestIntegration_DynamicPathGrant verifies that --add-dirs grants work.
func TestIntegration_DynamicPathGrant(t *testing.T) {
	skipIfNoSandboxExec(t)
	t.Parallel()

	worktree := t.TempDir()
	townRoot := t.TempDir()
	extraDir := t.TempDir()

	cfg := PolicyConfig{
		Home:        os.Getenv("HOME"),
		TownRoot:    townRoot,
		RigName:     "testrig",
		Worktree:    worktree,
		ExtraDirsRW: []string{extraDir},
	}

	testFile := filepath.Join(extraDir, "extra-write.txt")
	shellCmd := fmt.Sprintf("echo 'extra grant' > %q && cat %q", testFile, testFile)
	stdout, stderr, err := runInSandbox(t, cfg, shellCmd)
	if err != nil {
		t.Fatalf("write to extra granted dir should succeed: err=%v stderr=%q", err, stderr)
	}
	if !strings.Contains(stdout, "extra grant") {
		t.Errorf("expected written content from extra dir, got: %q", stdout)
	}
}

// TestIntegration_BeadsWriteFeature verifies that beads-write feature
// (enabled by default) grants write access to rig .beads directory.
func TestIntegration_BeadsWriteFeature(t *testing.T) {
	skipIfNoSandboxExec(t)
	t.Parallel()

	worktree := t.TempDir()
	townRoot := t.TempDir()
	beadsDir := filepath.Join(townRoot, "testrig", ".beads")
	os.MkdirAll(beadsDir, 0755)

	cfg := PolicyConfig{
		Home:     os.Getenv("HOME"),
		TownRoot: townRoot,
		RigName:  "testrig",
		Worktree: worktree,
	}

	testFile := filepath.Join(beadsDir, "test-bead.json")
	shellCmd := fmt.Sprintf("echo '{\"id\":\"test\"}' > %q && cat %q", testFile, testFile)
	stdout, stderr, err := runInSandbox(t, cfg, shellCmd)
	if err != nil {
		t.Fatalf("write to beads dir should succeed: err=%v stderr=%q", err, stderr)
	}
	if !strings.Contains(stdout, "test") {
		t.Errorf("expected bead content, got: %q", stdout)
	}
}

// TestIntegration_BuildCommandTokens_EndToEnd tests the full round trip:
// assemble policy, get tokens, execute via sandbox-exec.
func TestIntegration_BuildCommandTokens_EndToEnd(t *testing.T) {
	skipIfNoSandboxExec(t)
	t.Parallel()

	worktree := t.TempDir()
	townRoot := t.TempDir()

	cfg := PolicyConfig{
		Home:     os.Getenv("HOME"),
		TownRoot: townRoot,
		RigName:  "testrig",
		Worktree: worktree,
	}

	tokens, err := BuildCommandTokens(cfg)
	if err != nil {
		t.Fatalf("BuildCommandTokens failed: %v", err)
	}

	// Append a test command.
	testFile := filepath.Join(worktree, "e2e-test.txt")
	args := append(tokens, "/bin/sh", "-c",
		fmt.Sprintf("echo 'e2e-pass' > %q && cat %q", testFile, testFile))

	cmd := exec.Command(args[0], args[1:]...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("end-to-end execution failed: %v\noutput: %s", err, string(out))
	}

	if !strings.Contains(string(out), "e2e-pass") {
		t.Errorf("expected e2e-pass, got: %q", string(out))
	}

	// Clean up policy file.
	for i, tok := range tokens {
		if tok == "-f" && i+1 < len(tokens) {
			os.Remove(tokens[i+1])
			break
		}
	}
}
