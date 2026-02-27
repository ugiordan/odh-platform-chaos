package faults

import (
	"time"

	"github.com/opendatahub-io/odh-platform-chaos/pkg/sdk"
)

// DelayConfig creates a fault that adds a fixed delay.
func DelayConfig(delay time.Duration) sdk.FaultSpec {
	return sdk.FaultSpec{
		Delay: delay,
	}
}

// RandomDelayConfig creates a fault that sets a fixed delay at config creation time.
// Despite the name suggesting randomness, the delay value is determined once when
// the config is created. Use DelayConfig if you want explicit fixed-delay semantics.
func RandomDelayConfig(maxDelay time.Duration) sdk.FaultSpec {
	return sdk.FaultSpec{
		Delay: maxDelay,
	}
}

// DeadlineExceedConfig creates a fault that simulates context deadline exceeded.
func DeadlineExceedConfig(rate float64) sdk.FaultSpec {
	return sdk.FaultSpec{
		ErrorRate: rate,
		Error:     "context deadline exceeded",
	}
}
