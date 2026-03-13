// sandbox-test assembles a sandbox policy and prints the sandbox-exec command
// to launch claude (or any command) inside it. Used for live testing.
//
// Usage:
//
//	go run ./cmd/sandbox-test --worktree=/path/to/worktree [--debug] [--features=beads-write,runtime-write] [-- command args...]
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/steveyegge/gastown/internal/sandbox"
)

func main() {
	home := os.Getenv("HOME")
	townRoot := flag.String("town-root", findTownRoot(), "Town root directory")
	rigName := flag.String("rig", "gastown", "Rig name")
	worktree := flag.String("worktree", "", "Polecat worktree path (required)")
	agent := flag.String("agent", "claude-code", "Agent name for profile selection")
	features := flag.String("features", "", "Comma-separated features (beads-write,runtime-write,network-wide,docker,ssh)")
	debug := flag.Bool("debug", false, "Enable sandbox debug (logs denials)")
	dryRun := flag.Bool("dry-run", false, "Print command but don't execute")
	explain := flag.Bool("explain", false, "Print policy summary")
	flag.Parse()

	if *worktree == "" {
		fmt.Fprintln(os.Stderr, "error: --worktree is required")
		flag.Usage()
		os.Exit(1)
	}

	var featureList []sandbox.Feature
	if *features != "" {
		parsed, err := sandbox.ParseFeatures(strings.Split(*features, ","))
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		featureList = parsed
	}

	cfg := sandbox.PolicyConfig{
		Home:     home,
		TownRoot: *townRoot,
		RigName:  *rigName,
		Worktree: *worktree,
		Agent:    *agent,
		Features: featureList,
		Debug:    *debug,
	}

	if *explain {
		b := sandbox.NewBuilder()
		summary, err := b.Explain(cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Print(summary)
		return
	}

	tokens, err := sandbox.BuildCommandTokens(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// The command to run inside the sandbox.
	innerCmd := flag.Args()
	if len(innerCmd) == 0 {
		innerCmd = []string{"claude", "--dangerously-skip-permissions"}
	}

	fullArgs := append(tokens, innerCmd...)

	if *dryRun {
		fmt.Println("# Sandbox command:")
		fmt.Println(strings.Join(fullArgs, " \\\n  "))
		// Also print policy file path.
		for i, tok := range tokens {
			if tok == "-f" && i+1 < len(tokens) {
				fmt.Printf("\n# Policy file: %s\n", tokens[i+1])
				break
			}
		}
		return
	}

	fmt.Fprintf(os.Stderr, "Launching sandboxed session...\n")
	fmt.Fprintf(os.Stderr, "  Worktree: %s\n", *worktree)
	fmt.Fprintf(os.Stderr, "  Command:  %s\n", strings.Join(innerCmd, " "))
	fmt.Fprintf(os.Stderr, "  Debug:    %v\n", *debug)

	cmd := exec.Command(fullArgs[0], fullArgs[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func findTownRoot() string {
	// Walk up from CWD looking for a .beads directory (town marker).
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(dir + "/.beads"); err == nil {
			return dir
		}
		parent := dir[:strings.LastIndex(dir, "/")]
		if parent == dir || parent == "" {
			return os.Getenv("HOME") + "/gt"
		}
		dir = parent
	}
}
