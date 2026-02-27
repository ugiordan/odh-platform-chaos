package observer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewReconciliationChecker(t *testing.T) {
	rc := NewReconciliationChecker(nil)
	assert.NotNil(t, rc)
}

func TestReconciliationResultDefaults(t *testing.T) {
	result := &ReconciliationResult{}
	assert.False(t, result.AllReconciled)
	assert.Equal(t, 0, result.ReconcileCycles)
}

func TestResourceCheckResult(t *testing.T) {
	r := ResourceCheckResult{
		Kind:       "Deployment",
		Name:       "test-deploy",
		Namespace:  "test-ns",
		Reconciled: true,
		Details:    "exists",
	}
	assert.True(t, r.Reconciled)
	assert.Equal(t, "Deployment", r.Kind)
}
