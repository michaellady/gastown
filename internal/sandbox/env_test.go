package sandbox

import (
	"slices"
	"testing"
)

func TestIsAllowedEnvVar(t *testing.T) {
	tests := []struct {
		name    string
		allowed bool
	}{
		{"HOME", true},
		{"GT_ROOT", true},
		{"GT_RIG", true},
		{"ANTHROPIC_API_KEY", true},
		{"BD_ACTOR", true},
		{"SUPER_SECRET", false},
		{"AWS_SECRET_ACCESS_KEY", false},
		{"", false},
	}

	for _, tt := range tests {
		if got := IsAllowedEnvVar(tt.name); got != tt.allowed {
			t.Errorf("IsAllowedEnvVar(%q) = %v, want %v", tt.name, got, tt.allowed)
		}
	}
}

func TestSanitizeEnv(t *testing.T) {
	input := []string{
		"HOME=/Users/test",
		"GT_RIG=gastown",
		"AWS_SECRET_ACCESS_KEY=hunter2",
		"SUPER_SECRET=nope",
		"PATH=/usr/bin",
		"ANTHROPIC_API_KEY=sk-ant-xxx",
	}

	got := SanitizeEnv(input)

	if slices.Contains(got, "AWS_SECRET_ACCESS_KEY=hunter2") {
		t.Error("SanitizeEnv should filter AWS_SECRET_ACCESS_KEY")
	}
	if slices.Contains(got, "SUPER_SECRET=nope") {
		t.Error("SanitizeEnv should filter SUPER_SECRET")
	}
	if !slices.Contains(got, "HOME=/Users/test") {
		t.Error("SanitizeEnv should keep HOME")
	}
	if !slices.Contains(got, "GT_RIG=gastown") {
		t.Error("SanitizeEnv should keep GT_RIG")
	}
	if !slices.Contains(got, "ANTHROPIC_API_KEY=sk-ant-xxx") {
		t.Error("SanitizeEnv should keep ANTHROPIC_API_KEY")
	}
}

func TestSanitizeEnv_WithExtras(t *testing.T) {
	input := []string{"HOME=/Users/test"}
	extras := []string{"GT_SANDBOX=1", "CUSTOM_FLAG=yes"}

	got := SanitizeEnv(input, extras...)

	if !slices.Contains(got, "HOME=/Users/test") {
		t.Error("should keep allowed vars")
	}
	if !slices.Contains(got, "GT_SANDBOX=1") {
		t.Error("should include extras")
	}
	if !slices.Contains(got, "CUSTOM_FLAG=yes") {
		t.Error("should include all extras")
	}
}
