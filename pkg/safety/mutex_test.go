package safety

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExperimentLock(t *testing.T) {
	lock := NewLocalExperimentLock()

	// First lock should succeed
	err := lock.Acquire(context.Background(), "opendatahub-operator", "test-exp-1")
	require.NoError(t, err)

	// Second lock on same operator should fail
	err = lock.Acquire(context.Background(), "opendatahub-operator", "test-exp-2")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "test-exp-1")

	// Release and re-acquire should work
	lock.Release("opendatahub-operator")

	err = lock.Acquire(context.Background(), "opendatahub-operator", "test-exp-2")
	assert.NoError(t, err)

	lock.Release("opendatahub-operator")
}

func TestExperimentLockDifferentOperators(t *testing.T) {
	lock := NewLocalExperimentLock()

	err := lock.Acquire(context.Background(), "operator-a", "exp-1")
	require.NoError(t, err)

	// Different operator should work
	err = lock.Acquire(context.Background(), "operator-b", "exp-2")
	assert.NoError(t, err)

	lock.Release("operator-a")
	lock.Release("operator-b")
}
