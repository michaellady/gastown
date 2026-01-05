package beads

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetPrefixForRig(t *testing.T) {
	// Create a temporary directory with routes.jsonl
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		t.Fatal(err)
	}

	routesContent := `{"prefix": "gt-", "path": "gastown/mayor/rig"}
{"prefix": "bd-", "path": "beads/mayor/rig"}
{"prefix": "hq-", "path": "."}
`
	if err := os.WriteFile(filepath.Join(beadsDir, "routes.jsonl"), []byte(routesContent), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		rig      string
		expected string
	}{
		{"gastown", "gt"},
		{"beads", "bd"},
		{"unknown", "gt"}, // default
		{"", "gt"},        // empty rig -> default
	}

	for _, tc := range tests {
		t.Run(tc.rig, func(t *testing.T) {
			result := GetPrefixForRig(tmpDir, tc.rig)
			if result != tc.expected {
				t.Errorf("GetPrefixForRig(%q, %q) = %q, want %q", tmpDir, tc.rig, result, tc.expected)
			}
		})
	}
}

func TestGetPrefixForRig_NoRoutesFile(t *testing.T) {
	tmpDir := t.TempDir()
	// No routes.jsonl file

	result := GetPrefixForRig(tmpDir, "anything")
	if result != "gt" {
		t.Errorf("Expected default 'gt' when no routes file, got %q", result)
	}
}

func TestGetRigPathForPrefix(t *testing.T) {
	// Create a temporary directory with routes.jsonl
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		t.Fatal(err)
	}

	routesContent := `{"prefix": "ga-", "path": "gastown/mayor/rig"}
{"prefix": "bd-", "path": "beads/mayor/rig"}
`
	if err := os.WriteFile(filepath.Join(beadsDir, "routes.jsonl"), []byte(routesContent), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		prefix       string
		expectedPath string
		found        bool
	}{
		{"ga-", filepath.Join(tmpDir, "gastown/mayor/rig"), true},
		{"bd-", filepath.Join(tmpDir, "beads/mayor/rig"), true},
		{"unknown-", "", false},
		{"hq-", "", false}, // hq- is not in routes
	}

	for _, tc := range tests {
		t.Run(tc.prefix, func(t *testing.T) {
			result, found := GetRigPathForPrefix(tmpDir, tc.prefix)
			if found != tc.found {
				t.Errorf("GetRigPathForPrefix(%q, %q) found = %v, want %v", tmpDir, tc.prefix, found, tc.found)
			}
			if result != tc.expectedPath {
				t.Errorf("GetRigPathForPrefix(%q, %q) = %q, want %q", tmpDir, tc.prefix, result, tc.expectedPath)
			}
		})
	}
}

func TestExtractPrefixFromBeadID(t *testing.T) {
	tests := []struct {
		beadID   string
		expected string
	}{
		{"ga-nu4", "ga-"},
		{"bd-abc123", "bd-"},
		{"hq-mayor", "hq-"},
		{"simple", ""}, // no hyphen
		{"", ""},
		{"a-b-c", "a-"}, // only first segment
	}

	for _, tc := range tests {
		t.Run(tc.beadID, func(t *testing.T) {
			result := ExtractPrefixFromBeadID(tc.beadID)
			if result != tc.expected {
				t.Errorf("ExtractPrefixFromBeadID(%q) = %q, want %q", tc.beadID, result, tc.expected)
			}
		})
	}
}

func TestResolveRigPathFromBeadID(t *testing.T) {
	// Create a temporary directory with routes.jsonl
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		t.Fatal(err)
	}

	routesContent := `{"prefix": "ga-", "path": "gastown/mayor/rig"}
{"prefix": "bd-", "path": "beads/mayor/rig"}
`
	if err := os.WriteFile(filepath.Join(beadsDir, "routes.jsonl"), []byte(routesContent), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		beadID       string
		expectedPath string
	}{
		{"ga-nu4", filepath.Join(tmpDir, "gastown/mayor/rig")},
		{"bd-abc123", filepath.Join(tmpDir, "beads/mayor/rig")},
		{"hq-mayor", ""},       // hq- is town-level, returns empty
		{"unknown-xyz", ""},    // unknown prefix
		{"nohyphen", ""},       // no hyphen in bead ID
	}

	for _, tc := range tests {
		t.Run(tc.beadID, func(t *testing.T) {
			result := ResolveRigPathFromBeadID(tmpDir, tc.beadID)
			if result != tc.expectedPath {
				t.Errorf("ResolveRigPathFromBeadID(%q, %q) = %q, want %q", tmpDir, tc.beadID, result, tc.expectedPath)
			}
		})
	}
}

func TestAgentBeadIDsWithPrefix(t *testing.T) {
	tests := []struct {
		name     string
		fn       func() string
		expected string
	}{
		{"PolecatBeadIDWithPrefix bd beads obsidian",
			func() string { return PolecatBeadIDWithPrefix("bd", "beads", "obsidian") },
			"bd-beads-polecat-obsidian"},
		{"PolecatBeadIDWithPrefix gt gastown Toast",
			func() string { return PolecatBeadIDWithPrefix("gt", "gastown", "Toast") },
			"gt-gastown-polecat-Toast"},
		{"WitnessBeadIDWithPrefix bd beads",
			func() string { return WitnessBeadIDWithPrefix("bd", "beads") },
			"bd-beads-witness"},
		{"RefineryBeadIDWithPrefix bd beads",
			func() string { return RefineryBeadIDWithPrefix("bd", "beads") },
			"bd-beads-refinery"},
		{"CrewBeadIDWithPrefix bd beads max",
			func() string { return CrewBeadIDWithPrefix("bd", "beads", "max") },
			"bd-beads-crew-max"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.fn()
			if result != tc.expected {
				t.Errorf("got %q, want %q", result, tc.expected)
			}
		})
	}
}
