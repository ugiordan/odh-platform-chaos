package sdk

import (
	"math/rand"
	"sync"
	"time"
)

// Operation represents a client operation that can be fault-injected.
type Operation string

const (
	OpGet         Operation = "get"
	OpList        Operation = "list"
	OpCreate      Operation = "create"
	OpUpdate      Operation = "update"
	OpDelete      Operation = "delete"
	OpPatch       Operation = "patch"
	OpDeleteAllOf Operation = "deleteAllOf"
	OpReconcile   Operation = "reconcile"
	OpApply       Operation = "apply"
)

// FaultSpec defines a single fault injection point.
type FaultSpec struct {
	ErrorRate float64       `json:"errorRate" yaml:"errorRate"`
	Error     string        `json:"error" yaml:"error"`
	Delay     time.Duration `json:"delay,omitempty" yaml:"delay,omitempty"`
}

// FaultConfig holds the activation state and fault specifications.
// Thread-safe via sync.RWMutex.
type FaultConfig struct {
	mu     sync.RWMutex
	Active bool                 `json:"active" yaml:"active"`
	Faults map[Operation]FaultSpec `json:"faults,omitempty" yaml:"faults,omitempty"`
}

// IsActive returns whether fault injection is currently enabled.
func (f *FaultConfig) IsActive() bool {
	if f == nil {
		return false
	}
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.Active
}

// ChaosError is returned by MaybeInject when a fault is triggered.
// Use errors.As to distinguish chaos-injected errors from real errors.
type ChaosError struct {
	Operation Operation
	Message   string
}

func (e *ChaosError) Error() string {
	return e.Message
}

// MaybeInject checks if a fault should be injected for the given operation.
// Returns nil if no fault applies (inactive, no matching operation, or rate miss).
func (f *FaultConfig) MaybeInject(operation Operation) error {
	if f == nil {
		return nil
	}
	f.mu.RLock()
	active := f.Active
	spec, ok := f.Faults[operation]
	f.mu.RUnlock()

	if !active || !ok {
		return nil
	}

	if spec.Delay > 0 {
		time.Sleep(spec.Delay)
	}
	if spec.ErrorRate > 0 && rand.Float64() < spec.ErrorRate {
		return &ChaosError{Operation: operation, Message: spec.Error}
	}
	return nil
}
