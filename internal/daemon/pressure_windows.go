package daemon

// loadAverage1Sysctl is a no-op on Windows — load average is not natively available.
func loadAverage1Sysctl() float64 {
	return 0
}

// availableMemoryGB returns 0 on Windows (pressure checks effectively disabled).
func availableMemoryGB() float64 {
	return 0
}
