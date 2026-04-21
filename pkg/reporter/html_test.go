package reporter

import (
	"bytes"
	"testing"
	"time"

	v1alpha1 "github.com/opendatahub-io/operator-chaos/api/v1alpha1"
	"github.com/opendatahub-io/operator-chaos/pkg/evaluator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTMLReporterWriteReport(t *testing.T) {
	reports := []ExperimentReport{
		{
			Experiment: "dashboard-pod-kill",
			Timestamp:  time.Date(2026, 4, 21, 13, 0, 0, 0, time.UTC),
			Target:     TargetReport{Operator: "dashboard", Component: "rhods-dashboard"},
			Injection:  InjectionReport{Type: "PodKill", Details: map[string]string{"labelSelector": "app=rhods-dashboard"}},
			Evaluation: evaluator.EvaluationResult{
				Verdict:         v1alpha1.Resilient,
				RecoveryTime:    1200 * time.Millisecond,
				ReconcileCycles: 1,
				Confidence:      "1/1 steady-state checks passed",
			},
		},
		{
			Experiment: "dashboard-network-partition",
			Target:     TargetReport{Operator: "dashboard", Component: "rhods-dashboard"},
			Injection:  InjectionReport{Type: "NetworkPartition"},
			Evaluation: evaluator.EvaluationResult{
				Verdict:    v1alpha1.Failed,
				Deviations: []evaluator.Deviation{{Type: "steady-state", Detail: "post-check failed"}},
			},
			CleanupError: "leftover NetworkPolicy",
		},
	}

	var buf bytes.Buffer
	r := NewHTMLReporter("test-version")
	err := r.WriteReport(&buf, reports)
	require.NoError(t, err)

	out := buf.String()

	assert.Contains(t, out, "<!DOCTYPE html>")
	assert.Contains(t, out, "<title>Chaos Experiment Report</title>")
	assert.Contains(t, out, "</html>")
	assert.Contains(t, out, "<style>")
	assert.Contains(t, out, "Pass Rate: 50.0%")
	assert.Contains(t, out, "Experiments: 2")
	assert.Contains(t, out, "dashboard-pod-kill")
	assert.Contains(t, out, "badge-resilient")
	assert.Contains(t, out, "dashboard-network-partition")
	assert.Contains(t, out, "badge-failed")
	assert.Contains(t, out, "leftover NetworkPolicy")
	assert.Contains(t, out, "post-check failed")
	assert.Contains(t, out, "test-version")
}

func TestHTMLReporterEmpty(t *testing.T) {
	var buf bytes.Buffer
	r := NewHTMLReporter("dev")
	err := r.WriteReport(&buf, nil)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Experiments: 0")
}
