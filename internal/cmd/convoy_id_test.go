package cmd

import (
	"testing"
)

func TestGenerateShortID_Length(t *testing.T) {
	id := generateShortID()
	if len(id) != 5 {
		t.Errorf("generateShortID() = %q (len %d), want length 5", id, len(id))
	}
}

func TestGenerateShortID_ValidChars(t *testing.T) {
	const validChars = "0123456789abcdefghijklmnopqrstuvwxyz"
	valid := make(map[byte]bool)
	for i := range validChars {
		valid[validChars[i]] = true
	}

	for i := 0; i < 100; i++ {
		id := generateShortID()
		for j, c := range []byte(id) {
			if !valid[c] {
				t.Errorf("generateShortID()[%d] = %c, not in base36 alphabet", j, c)
			}
		}
	}
}

func TestGenerateShortID_Uniqueness(t *testing.T) {
	// With 5-char base36 (~60M possible values), birthday paradox gives ~0.82%
	// collision probability at 1000 IDs. Use 500 IDs to keep probability well
	// below 0.5%, avoiding flaky failures in CI while still validating that
	// the RNG produces reasonably distributed output.
	seen := make(map[string]bool)
	const n = 500
	for i := 0; i < n; i++ {
		id := generateShortID()
		if seen[id] {
			t.Errorf("collision after %d IDs: %q", i, id)
		}
		seen[id] = true
	}
}
