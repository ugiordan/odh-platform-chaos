package evaluator

import (
	"testing"
	"time"

	v1alpha1 "github.com/opendatahub-io/odh-platform-chaos/api/v1alpha1"
	"github.com/stretchr/testify/assert"
)

func TestEvaluateResilient(t *testing.T) {
	e := New(10)

	result := e.Evaluate(
		&v1alpha1.CheckResult{Passed: true, ChecksRun: 3, ChecksPassed: 3},
		&v1alpha1.CheckResult{Passed: true, ChecksRun: 3, ChecksPassed: 3},
		true, // all reconciled
		2,    // reconcile cycles
		12*time.Second, // recovery time
		v1alpha1.HypothesisSpec{RecoveryTimeout: v1alpha1.Duration{Duration: 60 * time.Second}},
	)

	assert.Equal(t, v1alpha1.Resilient, result.Verdict)
	assert.Equal(t, 12*time.Second, result.RecoveryTime)
	assert.Equal(t, 2, result.ReconcileCycles)
	assert.NotEmpty(t, result.Confidence)
}

func TestEvaluateFailed(t *testing.T) {
	e := New(10)

	result := e.Evaluate(
		&v1alpha1.CheckResult{Passed: true, ChecksRun: 3, ChecksPassed: 3},
		&v1alpha1.CheckResult{Passed: false, ChecksRun: 3, ChecksPassed: 1},
		false, 0, 120*time.Second,
		v1alpha1.HypothesisSpec{RecoveryTimeout: v1alpha1.Duration{Duration: 60 * time.Second}},
	)

	assert.Equal(t, v1alpha1.Failed, result.Verdict)
}

func TestEvaluateDegraded_SlowRecovery(t *testing.T) {
	e := New(10)

	result := e.Evaluate(
		&v1alpha1.CheckResult{Passed: true, ChecksRun: 3, ChecksPassed: 3},
		&v1alpha1.CheckResult{Passed: true, ChecksRun: 3, ChecksPassed: 3},
		true, 3, 90*time.Second,
		v1alpha1.HypothesisSpec{RecoveryTimeout: v1alpha1.Duration{Duration: 60 * time.Second}},
	)

	assert.Equal(t, v1alpha1.Degraded, result.Verdict)
}

func TestEvaluateDegraded_ExcessiveCycles(t *testing.T) {
	e := New(5) // max 5 cycles

	result := e.Evaluate(
		&v1alpha1.CheckResult{Passed: true, ChecksRun: 3, ChecksPassed: 3},
		&v1alpha1.CheckResult{Passed: true, ChecksRun: 3, ChecksPassed: 3},
		true, 15, 30*time.Second, // 15 cycles > max 5
		v1alpha1.HypothesisSpec{RecoveryTimeout: v1alpha1.Duration{Duration: 60 * time.Second}},
	)

	assert.Equal(t, v1alpha1.Degraded, result.Verdict)
	assert.NotEmpty(t, result.Deviations)
}

func TestEvaluateInconclusive(t *testing.T) {
	e := New(10)

	result := e.Evaluate(
		&v1alpha1.CheckResult{Passed: false, ChecksRun: 3, ChecksPassed: 1}, // pre-check failed
		&v1alpha1.CheckResult{Passed: false, ChecksRun: 3, ChecksPassed: 1},
		false, 0, 0,
		v1alpha1.HypothesisSpec{RecoveryTimeout: v1alpha1.Duration{Duration: 60 * time.Second}},
	)

	assert.Equal(t, v1alpha1.Inconclusive, result.Verdict)
}
