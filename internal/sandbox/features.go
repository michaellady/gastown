package sandbox

// Feature represents an optional sandbox capability.
type Feature string

const (
	// FeatureBeadsWrite grants RW access to the rig's .beads directory.
	FeatureBeadsWrite Feature = "beads-write"

	// FeatureRuntimeWrite grants RW access to the town .runtime directory.
	FeatureRuntimeWrite Feature = "runtime-write"

	// FeatureNetworkWide replaces loopback-only networking with full network access.
	FeatureNetworkWide Feature = "network-wide"

	// FeatureDocker grants access to Docker socket and config.
	FeatureDocker Feature = "docker"

	// FeatureSSH grants RW access to ~/.ssh for SSH agent forwarding.
	FeatureSSH Feature = "ssh"
)

// DefaultFeatures are enabled by default for polecat sessions.
var DefaultFeatures = []Feature{
	FeatureBeadsWrite,
	FeatureRuntimeWrite,
}

// AllFeatures lists all known features for validation.
var AllFeatures = []Feature{
	FeatureBeadsWrite,
	FeatureRuntimeWrite,
	FeatureNetworkWide,
	FeatureDocker,
	FeatureSSH,
}

// featureSet is a set of enabled features for fast lookup.
type featureSet map[Feature]bool

// resolveFeatures merges explicit features with defaults and returns a set.
func resolveFeatures(explicit []Feature) featureSet {
	s := make(featureSet, len(DefaultFeatures)+len(explicit))
	for _, f := range DefaultFeatures {
		s[f] = true
	}
	for _, f := range explicit {
		s[f] = true
	}
	return s
}

func (s featureSet) has(f Feature) bool {
	return s[f]
}

// ParseFeatures converts a slice of feature name strings to Feature values.
// Returns an error if any feature name is unrecognized.
func ParseFeatures(names []string) ([]Feature, error) {
	known := make(map[Feature]bool, len(AllFeatures))
	for _, f := range AllFeatures {
		known[f] = true
	}

	features := make([]Feature, 0, len(names))
	for _, name := range names {
		f := Feature(name)
		if !known[f] {
			return nil, &UnknownFeatureError{Name: name}
		}
		features = append(features, f)
	}
	return features, nil
}

// UnknownFeatureError is returned when an unrecognized feature name is used.
type UnknownFeatureError struct {
	Name string
}

func (e *UnknownFeatureError) Error() string {
	return "unknown sandbox feature: " + e.Name
}
