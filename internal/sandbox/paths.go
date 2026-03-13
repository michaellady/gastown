package sandbox

import (
	"fmt"
	"path/filepath"
	"strings"
)

// EmitAncestorLiterals generates SBPL rules granting file-read-metadata
// on each ancestor directory of the given path. Uses (literal ...) to
// allow directory traversal without granting recursive read access.
func EmitAncestorLiterals(path string) string {
	clean := filepath.Clean(path)
	parts := strings.Split(clean, string(filepath.Separator))

	var sb strings.Builder
	accumulated := ""
	for _, part := range parts {
		if part == "" {
			continue
		}
		accumulated += "/" + part
		fmt.Fprintf(&sb, "(allow file-read-metadata (literal %q))\n", accumulated)
	}
	return sb.String()
}

// EmitPathGrant generates SBPL rules granting access to a path with
// proper ancestor traversal. If writable is true, grants file-write* too.
func EmitPathGrant(path string, writable bool) string {
	clean := filepath.Clean(path)
	var sb strings.Builder

	// Ancestor traversal
	sb.WriteString(EmitAncestorLiterals(clean))

	// Actual grant
	if writable {
		fmt.Fprintf(&sb, "(allow file-read* file-write* (subpath %q))\n", clean)
	} else {
		fmt.Fprintf(&sb, "(allow file-read* (subpath %q))\n", clean)
	}

	return sb.String()
}
