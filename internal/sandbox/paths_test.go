package sandbox

import (
	"strings"
	"testing"
)

func TestEmitAncestorLiterals(t *testing.T) {
	result := EmitAncestorLiterals("/Users/alice/projects/myrepo")
	expected := []string{
		`(allow file-read-metadata (literal "/Users"))`,
		`(allow file-read-metadata (literal "/Users/alice"))`,
		`(allow file-read-metadata (literal "/Users/alice/projects"))`,
		`(allow file-read-metadata (literal "/Users/alice/projects/myrepo"))`,
	}
	for _, exp := range expected {
		if !strings.Contains(result, exp) {
			t.Errorf("missing: %s\ngot:\n%s", exp, result)
		}
	}
}

func TestEmitAncestorLiterals_Root(t *testing.T) {
	result := EmitAncestorLiterals("/")
	// Root path has no components to emit.
	if strings.Contains(result, "(allow") {
		t.Errorf("root path should produce no ancestors, got:\n%s", result)
	}
}

func TestEmitAncestorLiterals_SingleComponent(t *testing.T) {
	result := EmitAncestorLiterals("/tmp")
	if !strings.Contains(result, `(literal "/tmp")`) {
		t.Errorf("single component should emit /tmp, got:\n%s", result)
	}
}

func TestEmitPathGrant_ReadOnly(t *testing.T) {
	result := EmitPathGrant("/Users/test/shared/lib", false)

	// Should have ancestor literals.
	if !strings.Contains(result, `(literal "/Users/test/shared")`) {
		t.Error("missing ancestor literal")
	}

	// Should have read grant.
	if !strings.Contains(result, `(allow file-read* (subpath "/Users/test/shared/lib"))`) {
		t.Error("missing read grant")
	}

	// Should NOT have write grant.
	if strings.Contains(result, "file-write*") {
		t.Error("read-only grant should not include file-write*")
	}
}

func TestEmitPathGrant_ReadWrite(t *testing.T) {
	result := EmitPathGrant("/Users/test/scratch", true)

	// Should have both read and write.
	if !strings.Contains(result, `(allow file-read* file-write* (subpath "/Users/test/scratch"))`) {
		t.Error("missing read-write grant")
	}
}

func TestEmitPathGrant_TrailingSlash(t *testing.T) {
	result := EmitPathGrant("/Users/test/path/", true)
	// filepath.Clean should normalize the trailing slash.
	if !strings.Contains(result, `(subpath "/Users/test/path")`) {
		t.Errorf("trailing slash should be normalized, got:\n%s", result)
	}
}
