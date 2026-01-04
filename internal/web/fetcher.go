package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/steveyegge/gastown/internal/activity"
	"github.com/steveyegge/gastown/internal/workspace"
)

// MergeQueueRepos lists the GitHub repos to query for the merge queue.
var MergeQueueRepos = []string{
	"michaellady/roxas",
	"michaellady/gastown",
}

// LiveConvoyFetcher fetches convoy data from beads.
type LiveConvoyFetcher struct {
	townBeads string
}

// NewLiveConvoyFetcher creates a fetcher for the current workspace.
func NewLiveConvoyFetcher() (*LiveConvoyFetcher, error) {
	townRoot, err := workspace.FindFromCwdOrError()
	if err != nil {
		return nil, fmt.Errorf("not in a Gas Town workspace: %w", err)
	}

	return &LiveConvoyFetcher{
		townBeads: filepath.Join(townRoot, ".beads"),
	}, nil
}

// FetchConvoys fetches all open convoys with their activity data.
func (f *LiveConvoyFetcher) FetchConvoys() ([]ConvoyRow, error) {
	// List all open convoy-type issues
	listArgs := []string{"list", "--type=convoy", "--status=open", "--json"}
	listCmd := exec.Command("bd", listArgs...)
	listCmd.Dir = f.townBeads

	var stdout bytes.Buffer
	listCmd.Stdout = &stdout

	if err := listCmd.Run(); err != nil {
		return nil, fmt.Errorf("listing convoys: %w", err)
	}

	var convoys []struct {
		ID        string `json:"id"`
		Title     string `json:"title"`
		Status    string `json:"status"`
		CreatedAt string `json:"created_at"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &convoys); err != nil {
		return nil, fmt.Errorf("parsing convoy list: %w", err)
	}

	// Build convoy rows with activity data
	rows := make([]ConvoyRow, 0, len(convoys))
	for _, c := range convoys {
		row := ConvoyRow{
			ID:     c.ID,
			Title:  c.Title,
			Status: c.Status,
		}

		// Get tracked issues for progress and activity calculation
		tracked := f.getTrackedIssues(c.ID)
		row.Total = len(tracked)

		var mostRecentActivity time.Time
		for _, t := range tracked {
			if t.Status == "closed" {
				row.Completed++
			}
			// Track most recent activity from workers
			if t.LastActivity.After(mostRecentActivity) {
				mostRecentActivity = t.LastActivity
			}
		}

		row.Progress = fmt.Sprintf("%d/%d", row.Completed, row.Total)

		// Calculate activity info from most recent worker activity
		if !mostRecentActivity.IsZero() {
			row.LastActivity = activity.Calculate(mostRecentActivity)
		} else {
			row.LastActivity = activity.Info{
				FormattedAge: "no activity",
				ColorClass:   activity.ColorUnknown,
			}
		}

		// Get tracked issues for expandable view
		row.TrackedIssues = make([]TrackedIssue, len(tracked))
		for i, t := range tracked {
			row.TrackedIssues[i] = TrackedIssue{
				ID:       t.ID,
				Title:    t.Title,
				Status:   t.Status,
				Assignee: t.Assignee,
			}
		}

		rows = append(rows, row)
	}

	return rows, nil
}

// trackedIssueInfo holds info about an issue being tracked by a convoy.
type trackedIssueInfo struct {
	ID           string
	Title        string
	Status       string
	Assignee     string
	LastActivity time.Time
}

// getTrackedIssues fetches tracked issues for a convoy.
func (f *LiveConvoyFetcher) getTrackedIssues(convoyID string) []trackedIssueInfo {
	dbPath := filepath.Join(f.townBeads, "beads.db")

	// Query tracked dependencies from SQLite
	safeConvoyID := strings.ReplaceAll(convoyID, "'", "''")
	// #nosec G204 -- sqlite3 path is from trusted config, convoyID is escaped
	queryCmd := exec.Command("sqlite3", "-json", dbPath,
		fmt.Sprintf(`SELECT depends_on_id, type FROM dependencies WHERE issue_id = '%s' AND type = 'tracks'`, safeConvoyID))

	var stdout bytes.Buffer
	queryCmd.Stdout = &stdout
	if err := queryCmd.Run(); err != nil {
		return nil
	}

	var deps []struct {
		DependsOnID string `json:"depends_on_id"`
		Type        string `json:"type"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &deps); err != nil {
		return nil
	}

	// Collect issue IDs (normalize external refs)
	issueIDs := make([]string, 0, len(deps))
	for _, dep := range deps {
		issueID := dep.DependsOnID
		if strings.HasPrefix(issueID, "external:") {
			parts := strings.SplitN(issueID, ":", 3)
			if len(parts) == 3 {
				issueID = parts[2]
			}
		}
		issueIDs = append(issueIDs, issueID)
	}

	// Batch fetch issue details
	details := f.getIssueDetailsBatch(issueIDs)

	// Get worker info for activity timestamps
	workers := f.getWorkersForIssues(issueIDs)

	// Build result
	result := make([]trackedIssueInfo, 0, len(issueIDs))
	for _, id := range issueIDs {
		info := trackedIssueInfo{ID: id}

		if d, ok := details[id]; ok {
			info.Title = d.Title
			info.Status = d.Status
			info.Assignee = d.Assignee
		} else {
			info.Title = "(external)"
			info.Status = "unknown"
		}

		if w, ok := workers[id]; ok && w.LastActivity != nil {
			info.LastActivity = *w.LastActivity
		}

		result = append(result, info)
	}

	return result
}

