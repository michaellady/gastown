package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"github.com/steveyegge/gastown/internal/sandbox"
	"github.com/steveyegge/gastown/internal/workspace"
)

var sandboxCmd = &cobra.Command{
	Use:     "sandbox",
	GroupID: GroupDiag,
	Short:   "Inspect and test sandbox policies",
	RunE:    requireSubcommand,
}

var sandboxExplainCmd = &cobra.Command{
	Use:   "explain [--worktree PATH]",
	Short: "Show assembled sandbox policy summary",
	Long: `Show a human-readable summary of the sandbox policy that would be
assembled for a polecat session. Lists all layers, features, and
parameters.`,
	RunE: runSandboxExplain,
}

var sandboxAssembleCmd = &cobra.Command{
	Use:   "assemble [--worktree PATH]",
	Short: "Assemble and print the full SBPL policy",
	Long: `Assemble all sandbox profile layers and print the full SBPL policy
to stdout. Useful for debugging, auditing, and testing with
sandbox-exec directly.`,
	RunE: runSandboxAssemble,
}

var sandboxTestCmd = &cobra.Command{
	Use:   "test [--worktree PATH] -- command [args...]",
	Short: "Run a command inside the assembled sandbox",
	Long: `Assemble the sandbox policy, write it to a temp file, and execute
the given command inside sandbox-exec. Useful for testing that a
command works under sandbox restrictions.

Examples:
  gt sandbox test -- echo "hello"
  gt sandbox test --worktree /tmp/wt -- git status
  gt sandbox test --debug -- node -e "console.log('ok')"`,
	RunE: runSandboxTest,
}

// Flags
var (
	sandboxWorktree string
	sandboxRig      string
	sandboxAgent    string
	sandboxFeatures string
	sandboxDebug    bool
)

func init() {
	rootCmd.AddCommand(sandboxCmd)
	sandboxCmd.AddCommand(sandboxExplainCmd, sandboxAssembleCmd, sandboxTestCmd)

	for _, cmd := range []*cobra.Command{sandboxExplainCmd, sandboxAssembleCmd, sandboxTestCmd} {
		cmd.Flags().StringVar(&sandboxWorktree, "worktree", "", "Polecat worktree path (default: current directory)")
		cmd.Flags().StringVar(&sandboxRig, "rig", "", "Rig name (default: auto-detect)")
		cmd.Flags().StringVar(&sandboxAgent, "agent", "claude-code", "Agent name for profile selection")
		cmd.Flags().StringVar(&sandboxFeatures, "features", "", "Comma-separated extra features")
		cmd.Flags().BoolVar(&sandboxDebug, "debug", false, "Enable sandbox debug (logs denials)")
	}
}

func buildSandboxConfig() (sandbox.PolicyConfig, error) {
	home := os.Getenv("HOME")
	worktree := sandboxWorktree
	if worktree == "" {
		var err error
		worktree, err = os.Getwd()
		if err != nil {
			return sandbox.PolicyConfig{}, fmt.Errorf("getting cwd: %w", err)
		}
	}

	townRoot, _, err := workspace.FindFromCwdWithFallback()
	if err != nil {
		return sandbox.PolicyConfig{}, fmt.Errorf("finding town root: %w", err)
	}

	rigName := sandboxRig
	if rigName == "" {
		rigName = os.Getenv("GT_RIG")
		if rigName == "" {
			rigName = "gastown"
		}
	}

	var features []sandbox.Feature
	if sandboxFeatures != "" {
		features, err = sandbox.ParseFeatures(strings.Split(sandboxFeatures, ","))
		if err != nil {
			return sandbox.PolicyConfig{}, err
		}
	}

	return sandbox.PolicyConfig{
		Home:     home,
		TownRoot: townRoot,
		RigName:  rigName,
		Worktree: worktree,
		Agent:    sandboxAgent,
		Features: features,
		Debug:    sandboxDebug,
	}, nil
}

func runSandboxExplain(cmd *cobra.Command, args []string) error {
	cfg, err := buildSandboxConfig()
	if err != nil {
		return err
	}

	b := sandbox.NewBuilder()
	summary, err := b.Explain(cfg)
	if err != nil {
		return err
	}

	fmt.Print(summary)
	return nil
}

func runSandboxAssemble(cmd *cobra.Command, args []string) error {
	cfg, err := buildSandboxConfig()
	if err != nil {
		return err
	}

	b := sandbox.NewBuilder()
	policy, err := b.Assemble(cfg)
	if err != nil {
		return err
	}

	fmt.Print(policy.SBPL)
	return nil
}

func runSandboxTest(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: gt sandbox test -- command [args...]")
	}

	cfg, err := buildSandboxConfig()
	if err != nil {
		return err
	}

	tokens, err := sandbox.BuildCommandTokens(cfg)
	if err != nil {
		return err
	}

	fullArgs := append(tokens, args...)
	fmt.Fprintf(os.Stderr, "Running in sandbox: %s\n", strings.Join(args, " "))

	execCmd := exec.Command(fullArgs[0], fullArgs[1:]...)
	execCmd.Stdin = os.Stdin
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr

	return execCmd.Run()
}
