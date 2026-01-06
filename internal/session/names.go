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

// PropulsionNudge generates the GUPP (Gas Town Universal Propulsion Principle) nudge.
// This is sent after the beacon to trigger autonomous work execution.
// The agent receives this as user input, triggering the propulsion principle:
// "If work is on your hook, YOU RUN IT."
func PropulsionNudge() string {
	return "Run `gt hook` to check your hook and begin work."
}

// PropulsionNudgeForRole generates a role-specific GUPP nudge.
// Different roles have different startup flows:
// - polecat/crew: Check hook for slung work
// - witness/refinery: Start patrol cycle
// - deacon: Start heartbeat patrol
// - mayor: Check mail for coordination work
func PropulsionNudgeForRole(role string) string {
	switch role {
	case "polecat", "crew":
		return PropulsionNudge()
	case "witness":
		return "Run `gt prime` to check patrol status and begin work."
	case "refinery":
		return "Run `gt prime` to check MQ status and begin patrol."
	case "deacon":
		return "Run `gt prime` to check patrol status and begin heartbeat cycle."
	case "mayor":
		return "Run `gt prime` to check mail and begin coordination."
	default:
		return PropulsionNudge()
	}
}
