package sandbox

import (
	"errors"
	"testing"
)

func TestResolveFeatures_IncludesDefaults(t *testing.T) {
	s := resolveFeatures(nil)
	if !s.has(FeatureBeadsWrite) {
		t.Error("defaults should include beads-write")
	}
	if !s.has(FeatureRuntimeWrite) {
		t.Error("defaults should include runtime-write")
	}
	if s.has(FeatureDocker) {
		t.Error("docker should not be in defaults")
	}
}

func TestResolveFeatures_MergesExplicit(t *testing.T) {
	s := resolveFeatures([]Feature{FeatureDocker, FeatureSSH})
	if !s.has(FeatureDocker) {
		t.Error("explicit docker should be included")
	}
	if !s.has(FeatureSSH) {
		t.Error("explicit ssh should be included")
	}
	// Defaults should still be present.
	if !s.has(FeatureBeadsWrite) {
		t.Error("defaults should still be present")
	}
}

func TestParseFeatures_Valid(t *testing.T) {
	features, err := ParseFeatures([]string{"docker", "ssh", "beads-write"})
	if err != nil {
		t.Fatalf("ParseFeatures failed: %v", err)
	}
	if len(features) != 3 {
		t.Errorf("expected 3 features, got %d", len(features))
	}
}

func TestParseFeatures_Unknown(t *testing.T) {
	_, err := ParseFeatures([]string{"docker", "quantum-teleporter"})
	if err == nil {
		t.Fatal("expected error for unknown feature")
	}
	var uf *UnknownFeatureError
	if !errors.As(err, &uf) {
		t.Errorf("expected UnknownFeatureError, got %T", err)
	}
	if uf.Name != "quantum-teleporter" {
		t.Errorf("expected feature name 'quantum-teleporter', got %q", uf.Name)
	}
}

func TestParseFeatures_Empty(t *testing.T) {
	features, err := ParseFeatures(nil)
	if err != nil {
		t.Fatalf("ParseFeatures failed: %v", err)
	}
	if len(features) != 0 {
		t.Errorf("expected 0 features, got %d", len(features))
	}
}