// issueDetail holds basic issue info.
type issueDetail struct {
	ID       string
	Title    string
	Status   string
	Assignee string
}

// getIssueDetailsBatch fetches details for multiple issues.
func (f *LiveConvoyFetcher) getIssueDetailsBatch(issueIDs []string) map[string]*issueDetail {
	result := make(map[string]*issueDetail)
	if len(issueIDs) == 0 {
		return result
	}

	args := append([]string{"show"}, issueIDs...)
	args = append(args, "--json")

	// #nosec G204 -- bd is a trusted internal tool, args are issue IDs
	showCmd := exec.Command("bd", args...)
	var stdout bytes.Buffer
	showCmd.Stdout = &stdout

	if err := showCmd.Run(); err != nil {
		return result
	}

	var issues []struct {
		ID       string `json:"id"`
		Title    string `json:"title"`
		Status   string `json:"status"`
		Assignee string `json:"assignee"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &issues); err != nil {
		return result
	}

	for _, issue := range issues {
		result[issue.ID] = &issueDetail{
			ID:       issue.ID,
			Title:    issue.Title,
			Status:   issue.Status,
			Assignee: issue.Assignee,
		}
	}

	return result
}

// workerDetail holds worker info including last activity.
type workerDetail struct {
	Worker       string
	LastActivity *time.Time
}

// getWorkersForIssues finds workers and their last activity for issues.
func (f *LiveConvoyFetcher) getWorkersForIssues(issueIDs []string) map[string]*workerDetail {
	result := make(map[string]*workerDetail)
	if len(issueIDs) == 0 {
		return result
	}

	townRoot, _ := workspace.FindFromCwd()
	if townRoot == "" {
		return result
	}

	// Find all rig beads databases
	rigDirs, _ := filepath.Glob(filepath.Join(townRoot, "*", "mayor", "rig", ".beads", "beads.db"))

	for _, dbPath := range rigDirs {
		for _, issueID := range issueIDs {
			if _, ok := result[issueID]; ok {
				continue
			}

			safeID := strings.ReplaceAll(issueID, "'", "''")
			query := fmt.Sprintf(
				`SELECT id, hook_bead, last_activity FROM issues WHERE issue_type = 'agent' AND status = 'open' AND hook_bead = '%s' LIMIT 1`,
				safeID)

			// #nosec G204 -- sqlite3 path is from trusted glob, issueID is escaped
			queryCmd := exec.Command("sqlite3", "-json", dbPath, query)
			var stdout bytes.Buffer
			queryCmd.Stdout = &stdout
			if err := queryCmd.Run(); err != nil {
				continue
			}

			var agents []struct {
				ID           string `json:"id"`
				HookBead     string `json:"hook_bead"`
				LastActivity string `json:"last_activity"`
			}
			if err := json.Unmarshal(stdout.Bytes(), &agents); err != nil || len(agents) == 0 {
				continue
			}

			agent := agents[0]
			detail := &workerDetail{
				Worker: agent.ID,
			}

			if agent.LastActivity != "" {
				if t, err := time.Parse(time.RFC3339, agent.LastActivity); err == nil {
					detail.LastActivity = &t
				}
			}

			result[issueID] = detail
		}
	}

	return result
}

// FetchMergeQueue fetches open PRs from configured repos.
func (f *LiveConvoyFetcher) FetchMergeQueue() []MergeQueuePR {
	var prs []MergeQueuePR

	for _, repo := range MergeQueueRepos {
		repoPRs := f.fetchRepoPRs(repo)
		prs = append(prs, repoPRs...)
	}

	return prs
}

// fetchRepoPRs fetches open PRs for a single repo using gh CLI.
func (f *LiveConvoyFetcher) fetchRepoPRs(repo string) []MergeQueuePR {
	// gh pr list --repo <repo> --state open --json number,title,url,statusCheckRollup,mergeable
	args := []string{
		"pr", "list",
		"--repo", repo,
		"--state", "open",
		"--json", "number,title,url,statusCheckRollup,mergeable",
	}

	cmd := exec.Command("gh", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil
	}

	var ghPRs []struct {
		Number            int    `json:"number"`
		Title             string `json:"title"`
		URL               string `json:"url"`
		Mergeable         string `json:"mergeable"`
		StatusCheckRollup []struct {
			Conclusion string `json:"conclusion"`
			Status     string `json:"status"`
		} `json:"statusCheckRollup"`
	}

	if err := json.Unmarshal(stdout.Bytes(), &ghPRs); err != nil {
		return nil
	}

	// Extract short repo name (last part after /)
	repoShort := repo
	if idx := strings.LastIndex(repo, "/"); idx >= 0 {
		repoShort = repo[idx+1:]
	}

	result := make([]MergeQueuePR, 0, len(ghPRs))
	for _, pr := range ghPRs {
		mqPR := MergeQueuePR{
			Number:   pr.Number,
			Title:    pr.Title,
			Repo:     repoShort,
			RepoFull: repo,
			URL:      pr.URL,
		}

		// Determine CI status from statusCheckRollup
		mqPR.CIStatus = determineCIStatus(pr.StatusCheckRollup)

		// Determine mergeable status
		mqPR.Mergeable = pr.Mergeable == "MERGEABLE"

		// Determine color class
		mqPR.ColorClass = determinePRColor(mqPR.CIStatus, mqPR.Mergeable)

		result = append(result, mqPR)
	}

	return result
}

// determineCIStatus examines status checks and returns overall status.
func determineCIStatus(checks []struct {
	Conclusion string `json:"conclusion"`
	Status     string `json:"status"`
}) string {
	if len(checks) == 0 {
		return "pending"
	}

	hasFailure := false
	hasPending := false

	for _, check := range checks {
		switch check.Conclusion {
		case "FAILURE", "CANCELLED", "CANCELED", "TIMED_OUT": //nolint:misspell // GitHub API uses CANCELLED
			hasFailure = true
		case "SUCCESS":
			// continue
		default:
			// If conclusion is empty, check status
			if check.Status == "IN_PROGRESS" || check.Status == "QUEUED" || check.Status == "PENDING" {
				hasPending = true
			} else if check.Conclusion == "" {
				hasPending = true
			}
		}
	}

	if hasFailure {
		return "failure"
	}
	if hasPending {
		return "pending"
	}
	return "success"
}

// determinePRColor returns the CSS color class for a PR based on its status.
func determinePRColor(ciStatus string, mergeable bool) string {
	if ciStatus == "failure" || !mergeable {
		return "mq-red"
	}
	if ciStatus == "pending" {
		return "mq-yellow"
	}
	return "mq-green"
}
