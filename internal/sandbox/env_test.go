package sandbox

import "testing"

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
