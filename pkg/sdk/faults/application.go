package faults

import "github.com/opendatahub-io/odh-platform-chaos/pkg/sdk"

// ForceErrorConfig creates a fault that forces an error return.
func ForceErrorConfig(errMsg string, rate float64) sdk.FaultSpec {
	return sdk.FaultSpec{
		ErrorRate: rate,
		Error:     errMsg,
	}
}

// SkipConfig creates a fault that simulates skipping reconciliation.
func SkipConfig(rate float64) sdk.FaultSpec {
	return sdk.FaultSpec{
		ErrorRate: rate,
		Error:     "reconciliation skipped by chaos",
	}
}

// SimulatedPanicConfig creates a fault that returns an error with a "panic:" prefix.
// It does not actually call panic(); it simulates a panic-like failure via error return.
func SimulatedPanicConfig(msg string, rate float64) sdk.FaultSpec {
	return sdk.FaultSpec{
		ErrorRate: rate,
		Error:     "panic: " + msg,
	}
}
