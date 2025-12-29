// Package session provides polecat session lifecycle management.
package session

import "fmt"

// Prefix is the common prefix for all Gas Town tmux session names.
const Prefix = "gt-"

// MayorSessionName returns the session name for the Mayor agent.
func MayorSessionName() string {
	return Prefix + "mayor"
}

// DeaconSessionName returns the session name for the Deacon agent.
func DeaconSessionName() string {
	return Prefix + "deacon"
}

// WitnessSessionName returns the session name for a rig's Witness agent.
func WitnessSessionName(rig string) string {
	return fmt.Sprintf("%s%s-witness", Prefix, rig)
}

// RefinerySessionName returns the session name for a rig's Refinery agent.
func RefinerySessionName(rig string) string {
	return fmt.Sprintf("%s%s-refinery", Prefix, rig)
}

// CrewSessionName returns the session name for a crew worker in a rig.
func CrewSessionName(rig, name string) string {
	return fmt.Sprintf("%s%s-crew-%s", Prefix, rig, name)
}

// PolecatSessionName returns the session name for a polecat in a rig.
func PolecatSessionName(rig, name string) string {
	return fmt.Sprintf("%s%s-%s", Prefix, rig, name)
}
