package sdk

import (
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOperationConstants(t *testing.T) {
	ops := []Operation{
		OpGet,
		OpList,
		OpCreate,
		OpUpdate,
		OpDelete,
		OpPatch,
		OpDeleteAllOf,
		OpReconcile,
		OpApply,
	}
	for _, op := range ops {
		assert.NotEmpty(t, string(op), "Operation constant must be non-empty")
	}
}

func TestFaultConfigDefaults(t *testing.T) {
	cfg := &FaultConfig{}
	assert.False(t, cfg.IsActive())
	assert.Nil(t, cfg.MaybeInject(OpGet))
}

func TestFaultConfigNil(t *testing.T) {
	var cfg *FaultConfig
	assert.False(t, cfg.IsActive())
	assert.Nil(t, cfg.MaybeInject(OpGet))
}

func TestFaultConfigActive(t *testing.T) {
	cfg := &FaultConfig{
		Active: true,
		Faults: map[Operation]FaultSpec{
			OpGet: {ErrorRate: 1.0, Error: "simulated error"},
		},
	}
	assert.True(t, cfg.IsActive())
	err := cfg.MaybeInject(OpGet)
	assert.Error(t, err)
	assert.Equal(t, "simulated error", err.Error())
}

func TestFaultConfigInactiveNoInjection(t *testing.T) {
	cfg := &FaultConfig{
		Active: false,
		Faults: map[Operation]FaultSpec{
			OpGet: {ErrorRate: 1.0, Error: "simulated error"},
		},
	}
	assert.Nil(t, cfg.MaybeInject(OpGet))
}

func TestFaultConfigNoMatchingOperation(t *testing.T) {
	cfg := &FaultConfig{
		Active: true,
		Faults: map[Operation]FaultSpec{
			OpGet: {ErrorRate: 1.0, Error: "simulated error"},
		},
	}
	assert.Nil(t, cfg.MaybeInject(OpCreate))
}

func TestFaultConfigPartialErrorRate(t *testing.T) {
	cfg := &FaultConfig{
		Active: true,
		Faults: map[Operation]FaultSpec{
			OpGet: {ErrorRate: 0.0, Error: "should not fire"},
		},
	}
	// 0% error rate should never inject
	for i := 0; i < 100; i++ {
		assert.Nil(t, cfg.MaybeInject(OpGet))
	}
}

func TestFaultConfigConcurrentAccess(t *testing.T) {
	cfg := &FaultConfig{
		Active: true,
		Faults: map[Operation]FaultSpec{
			OpGet: {ErrorRate: 0.5, Error: "concurrent error"},
		},
	}

	var wg sync.WaitGroup

	// Spawn 50 goroutines that each call MaybeInject(OpGet) 100 times
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = cfg.MaybeInject(OpGet)
			}
		}()
	}

	// Spawn 10 goroutines that toggle Active 100 times each
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				cfg.mu.Lock()
				cfg.Active = !cfg.Active
				cfg.mu.Unlock()
			}
		}()
	}

	wg.Wait()
}

func TestMaybeInjectReturnsChaosError(t *testing.T) {
	cfg := &FaultConfig{
		Active: true,
		Faults: map[Operation]FaultSpec{
			OpGet: {ErrorRate: 1.0, Error: "injected failure"},
		},
	}
	err := cfg.MaybeInject(OpGet)
	require.Error(t, err)

	var chaosErr *ChaosError
	require.True(t, errors.As(err, &chaosErr))
	assert.Equal(t, OpGet, chaosErr.Operation)
	assert.Equal(t, "injected failure", chaosErr.Message)
}
