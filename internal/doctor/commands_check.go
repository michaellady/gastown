package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/steveyegge/gastown/internal/templates"
)

// CommandsCheck validates that crew/polecat workspaces have .claude/commands/ provisioned.
// This ensures all agents have access to slash commands like /handoff.
type CommandsCheck struct {
	FixableCheck
	missingWorkspaces []workspaceWithMissingCommands // Cached during Run for use in Fix
}

type workspaceWithMissingCommands struct {
	path         string
	rigName      string
	workerName   string
	workerType   string // "crew" or "polecat"
	missingFiles []string
}

// NewCommandsCheck creates a new commands check.
func NewCommandsCheck() *CommandsCheck {
	return &CommandsCheck{
		FixableCheck: FixableCheck{
			BaseCheck: BaseCheck{
				CheckName:        "commands-provisioned",
				CheckDescription: "Check .claude/commands/ is provisioned in crew/polecat workspaces",
			},
		},
	}
}

// Run checks all crew and polecat workspaces for missing slash commands.
func (c *CommandsCheck) Run(ctx *CheckContext) *CheckResult {
	c.missingWorkspaces = nil

	workspaces := c.findAllWorkerDirs(ctx.TownRoot)
	if len(workspaces) == 0 {
		return &CheckResult{
			Name:    c.Name(),
			Status:  StatusOK,
			Message: "No crew/polecat workspaces found",
		}
	}

	var validCount int
	var details []string

	for _, ws := range workspaces {
		missing, err := templates.MissingCommands(ws.path)
		if err != nil {
			details = append(details, fmt.Sprintf("%s/%s/%s: error checking commands: %v",
				ws.rigName, ws.workerType, ws.workerName, err))
			continue
		}

		if len(missing) > 0 {
			c.missingWorkspaces = append(c.missingWorkspaces, workspaceWithMissingCommands{
				path:         ws.path,
				rigName:      ws.rigName,
				workerName:   ws.workerName,
				workerType:   ws.workerType,
				missingFiles: missing,
			})
			details = append(details, fmt.Sprintf("%s/%s/%s: missing %s",
				ws.rigName, ws.workerType, ws.workerName, strings.Join(missing, ", ")))
		} else {
			validCount++
		}
	}

	if len(c.missingWorkspaces) == 0 {
		return &CheckResult{
			Name:    c.Name(),
			Status:  StatusOK,
			Message: fmt.Sprintf("All %d workspaces have slash commands provisioned", validCount),
		}
	}

	return &CheckResult{
		Name:    c.Name(),
		Status:  StatusWarning,
		Message: fmt.Sprintf("%d workspace(s) missing slash commands (e.g., /handoff)", len(c.missingWorkspaces)),
		Details: details,
		FixHint: "Run 'gt doctor --fix' to provision missing commands",
	}
}

// Fix provisions missing slash commands to workspaces.
func (c *CommandsCheck) Fix(ctx *CheckContext) error {
	if len(c.missingWorkspaces) == 0 {
		return nil
	}

	var lastErr error
	for _, ws := range c.missingWorkspaces {
		if err := templates.ProvisionCommands(ws.path); err != nil {
			lastErr = fmt.Errorf("%s/%s/%s: %w", ws.rigName, ws.workerType, ws.workerName, err)
			continue
		}
	}

	return lastErr
}

type workerDir struct {
	path       string
	rigName    string
	workerName string
	workerType string // "crew" or "polecat"
}

// findAllWorkerDirs finds all crew and polecat directories in the workspace.
func (c *CommandsCheck) findAllWorkerDirs(townRoot string) []workerDir {
	var dirs []workerDir

	entries, err := os.ReadDir(townRoot)
	if err != nil {
		return dirs
	}

	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") || entry.Name() == "mayor" {
			continue
		}

		rigName := entry.Name()

		// Check crew directory
		crewPath := filepath.Join(townRoot, rigName, "crew")
		if crewEntries, err := os.ReadDir(crewPath); err == nil {
			for _, crew := range crewEntries {
				if !crew.IsDir() || strings.HasPrefix(crew.Name(), ".") {
					continue
				}
				dirs = append(dirs, workerDir{
					path:       filepath.Join(crewPath, crew.Name()),
					rigName:    rigName,
					workerName: crew.Name(),
					workerType: "crew",
				})
			}
		}

		// Check polecats directory
		polecatsPath := filepath.Join(townRoot, rigName, "polecats")
		if polecatEntries, err := os.ReadDir(polecatsPath); err == nil {
			for _, polecat := range polecatEntries {
				if !polecat.IsDir() || strings.HasPrefix(polecat.Name(), ".") {
					continue
				}
				dirs = append(dirs, workerDir{
					path:       filepath.Join(polecatsPath, polecat.Name()),
					rigName:    rigName,
					workerName: polecat.Name(),
					workerType: "polecat",
				})
			}
		}
	}

	return dirs
}
